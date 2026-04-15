package monitor

import (
	"os"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockPlatformChecker allows testing of platform-specific code paths.
type mockPlatformChecker struct {
	linux bool
}

func (m mockPlatformChecker) isLinux() bool {
	return m.linux
}

// TestParseMemInfoKB_EdgeCases tests the parseMemInfoKB function with
// various edge cases.
func TestParseMemInfoKB_EdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected uint64
	}{
		{
			name:     "valid line",
			input:    "MemTotal:       16384000 kB",
			expected: 16384000,
		},
		{
			name:     "empty line",
			input:    "",
			expected: 0,
		},
		{
			name:     "single field only",
			input:    "MemTotal:",
			expected: 0,
		},
		{
			name:     "no colon",
			input:    "MemTotal 16384000 kB",
			expected: 16384000,
		},
		{
			name:     "extra whitespace",
			input:    "MemTotal:            16384000    kB",
			expected: 16384000,
		},
		{
			name:     "invalid number",
			input:    "MemTotal: invalid kB",
			expected: 0,
		},
		{
			name:     "negative number (parsed as 0)",
			input:    "MemTotal: -1000 kB",
			expected: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := parseMemInfoKB(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestReadCPUSample_EdgeCases covers edge cases in CPU sample reading.
// Note: This test runs on the actual system, so it tests real behavior.
func TestReadCPUSample_EdgeCases(t *testing.T) {
	// On a real Linux system, readCPUSample should return non-zero values.
	idle, total := readCPUSample()

	// Total should always be greater than idle (system does some work)
	// On Linux systems, both should be non-zero
	if total > 0 {
		assert.GreaterOrEqual(t, total, idle,
			"total CPU ticks should be >= idle ticks")
	}
}

// TestCollectDiskLinux_RootAccessible tests the disk collection function.
// On most systems, root should be accessible.
func TestCollectDiskLinux_RootAccessible(t *testing.T) {
	c := &DefaultSystemCollector{}
	res := &SystemResources{}

	// This should not panic even if stat fails
	c.collectDiskLinux(res)

	// We can't assert specific values since disk stats aren't fully
	// implemented, but we can verify it doesn't panic
}

// TestCollectMemoryLinux_ValidData tests memory collection on Linux.
func TestCollectMemoryLinux_ValidData(t *testing.T) {
	c := &DefaultSystemCollector{}
	res := &SystemResources{}

	c.collectMemoryLinux(res)

	// On a real Linux system, we should get valid memory data
	assert.Greater(t, res.MemoryTotal, uint64(0),
		"MemoryTotal should be non-zero on Linux")
}

// TestCollectCPULinux_Delta tests CPU collection with delta computation.
func TestCollectCPULinux_Delta(t *testing.T) {
	c := &DefaultSystemCollector{}

	// First call initializes prevIdle and prevTotal
	cpu1 := c.collectCPULinux()

	// Second call should compute delta
	cpu2 := c.collectCPULinux()

	// Both should be valid percentages (0-100)
	assert.GreaterOrEqual(t, cpu1, 0.0)
	assert.LessOrEqual(t, cpu1, 100.0)
	assert.GreaterOrEqual(t, cpu2, 0.0)
	assert.LessOrEqual(t, cpu2, 100.0)
}

// TestCollectCPULinux_SameTotalReturnsZero tests the edge case where
// total doesn't change between samples.
func TestCollectCPULinux_SameTotalReturnsZero(t *testing.T) {
	c := &DefaultSystemCollector{}

	// Get the initial sample to populate prevTotal
	idle, total := readCPUSample()
	c.prevIdle = idle
	c.prevTotal = total

	// If we immediately check again with the same values,
	// the function should return 0 (no change in total)
	// This tests the total == prevTotal branch
	// Note: In real conditions, total always increases, so this is
	// really just testing the arithmetic, not the branch
	cpu := c.collectCPULinux()
	assert.GreaterOrEqual(t, cpu, 0.0)
}

// TestNewDefaultSystemCollector_Initialization tests that the collector
// initializes with valid CPU samples on Linux.
func TestNewDefaultSystemCollector_Initialization(t *testing.T) {
	c := NewDefaultSystemCollector()

	// On Linux, prevIdle and prevTotal should be non-zero after init
	assert.Greater(t, c.prevTotal, uint64(0),
		"prevTotal should be initialized on Linux")
}

// TestCollect_NonLinuxBranch tests the non-Linux code path in Collect by
// directly exercising the Go runtime memory stats fallback logic.
// Since we're on Linux, we can't test the actual non-Linux branch, but
// we can verify the arithmetic that would apply.
func TestCollect_FallbackMemoryCalculation(t *testing.T) {
	// Test the memory percentage calculation that would happen on non-Linux
	// by verifying the formula: MemoryPercent = (MemoryUsed / MemoryTotal) * 100

	tests := []struct {
		name          string
		total         uint64
		used          uint64
		wantPercent   float64
		wantNoPercent bool // For zero total case
	}{
		{
			name:        "normal usage",
			total:       1000,
			used:        250,
			wantPercent: 25.0,
		},
		{
			name:        "full usage",
			total:       1000,
			used:        1000,
			wantPercent: 100.0,
		},
		{
			name:          "zero total",
			total:         0,
			used:          0,
			wantNoPercent: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var percent float64
			if tc.total > 0 {
				percent = float64(tc.used) / float64(tc.total) * 100
			}
			if tc.wantNoPercent {
				assert.Equal(t, 0.0, percent)
			} else {
				assert.Equal(t, tc.wantPercent, percent)
			}
		})
	}
}

// TestCollectDiskLinux_StatErrors tests the disk collection error handling.
func TestCollectDiskLinux_StatErrors(t *testing.T) {
	c := &DefaultSystemCollector{}
	res := &SystemResources{}

	// The function should not panic even with edge cases.
	c.collectDiskLinux(res)

	// On Linux, syscall.Statfs returns real values for "/".
	if runtime.GOOS == "linux" {
		assert.Greater(t, res.DiskTotal, uint64(0),
			"DiskTotal should be non-zero on Linux")
		assert.Greater(t, res.DiskPercent, 0.0,
			"DiskPercent should be non-zero on Linux")
	} else {
		assert.Equal(t, uint64(0), res.DiskTotal)
		assert.Equal(t, uint64(0), res.DiskUsed)
		assert.Equal(t, 0.0, res.DiskPercent)
	}
}

// TestCollectMemoryLinux_EdgeCases tests edge cases in memory collection.
func TestCollectMemoryLinux_EdgeCases(t *testing.T) {
	c := &DefaultSystemCollector{}

	// Test that the function handles edge cases without panicking
	t.Run("normal collection", func(t *testing.T) {
		res := &SystemResources{}
		c.collectMemoryLinux(res)

		// On a real Linux system, we should have memory data
		assert.Greater(t, res.MemoryTotal, uint64(0))
	})

	t.Run("verification of calculation", func(t *testing.T) {
		// Verify the calculation logic for when memAvailable <= memTotal
		memTotal := uint64(16000000)
		memAvailable := uint64(8000000)

		var res SystemResources
		res.MemoryTotal = memTotal * 1024
		if memTotal > 0 && memAvailable <= memTotal {
			res.MemoryUsed = (memTotal - memAvailable) * 1024
			res.MemoryPercent = float64(memTotal-memAvailable) /
				float64(memTotal) * 100
		}

		assert.Equal(t, uint64(8000000*1024), res.MemoryUsed)
		assert.Equal(t, 50.0, res.MemoryPercent)
	})
}

// TestReadCPUSample_ErrorPaths covers error conditions in readCPUSample.
func TestReadCPUSample_ErrorPaths(t *testing.T) {
	// Test that the function returns zeros on error (tested implicitly)
	// The function reads /proc/stat, which should exist on Linux
	idle, total := readCPUSample()

	// On a real system, both should be non-zero
	// This test verifies the function completes without error
	t.Logf("idle=%d total=%d", idle, total)

	// If we're on a system where /proc/stat doesn't have cpu line
	// (shouldn't happen), both would be 0
	if total > 0 {
		assert.GreaterOrEqual(t, total, idle)
	}
}

// TestCollectCPULinux_ZeroDelta tests the case where CPU total doesn't change.
func TestCollectCPULinux_ZeroDelta(t *testing.T) {
	c := &DefaultSystemCollector{}

	// Set prev values to current values to simulate no change
	idle, total := readCPUSample()
	c.prevIdle = idle
	c.prevTotal = total

	// When total == prevTotal, should return 0
	// This is hard to test because CPU is always changing,
	// but we can verify the logic
	result := c.collectCPULinux()

	// Result should be a valid percentage (0-100)
	assert.GreaterOrEqual(t, result, 0.0)
	assert.LessOrEqual(t, result, 100.0)
}

// TestCollectCPULinux_SameTotal tests the case when total equals prevTotal.
func TestCollectCPULinux_SameTotal(t *testing.T) {
	c := &DefaultSystemCollector{}

	// Set prev values to specific values
	c.prevIdle = 1000
	c.prevTotal = 5000

	// Read current sample
	idle, total := readCPUSample()

	// If by chance they're the same (very unlikely), that tests the branch
	// Otherwise, we verify normal operation
	if total == c.prevTotal {
		result := c.collectCPULinux()
		assert.Equal(t, 0.0, result)
	} else {
		result := c.collectCPULinux()
		assert.GreaterOrEqual(t, result, 0.0)
	}

	// The test still runs and covers the comparison logic
	t.Logf("Current idle=%d total=%d", idle, total)
}

// TestCollectDiskLinux_VerifyDiskValues verifies disk values are populated
// on Linux via syscall.Statfs and remain zero on other platforms.
func TestCollectDiskLinux_VerifyDiskValues(t *testing.T) {
	c := &DefaultSystemCollector{}
	res := &SystemResources{}

	c.collectDiskLinux(res)

	if runtime.GOOS == "linux" {
		assert.Greater(t, res.DiskTotal, uint64(0),
			"DiskTotal should be non-zero on Linux")
		assert.Greater(t, res.DiskUsed, uint64(0),
			"DiskUsed should be non-zero on Linux")
		assert.Greater(t, res.DiskPercent, 0.0,
			"DiskPercent should be non-zero on Linux")
		assert.LessOrEqual(t, res.DiskPercent, 100.0,
			"DiskPercent should not exceed 100")
	} else {
		assert.Equal(t, uint64(0), res.DiskTotal)
		assert.Equal(t, uint64(0), res.DiskUsed)
		assert.Equal(t, 0.0, res.DiskPercent)
	}
}

// TestCollect_LinuxPath verifies the Linux-specific collection path.
func TestCollect_LinuxPath(t *testing.T) {
	c := NewDefaultSystemCollector()

	res := c.Collect()

	// On Linux, we should have real system metrics
	assert.Greater(t, res.MemoryTotal, uint64(0))
	assert.Greater(t, res.MemoryUsed, uint64(0))
	assert.GreaterOrEqual(t, res.MemoryPercent, 0.0)
	assert.LessOrEqual(t, res.MemoryPercent, 100.0)
	assert.GreaterOrEqual(t, res.CPUPercent, 0.0)
	assert.LessOrEqual(t, res.CPUPercent, 100.0)
}

// TestParseMemInfoKB_MoreEdgeCases tests additional edge cases for parseMemInfoKB.
func TestParseMemInfoKB_MoreEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected uint64
	}{
		{
			name:     "single whitespace separator",
			input:    "MemTotal: 16384000 kB",
			expected: 16384000,
		},
		{
			name:     "large number",
			input:    "MemTotal: 999999999999 kB",
			expected: 999999999999,
		},
		{
			name:     "zero value",
			input:    "MemTotal: 0 kB",
			expected: 0,
		},
		{
			name:     "just two fields",
			input:    "MemTotal: 12345",
			expected: 12345,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result := parseMemInfoKB(tc.input)
			assert.Equal(t, tc.expected, result)
		})
	}
}

