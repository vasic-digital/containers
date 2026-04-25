package exec

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRun_SuccessfulExecution(t *testing.T) {
	tests := []struct {
		name           string
		command        string
		args           []string
		wantStdout     string
		wantStderr     string
		wantErrContain string
	}{
		{
			name:       "echo simple text",
			command:    "echo",
			args:       []string{"hello"},
			wantStdout: "hello\n",
			wantStderr: "",
		},
		{
			name:       "echo multiple arguments",
			command:    "echo",
			args:       []string{"hello", "world"},
			wantStdout: "hello world\n",
			wantStderr: "",
		},
		{
			name:       "echo with no arguments",
			command:    "echo",
			args:       nil,
			wantStdout: "\n",
			wantStderr: "",
		},
		{
			name:       "true command succeeds",
			command:    "true",
			args:       nil,
			wantStdout: "",
			wantStderr: "",
		},
		{
			name:           "false command fails",
			command:        "false",
			args:           nil,
			wantStdout:     "",
			wantStderr:     "",
			wantErrContain: "exec false",
		},
		{
			name:       "printf without newline",
			command:    "printf",
			args:       []string{"%s", "no-newline"},
			wantStdout: "no-newline",
			wantStderr: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stdout, stderr, err := Run(ctx, tt.command, tt.args...)

			if tt.wantErrContain != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tt.wantErrContain)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, tt.wantStdout, stdout)
			assert.Equal(t, tt.wantStderr, stderr)
		})
	}
}

func TestRun_CommandNotFound(t *testing.T) {
	ctx := context.Background()

	stdout, stderr, err := Run(ctx, "nonexistent-command-xyz123")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exec nonexistent-command-xyz123")
	assert.Contains(t, err.Error(), "executable file not found")
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
}

func TestRun_Timeout(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	// sleep for longer than the context timeout
	stdout, stderr, err := Run(ctx, "sleep", "5")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exec sleep")
	// Context deadline exceeded causes signal killed
	assert.True(t,
		strings.Contains(err.Error(), "signal: killed") ||
			strings.Contains(err.Error(), "context deadline exceeded"),
		"error should indicate timeout: %v", err)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
}

func TestRun_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel context after a short delay
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	stdout, stderr, err := Run(ctx, "sleep", "5")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exec sleep")
	assert.True(t,
		strings.Contains(err.Error(), "signal: killed") ||
			strings.Contains(err.Error(), "context canceled"),
		"error should indicate cancellation: %v", err)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
}

func TestRun_StderrCapture(t *testing.T) {
	ctx := context.Background()

	// Use sh -c to write to stderr
	stdout, stderr, err := Run(ctx, "sh", "-c", "echo error >&2; exit 1")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exec sh")
	assert.Contains(t, err.Error(), "stderr: error")
	assert.Empty(t, stdout)
	assert.Equal(t, "error\n", stderr)
}

func TestRun_MixedOutput(t *testing.T) {
	ctx := context.Background()

	// Write to both stdout and stderr, then succeed
	stdout, stderr, err := Run(ctx, "sh", "-c", "echo out; echo err >&2")

	require.NoError(t, err)
	assert.Equal(t, "out\n", stdout)
	assert.Equal(t, "err\n", stderr)
}

func TestRunInDir_SuccessfulExecution(t *testing.T) {
	// Create a temporary directory
	tmpDir, err := os.MkdirTemp("", "exec-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	tests := []struct {
		name       string
		dir        string
		command    string
		args       []string
		wantStdout string
	}{
		{
			name:       "pwd in temp directory",
			dir:        tmpDir,
			command:    "pwd",
			args:       nil,
			wantStdout: tmpDir + "\n",
		},
		{
			name:       "echo in temp directory",
			dir:        tmpDir,
			command:    "echo",
			args:       []string{"test"},
			wantStdout: "test\n",
		},
		{
			name:       "empty dir uses current directory",
			dir:        "",
			command:    "echo",
			args:       []string{"current"},
			wantStdout: "current\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			stdout, stderr, err := RunInDir(ctx, tt.dir, tt.command, tt.args...)

			require.NoError(t, err)
			assert.Equal(t, tt.wantStdout, stdout)
			assert.Empty(t, stderr)
		})
	}
}

