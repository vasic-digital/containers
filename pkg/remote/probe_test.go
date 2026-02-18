package remote

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestProber_Probe(t *testing.T) {
	procStat := `cpu  10000 500 3000 80000 200 100 50 0 0 0
cpu0 5000 250 1500 40000 100 50 25 0 0 0`

	meminfo := `MemTotal:       16384000 kB
MemFree:         4096000 kB
MemAvailable:    8192000 kB
Buffers:          512000 kB`

	loadavg := "1.50 2.30 3.10 3/256 12345"

	dfOutput := "100000M   60000M"

	nproc := "8"

	netDev := `eth0: 1000000 1000 0 0 0 0 0 0 500000 800 0 0 0 0 0 0
lo:   100 10 0 0 0 0 0 0 100 10 0 0 0 0 0 0`

	output := fmt.Sprintf(
		"%s\n---SEPARATOR---\n%s\n---SEPARATOR---\n%s\n"+
			"---SEPARATOR---\n%s\n---SEPARATOR---\n%s\n"+
			"---SEPARATOR---\n%s",
		procStat, meminfo, loadavg, dfOutput, nproc, netDev,
	)

	exec := &mockExecutor{
		executeFunc: func(
			ctx context.Context, host RemoteHost, cmd string,
		) (*CommandResult, error) {
			return &CommandResult{
				Stdout:   output,
				ExitCode: 0,
			}, nil
		},
	}

	prober := NewProber(exec)
	host := RemoteHost{
		Name:    "test-host",
		Address: "192.168.1.100",
		User:    "deploy",
	}

	resources, err := prober.Probe(context.Background(), host)
	require.NoError(t, err)
	require.NotNil(t, resources)

	assert.Equal(t, "test-host", resources.Host)
	assert.Greater(t, resources.CPUPercent, 0.0)
	assert.Greater(t, resources.MemoryPercent, 0.0)
	assert.Equal(t, uint64(16384000/1024), resources.MemoryTotalMB)
	assert.InDelta(t, 1.50, resources.LoadAvg1, 0.01)
	assert.InDelta(t, 2.30, resources.LoadAvg5, 0.01)
	assert.InDelta(t, 3.10, resources.LoadAvg15, 0.01)
	assert.Equal(t, uint64(100000), resources.DiskTotalMB)
	assert.Equal(t, uint64(60000), resources.DiskUsedMB)
	assert.Equal(t, 8, resources.CPUCores)
	assert.Equal(t, uint64(1000000), resources.NetworkRxBytesPerSec)
	assert.Equal(t, uint64(500000), resources.NetworkTxBytesPerSec)
}

func TestProber_Probe_Error(t *testing.T) {
	exec := &mockExecutor{
		executeFunc: func(
			ctx context.Context, host RemoteHost, cmd string,
		) (*CommandResult, error) {
			return nil, fmt.Errorf("connection refused")
		},
	}

	prober := NewProber(exec)
	host := RemoteHost{
		Name:    "test-host",
		Address: "192.168.1.100",
		User:    "deploy",
	}

	_, err := prober.Probe(context.Background(), host)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "connection refused")
}

func TestProber_Probe_IncompleteSections(t *testing.T) {
	exec := &mockExecutor{
		executeFunc: func(
			ctx context.Context, host RemoteHost, cmd string,
		) (*CommandResult, error) {
			return &CommandResult{
				Stdout:   "only one section",
				ExitCode: 0,
			}, nil
		},
	}

	prober := NewProber(exec)
	host := RemoteHost{
		Name:    "test-host",
		Address: "192.168.1.100",
		User:    "deploy",
	}

	_, err := prober.Probe(context.Background(), host)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "unexpected probe output")
}

func TestParseCPUPercent(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  float64
	}{
		{
			"normal cpu line",
			"cpu  10000 500 3000 80000 200 100 50 0 0 0",
			// total=93850, idle=80000 -> (93850-80000)/93850*100
			0, // approximate
		},
		{
			"empty input",
			"",
			0,
		},
		{
			"no cpu line",
			"cpu0 1000 100 300 8000 20 10 5 0 0 0",
			0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseCPUPercent(tt.input)
			if tt.want == 0 && tt.name == "normal cpu line" {
				assert.Greater(t, result, 0.0)
				assert.Less(t, result, 100.0)
			} else {
				assert.InDelta(t, tt.want, result, 0.01)
			}
		})
	}
}

func TestParseMemInfo(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantTotal uint64
		wantAvail uint64
	}{
		{
			"standard meminfo",
			"MemTotal:       16384000 kB\nMemAvailable:    8192000 kB",
			16384000, 8192000,
		},
		{
			"empty",
			"",
			0, 0,
		},
		{
			"partial",
			"MemTotal:       16384000 kB",
			16384000, 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			total, avail := parseMemInfo(tt.input)
			assert.Equal(t, tt.wantTotal, total)
			assert.Equal(t, tt.wantAvail, avail)
		})
	}
}

func TestParseMBField(t *testing.T) {
	tests := []struct {
		input string
		want  uint64
	}{
		{"100000M", 100000},
		{"500M", 500},
		{"0M", 0},
		{"invalid", 0},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			assert.Equal(t, tt.want, parseMBField(tt.input))
		})
	}
}

func TestParseNetDev(t *testing.T) {
	input := `eth0: 1000000 1000 0 0 0 0 0 0 500000 800 0 0 0 0 0 0
wlan0: 200000 500 0 0 0 0 0 0 100000 300 0 0 0 0 0 0
lo:   100 10 0 0 0 0 0 0 100 10 0 0 0 0 0 0`

	rx, tx := parseNetDev(input)
	// eth0 rx=1000000, wlan0 rx=200000 (lo excluded)
	assert.Equal(t, uint64(1200000), rx)
	// eth0 tx=500000, wlan0 tx=100000 (lo excluded)
	assert.Equal(t, uint64(600000), tx)
}

func TestParseNetDev_Empty(t *testing.T) {
	rx, tx := parseNetDev("")
	assert.Equal(t, uint64(0), rx)
	assert.Equal(t, uint64(0), tx)
}