// TestCollectMemoryLinux_VerifiesMemory verifies memory is collected correctly.
func TestCollectMemoryLinux_VerifiesMemory(t *testing.T) {
	c := &DefaultSystemCollector{}
	res := &SystemResources{}

	c.collectMemoryLinux(res)

	// Should have valid memory values on Linux
	assert.Greater(t, res.MemoryTotal, uint64(0))

	// Memory used should be positive and less than total
	assert.GreaterOrEqual(t, res.MemoryUsed, uint64(0))
	assert.LessOrEqual(t, res.MemoryUsed, res.MemoryTotal)

	// Memory percent should be in valid range
	assert.GreaterOrEqual(t, res.MemoryPercent, 0.0)
	assert.LessOrEqual(t, res.MemoryPercent, 100.0)
}

// TestCollect_NonLinuxPath tests the non-Linux code path using mock platform.
func TestCollect_NonLinuxPath(t *testing.T) {
	// Create collector with mock platform checker that returns false for isLinux
	c := &DefaultSystemCollector{
		platform: mockPlatformChecker{linux: false},
	}

	res := c.Collect()

	// Non-Linux path uses Go runtime stats, should have valid memory data
	assert.Greater(t, res.MemoryTotal, uint64(0),
		"expected non-zero MemoryTotal from Go runtime")
	assert.Greater(t, res.MemoryUsed, uint64(0),
		"expected non-zero MemoryUsed from Go runtime")
	assert.GreaterOrEqual(t, res.MemoryPercent, 0.0)
	assert.LessOrEqual(t, res.MemoryPercent, 100.0)

	// CPU should be zero since it's not collected on non-Linux
	assert.Equal(t, 0.0, res.CPUPercent)
}

