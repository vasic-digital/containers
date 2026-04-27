package remote

import (
	"context"
	"io"
	"testing"

	"github.com/stretchr/testify/require"
)

type fakeExec struct {
	results map[string]*CommandResult
	err     error
}

func (f *fakeExec) Execute(_ context.Context, _ RemoteHost, cmd string) (*CommandResult, error) {
	if f.err != nil {
		return nil, f.err
	}
	if r, ok := f.results[cmd]; ok {
		return r, nil
	}
	return &CommandResult{ExitCode: 127}, nil
}

func (f *fakeExec) ExecuteStream(context.Context, RemoteHost, string) (io.ReadCloser, error) {
	return nil, nil
}

func (f *fakeExec) CopyFile(context.Context, RemoteHost, string, string) error { return nil }
func (f *fakeExec) CopyDir(context.Context, RemoteHost, string, string) error  { return nil }
func (f *fakeExec) IsReachable(context.Context, RemoteHost) bool               { return true }

func TestProbeGPU_NoGPU(t *testing.T) {
	exec := &fakeExec{results: map[string]*CommandResult{}}
	devs, err := ProbeGPU(context.Background(), exec, RemoteHost{Name: "host"})
	require.NoError(t, err)
	require.Empty(t, devs)
}

func TestProbeGPU_NvidiaOnly(t *testing.T) {
	exec := &fakeExec{results: map[string]*CommandResult{
		probeNvidiaCmd: {ExitCode: 0, Stdout: sampleNvidiaSmi},
	}}
	devs, err := ProbeGPU(context.Background(), exec, RemoteHost{Name: "t"})
	require.NoError(t, err)
	require.Len(t, devs, 1)
	require.Equal(t, "nvidia", devs[0].Vendor)
}
