package volume

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"digital.vasic.containers/pkg/logging"
	"digital.vasic.containers/pkg/remote"
)

func newTestRsyncSyncer(
	exec remote.RemoteExecutor,
) *RsyncSyncer {
	return NewRsyncSyncer(
		exec,
		logging.NopLogger{},
		DefaultMountOptions(),
	)
}

func TestRsyncSync_Sync_NoExecutor(t *testing.T) {
	tests := []struct {
		name     string
		executor remote.RemoteExecutor
		host     remote.RemoteHost
		mount    VolumeMount
	}{
		{
			name:     "nil executor causes panic or error",
			executor: nil,
			host:     testHost(),
			mount: VolumeMount{
				Name:       "sync-data",
				Type:       MountRsync,
				LocalPath:  "/local/sync",
				RemotePath: "/remote/sync",
				HostName:   "test-host",
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			syncer := NewRsyncSyncer(
				tc.executor,
				logging.NopLogger{},
				DefaultMountOptions(),
			)
			require.NotNil(t, syncer)

			panicked := false
			func() {
				defer func() {
					if r := recover(); r != nil {
						panicked = true
					}
				}()
				err := syncer.Sync(
					context.Background(), tc.host, tc.mount,
				)
				if err != nil {
					assert.Error(t, err)
					return
				}
			}()

			if panicked {
				assert.True(t, panicked,
					"Sync with nil executor should panic "+
						"or return error")
			}
		})
	}
}

func TestRsyncSync_Sync_ExecutorError(t *testing.T) {
	tests := []struct {
		name        string
		executeFunc func(ctx context.Context, host remote.RemoteHost, cmd string) (*remote.CommandResult, error)
		errContains string
	}{
		{
			name: "executor returns connection error",
			executeFunc: func(
				ctx context.Context,
				host remote.RemoteHost,
				cmd string,
			) (*remote.CommandResult, error) {
				return nil, fmt.Errorf("connection refused")
			},
			errContains: "connection refused",
		},
		{
			name: "executor returns non-zero exit on mkdir",
			executeFunc: func(
				ctx context.Context,
				host remote.RemoteHost,
				cmd string,
			) (*remote.CommandResult, error) {
				return &remote.CommandResult{
					ExitCode: 1,
					Stderr:   "no space left on device",
				}, nil
			},
			errContains: "no space left on device",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			exec := &mockExecutor{executeFunc: tc.executeFunc}
			syncer := newTestRsyncSyncer(exec)

			mount := VolumeMount{
				Name:       "sync-data",
				Type:       MountRsync,
				LocalPath:  "/local/sync",
				RemotePath: "/remote/sync",
				HostName:   "test-host",
			}

			err := syncer.Sync(
				context.Background(), testHost(), mount,
			)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tc.errContains)
		})
	}
}

func TestRsyncSync_Status(t *testing.T) {
	tests := []struct {
		name          string
		mountName     string
		setupMount    bool
		expectedState MountState
		expectErr     bool
	}{
		{
			name:          "status of synced mount",
			mountName:     "sync-data",
			setupMount:    true,
			expectedState: MountMounted,
			expectErr:     false,
		},
		{
			name:       "status of non-existent mount",
			mountName:  "missing",
			setupMount: false,
			expectErr:  true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			exec := &mockExecutor{}
			hm := &mockHostManager{
				hosts: map[string]remote.RemoteHost{
					"test-host": testHost(),
				},
			}
			mgr := NewVolumeManager(
				hm, exec, logging.NopLogger{},
			)

			if tc.setupMount {
				mount := VolumeMount{
					Name:       tc.mountName,
					Type:       MountRsync,
					LocalPath:  "/local/sync",
					RemotePath: "/remote/sync",
					HostName:   "test-host",
				}
				err := mgr.Mount(context.Background(), mount)
				require.NoError(t, err)
			}

			info, err := mgr.Status(tc.mountName)
			if tc.expectErr {
				assert.Error(t, err)
				assert.Nil(t, info)
				return
			}

			require.NoError(t, err)
			require.NotNil(t, info)
			assert.Equal(t, tc.expectedState, info.State)
			assert.Equal(t, tc.mountName, info.Mount.Name)
		})
	}
}

func TestRsyncSync_Type(t *testing.T) {
	tests := []struct {
		name     string
		mount    VolumeMount
		expected MountType
	}{
		{
			name: "rsync mount type",
			mount: VolumeMount{
				Name: "sync-data",
				Type: MountRsync,
			},
			expected: MountRsync,
		},
		{
			name: "rsync type string value",
			mount: VolumeMount{
				Name: "typed-sync",
				Type: MountRsync,
			},
			expected: "rsync",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			assert.Equal(t, tc.expected, tc.mount.Type)
			assert.Equal(t, "rsync", string(MountRsync))
		})
	}
}
