package connection

import (
	"context"
	"io"
)

// Connection defines the interface for all remote connection types.
// This interface is inspired by the Mail Server Factory project's
// comprehensive connection abstraction.
type Connection interface {
	Connect(ctx context.Context) (*ConnectionResult, error)
	
	Disconnect() error
	
	IsConnected() bool
	
	Execute(ctx context.Context, command string, opts ...ExecuteOption) (*ExecutionResult, error)
	
	Upload(ctx context.Context, localPath, remotePath string, opts ...TransferOption) (*TransferResult, error)
	
	Download(ctx context.Context, remotePath, localPath string, opts ...TransferOption) (*TransferResult, error)
	
	HealthCheck(ctx context.Context) (*ConnectionHealth, error)
	
	Metadata() *ConnectionMetadata
	
	Type() ConnectionType
	
	io.Closer
}

// RemoteConnection extends Connection with remote-specific operations.
type RemoteConnection interface {
	Connection
	
	Shell(ctx context.Context, opts ...ShellOption) (io.ReadWriteCloser, error)
	
	Tunnel(ctx context.Context, localPort, remotePort int) (Tunnel, error)
	
	Host() string
	
	Port() int
}

// ContainerConnection extends Connection for container environments.
type ContainerConnection interface {
	Connection
	
	ContainerExec(ctx context.Context, containerID, command string, opts ...ExecuteOption) (*ExecutionResult, error)
	
	ContainerLogs(ctx context.Context, containerID string, opts ...LogOption) (io.ReadCloser, error)
	
	ListContainers(ctx context.Context, filter ContainerFilter) ([]ContainerInfo, error)
}

// CloudConnection extends Connection for cloud provider access.
type CloudConnection interface {
	Connection
	
	InstanceID() string
	
	Region() string
	
	Provider() CloudProvider
}

// CloudProvider identifies the cloud provider.
type CloudProvider string

const (
	ProviderAWS   CloudProvider = "aws"
	ProviderAzure CloudProvider = "azure"
	ProviderGCP   CloudProvider = "gcp"
)

// ContainerFilter defines filter criteria for container listing.
type ContainerFilter struct {
	All     bool
	Labels  map[string]string
	Names   []string
	Status  []string
}

// ContainerInfo holds container information.
type ContainerInfo struct {
	ID      string
	Name    string
	Image   string
	State   string
	Status  string
	Labels  map[string]string
	Created int64
}

// Tunnel represents an established port tunnel.
type Tunnel interface {
	LocalAddr() string
	RemoteAddr() string
	Close() error
	Wait() error
}

// ExecuteOption configures command execution.
type ExecuteOption func(*ExecuteConfig)

// ExecuteConfig holds execution configuration.
type ExecuteConfig struct {
	Timeout     int
	Env         map[string]string
	WorkingDir  string
	User        string
	Stdin       io.Reader
	CaptureStderr bool
}

// WithTimeout sets the execution timeout in seconds.
func WithTimeout(seconds int) ExecuteOption {
	return func(c *ExecuteConfig) {
		c.Timeout = seconds
	}
}

// WithEnv sets environment variables for execution.
func WithEnv(env map[string]string) ExecuteOption {
	return func(c *ExecuteConfig) {
		c.Env = env
	}
}

// WithWorkingDir sets the working directory.
func WithWorkingDir(dir string) ExecuteOption {
	return func(c *ExecuteConfig) {
		c.WorkingDir = dir
	}
}

// WithUser sets the user to run as.
func WithUser(user string) ExecuteOption {
	return func(c *ExecuteConfig) {
		c.User = user
	}
}

// WithStdin sets stdin for the command.
func WithStdin(r io.Reader) ExecuteOption {
	return func(c *ExecuteConfig) {
		c.Stdin = r
	}
}

// WithCaptureStderr captures stderr separately.
func WithCaptureStderr(capture bool) ExecuteOption {
	return func(c *ExecuteConfig) {
		c.CaptureStderr = capture
	}
}

// TransferOption configures file transfer.
type TransferOption func(*TransferConfig)

// TransferConfig holds transfer configuration.
type TransferConfig struct {
	Timeout     int
	Permissions string
	Overwrite   bool
	Progress    func(transferred, total int64)
}

// WithTransferTimeout sets the transfer timeout.
func WithTransferTimeout(seconds int) TransferOption {
	return func(c *TransferConfig) {
		c.Timeout = seconds
	}
}

// WithPermissions sets file permissions.
func WithPermissions(perm string) TransferOption {
	return func(c *TransferConfig) {
		c.Permissions = perm
	}
}

// WithOverwrite enables overwriting existing files.
func WithOverwrite(overwrite bool) TransferOption {
	return func(c *TransferConfig) {
		c.Overwrite = overwrite
	}
}

// WithProgress sets a progress callback.
func WithProgress(fn func(transferred, total int64)) TransferOption {
	return func(c *TransferConfig) {
		c.Progress = fn
	}
}

// ShellOption configures shell session.
type ShellOption func(*ShellConfig)

// ShellConfig holds shell configuration.
type ShellConfig struct {
	Terminal string
	Env      map[string]string
	Rows     uint16
	Cols     uint16
}

// WithTerminal sets the terminal type.
func WithTerminal(term string) ShellOption {
	return func(c *ShellConfig) {
		c.Terminal = term
	}
}

// WithShellEnv sets environment variables.
func WithShellEnv(env map[string]string) ShellOption {
	return func(c *ShellConfig) {
		c.Env = env
	}
}

// WithTerminalSize sets terminal dimensions.
func WithTerminalSize(rows, cols uint16) ShellOption {
	return func(c *ShellConfig) {
		c.Rows = rows
		c.Cols = cols
	}
}

// LogOption configures log retrieval.
type LogOption func(*LogConfig)

// LogConfig holds log configuration.
type LogConfig struct {
	Follow     bool
	Since      string
	Until      string
	Tail       string
	Timestamps bool
}

// WithFollow enables log following.
func WithFollow(follow bool) LogOption {
	return func(c *LogConfig) {
		c.Follow = follow
	}
}

// WithSince sets the start time for logs.
func WithSince(since string) LogOption {
	return func(c *LogConfig) {
		c.Since = since
	}
}

// WithUntil sets the end time for logs.
func WithUntil(until string) LogOption {
	return func(c *LogConfig) {
		c.Until = until
	}
}

// WithTail sets the number of lines to tail.
func WithTail(tail string) LogOption {
	return func(c *LogConfig) {
		c.Tail = tail
	}
}

// WithTimestamps enables log timestamps.
func WithTimestamps(timestamps bool) LogOption {
	return func(c *LogConfig) {
		c.Timestamps = timestamps
	}
}