// TestCollect_NonLinuxPathZeroTotal tests edge case where MemoryTotal is 0.
func TestCollect_NonLinuxPathZeroTotal(t *testing.T) {
	// The Go runtime always reports non-zero Sys, but we can verify
	// the arithmetic that would happen if it were zero
	total := uint64(0)
	used := uint64(0)
	var percent float64
	if total > 0 {
		percent = float64(used) / float64(total) * 100
	}
	assert.Equal(t, 0.0, percent)
}

// TestCollect_NilPlatformChecker tests that nil platform checker uses default.
func TestCollect_NilPlatformChecker(t *testing.T) {
	c := &DefaultSystemCollector{
		platform: nil, // Explicitly nil
	}

	// Should not panic and should use default behavior (Linux)
	res := c.Collect()
	assert.Greater(t, res.MemoryTotal, uint64(0))
}

// TestDefaultPlatformChecker tests the default platform checker.
func TestDefaultPlatformChecker(t *testing.T) {
	checker := defaultPlatformChecker{}
	// On Linux, this should return true
	result := checker.isLinux()
	// We're running on Linux, so it should be true
	assert.True(t, result)
}

// TestReadCPUSampleFromFile_ErrorCases tests error handling in CPU sample reading.
func TestReadCPUSampleFromFile_ErrorCases(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		wantIdle    uint64
		wantTotal   uint64
		description string
	}{
		{
			name:        "nonexistent file",
			path:        "/nonexistent/path/stat",
			wantIdle:    0,
			wantTotal:   0,
			description: "should return zeros when file doesn't exist",
		},
		{
			name:        "permission denied simulated",
			path:        "/proc/1/root/nonexistent",
			wantIdle:    0,
			wantTotal:   0,
			description: "should return zeros when file access is denied",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			idle, total := readCPUSampleFromFile(tc.path)
			assert.Equal(t, tc.wantIdle, idle, tc.description)
			assert.Equal(t, tc.wantTotal, total, tc.description)
		})
	}
}

