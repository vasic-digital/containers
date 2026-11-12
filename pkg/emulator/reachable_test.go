package emulator

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
)

// G11 (CAT-DEVEMU) PROVISIONAL: the API URL each emulated target can actually
// reach. Unit tests with a fake executor prove the mapping, validation and
// evidence shape; they do NOT prove the in-device probe command works on a
// real Android image (UNCONFIRMED until a live run).

func TestReachable_AndroidEmulatorUsesAlias10_0_2_2(t *testing.T) {
	r, err := ReachableAPI(context.Background(), Target{Kind: TargetAndroidEmulator}, 8080, nil)
	if err != nil {
		t.Fatal(err)
	}
	if r.URL != "http://10.0.2.2:8080" {
		t.Errorf("url = %q", r.URL)
	}
	if r.Verified {
		t.Error("no probe given => Verified must be false (never claimed)")
	}
	if r.Kind != TargetAndroidEmulator || r.Host != "10.0.2.2" || r.Port != 8080 {
		t.Errorf("%+v", r)
	}
}

func TestReachable_ContainerGatewayPerRuntime(t *testing.T) {
	cases := map[string]string{
		"podman":  "http://host.containers.internal:9000",
		"docker":  "http://host.docker.internal:9000",
		"nerdctl": "http://host.docker.internal:9000",
	}
	for rt, want := range cases {
		r, err := ReachableAPI(context.Background(), Target{Kind: TargetContainer, Runtime: rt}, 9000, nil)
		if err != nil {
			t.Fatalf("%s: %v", rt, err)
		}
		if r.URL != want {
			t.Errorf("%s: url = %q want %q", rt, r.URL, want)
		}
	}
	if _, err := ReachableAPI(context.Background(), Target{Kind: TargetContainer}, 9000, nil); err == nil {
		t.Error("container target without a runtime must be refused (gateway name differs per runtime)")
	}
	if _, err := ReachableAPI(context.Background(), Target{Kind: TargetContainer, Runtime: "lxd"}, 9000, nil); err == nil {
		t.Error("unknown runtime must be refused, not defaulted")
	}
}

func TestReachable_BridgeNeedsRoutableIP(t *testing.T) {
	r, err := ReachableAPI(context.Background(), Target{Kind: TargetBridge, BridgeIP: "10.89.0.1"}, 443, nil)
	if err != nil || r.URL != "http://10.89.0.1:443" {
		t.Fatalf("%v %+v", err, r)
	}
	for _, bad := range []string{"", "localhost", "127.0.0.1", "::1", "0.0.0.0", "10.0.0.1:80", "10.0.0.1/24", "not-an-ip"} {
		if _, err := ReachableAPI(context.Background(), Target{Kind: TargetBridge, BridgeIP: bad}, 443, nil); err == nil {
			t.Errorf("bridge ip %q must be refused (loopback inside a container is the container itself)", bad)
		}
	}
	// IPv6 literal is bracketed
	r, err = ReachableAPI(context.Background(), Target{Kind: TargetBridge, BridgeIP: "fd00::1"}, 80, nil)
	if err != nil || r.URL != "http://[fd00::1]:80" {
		t.Fatalf("%v %+v", err, r)
	}
}

func TestReachable_HostTarget(t *testing.T) {
	r, err := ReachableAPI(context.Background(), Target{Kind: TargetHost}, 80, nil)
	if err != nil || r.URL != "http://127.0.0.1:80" {
		t.Fatalf("%v %+v", err, r)
	}
}

func TestReachable_PortAndKindValidation(t *testing.T) {
	for _, p := range []int{0, -1, 65536, 100000} {
		if _, err := ReachableAPI(context.Background(), Target{Kind: TargetHost}, p, nil); err == nil {
			t.Errorf("port %d must be refused", p)
		}
	}
	if _, err := ReachableAPI(context.Background(), Target{Kind: "teapot"}, 80, nil); err == nil {
		t.Error("unknown kind must be refused")
	}
}

func TestReachable_ProbeSuccessIsRecordedAsEvidence(t *testing.T) {
	var gotURL string
	probe := func(_ context.Context, tg Target, host string, port int) (ProbeResult, error) {
		gotURL = host
		return ProbeResult{Command: "nc -z -w 3 " + host, Output: "ok", ExitOK: true}, nil
	}
	r, err := ReachableAPI(context.Background(), Target{Kind: TargetAndroidEmulator, Serial: "emulator-5554"}, 8080, probe)
	if err != nil {
		t.Fatal(err)
	}
	if !r.Verified || gotURL != "10.0.2.2" {
		t.Errorf("%+v probedHost=%q", r, gotURL)
	}
	b, _ := json.Marshal(r)
	for _, want := range []string{`"url":"http://10.0.2.2:8080"`, `"verified":true`, `"probe_command"`} {
		if !strings.Contains(string(b), want) {
			t.Errorf("evidence JSON lacks %s: %s", want, b)
		}
	}
}

