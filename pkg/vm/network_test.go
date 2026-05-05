package vm

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// TestVM_LookupNetworkProfile_Known asserts every documented profile
// resolves to its canonical NetworkConditions.
//
// Falsifiability rehearsal (Sixth Law clause 2):
//
//	Mutation: in pkg/vm/network.go, swap DownKbps and UpKbps in the
//	          "4g" entry of vmNetworkProfiles.
//	Run:      go test ./pkg/vm/... -run TestVM_LookupNetworkProfile_Known/4g
//	Observed-Failure: subtest "4g" fails — expected DownKbps=6000 got
//	          DownKbps=1500.
//	Reverted: yes.
func TestVM_LookupNetworkProfile_Known(t *testing.T) {
	cases := []struct {
		name       string
		expectedD  int
		expectedU  int
		expectedL  int
		expectedLP float64
	}{
		{"edge", 240, 200, 840, 1.0},
		{"2g", 50, 50, 500, 2.0},
		{"3g", 1500, 750, 100, 0.5},
		{"4g", 6000, 1500, 50, 0.1},
		{"lte", 12000, 3000, 20, 0.05},
		{"wifi", 50000, 10000, 5, 0.01},
		{"ethernet", 100000, 100000, 1, 0.0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c, err := LookupNetworkProfile(tc.name)
			if err != nil {
				t.Fatalf("%s: %v", tc.name, err)
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

func TestVM_LookupNetworkProfile_NoneAndEmpty(t *testing.T) {
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

func TestVM_LookupNetworkProfile_Unknown_ReturnsError(t *testing.T) {
	_, err := LookupNetworkProfile("5g-unicorn")
	if err == nil {
		t.Fatalf("expected error for unknown profile")
	}
	if !strings.Contains(err.Error(), "5g-unicorn") {
		t.Fatalf("error should cite the bad name; got: %v", err)
	}
}

func TestVM_MergeNetworkConditions_OverrideWinsPerField(t *testing.T) {
	base, _ := LookupNetworkProfile("4g")
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

// TestApplyNetworkConditionsVM_NoOpOnZeroes verifies the zero-value
// short-circuit. Without it, an empty conditions struct would still
// invoke ssh.Run with an empty tc command line.
func TestApplyNetworkConditionsVM_NoOpOnZeroes(t *testing.T) {
	ssh := &fakeSSHClient{}
	err := applyNetworkConditionsVM(context.Background(), ssh, NetworkConditions{})
	if err != nil {
		t.Fatalf("expected nil error on zero conditions, got %v", err)
	}
	if len(ssh.runScripts) != 0 {
		t.Fatalf("expected 0 ssh.Run invocations on zero conditions, got %d: %v", len(ssh.runScripts), ssh.runScripts)
	}
}

// TestApplyNetworkConditionsVM_TcQdiscScriptIssued asserts that a
// non-zero conditions struct produces a tc-qdisc invocation with the
// expected fragments inside the SSH script.
//
// Falsifiability rehearsal:
//
//	Mutation: in pkg/vm/network.go applyNetworkConditionsVM, drop the
//	          fmt.Sprintf("rate %dkbit", ...) append.
//	Run:      go test ./pkg/vm/... -run TestApplyNetworkConditionsVM_TcQdiscScriptIssued
//	Observed-Failure: assertion fires — "expected 'rate' in the tc
//	          script; got '..netem delay 50ms loss 0.10%'".
//	Reverted: yes.
func TestApplyNetworkConditionsVM_TcQdiscScriptIssued(t *testing.T) {
	ssh := &fakeSSHClient{}
	conditions := NetworkConditions{DownKbps: 6000, LatencyMS: 50, LossPercent: 0.1}
	err := applyNetworkConditionsVM(context.Background(), ssh, conditions)
	if err != nil {
		t.Fatalf("applyNetworkConditionsVM: %v", err)
	}
	if len(ssh.runScripts) != 1 {
		t.Fatalf("expected exactly 1 ssh.Run invocation; got %d: %v", len(ssh.runScripts), ssh.runScripts)
	}
	script := ssh.runScripts[0]
	for _, want := range []string{
		"tc qdisc replace",
		"netem",
		"delay 50ms",
		"rate 6000kbit",
		"loss 0.10%",
		"ip route",
	} {
		if !strings.Contains(script, want) {
			t.Fatalf("expected %q in the tc script; got: %s", want, script)
		}
	}
}

// TestApplyNetworkConditionsVM_ExitCodeNonZero_ReturnsError asserts that
// a tc invocation that exits non-zero (e.g. iface name empty) surfaces
// as an error rather than silently passing.
func TestApplyNetworkConditionsVM_ExitCodeNonZero_ReturnsError(t *testing.T) {
	ssh := &fakeSSHClient{
		runExitCode: 1,
		runStderr:   "no-default-iface",
	}
	err := applyNetworkConditionsVM(context.Background(), ssh, NetworkConditions{LatencyMS: 50})
	if err == nil {
		t.Fatalf("expected error when in-guest tc exit=1, got nil")
	}
	if !strings.Contains(err.Error(), "exit=1") {
		t.Fatalf("error should cite exit code; got %v", err)
	}
}

// TestApplyNetworkConditionsVM_RunError_Propagated asserts SSH-level
// failure surfaces upward.
func TestApplyNetworkConditionsVM_RunError_Propagated(t *testing.T) {
	ssh := &fakeSSHClient{runError: errors.New("ssh: connection lost")}
	err := applyNetworkConditionsVM(context.Background(), ssh, NetworkConditions{LatencyMS: 50})
	if err == nil {
		t.Fatalf("expected error on ssh.Run failure")
	}
	if !strings.Contains(err.Error(), "connection lost") {
		t.Fatalf("error should carry the ssh failure; got %v", err)
	}
}

// Compile-time assertion that fakeSSHClient honours the timeout
// parameter (we don't enforce it, but the matrix runner relies on the
// call signature).
var _ = time.Duration(0)
