package vm

import (
	"context"
	"fmt"
	"sort"
	"strings"
	"time"
)

// vmNetworkProfiles mirrors pkg/emulator.networkProfiles. Per the
// Decoupled Reusable Architecture rule, the values are duplicated rather
// than imported across packages; both files MUST be kept in sync via
// reviewer discipline. The duplication is small (8 entries) and visible
// (a reviewer can grep both files in seconds).
var vmNetworkProfiles = map[string]NetworkConditions{
	"edge":     {DownKbps: 240, UpKbps: 200, LatencyMS: 840, LossPercent: 1.0},
	"2g":       {DownKbps: 50, UpKbps: 50, LatencyMS: 500, LossPercent: 2.0},
	"3g":       {DownKbps: 1500, UpKbps: 750, LatencyMS: 100, LossPercent: 0.5},
	"4g":       {DownKbps: 6000, UpKbps: 1500, LatencyMS: 50, LossPercent: 0.1},
	"lte":      {DownKbps: 12000, UpKbps: 3000, LatencyMS: 20, LossPercent: 0.05},
	"wifi":     {DownKbps: 50000, UpKbps: 10000, LatencyMS: 5, LossPercent: 0.01},
	"ethernet": {DownKbps: 100000, UpKbps: 100000, LatencyMS: 1, LossPercent: 0.0},
	"none":     {},
}

// LookupNetworkProfile returns the conditions for a named profile.
// "" or "none" means no shaping.
func LookupNetworkProfile(name string) (NetworkConditions, error) {
	if name == "" || name == "none" {
		return NetworkConditions{}, nil
	}
	if c, ok := vmNetworkProfiles[name]; ok {
		return c, nil
	}
	valid := make([]string, 0, len(vmNetworkProfiles))
	for k := range vmNetworkProfiles {
		valid = append(valid, k)
	}
	sort.Strings(valid)
	return NetworkConditions{}, fmt.Errorf("unknown network profile %q; valid: %v", name, valid)
}

// MergeNetworkConditions returns the merged conditions: any non-zero
// override field wins over the profile's value. Per-field merge so an
// operator can use a profile baseline and harshen a single dimension.
func MergeNetworkConditions(profile, override NetworkConditions) NetworkConditions {
	out := profile
	if override.DownKbps != 0 {
		out.DownKbps = override.DownKbps
	}
	if override.UpKbps != 0 {
		out.UpKbps = override.UpKbps
	}
	if override.LatencyMS != 0 {
		out.LatencyMS = override.LatencyMS
	}
	if override.LossPercent != 0 {
		out.LossPercent = override.LossPercent
	}
	return out
}

// applyNetworkConditionsVM applies network shaping inside the running
// VM via in-guest `tc qdisc add ... netem`. The interface is
// auto-detected via `ip route | awk '/^default/{print $5}'`.
//
// Why in-guest, not on the host TAP: QEMU's hostfwd path doesn't
// straightforwardly expose a single TAP interface we can shape from the
// host (rootless QEMU uses SLIRP, which is in-process). In-guest tc is
// universally available on Linux distros and matches the matrix's
// "real distro behaviour" mandate (we shape what the actual TCP stack
// inside the guest sees, which is what an end-user-visible flow would
// experience).
//
// The function is a best-effort enrichment: per Sixth Law clause 3, any
// failure here is logged by the caller (matrix runner) but does NOT
// flip the row's Passed signal. The row's gating signal stays bound to
// the script outcome itself.
//
// Returns nil on a zero-value conditions struct (no shaping requested).
func applyNetworkConditionsVM(
	ctx context.Context,
	ssh sshClient,
	conditions NetworkConditions,
) error {
	if conditions == (NetworkConditions{}) {
		return nil
	}
	// Compose a one-liner that resolves the iface, then applies tc.
	// The resulting tc command:
	//   tc qdisc replace dev <iface> root netem [delay <ms>ms] [rate <kbps>kbit] [loss <%>%]
	parts := []string{}
	if conditions.LatencyMS > 0 {
		parts = append(parts, fmt.Sprintf("delay %dms", conditions.LatencyMS))
	}
	if conditions.DownKbps > 0 {
		// netem accepts a single rate parameter (it shapes both
		// directions on the egress qdisc). Use DownKbps as the cap;
		// asymmetric shaping requires htb-on-ifb which is out of scope
		// for v0.1 and would need root inside the guest.
		parts = append(parts, fmt.Sprintf("rate %dkbit", conditions.DownKbps))
	}
	if conditions.LossPercent > 0 {
		parts = append(parts, fmt.Sprintf("loss %.2f%%", conditions.LossPercent))
	}
	if len(parts) == 0 {
		return nil
	}
	tcArgs := strings.Join(parts, " ")
	// `replace` is idempotent — no-op if no qdisc, replaces if present.
	script := fmt.Sprintf(
		"set -e; iface=$(ip route | awk '/^default/{print $5; exit}'); test -n \"$iface\" || { echo no-default-iface >&2; exit 1; }; tc qdisc replace dev \"$iface\" root netem %s",
		tcArgs,
	)
	stdout, stderr, exitCode, err := ssh.Run(ctx, script, nil, 30*time.Second)
	if err != nil {
		return fmt.Errorf("apply network conditions (in-guest tc): %w; stdout=%q stderr=%q", err, stdout, stderr)
	}
	if exitCode != 0 {
		return fmt.Errorf("in-guest tc exit=%d; stderr=%q", exitCode, stderr)
	}
	return nil
}