func TestReachable_ProbeFailureIsAnErrorNotAnUnverifiedPass(t *testing.T) {
	probe := func(context.Context, Target, string, int) (ProbeResult, error) {
		return ProbeResult{Command: "nc", Output: "refused", ExitOK: false}, nil
	}
	r, err := ReachableAPI(context.Background(), Target{Kind: TargetContainer, Runtime: "podman"}, 80, probe)
	if !errors.Is(err, ErrNotReachable) {
		t.Fatalf("a failed probe must wrap ErrNotReachable, got %v", err)
	}
	if r.Verified {
		t.Error("Verified must be false on a failed probe")
	}
	if r.URL == "" {
		t.Error("the attempted URL must still be reported for diagnostics")
	}
	probeErr := func(context.Context, Target, string, int) (ProbeResult, error) {
		return ProbeResult{}, errors.New("adb: device offline")
	}
	_, err = ReachableAPI(context.Background(), Target{Kind: TargetContainer, Runtime: "podman"}, 80, probeErr)
	if !errors.Is(err, ErrNotReachable) {
		t.Fatalf("probe execution error must also wrap ErrNotReachable: %v", err)
	}
}

func TestADBProbe_ArgvAndInjectionGuards(t *testing.T) {
	fake := &fakeExecutor{scripts: map[string]fakeScript{"adb": {Out: []byte("")}}}
	p := ADBProbe(fake, "adb")
	res, err := p(context.Background(), Target{Kind: TargetAndroidEmulator, Serial: "emulator-5554"}, "10.0.2.2", 8080)
	if err != nil || !res.ExitOK {
		t.Fatalf("%v %+v", err, res)
	}
	if len(fake.calls) != 1 {
		t.Fatalf("calls: %+v", fake.calls)
	}
	a := fake.calls[0].Args
	if a[0] != "-s" || a[1] != "emulator-5554" || a[2] != "shell" {
		t.Errorf("argv = %v", a)
	}
	if !strings.Contains(strings.Join(a, " "), "10.0.2.2") || !strings.Contains(strings.Join(a, " "), "8080") {
		t.Errorf("probe must target host and port: %v", a)
	}
	for _, bad := range []string{"", "emu;rm -rf /", "-s", "a b", "emulator-5554\nx"} {
		fake.calls = nil
		if _, err := p(context.Background(), Target{Kind: TargetAndroidEmulator, Serial: bad}, "10.0.2.2", 8080); err == nil {
			t.Errorf("serial %q must be refused", bad)
		}
		if len(fake.calls) != 0 {
			t.Errorf("serial %q: executor must not be reached", bad)
		}
	}
	// non-zero exit => ExitOK false, no Go error
	failing := &fakeExecutor{scripts: map[string]fakeScript{"adb": {Out: []byte("failed"), Err: errors.New("exit status 1")}}}
	res, err = ADBProbe(failing, "adb")(context.Background(), Target{Kind: TargetAndroidEmulator, Serial: "emulator-5554"}, "10.0.2.2", 8080)
	if err != nil || res.ExitOK {
		t.Errorf("failing probe: %v %+v", err, res)
	}
}

func TestContainerProbe_ArgvAndInjectionGuards(t *testing.T) {
	fake := &fakeExecutor{scripts: map[string]fakeScript{"podman": {Out: []byte("")}}}
	p := ContainerProbe(fake)
	res, err := p(context.Background(), Target{Kind: TargetContainer, Runtime: "podman", ContainerName: "lava-emu-x-1"}, "host.containers.internal", 9000)
	if err != nil || !res.ExitOK {
		t.Fatalf("%v %+v", err, res)
	}
	a := fake.calls[0].Args
	if fake.calls[0].Name != "podman" || a[0] != "exec" || a[1] != "lava-emu-x-1" {
		t.Errorf("call = %+v", fake.calls[0])
	}
	for _, bad := range []string{"", "-rm", "a b", "x;y", "$(id)"} {
		fake.calls = nil
		if _, err := p(context.Background(), Target{Kind: TargetContainer, Runtime: "podman", ContainerName: bad}, "h", 9000); err == nil {
			t.Errorf("container name %q must be refused", bad)
		}
		if len(fake.calls) != 0 {
			t.Errorf("container name %q reached the executor", bad)
		}
	}
	if _, err := p(context.Background(), Target{Kind: TargetContainer, Runtime: "podman; id", ContainerName: "ok"}, "h", 1); err == nil {
		t.Error("runtime binary must be one of the known names")
	}
}