func TestRunInDir_InvalidDirectory(t *testing.T) {
	ctx := context.Background()
	nonexistentDir := "/nonexistent-directory-xyz123"

	stdout, stderr, err := RunInDir(ctx, nonexistentDir, "pwd")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exec pwd")
	// The error message varies by OS, but should indicate directory issue
	assert.True(t,
		strings.Contains(err.Error(), "no such file or directory") ||
			strings.Contains(err.Error(), "does not exist") ||
			strings.Contains(err.Error(), "chdir"),
		"error should indicate directory issue: %v", err)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
}

func TestRunInDir_CommandNotFound(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "exec-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	ctx := context.Background()

	stdout, stderr, err := RunInDir(ctx, tmpDir, "nonexistent-command-xyz123")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exec nonexistent-command-xyz123")
	assert.Contains(t, err.Error(), "executable file not found")
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
}

func TestRunInDir_Timeout(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "exec-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	stdout, stderr, err := RunInDir(ctx, tmpDir, "sleep", "5")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exec sleep")
	assert.True(t,
		strings.Contains(err.Error(), "signal: killed") ||
			strings.Contains(err.Error(), "context deadline exceeded"),
		"error should indicate timeout: %v", err)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
}

func TestRunInDir_ContextCancellation(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "exec-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	stdout, stderr, err := RunInDir(ctx, tmpDir, "sleep", "5")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exec sleep")
	assert.True(t,
		strings.Contains(err.Error(), "signal: killed") ||
			strings.Contains(err.Error(), "context canceled"),
		"error should indicate cancellation: %v", err)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
}

func TestRunInDir_FileInDirectory(t *testing.T) {
	// Create a temporary directory with a file
	tmpDir, err := os.MkdirTemp("", "exec-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	// Create a test file
	testFile := filepath.Join(tmpDir, "test.txt")
	err = os.WriteFile(testFile, []byte("content"), 0644)
	require.NoError(t, err)

	ctx := context.Background()

	// List files in the directory
	stdout, stderr, err := RunInDir(ctx, tmpDir, "ls")

	require.NoError(t, err)
	assert.Contains(t, stdout, "test.txt")
	assert.Empty(t, stderr)
}

func TestRunInDir_RelativePathInDir(t *testing.T) {
	// Create a temporary directory structure
	tmpDir, err := os.MkdirTemp("", "exec-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	subDir := filepath.Join(tmpDir, "subdir")
	err = os.Mkdir(subDir, 0755)
	require.NoError(t, err)

	// Create a file in subdir
	testFile := filepath.Join(subDir, "file.txt")
	err = os.WriteFile(testFile, []byte("data"), 0644)
	require.NoError(t, err)

	ctx := context.Background()

	// Run cat with relative path from tmpDir
	stdout, stderr, err := RunInDir(ctx, tmpDir, "cat", "subdir/file.txt")

	require.NoError(t, err)
	assert.Equal(t, "data", stdout)
	assert.Empty(t, stderr)
}

func TestRun_LargeOutput(t *testing.T) {
	ctx := context.Background()

	// Generate a large output (10000 lines)
	stdout, stderr, err := Run(ctx, "sh", "-c", "seq 1 10000")

	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(stdout), "\n")
	assert.Equal(t, 10000, len(lines))
	assert.Equal(t, "1", lines[0])
	assert.Equal(t, "10000", lines[9999])
	assert.Empty(t, stderr)
}

func TestRun_ExitCode(t *testing.T) {
	tests := []struct {
		name     string
		exitCode int
		wantErr  bool
	}{
		{name: "exit 0", exitCode: 0, wantErr: false},
		{name: "exit 1", exitCode: 1, wantErr: true},
		{name: "exit 2", exitCode: 2, wantErr: true},
		{name: "exit 127", exitCode: 127, wantErr: true},
		{name: "exit 255", exitCode: 255, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			_, _, err := Run(ctx, "sh", "-c", "exit "+string(rune('0'+tt.exitCode%10)))

			// Use proper exit code generation
			_, _, err = Run(ctx, "sh", "-c",
				"exit "+strings.TrimPrefix(tt.name, "exit "))

			if tt.wantErr {
				require.Error(t, err)
				assert.Contains(t, err.Error(), "exec sh")
			} else {
				require.NoError(t, err)
			}
		})
	}
}

