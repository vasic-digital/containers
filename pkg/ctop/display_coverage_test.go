package ctop

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDisplay_Stop_WithoutRunning(t *testing.T) {
	exec := &mockExecutor{responses: map[string][]byte{}}
	c := NewCollectorWithExecutor("podman", nil, exec)
	var buf bytes.Buffer
	d := NewDisplayWithWriter(c, DefaultDisplayConfig(), &buf)
	// Should not panic when cancel is nil
	d.Stop()
}

func TestDisplay_SetSortBy(t *testing.T) {
	exec := &mockExecutor{responses: map[string][]byte{}}
	c := NewCollectorWithExecutor("podman", nil, exec)
	var buf bytes.Buffer
	d := NewDisplayWithWriter(c, DefaultDisplayConfig(), &buf)
	d.SetSortBy(SortByMemory)
	assert.Equal(t, SortByMemory, d.sortBy)
}

func TestDisplay_SetSortOrder(t *testing.T) {
	exec := &mockExecutor{responses: map[string][]byte{}}
	c := NewCollectorWithExecutor("podman", nil, exec)
	var buf bytes.Buffer
	d := NewDisplayWithWriter(c, DefaultDisplayConfig(), &buf)
	d.SetSortOrder(SortAsc)
	assert.Equal(t, SortAsc, d.sortOrder)
}

func TestDisplay_SetFilterHost(t *testing.T) {
	exec := &mockExecutor{responses: map[string][]byte{}}
	c := NewCollectorWithExecutor("podman", nil, exec)
	var buf bytes.Buffer
	d := NewDisplayWithWriter(c, DefaultDisplayConfig(), &buf)
	d.SetFilterHost("gpu-server")
	assert.Equal(t, "gpu-server", d.filterHost)
}

func TestDisplay_SetFilterName(t *testing.T) {
	exec := &mockExecutor{responses: map[string][]byte{}}
	c := NewCollectorWithExecutor("podman", nil, exec)
	var buf bytes.Buffer
	d := NewDisplayWithWriter(c, DefaultDisplayConfig(), &buf)
	d.SetFilterName("nginx")
	assert.Equal(t, "nginx", d.filterName)
}

func TestDisplay_ToggleSortOrder_DescToAsc(t *testing.T) {
	exec := &mockExecutor{responses: map[string][]byte{}}
	c := NewCollectorWithExecutor("podman", nil, exec)
	var buf bytes.Buffer
	d := NewDisplayWithWriter(c, DefaultDisplayConfig(), &buf)
	// Default is SortDesc
	assert.Equal(t, SortDesc, d.sortOrder)
	d.ToggleSortOrder()
	assert.Equal(t, SortAsc, d.sortOrder)
}

func TestDisplay_ToggleSortOrder_AscToDesc(t *testing.T) {
	exec := &mockExecutor{responses: map[string][]byte{}}
	c := NewCollectorWithExecutor("podman", nil, exec)
	config := DefaultDisplayConfig()
	var buf bytes.Buffer
	d := NewDisplayWithWriter(c, config, &buf)
	d.SetSortOrder(SortAsc)
	d.ToggleSortOrder()
	assert.Equal(t, SortDesc, d.sortOrder)
}

func TestDisplay_StateColor(t *testing.T) {
	exec := &mockExecutor{responses: map[string][]byte{}}
	c := NewCollectorWithExecutor("podman", nil, exec)
	var buf bytes.Buffer
	d := NewDisplayWithWriter(c, DefaultDisplayConfig(), &buf)

	assert.Equal(t, colorGreen, d.stateColor("running"))
	assert.Equal(t, colorRed, d.stateColor("exited"))
	assert.Equal(t, colorRed, d.stateColor("stopped"))
	assert.Equal(t, colorYellow, d.stateColor("paused"))
	assert.Equal(t, colorCyan, d.stateColor("restarting"))
	assert.Equal(t, colorWhite, d.stateColor("unknown"))
}

func TestDisplay_HostColor(t *testing.T) {
	exec := &mockExecutor{responses: map[string][]byte{}}
	c := NewCollectorWithExecutor("podman", nil, exec)
	var buf bytes.Buffer
	d := NewDisplayWithWriter(c, DefaultDisplayConfig(), &buf)

	assert.Equal(t, colorGreen, d.hostColor("local"))
	assert.Equal(t, colorPurple, d.hostColor("remote:host1"))
	assert.Equal(t, colorCyan, d.hostColor("other-host"))
}

