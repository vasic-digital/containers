package emulator

import (
	"context"
	"fmt"
	"sort"
)

// networkProfiles is the canonical map of named network conditions.
// Values follow the conventional Android Studio "AVD Manager" presets,
// chosen so the matrix runner can exercise common mobile-network
// regimes without operators having to memorise numeric constants.
//
// Anti-bluff posture (clauses 6.J/6.L): the constants are recorded here
// alongside the lookup helper so a reviewer can see at a glance whether
// a given profile is "fast" or "slow" — there is no out-of-band table.
// A future profile addition MUST land here AND in the LookupNetworkProfile
// error-listing branch (the test file pins the full set).
var networkProfiles = map[string]NetworkConditions{
	"edge":     {DownKbps: 240, UpKbps: 200, LatencyMS: 840, LossPercent: 1.0},
	"2g":       {DownKbps: 50, UpKbps: 50, LatencyMS: 500, LossPercent: 2.0},
	"3g":       {DownKbps: 1500, UpKbps: 750, LatencyMS: 100, LossPercent: 0.5},
	"4g":       {DownKbps: 6000, UpKbps: 1500, LatencyMS: 50, LossPercent: 0.1},
	"lte":      {DownKbps: 12000, UpKbps: 3000, LatencyMS: 20, LossPercent: 0.05},
	"wifi":     {DownKbps: 50000, UpKbps: 10000, LatencyMS: 5, LossPercent: 0.01},
	"ethernet": {DownKbps: 100000, UpKbps: 100000, LatencyMS: 1, LossPercent: 0.0},
	"none":     {},
}

// LookupNetworkProfile returns the NetworkConditions for a named profile.
// Empty name OR "none" returns zero-value (no shaping). Unknown profiles
// return an error listing valid names so the operator can correct a typo
// without consulting source.
func LookupNetworkProfile(name string) (NetworkConditions, error) {
	if name == "" || name == "none" {
		return NetworkConditions{}, nil
	}
	if c, ok := networkProfiles[name]; ok {
		return c, nil
	}
	valid := make([]string, 0, len(networkProfiles))
	for k := range networkProfiles {
		valid = append(valid, k)
	}
	sort.Strings(valid)
	return NetworkConditions{}, fmt.Errorf("unknown network profile %q; valid: %v", name, valid)
}

// MergeNetworkConditions returns the merged conditions: any non-zero
// override field wins over the profile's value. The merge is per-field
// (not all-or-nothing) so the operator can use "4g" as a baseline AND
// override one knob (e.g. loss=0.5 to harshen).
//
// A zero-value override (every field zero) preserves the profile bytes
// for byte. This keeps the common path "operator passes a profile only,
// no overrides" entirely inert.
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

// applyNetworkConditions issues `adb -s <serial> emu network speed/delay`
// commands to apply the requested shaping. The Android emulator console
// accepts numeric values directly:
//
//	network speed <upKbps>:<downKbps>
//	network delay <latencyMS>
//
// Loss is NOT supported by the emulator console — we ignore it for
// Android (real-network host-side `tc qdisc` against the emulator's
// TAP interface is the way to inject loss; out of scope for v0.1).
//
// Returns nil and is a no-op when conditions is the zero value (no
// shaping).
//
// Anti-bluff posture: the function asserts on observable host-shell
// behaviour (the `adb emu network` invocations recorded by the
// fakeExecutor under test). A "succeeded silently" signal would be a
// bluff vector; we wrap every adb error with the speed/delay arg so
// the failure is attributable.
func applyNetworkConditions(
	ctx context.Context,
	exec CommandExecutor,
	adbBinary, serial string,
	conditions NetworkConditions,
) error {
	if conditions == (NetworkConditions{}) {
		return nil // no shaping
	}
	target := fmt.Sprintf("localhost:%d", parseSerialPort(serial))
	if conditions.DownKbps != 0 || conditions.UpKbps != 0 {
		// emulator console: `network speed <up>:<down>`
		speedArg := fmt.Sprintf("%d:%d", conditions.UpKbps, conditions.DownKbps)
		if _, err := exec.Execute(ctx, adbBinary, "-s", target, "emu", "network", "speed", speedArg); err != nil {
			return fmt.Errorf("apply network speed %s: %w", speedArg, err)
		}
	}
	if conditions.LatencyMS != 0 {
		latencyArg := fmt.Sprintf("%d", conditions.LatencyMS)
		if _, err := exec.Execute(ctx, adbBinary, "-s", target, "emu", "network", "delay", latencyArg); err != nil {
			return fmt.Errorf("apply network delay %s: %w", latencyArg, err)
		}
	}
	return nil
}

// parseSerialPort extracts the port number from the canonical emulator
// serial forms. Returns 0 on parse failure (caller treats as "no
// recognisable port" — the resulting empty target string makes adb fail
// noisily, which is desirable: silent default-port use is a clause-6.I
// bluff vector).
func parseSerialPort(serial string) int {
	var p int
	if _, err := fmt.Sscanf(serial, "emulator-%d", &p); err == nil {
		return p
	}
	if _, err := fmt.Sscanf(serial, "localhost:%d", &p); err == nil {
		return p
	}
	if _, err := fmt.Sscanf(serial, "%d", &p); err == nil {
		return p
	}
	return 0
}