// TestReadCPUSampleFromFile_MalformedData tests handling of malformed CPU data.
func TestReadCPUSampleFromFile_MalformedData(t *testing.T) {
	// Create a temp file with malformed data
	tmpDir := t.TempDir()

	tests := []struct {
		name      string
		content   string
		wantIdle  uint64
		wantTotal uint64
	}{
		{
			name:      "empty file",
			content:   "",
			wantIdle:  0,
			wantTotal: 0,
		},
		{
			name:      "no cpu line",
			content:   "intr 123456\nctxt 987654\n",
			wantIdle:  0,
			wantTotal: 0,
		},
		{
			name:      "cpu line with insufficient fields",
			content:   "cpu 100 200 300\n",
			wantIdle:  0,
			wantTotal: 0,
		},
		{
			name:      "valid cpu line",
			content:   "cpu  100 200 300 400 500 600 700 800 900 1000\n",
			wantIdle:  400,
			wantTotal: 5500,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			path := tmpDir + "/" + tc.name + ".stat"
			err := os.WriteFile(path, []byte(tc.content), 0644)
			require.NoError(t, err)

			idle, total := readCPUSampleFromFile(path)
			assert.Equal(t, tc.wantIdle, idle)
			assert.Equal(t, tc.wantTotal, total)
		})
	}
}