func TestRunInDir_PermissionDenied(t *testing.T) {
	// Skip if running as root (root can access any directory)
	if os.Getuid() == 0 {
		t.Skip("Skipping permission test when running as root")  // SKIP-OK: #legacy-untriaged
	}

	// Create a directory without execute permission
	tmpDir, err := os.MkdirTemp("", "exec-test-*")
	require.NoError(t, err)
	defer func() {
		// Restore permissions before cleanup
		os.Chmod(tmpDir, 0755)
		os.RemoveAll(tmpDir)
	}()

	// Remove execute permission (can't cd into directory)
	err = os.Chmod(tmpDir, 0644)
	require.NoError(t, err)

	ctx := context.Background()

	stdout, stderr, err := RunInDir(ctx, tmpDir, "pwd")

	require.Error(t, err)
	assert.Contains(t, err.Error(), "exec pwd")
	assert.True(t,
		strings.Contains(err.Error(), "permission denied") ||
			strings.Contains(err.Error(), "chdir"),
		"error should indicate permission issue: %v", err)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
}

func TestRun_EmptyCommand(t *testing.T) {
	ctx := context.Background()

	stdout, stderr, err := Run(ctx, "")

	require.Error(t, err)
	assert.Empty(t, stdout)
	assert.Empty(t, stderr)
}

func TestRun_SpecialCharacters(t *testing.T) {
	ctx := context.Background()

	tests := []struct {
		name       string
		args       []string
		wantStdout string
	}{
		{
			name:       "spaces in argument",
			args:       []string{"hello world"},
			wantStdout: "hello world\n",
		},
		{
			name:       "quotes in argument",
			args:       []string{"hello \"world\""},
			wantStdout: "hello \"world\"\n",
		},
		{
			name:       "newline in argument",
			args:       []string{"hello\nworld"},
			wantStdout: "hello\nworld\n",
		},
		{
			name:       "tab in argument",
			args:       []string{"hello\tworld"},
			wantStdout: "hello\tworld\n",
		},
		{
			name:       "backslash in argument",
			args:       []string{"hello\\world"},
			wantStdout: "hello\\world\n",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			stdout, stderr, err := Run(ctx, "printf", append([]string{"%s\n"}, tt.args...)...)

			require.NoError(t, err)
			assert.Equal(t, tt.wantStdout, stdout)
			assert.Empty(t, stderr)
		})
	}
}

func TestRunInDir_SymlinkDirectory(t *testing.T) {
	// Create a temporary directory and a symlink to it
	tmpDir, err := os.MkdirTemp("", "exec-test-*")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	symlinkDir := tmpDir + "-symlink"
	err = os.Symlink(tmpDir, symlinkDir)
	require.NoError(t, err)
	defer os.Remove(symlinkDir)

	ctx := context.Background()

	// pwd -P resolves symlinks
	stdout, stderr, err := RunInDir(ctx, symlinkDir, "pwd", "-P")

	require.NoError(t, err)
	assert.Equal(t, tmpDir+"\n", stdout)
	assert.Empty(t, stderr)
}

func TestRun_ConcurrentExecution(t *testing.T) {
	ctx := context.Background()
	const numGoroutines = 10

	results := make(chan struct {
		stdout string
		stderr string
		err    error
	}, numGoroutines)

	for i := 0; i < numGoroutines; i++ {
		go func(n int) {
			stdout, stderr, err := Run(ctx, "echo", "test")
			results <- struct {
				stdout string
				stderr string
				err    error
			}{stdout, stderr, err}
		}(i)
	}

	for i := 0; i < numGoroutines; i++ {
		result := <-results
		require.NoError(t, result.err)
		assert.Equal(t, "test\n", result.stdout)
		assert.Empty(t, result.stderr)
	}
}
