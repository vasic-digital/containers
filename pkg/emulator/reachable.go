package emulator

import (
	"context"
	"errors"
	"fmt"
	"net"
	"regexp"
	"strconv"
	"strings"
)

// Reachable-endpoint resolver (CAT-DEVEMU G11, PROVISIONAL).
//
// The same host service is reachable under a different address depending on
// WHERE the client runs: inside the Android emulator the host is the fixed
// alias 10.0.2.2; inside a rootless container the host's LAN IP resolves to
// the container itself (pasta), so the runtime's gateway name must be used;
// inside a user-defined bridge network the bridge gateway IP is used. Every
// run must record which URL it actually used, and — when a probe is given —
// that the URL was reached from inside the target.
//
// The resolver is project-agnostic: ports, bridge IPs and names are caller DATA.

// TargetKind is the closed set of client locations.
type TargetKind string

const (
	TargetAndroidEmulator TargetKind = "android-emulator"
	TargetContainer       TargetKind = "container"
	TargetBridge          TargetKind = "bridge"
	TargetHost            TargetKind = "host"
)

// Target describes where the client runs.
type Target struct {
	Kind TargetKind
	// Runtime is "podman", "docker" or "nerdctl" (TargetContainer only).
	Runtime string
	// BridgeIP is the gateway IP of the bridge network (TargetBridge only).
	BridgeIP string
	// Serial is the adb serial (TargetAndroidEmulator probes).
	Serial string
	// ContainerName is the container the probe execs into (TargetContainer probes).
	ContainerName string
}

// ProbeResult is what an in-target reachability probe observed.
type ProbeResult struct {
	Command string `json:"probe_command"`
	Output  string `json:"probe_output"`
	ExitOK  bool   `json:"probe_exit_ok"`
}

// ProbeFunc checks, from inside the target, that host:port accepts a
// connection. A non-nil error means the probe could not run; ExitOK=false
// means it ran and the connection was not accepted.
type ProbeFunc func(ctx context.Context, t Target, host string, port int) (ProbeResult, error)

// Reachable is the evidence of one resolution.
type Reachable struct {
	URL      string     `json:"url"`
	Kind     TargetKind `json:"kind"`
	Host     string     `json:"host"`
	Port     int        `json:"port"`
	Verified bool       `json:"verified"`
	ProbeResult
}

// ErrNotReachable wraps a failed or un-runnable probe. The attempted URL is
// still returned in Reachable for diagnostics, with Verified=false.
var ErrNotReachable = errors.New("endpoint not reachable from the target")

// ReachableAPI resolves the URL a client in target t uses to reach a service
// listening on the HOST at hostPort. With a nil probe the result is
// Verified=false (an honest "resolved, not checked"); with a probe, a failure
// is an error.
func ReachableAPI(ctx context.Context, t Target, hostPort int, probe ProbeFunc) (Reachable, error) {
	r := Reachable{Kind: t.Kind, Port: hostPort}
	if hostPort < 1 || hostPort > 65535 {
		return r, fmt.Errorf("reachable: port %d out of range", hostPort)
	}
	switch t.Kind {
	case TargetAndroidEmulator:
		r.Host = "10.0.2.2"
	case TargetContainer:
		switch t.Runtime {
		case "podman":
			r.Host = "host.containers.internal"
		case "docker", "nerdctl":
			r.Host = "host.docker.internal"
		default:
			return r, fmt.Errorf("reachable: container target needs runtime podman|docker|nerdctl, got %q", t.Runtime)
		}
	case TargetBridge:
		ip := net.ParseIP(t.BridgeIP)
		if ip == nil || ip.IsLoopback() || ip.IsUnspecified() {
			return r, fmt.Errorf("reachable: bridge ip %q must be a routable literal IP (loopback inside a container is the container itself)", t.BridgeIP)
		}
		r.Host = t.BridgeIP
	case TargetHost:
		r.Host = "127.0.0.1"
	default:
		return r, fmt.Errorf("reachable: unknown target kind %q", t.Kind)
	}
	hostInURL := r.Host
	if strings.Contains(hostInURL, ":") {
		hostInURL = "[" + hostInURL + "]"
	}
	r.URL = "http://" + hostInURL + ":" + strconv.Itoa(hostPort)

	if probe == nil {
		return r, nil
	}
	res, err := probe(ctx, t, r.Host, hostPort)
	r.ProbeResult = res
	if err != nil {
		return r, fmt.Errorf("%w: %s: probe could not run: %v", ErrNotReachable, r.URL, err)
	}
	if !res.ExitOK {
		return r, fmt.Errorf("%w: %s: probe says no connection (%s)", ErrNotReachable, r.URL, strings.TrimSpace(res.Output))
	}
	r.Verified = true
	return r, nil
}

var (
	serialRe    = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]*$`)
	ctrNameRe   = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.-]*$`)
	probeHostRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.:-]*$`)
)

// ADBProbe probes from inside an Android emulator via `adb -s <serial> shell
// nc -z -w 3 <host> <port>`. UNCONFIRMED on a live image: whether the image's
// toybox `nc` accepts -z must be proven by a live run (CAT-DEVEMU).
func ADBProbe(ex CommandExecutor, adbBinary string) ProbeFunc {
	if adbBinary == "" {
		adbBinary = "adb"
	}
	return func(ctx context.Context, t Target, host string, port int) (ProbeResult, error) {
		if !serialRe.MatchString(t.Serial) {
			return ProbeResult{}, fmt.Errorf("adb probe: unsafe serial %q", t.Serial)
		}
		if !probeHostRe.MatchString(host) || port < 1 || port > 65535 {
			return ProbeResult{}, fmt.Errorf("adb probe: unsafe target %q:%d", host, port)
		}
		args := []string{"-s", t.Serial, "shell", "nc", "-z", "-w", "3", host, strconv.Itoa(port)}
		out, err := ex.Execute(ctx, adbBinary, args...)
		return ProbeResult{
			Command: adbBinary + " " + strings.Join(args, " "),
			Output:  string(out),
			ExitOK:  err == nil,
		}, nil
	}
}

// ContainerProbe probes from inside a container via `<runtime> exec <name> nc
// -z -w 3 <host> <port>`. UNCONFIRMED: needs `nc` in the image.
func ContainerProbe(ex CommandExecutor) ProbeFunc {
	return func(ctx context.Context, t Target, host string, port int) (ProbeResult, error) {
		switch t.Runtime {
		case "podman", "docker", "nerdctl":
		default:
			return ProbeResult{}, fmt.Errorf("container probe: unknown runtime %q", t.Runtime)
		}
		if !ctrNameRe.MatchString(t.ContainerName) {
			return ProbeResult{}, fmt.Errorf("container probe: unsafe container name %q", t.ContainerName)
		}
		if !probeHostRe.MatchString(host) || port < 1 || port > 65535 {
			return ProbeResult{}, fmt.Errorf("container probe: unsafe target %q:%d", host, port)
		}
		args := []string{"exec", t.ContainerName, "nc", "-z", "-w", "3", host, strconv.Itoa(port)}
		out, err := ex.Execute(ctx, t.Runtime, args...)
		return ProbeResult{
			Command: t.Runtime + " " + strings.Join(args, " "),
			Output:  string(out),
			ExitOK:  err == nil,
		}, nil
	}
}