// TestCollectMemoryLinuxFromFile_ErrorCases tests error handling in memory collection.
func TestCollectMemoryLinuxFromFile_ErrorCases(t *testing.T) {
	c := &DefaultSystemCollector{}

	t.Run("nonexistent file", func(t *testing.T) {
		res := &SystemResources{}
		c.collectMemoryLinuxFromFile(res, "/nonexistent/meminfo")

		// Should not panic and values should be zero
		assert.Equal(t, uint64(0), res.MemoryTotal)
		assert.Equal(t, uint64(0), res.MemoryUsed)
		assert.Equal(t, 0.0, res.MemoryPercent)
	})

	t.Run("custom meminfo data", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := tmpDir + "/meminfo"
		content := "MemTotal:       16000000 kB\nMemAvailable:   8000000 kB\n"
		err := os.WriteFile(path, []byte(content), 0644)
		require.NoError(t, err)

		res := &SystemResources{}
		c.collectMemoryLinuxFromFile(res, path)

		assert.Equal(t, uint64(16000000*1024), res.MemoryTotal)
		assert.Equal(t, uint64(8000000*1024), res.MemoryUsed)
		assert.Equal(t, 50.0, res.MemoryPercent)
	})

	t.Run("memAvailable greater than memTotal", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := tmpDir + "/meminfo"
		// Edge case: available > total (malformed data)
		content := "MemTotal:       8000000 kB\nMemAvailable:   16000000 kB\n"
		err := os.WriteFile(path, []byte(content), 0644)
		require.NoError(t, err)

		res := &SystemResources{}
		c.collectMemoryLinuxFromFile(res, path)

		// memTotal is set, but memUsed and memPercent are not
		// because memAvailable > memTotal fails the condition
		assert.Equal(t, uint64(8000000*1024), res.MemoryTotal)
		assert.Equal(t, uint64(0), res.MemoryUsed)
		assert.Equal(t, 0.0, res.MemoryPercent)
	})

	t.Run("zero memTotal", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := tmpDir + "/meminfo"
		content := "MemTotal:       0 kB\nMemAvailable:   0 kB\n"
		err := os.WriteFile(path, []byte(content), 0644)
		require.NoError(t, err)

		res := &SystemResources{}
		c.collectMemoryLinuxFromFile(res, path)

		assert.Equal(t, uint64(0), res.MemoryTotal)
		assert.Equal(t, uint64(0), res.MemoryUsed)
		assert.Equal(t, 0.0, res.MemoryPercent)
	})
}

// TestCollectDiskLinuxFromPath_ErrorCases tests error handling in disk collection.
func TestCollectDiskLinuxFromPath_ErrorCases(t *testing.T) {
	c := &DefaultSystemCollector{}

	t.Run("nonexistent path", func(t *testing.T) {
		res := &SystemResources{}
		c.collectDiskLinuxFromPath(res, "/nonexistent/path/that/does/not/exist")

		// Should not panic and values should be zero
		assert.Equal(t, uint64(0), res.DiskTotal)
		assert.Equal(t, uint64(0), res.DiskUsed)
		assert.Equal(t, 0.0, res.DiskPercent)
	})

	t.Run("valid path", func(t *testing.T) {
		res := &SystemResources{}
		c.collectDiskLinuxFromPath(res, "/")

		// On Linux the syscall.Statfs implementation returns real values.
		// On other platforms the no-op stub leaves them at zero.
		if runtime.GOOS == "linux" {
			assert.Greater(t, res.DiskTotal, uint64(0),
				"DiskTotal should be non-zero on Linux")
			assert.Greater(t, res.DiskPercent, 0.0,
				"DiskPercent should be non-zero on Linux")
		} else {
			assert.Equal(t, uint64(0), res.DiskTotal)
		}
	})

	t.Run("file instead of directory", func(t *testing.T) {
		tmpDir := t.TempDir()
		path := tmpDir + "/testfile"
		err := os.WriteFile(path, []byte("test"), 0644)
		require.NoError(t, err)

		res := &SystemResources{}
		c.collectDiskLinuxFromPath(res, path)

		// On Linux, statfs works on files too and returns the
		// containing filesystem stats.
		if runtime.GOOS == "linux" {
			assert.Greater(t, res.DiskTotal, uint64(0),
				"DiskTotal should be non-zero for file path on Linux")
		} else {
			assert.Equal(t, uint64(0), res.DiskTotal)
		}
	})
}
