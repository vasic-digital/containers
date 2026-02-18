package remote

import (
	"context"
	"io"
)

// RemoteExecutor defines the interface for executing commands and
// transferring files on remote hosts.
type RemoteExecutor interface {
	// Execute runs a command on the given remote host and returns
	// the result.
	Execute(
		ctx context.Context,
		host RemoteHost,
		command string,
	) (*CommandResult, error)

	// ExecuteStream runs a command on the given remote host and
	// returns a streaming reader for its output.
	ExecuteStream(
		ctx context.Context,
		host RemoteHost,
		command string,
	) (io.ReadCloser, error)

	// CopyFile copies a local file to a remote path on the host.
	CopyFile(
		ctx context.Context,
		host RemoteHost,
		localPath, remotePath string,
	) error

	// CopyDir copies a local directory to a remote path on the
	// host.
	CopyDir(
		ctx context.Context,
		host RemoteHost,
		localDir, remoteDir string,
	) error

	// IsReachable checks whether the host is reachable via SSH.
	IsReachable(ctx context.Context, host RemoteHost) bool
}
