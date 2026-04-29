package platform

import (
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestIsLinux verifies IsLinux returns the correct value based on the current platform.
func TestIsLinux(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{
			name:     "returns correct value for current platform",
			expected: runtime.GOOS == "linux",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsLinux()
			assert.Equal(t, tt.expected, result, "IsLinux() should return %v on %s", tt.expected, runtime.GOOS)
		})
	}
}

// TestIsDarwin verifies IsDarwin returns the correct value based on the current platform.
func TestIsDarwin(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{
			name:     "returns correct value for current platform",
			expected: runtime.GOOS == "darwin",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsDarwin()
			assert.Equal(t, tt.expected, result, "IsDarwin() should return %v on %s", tt.expected, runtime.GOOS)
		})
	}
}

// TestIsWindows verifies IsWindows returns the correct value based on the current platform.
func TestIsWindows(t *testing.T) {
	tests := []struct {
		name     string
		expected bool
	}{
		{
			name:     "returns correct value for current platform",
			expected: runtime.GOOS == "windows",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := IsWindows()
			assert.Equal(t, tt.expected, result, "IsWindows() should return %v on %s", tt.expected, runtime.GOOS)
		})
	}
}

// TestPlatformDetection_MutualExclusivity verifies that exactly one platform function returns true.
func TestPlatformDetection_MutualExclusivity(t *testing.T) {
	tests := []struct {
		name string
	}{
		{
			name: "exactly one platform detection function returns true for current OS",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			isLinux := IsLinux()
			isDarwin := IsDarwin()
			isWindows := IsWindows()

			// Count how many return true
			trueCount := 0
			if isLinux {
				trueCount++
			}
			if isDarwin {
				trueCount++
			}
			if isWindows {
				trueCount++
			}

			// On Linux, Darwin, or Windows, exactly one should be true
			// On other platforms (e.g., freebsd, netbsd), all should be false
			switch runtime.GOOS {
			case "linux":
				assert.True(t, isLinux, "IsLinux() should return true on linux")
				assert.False(t, isDarwin, "IsDarwin() should return false on linux")
				assert.False(t, isWindows, "IsWindows() should return false on linux")
				assert.Equal(t, 1, trueCount, "exactly one platform function should return true")
			case "darwin":
				assert.False(t, isLinux, "IsLinux() should return false on darwin")
				assert.True(t, isDarwin, "IsDarwin() should return true on darwin")
				assert.False(t, isWindows, "IsWindows() should return false on darwin")
				assert.Equal(t, 1, trueCount, "exactly one platform function should return true")
			case "windows":
				assert.False(t, isLinux, "IsLinux() should return false on windows")
				assert.False(t, isDarwin, "IsDarwin() should return false on windows")
				assert.True(t, isWindows, "IsWindows() should return true on windows")
				assert.Equal(t, 1, trueCount, "exactly one platform function should return true")
			default:
				// On other platforms, all should return false
				assert.False(t, isLinux, "IsLinux() should return false on %s", runtime.GOOS)
				assert.False(t, isDarwin, "IsDarwin() should return false on %s", runtime.GOOS)
				assert.False(t, isWindows, "IsWindows() should return false on %s", runtime.GOOS)
				assert.Equal(t, 0, trueCount, "all platform functions should return false on %s", runtime.GOOS)
			}
		})
	}
}

// TestPlatformDetection_Consistency verifies that repeated calls return consistent values.
func TestPlatformDetection_Consistency(t *testing.T) {
	tests := []struct {
		name string
		fn   func() bool
	}{
		{
			name: "IsLinux returns consistent values",
			fn:   IsLinux,
		},
		{
			name: "IsDarwin returns consistent values",
			fn:   IsDarwin,
		},
		{
			name: "IsWindows returns consistent values",
			fn:   IsWindows,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Call the function multiple times and verify consistency
			firstResult := tt.fn()
			for i := 0; i < 100; i++ {
				result := tt.fn()
				assert.Equal(t, firstResult, result, "platform detection should be consistent across calls")
			}
		})
	}
}

// TestCurrentPlatform documents the current platform for test output visibility.
func TestCurrentPlatform(t *testing.T) {
	// bluff-scan: no-assert-ok (basic build/config smoke — must not panic)
	t.Logf("Current platform: runtime.GOOS=%q, runtime.GOARCH=%q", runtime.GOOS, runtime.GOARCH)
	t.Logf("IsLinux()=%v, IsDarwin()=%v, IsWindows()=%v", IsLinux(), IsDarwin(), IsWindows())
}