func TestDisplay_CPUColor(t *testing.T) {
	exec := &mockExecutor{responses: map[string][]byte{}}
	c := NewCollectorWithExecutor("podman", nil, exec)
	var buf bytes.Buffer
	d := NewDisplayWithWriter(c, DefaultDisplayConfig(), &buf)

	assert.Equal(t, colorRed, d.cpuColor(90.0))
	assert.Equal(t, colorYellow, d.cpuColor(60.0))
	assert.Equal(t, colorGreen, d.cpuColor(30.0))
}

func TestDisplay_MemColor(t *testing.T) {
	exec := &mockExecutor{responses: map[string][]byte{}}
	c := NewCollectorWithExecutor("podman", nil, exec)
	var buf bytes.Buffer
	d := NewDisplayWithWriter(c, DefaultDisplayConfig(), &buf)

	assert.Equal(t, colorRed, d.memColor(85.0))
	assert.Equal(t, colorYellow, d.memColor(55.0))
	assert.Equal(t, colorGreen, d.memColor(20.0))
}

func TestDisplay_Colorize(t *testing.T) {
	exec := &mockExecutor{responses: map[string][]byte{}}
	c := NewCollectorWithExecutor("podman", nil, exec)

	// With color enabled
	var buf bytes.Buffer
	config := DefaultDisplayConfig()
	config.NoColor = false
	d := NewDisplayWithWriter(c, config, &buf)
	result := d.colorize(colorGreen, "hello")
	assert.Contains(t, result, "hello")
	assert.Contains(t, result, colorGreen)

	// With color disabled
	config2 := DefaultDisplayConfig()
	config2.NoColor = true
	d2 := NewDisplayWithWriter(c, config2, &buf)
	result2 := d2.colorize(colorGreen, "hello")
	assert.Equal(t, "hello", result2)
}

func TestDisplay_ClearScreen(t *testing.T) {
	exec := &mockExecutor{responses: map[string][]byte{}}
	c := NewCollectorWithExecutor("podman", nil, exec)
	var buf bytes.Buffer
	d := NewDisplayWithWriter(c, DefaultDisplayConfig(), &buf)
	d.clearScreen()
	assert.Contains(t, buf.String(), "\033[2J")
}

func TestDisplay_MoveCursor(t *testing.T) {
	exec := &mockExecutor{responses: map[string][]byte{}}
	c := NewCollectorWithExecutor("podman", nil, exec)
	var buf bytes.Buffer
	d := NewDisplayWithWriter(c, DefaultDisplayConfig(), &buf)
	d.moveCursor(5, 10)
	assert.Contains(t, buf.String(), "\033[5;10H")
}

func TestDisplay_HideShowCursor(t *testing.T) {
	exec := &mockExecutor{responses: map[string][]byte{}}
	c := NewCollectorWithExecutor("podman", nil, exec)
	var buf bytes.Buffer
	d := NewDisplayWithWriter(c, DefaultDisplayConfig(), &buf)
	d.hideCursor()
	assert.Contains(t, buf.String(), "\033[?25l")
	buf.Reset()
	d.showCursor()
	assert.Contains(t, buf.String(), "\033[?25h")
}

func TestDisplay_UpdateSize(t *testing.T) {
	exec := &mockExecutor{responses: map[string][]byte{}}
	c := NewCollectorWithExecutor("podman", nil, exec)
	var buf bytes.Buffer
	d := NewDisplayWithWriter(c, DefaultDisplayConfig(), &buf)
	// Should not panic (will use fallback 80x24 since output is a buffer not a terminal)
	d.updateSize()
	assert.GreaterOrEqual(t, d.width, 0)
	assert.GreaterOrEqual(t, d.height, 0)
}

func TestTruncate(t *testing.T) {
	assert.Equal(t, "hello world", truncate("hello world", 20))
	assert.Equal(t, "he...", truncate("hello world", 5))
	assert.Equal(t, "hello", truncate("hello", 5))
}

func TestFormatBytes(t *testing.T) {
	assert.Equal(t, "512B", formatBytes(512))
	assert.Equal(t, "1.0KB", formatBytes(1024))
	assert.Equal(t, "1.0MB", formatBytes(1024*1024))
	assert.Equal(t, "1.0GB", formatBytes(1024*1024*1024))
}

func TestNewCollectorWithSSH(t *testing.T) {
	c := NewCollectorWithSSH("podman", nil, nil)
	assert.NotNil(t, c)
	assert.Equal(t, "podman", c.runtime)
}
