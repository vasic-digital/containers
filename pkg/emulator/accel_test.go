package emulator

import "testing"

// accel_test.go — table-driven coverage of the OS→acceleration model
// in accel.go.
//
// Falsifiability rehearsal (Sixth Law clause 2 / §6.N anti-bluff):
//
//	Mutation: in accel.go AccelProfileForOS, change the "darwin" case
//	          to return Runner: RunnerContainerized (and Accel:
//	          AccelKVM) — i.e. claim macOS can use the container path.
//	Run:      GOMAXPROCS=2 go test ./pkg/emulator/ -run 'Accel|Resolve' -count=1
//	Observed-Failure: TestAccelProfileForOS/darwin fails — the
//	          assertion "Runner = host-direct" reports got
//	          "containerized" want "host-direct"; TestResolveRunner
//	          subtests auto/darwin also fail for the same reason.
//	Reverted: yes — post-revert all subtests pass again.
//
// The tests assert on the actual returned struct fields (GOOS, Accel,
// Runner) — real behavior, not metadata. A regression that mis-maps
// any OS to the wrong accelerator or runner fails the matching
// subtest.

func TestAccelProfileForOS(t *testing.T) {
	cases := []struct {
		name       string
		goos       string
		wantGOOS   string
		wantAccel  AccelBackend
		wantRunner RunnerKind
	}{
		{
			name:       "linux",
			goos:       "linux",
			wantGOOS:   "linux",
			wantAccel:  AccelKVM,
			wantRunner: RunnerContainerized,
		},
		{
			name:       "darwin",
			goos:       "darwin",
			wantGOOS:   "darwin",
			wantAccel:  AccelHVF,
			wantRunner: RunnerHostDirect,
		},
		{
			name:       "windows",
			goos:       "windows",
			wantGOOS:   "windows",
			wantAccel:  AccelWHPX,
			wantRunner: RunnerHostDirect,
		},
		{
			name:       "unknown",
			goos:       "plan9",
			wantGOOS:   "plan9",
			wantAccel:  AccelNone,
			wantRunner: RunnerHostDirect,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := AccelProfileForOS(tc.goos)
			if got.GOOS != tc.wantGOOS {
				t.Errorf("GOOS: got %q want %q", got.GOOS, tc.wantGOOS)
			}
			if got.Accel != tc.wantAccel {
				t.Errorf("Accel: got %q want %q", got.Accel, tc.wantAccel)
			}
			if got.Runner != tc.wantRunner {
				t.Errorf("Runner: got %q want %q", got.Runner, tc.wantRunner)
			}
			// Rationale must be a non-empty honest explanation — an
			// empty Rationale would let a future mis-mapping ship
			// without the reviewer-facing reason.
			if got.Rationale == "" {
				t.Errorf("Rationale: got empty string, want a non-empty explanation")
			}
		})
	}
}

func TestResolveRunner(t *testing.T) {
	cases := []struct {
		name      string
		requested string
		goos      string
		want      RunnerKind
		wantErr   bool
	}{
		// auto resolves to the OS-correct runner.
		{
			name:      "auto/linux resolves to containerized",
			requested: "auto",
			goos:      "linux",
			want:      RunnerContainerized,
		},
		{
			name:      "auto/darwin resolves to host-direct",
			requested: "auto",
			goos:      "darwin",
			want:      RunnerHostDirect,
		},
		{
			name:      "auto/windows resolves to host-direct",
			requested: "auto",
			goos:      "windows",
			want:      RunnerHostDirect,
		},
		{
			name:      "auto/unknown resolves to host-direct",
			requested: "auto",
			goos:      "plan9",
			want:      RunnerHostDirect,
		},
		// Explicit values are returned verbatim regardless of goos.
		{
			name:      "explicit containerized on darwin returns containerized",
			requested: "containerized",
			goos:      "darwin",
			want:      RunnerContainerized,
		},
		{
			name:      "explicit host-direct on linux returns host-direct",
			requested: "host-direct",
			goos:      "linux",
			want:      RunnerHostDirect,
		},
		// Invalid values are configuration errors.
		{
			name:      "invalid value returns error",
			requested: "qemu-magic",
			goos:      "linux",
			wantErr:   true,
		},
		{
			name:      "empty value returns error",
			requested: "",
			goos:      "linux",
			wantErr:   true,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveRunner(tc.requested, tc.goos)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ResolveRunner(%q, %q): got nil error, want error",
						tc.requested, tc.goos)
				}
				return
			}
			if err != nil {
				t.Fatalf("ResolveRunner(%q, %q): unexpected error: %v",
					tc.requested, tc.goos, err)
			}
			if got != tc.want {
				t.Errorf("ResolveRunner(%q, %q): got %q want %q",
					tc.requested, tc.goos, got, tc.want)
			}
		})
	}
}

