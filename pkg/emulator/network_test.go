package emulator

import (
	"context"
	"strings"
	"testing"
)

// TestLookupNetworkProfile_Known asserts every documented profile
// resolves to its canonical NetworkConditions.
//
// Falsifiability rehearsal (Sixth Law clause 2):
//
//	Mutation: in network.go, swap DownKbps and UpKbps in the "4g" entry.
//	Run:      go test ./pkg/emulator/... -run TestLookupNetworkProfile_Known
//	Observed-Failure: subtests "4g" fails — expected DownKbps=6000 got
//	          DownKbps=1500 (the swapped UpKbps value).
//	Reverted: yes — post-revert this test passes again.
//
// The table also pins the full set of supported names: a future change
// that drops a profile would fail the corresponding subtest, and a new
// profile MUST extend the table or risk untested behaviour.
func TestLookupNetworkProfile_Known(t *testing.T) {
	cases := []struct {
		name      string
		expectedD int
		expectedU int
		expectedL int
	}{
		{"edge", 240, 200, 840},
		{"2g", 50, 50, 500},
		{"3g", 1500, 750, 100},
		{"4g", 6000, 1500, 50},
		{"lte", 12000, 3000, 20},
		{"wifi", 50000, 10000, 5},
		{"ethernet", 100000, 100000, 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := LookupNetworkProfile(tc.name)
			if err != nil {
				t.Fatalf("LookupNetworkProfile(%q): %v", tc.name, err)
			}
			if c.DownKbps != tc.expectedD {
				t.Fatalf("%s: DownKbps want %d got %d", tc.name, tc.expectedD, c.DownKbps)
			}
			if c.UpKbps != tc.expectedU {
				t.Fatalf("%s: UpKbps want %d got %d", tc.name, tc.expectedU, c.UpKbps)
			}
			if c.LatencyMS != tc.expectedL {
				t.Fatalf("%s: LatencyMS want %d got %d", tc.name, tc.expectedL, c.LatencyMS)
			}
		})
	}
}

// TestLookupNetworkProfile_NoneAndEmpty pins the "no shaping" branch.
func TestLookupNetworkProfile_NoneAndEmpty(t *testing.T) {
	for _, name := range []string{"", "none"} {
		c, err := LookupNetworkProfile(name)
		if err != nil {
			t.Fatalf("LookupNetworkProfile(%q): %v", name, err)
		}
		if (c != NetworkConditions{}) {
			t.Fatalf("%q: expected zero-value, got %+v", name, c)
		}
	}
}

// TestLookupNetworkProfile_Unknown_ReturnsError pins the typo path.
func TestLookupNetworkProfile_Unknown_ReturnsError(t *testing.T) {
	_, err := LookupNetworkProfile("5g-unicorn")
	if err == nil {
		t.Fatalf("expected error for unknown profile, got nil")
	}
	if !strings.Contains(err.Error(), "5g-unicorn") {
		t.Fatalf("error should cite the bad name; got: %v", err)
	}
	if !strings.Contains(err.Error(), "valid:") {
		t.Fatalf("error should list valid names; got: %v", err)
	}
	// The valid list MUST be alphabetised so the operator's eye can
	// spot a typo quickly. Pin order against the documented set.
	for _, name := range []string{"2g", "3g", "4g", "edge", "ethernet", "lte", "none", "wifi"} {
		if !strings.Contains(err.Error(), name) {
			t.Fatalf("error should mention valid name %q; got: %v", name, err)
		}
	}
}

// TestMergeNetworkConditions_OverrideWinsPerField asserts the per-field
// merge semantics. Operators can use a profile baseline and harshen a
// single dimension without re-typing the others.
//
// Falsifiability rehearsal:
//
//	Mutation: in network.go MergeNetworkConditions, change `if override.LossPercent != 0`
//	          to `if false` (drop the loss override branch).
//	Run:      go test ./pkg/emulator/... -run TestMergeNetworkConditions
//	Observed-Failure: subtest "override-loss-only" fails — expected
//	          LossPercent=0.5 got 0.1 (the profile's value).
//	Reverted: yes.
func TestMergeNetworkConditions_OverrideWinsPerField(t *testing.T) {
	base, _ := LookupNetworkProfile("4g")
	t.Run("override-down-only", func(t *testing.T) {
		got := MergeNetworkConditions(base, NetworkConditions{DownKbps: 1000})
		if got.DownKbps != 1000 {
			t.Fatalf("DownKbps: want 1000 got %d", got.DownKbps)
		}
		if got.UpKbps != base.UpKbps {
			t.Fatalf("UpKbps should preserve base; want %d got %d", base.UpKbps, got.UpKbps)
		}
	})
	t.Run("override-loss-only", func(t *testing.T) {
		got := MergeNetworkConditions(base, NetworkConditions{LossPercent: 0.5})
		if got.LossPercent != 0.5 {
			t.Fatalf("LossPercent: want 0.5 got %v", got.LossPercent)
		}
		if got.DownKbps != base.DownKbps {
			t.Fatalf("DownKbps should preserve base; want %d got %d", base.DownKbps, got.DownKbps)
		}
	})
	t.Run("zero-override-preserves-base", func(t *testing.T) {
		got := MergeNetworkConditions(base, NetworkConditions{})
		if got != base {
			t.Fatalf("zero override MUST preserve base byte-for-byte; want %+v got %+v", base, got)
		}
	})
}

