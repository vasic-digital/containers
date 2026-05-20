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
