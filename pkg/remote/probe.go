package remote

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Prober collects resource information from remote hosts.
type Prober struct {
	executor RemoteExecutor
}

// NewProber creates a Prober that uses the given executor.
func NewProber(executor RemoteExecutor) *Prober {
	return &Prober{executor: executor}
}

// Probe collects a resource snapshot from the remote host.
func (p *Prober) Probe(
	ctx context.Context, host RemoteHost,
) (*HostResources, error) {
	// Single compound command to minimize SSH round-trips.
	cmd := strings.Join([]string{
		"cat /proc/stat",
		"echo '---SEPARATOR---'",
		"cat /proc/meminfo",
		"echo '---SEPARATOR---'",
		"cat /proc/loadavg",
		"echo '---SEPARATOR---'",
		"df -BM --output=size,used / | tail -1",
		"echo '---SEPARATOR---'",
		"nproc",
		"echo '---SEPARATOR---'",
		"cat /proc/net/dev | tail -n +3",
	}, " && ")

	result, err := p.executor.Execute(ctx, host, cmd)
	if err != nil {
		return nil, fmt.Errorf(
			"probe %s: %w", host.Name, err,
		)
	}

	return p.parseProbeOutput(host.Name, result.Stdout)
}

func (p *Prober) parseProbeOutput(
	hostName, output string,
) (*HostResources, error) {
	sections := strings.Split(output, "---SEPARATOR---")
	if len(sections) < 6 {
		return nil, fmt.Errorf(
			"unexpected probe output: got %d sections, want 6",
			len(sections),
		)
	}

	res := &HostResources{
		Host:      hostName,
		Timestamp: time.Now(),
	}

	// Parse CPU from /proc/stat.
	res.CPUPercent = parseCPUPercent(
		strings.TrimSpace(sections[0]),
	)

	// Parse memory from /proc/meminfo.
	memTotal, memAvail := parseMemInfo(
		strings.TrimSpace(sections[1]),
	)
	res.MemoryTotalMB = memTotal / 1024
	if memTotal > 0 {
		res.MemoryUsedMB = (memTotal - memAvail) / 1024
		res.MemoryPercent = float64(memTotal-memAvail) /
			float64(memTotal) * 100.0
	}

	// Parse load average.
	loadFields := strings.Fields(
		strings.TrimSpace(sections[2]),
	)
	if len(loadFields) >= 3 {
		res.LoadAvg1, _ = strconv.ParseFloat(loadFields[0], 64)
		res.LoadAvg5, _ = strconv.ParseFloat(loadFields[1], 64)
		res.LoadAvg15, _ = strconv.ParseFloat(loadFields[2], 64)
	}

	// Parse disk from df output.
	diskFields := strings.Fields(
		strings.TrimSpace(sections[3]),
	)
	if len(diskFields) >= 2 {
		res.DiskTotalMB = parseMBField(diskFields[0])
		res.DiskUsedMB = parseMBField(diskFields[1])
		if res.DiskTotalMB > 0 {
			res.DiskPercent = float64(res.DiskUsedMB) /
				float64(res.DiskTotalMB) * 100.0
		}
	}

	// Parse CPU cores.
	coresStr := strings.TrimSpace(sections[4])
	res.CPUCores, _ = strconv.Atoi(coresStr)

	// Parse network from /proc/net/dev.
	rx, tx := parseNetDev(strings.TrimSpace(sections[5]))
	res.NetworkRxBytesPerSec = rx
	res.NetworkTxBytesPerSec = tx

	return res, nil
}

// parseCPUPercent extracts aggregate CPU usage from /proc/stat.
// It reads the first "cpu" line with aggregate values.
func parseCPUPercent(procStat string) float64 {
	for _, line := range strings.Split(procStat, "\n") {
		if !strings.HasPrefix(line, "cpu ") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 5 {
			return 0
		}
		// fields: cpu user nice system idle iowait irq softirq
		var total, idle uint64
		for i := 1; i < len(fields); i++ {
			v, _ := strconv.ParseUint(fields[i], 10, 64)
			total += v
			if i == 4 { // idle
				idle = v
			}
		}
		if total == 0 {
			return 0
		}
		return float64(total-idle) / float64(total) * 100.0
	}
	return 0
}

// parseMemInfo extracts MemTotal and MemAvailable from /proc/meminfo
// in kilobytes.
func parseMemInfo(meminfo string) (total, available uint64) {
	for _, line := range strings.Split(meminfo, "\n") {
		if strings.HasPrefix(line, "MemTotal:") {
			total = parseKBLine(line)
		} else if strings.HasPrefix(line, "MemAvailable:") {
			available = parseKBLine(line)
		}
	}
	return total, available
}

func parseKBLine(line string) uint64 {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return 0
	}
	v, _ := strconv.ParseUint(fields[1], 10, 64)
	return v
}

func parseMBField(field string) uint64 {
	field = strings.TrimSuffix(field, "M")
	v, _ := strconv.ParseUint(field, 10, 64)
	return v
}

// parseNetDev sums up rx and tx bytes across all interfaces
// from /proc/net/dev.
func parseNetDev(netDev string) (rx, tx uint64) {
	for _, line := range strings.Split(netDev, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		// Skip loopback.
		if strings.HasPrefix(line, "lo:") {
			continue
		}
		// Remove interface name prefix.
		parts := strings.SplitN(line, ":", 2)
		if len(parts) < 2 {
			continue
		}
		fields := strings.Fields(parts[1])
		if len(fields) < 10 {
			continue
		}
		rxBytes, _ := strconv.ParseUint(fields[0], 10, 64)
		txBytes, _ := strconv.ParseUint(fields[8], 10, 64)
		rx += rxBytes
		tx += txBytes
	}
	return rx, tx
}
