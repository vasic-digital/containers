// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Milos Vasic

package buildpkg

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"digital.vasic.containers/pkg/remote"
)

// stubFailingExecutor is a unit-test-only RemoteExecutor (CONST-050(A)
// — fakes allowed in unit tests) that drives BuildExecutor.LaunchRemoteBuild
// through both i18n-migrated error branches. Production code MUST NOT
// import this struct — it is package-private and lives in *_test.go so
// the linker excludes it from non-test builds.
type stubFailingExecutor struct {
	reachable bool
	execErr   error
	result    *remote.CommandResult
}

func (s stubFailingExecutor) IsReachable(_ context.Context, _ remote.RemoteHost) bool {
	return s.reachable
}

func (s stubFailingExecutor) Execute(_ context.Context, _ remote.RemoteHost, _ string) (*remote.CommandResult, error) {
	return s.result, s.execErr
}

func (s stubFailingExecutor) CopyDir(_ context.Context, _ remote.RemoteHost, _, _ string) error {
	return nil
}

// TestBuildExecutor_ExecutionFailure_EmitsMsgID drives the
// LaunchRemoteBuild path through the executor-returns-error branch.
// Under the default NoopTranslator (installed by NewBuildExecutor) the
// BuildResult.Error MUST contain the `containers_buildpkg_execution_failed`
// message ID verbatim. CONST-035 evidence: real error string captured
// from the production call site, not a mocked translator return.
func TestBuildExecutor_ExecutionFailure_EmitsMsgID(t *testing.T) {
	be := NewBuildExecutor(stubFailingExecutor{
		reachable: true,
		execErr:   errors.New("ssh-mock-failure"),
	}, "/proj", "/remote")
	be = be.WithBuildTimeout(5 * time.Second)

	res, err := be.LaunchRemoteBuild(context.Background(),
		remote.RemoteHost{Name: "host-1", Address: "10.0.0.1"},
		"componentX", "v1.0.0", false)
	if err == nil {
		t.Fatal("LaunchRemoteBuild: want error, got nil")
	}
	if res == nil {
		t.Fatal("LaunchRemoteBuild: want non-nil result on failure")
	}
	const wantMsgID = "containers_buildpkg_execution_failed"
	if !strings.Contains(res.Error, wantMsgID) {
		t.Fatalf("BuildResult.Error mismatch:\n  got = %q\n want substring %q (noop fallback MUST emit msgID verbatim)",
			res.Error, wantMsgID)
	}
}

// TestBuildExecutor_NonZeroExit_EmitsMsgID drives the
// LaunchRemoteBuild path through the result.ExitCode != 0 branch.
// Under the default NoopTranslator the BuildResult.Error MUST contain
// the `containers_buildpkg_exit_code_failed` message ID verbatim.
func TestBuildExecutor_NonZeroExit_EmitsMsgID(t *testing.T) {
	be := NewBuildExecutor(stubFailingExecutor{
		reachable: true,
		result: &remote.CommandResult{
			ExitCode: 42,
			Stderr:   "compilation aborted",
			Duration: 2 * time.Second,
		},
	}, "/proj", "/remote")

	res, err := be.LaunchRemoteBuild(context.Background(),
		remote.RemoteHost{Name: "host-2", Address: "10.0.0.2"},
		"componentY", "v2.0.0", false)
	if err != nil {
		t.Fatalf("LaunchRemoteBuild: want nil err on non-zero exit, got %v", err)
	}
	if res == nil {
		t.Fatal("LaunchRemoteBuild: want non-nil result on non-zero exit")
	}
	const wantMsgID = "containers_buildpkg_exit_code_failed"
	if !strings.Contains(res.Error, wantMsgID) {
		t.Fatalf("BuildResult.Error mismatch:\n  got = %q\n want substring %q (noop fallback MUST emit msgID verbatim)",
			res.Error, wantMsgID)
	}
}