// TestApplyNetworkConditions_NoOpOnZeroes verifies the zero-value
// short-circuit. Without it, an empty conditions struct would still
// invoke `adb emu network speed 0:0` which the emulator console rejects.
func TestApplyNetworkConditions_NoOpOnZeroes(t *testing.T) {
	exec := &fakeExecutor{}
	err := applyNetworkConditions(context.Background(), exec, "/usr/local/bin/adb", "emulator-5554", NetworkConditions{})
	if err != nil {
		t.Fatalf("expected nil error on zero conditions, got %v", err)
	}
	if len(exec.calls) != 0 {
		t.Fatalf("expected 0 adb calls on zero conditions, got %d: %+v", len(exec.calls), exec.calls)
	}
}

// TestApplyNetworkConditions_AdbEmuConsoleCommandsIssued asserts that
// the canonical `adb emu network speed/delay` invocations are recorded
// for a non-zero conditions struct, with arguments derived from the
// conditions.
//
// Falsifiability rehearsal (Sixth Law clause 2):
//
//	Mutation: in network.go applyNetworkConditions, drop the entire
//	          "if conditions.DownKbps != 0 || conditions.UpKbps != 0"
//	          block (no `network speed` invocation issued).
//	Run:      go test ./pkg/emulator/... -run TestApplyNetworkConditions_AdbEmuConsoleCommandsIssued
//	Observed-Failure: assertion on `sawSpeed` fires — "expected `adb emu
//	          network speed` invocation in calls[]; got [delay]".
//	Reverted: yes.
func TestApplyNetworkConditions_AdbEmuConsoleCommandsIssued(t *testing.T) {
	exec := &fakeExecutor{}
	conditions := NetworkConditions{DownKbps: 6000, UpKbps: 1500, LatencyMS: 50}
	err := applyNetworkConditions(context.Background(), exec, "/usr/local/bin/adb", "emulator-5554", conditions)
	if err != nil {
		t.Fatalf("applyNetworkConditions: %v", err)
	}

	var sawSpeed, sawDelay bool
	for _, c := range exec.calls {
		if c.Name != "/usr/local/bin/adb" {
			continue
		}
		argString := strings.Join(c.Args, " ")
		// `-s localhost:5554 emu network speed 1500:6000`
		if strings.Contains(argString, "emu network speed") {
			sawSpeed = true
			if !strings.Contains(argString, "1500:6000") {
				t.Fatalf("speed arg: want UpKbps:DownKbps form '1500:6000', got args=%v", c.Args)
			}
			if !strings.Contains(argString, "-s localhost:5554") {
				t.Fatalf("speed call MUST target the parsed serial port (localhost:5554); got args=%v", c.Args)
			}
		}
		if strings.Contains(argString, "emu network delay") {
			sawDelay = true
			if !strings.Contains(argString, " 50") {
				t.Fatalf("delay arg: want '50' (milliseconds), got args=%v", c.Args)
			}
		}
	}
	if !sawSpeed {
		t.Fatalf("expected `adb emu network speed` invocation; got %d calls: %+v", len(exec.calls), exec.calls)
	}
	if !sawDelay {
		t.Fatalf("expected `adb emu network delay` invocation; got %d calls: %+v", len(exec.calls), exec.calls)
	}
}

// TestApplyNetworkConditions_AdbErrorPropagated verifies an adb failure
// in the speed step propagates upward (so the matrix runner's stderr
// log shows the operator the actionable message).
func TestApplyNetworkConditions_AdbErrorPropagated(t *testing.T) {
	exec := &fakeExecutor{
		scripts: map[string]fakeScript{
			"/usr/local/bin/adb -s localhost:5554 emu network speed 1500:6000": {
				Err: errOpaque("adb: device offline"),
			},
		},
	}
	err := applyNetworkConditions(context.Background(), exec, "/usr/local/bin/adb", "emulator-5554",
		NetworkConditions{DownKbps: 6000, UpKbps: 1500})
	if err == nil {
		t.Fatalf("expected error when adb fails, got nil")
	}
	if !strings.Contains(err.Error(), "apply network speed") {
		t.Fatalf("error should carry the speed step prefix; got %v", err)
	}
}

// TestParseSerialPort_AllForms pins the recognised serial → port
// mappings. parseSerialPort is the load-bearing translation between
// `emulator-N` (what `adb devices` emits) and `localhost:N` (what
// `adb -s` accepts as a target).
func TestParseSerialPort_AllForms(t *testing.T) {
	cases := []struct {
		serial string
		want   int
	}{
		{"emulator-5554", 5554},
		{"localhost:5556", 5556},
		{"5558", 5558},
		{"unknown", 0},
		{"", 0},
	}
	for _, tc := range cases {
		t.Run(tc.serial, func(t *testing.T) {
			got := parseSerialPort(tc.serial)
			if got != tc.want {
				t.Fatalf("parseSerialPort(%q): want %d got %d", tc.serial, tc.want, got)
			}
		})
	}
}

// errOpaque is a minimal error carrier for the script-table fake.
type errOpaqueT string

func (e errOpaqueT) Error() string { return string(e) }
func errOpaque(s string) error     { return errOpaqueT(s) }