// TestGateEligibleForOS asserts the (runner, goos) → gate-eligibility
// truth table. The function is the basis of cmd/emulator-matrix's
// OS-aware [§6.X] print line, so a mis-classification here would let
// the matrix runner print "workstation iteration mode" for an
// OS-correct accelerated gate run (or vice versa).
//
// Falsifiability rehearsal (Sixth Law clause 2 / §6.N anti-bluff):
//
//	Mutation: in accel.go GateEligibleForOS, replace the body with
//	          `return true` — claim every runner is gate-eligible on
//	          every OS.
//	Run:      GOMAXPROCS=2 go test ./pkg/emulator/ -run 'Gate' -count=1
//	Observed-Failure: the two host-direct-on-linux and
//	          containerized-on-darwin/windows subtests fail — the
//	          assertion reports got true want false.
//	Reverted: yes — post-revert all subtests pass again.
func TestGateEligibleForOS(t *testing.T) {
	cases := []struct {
		name   string
		runner RunnerKind
		goos   string
		want   bool
	}{
		// Each OS's OS-correct runner IS gate-eligible.
		{
			name:   "host-direct on darwin is gate-eligible (HVF)",
			runner: RunnerHostDirect,
			goos:   "darwin",
			want:   true,
		},
		{
			name:   "host-direct on windows is gate-eligible (WHPX)",
			runner: RunnerHostDirect,
			goos:   "windows",
			want:   true,
		},
		{
			name:   "containerized on linux is gate-eligible (KVM)",
			runner: RunnerContainerized,
			goos:   "linux",
			want:   true,
		},
		// host-direct on linux skips KVM-in-container — NOT gate-eligible.
		{
			name:   "host-direct on linux is not gate-eligible",
			runner: RunnerHostDirect,
			goos:   "linux",
			want:   false,
		},
		// containerized on macOS/Windows cannot reach the host-only
		// accelerator — NOT gate-eligible.
		{
			name:   "containerized on darwin is not gate-eligible",
			runner: RunnerContainerized,
			goos:   "darwin",
			want:   false,
		},
		{
			name:   "containerized on windows is not gate-eligible",
			runner: RunnerContainerized,
			goos:   "windows",
			want:   false,
		},
		// On an unknown OS the OS-correct runner is host-direct.
		{
			name:   "host-direct on unknown OS is gate-eligible",
			runner: RunnerHostDirect,
			goos:   "plan9",
			want:   true,
		},
		{
			name:   "containerized on unknown OS is not gate-eligible",
			runner: RunnerContainerized,
			goos:   "plan9",
			want:   false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := GateEligibleForOS(tc.runner, tc.goos)
			if got != tc.want {
				t.Errorf("GateEligibleForOS(%q, %q): got %v want %v",
					tc.runner, tc.goos, got, tc.want)
			}
			// Cross-check: gate-eligibility MUST agree with the
			// OS-correct runner from AccelProfileForOS. A drift between
			// the two would itself be a bug.
			wantByProfile := tc.runner == AccelProfileForOS(tc.goos).Runner
			if got != wantByProfile {
				t.Errorf("GateEligibleForOS(%q, %q)=%v disagrees with AccelProfileForOS().Runner cross-check %v",
					tc.runner, tc.goos, got, wantByProfile)
			}
		})
	}
}
