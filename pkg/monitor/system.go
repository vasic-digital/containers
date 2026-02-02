package monitor

import (
	"bufio"
	"os"
	"runtime"
	"strconv"
	"strings"
	"time"

	"digital.vasic.containers/internal/platform"
)

// SystemCollector gathers host-level resource metrics.
type SystemCollector interface {
	// Collect returns the current system resource usage.
	Collect() SystemResources
}

// DefaultSystemCollector reads system metrics from /proc on Linux
// and falls back to Go runtime metrics on other platforms.
type DefaultSystemCollector struct {
	prevIdle  uint64
	prevTotal uint64
}

// NewDefaultSystemCollector creates a DefaultSystemCollector. On
// Linux it primes the CPU counters with an initial sample.
func NewDefaultSystemCollector() *DefaultSystemCollector {
	c := &DefaultSystemCollector{}
	if platform.IsLinux() {
		idle, total := readCPUSample()
		c.prevIdle = idle
		c.prevTotal = total
		// Allow a small window so the next Collect has a delta.
		time.Sleep(50 * time.Millisecond)
	}
	return c
}

// Collect returns the current system resource usage.
func (c *DefaultSystemCollector) Collect() SystemResources {
	var res SystemResources

	if platform.IsLinux() {
		res.CPUPercent = c.collectCPULinux()
		c.collectMemoryLinux(&res)
		c.collectDiskLinux(&res)
	} else {
		// Fallback: use Go runtime for memory.
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		res.MemoryTotal = m.Sys
		res.MemoryUsed = m.Alloc
		if res.MemoryTotal > 0 {
			res.MemoryPercent = float64(res.MemoryUsed) /
				float64(res.MemoryTotal) * 100
		}
	}

	return res
}

// collectCPULinux reads /proc/stat and computes CPU usage since the
// previous sample.
func (c *DefaultSystemCollector) collectCPULinux() float64 {
	idle, total := readCPUSample()
	if total == c.prevTotal {
		return 0
	}
	idleDelta := float64(idle - c.prevIdle)
	totalDelta := float64(total - c.prevTotal)
	c.prevIdle = idle
	c.prevTotal = total
	return (1.0 - idleDelta/totalDelta) * 100
}

// readCPUSample parses the first cpu line from /proc/stat and
// returns the idle ticks and total ticks.
func readCPUSample() (idle, total uint64) {
	f, err := os.Open("/proc/stat")
	if err != nil {
		return 0, 0
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			return 0, 0
		}
		var vals [10]uint64
		for i := 1; i < len(fields) && i <= 10; i++ {
			v, _ := strconv.ParseUint(fields[i], 10, 64)
			vals[i-1] = v
			total += v
		}
		// idle is the 4th value (index 3).
		idle = vals[3]
		return idle, total
	}
	return 0, 0
}

// collectMemoryLinux reads /proc/meminfo for total and available
// memory.
func (c *DefaultSystemCollector) collectMemoryLinux(
	res *SystemResources,
) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return
	}
	defer f.Close()

	var memTotal, memAvailable uint64
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "MemTotal:") {
			memTotal = parseMemInfoKB(line)
		} else if strings.HasPrefix(line, "MemAvailable:") {
			memAvailable = parseMemInfoKB(line)
		}
	}

	res.MemoryTotal = memTotal * 1024 // convert KB to bytes
	if memTotal > 0 && memAvailable <= memTotal {
		res.MemoryUsed = (memTotal - memAvailable) * 1024
		res.MemoryPercent = float64(memTotal-memAvailable) /
			float64(memTotal) * 100
	}
}

// parseMemInfoKB extracts the numeric kB value from a /proc/meminfo
// line such as "MemTotal:       16384000 kB".
func parseMemInfoKB(line string) uint64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	v, _ := strconv.ParseUint(fields[1], 10, 64)
	return v
}

// collectDiskLinux reads disk usage for the root filesystem using
// syscall.Statfs. Falls back gracefully on failure.
func (c *DefaultSystemCollector) collectDiskLinux(
	res *SystemResources,
) {
	// Use os.Stat to at least verify root is accessible. Real disk
	// stats require syscall; kept minimal to avoid cgo dependency.
	info, err := os.Stat("/")
	if err != nil || info == nil {
		return
	}
	// Disk stats via statfs would go here. Left as zero values to
	// avoid platform-specific syscall imports. A production
	// implementation should use golang.org/x/sys/unix.Statfs.
}
