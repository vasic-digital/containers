package ctop

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestDisplay_Render_NoColor exercises the render path with NoColor=true.
func TestDisplay_Render_NoColor(t *testing.T) {
	psOutput := `[{"Id":"render1","Names":["/render-test"],"Image":"nginx:latest","Created":"2024-01-01T00:00:00Z","State":{"Status":"running","StartedAt":"2024-01-01T00:00:00Z"}}]`
	statsOutput := `{"CPUPerc":"10.0%","MemUsage":"100MiB / 1GiB","MemPerc":"10.0%","NetIO":"1MiB / 1MiB","BlockIO":"10MiB / 10MiB","PIDs":"2"}`

	exec := &mockExecutor{
		responses: map[string][]byte{
			"podman ps":    []byte(psOutput),
			"podman stats": []byte(statsOutput),
		},
	}
	c := NewCollectorWithExecutor("podman", nil, exec)

	config := DefaultDisplayConfig()
	config.NoColor = true
	var buf bytes.Buffer
	d := NewDisplayWithWriter(c, config, &buf)

	d.render(context.Background())

	output := buf.String()
	assert.Contains(t, output, "render-test")
}

// TestDisplay_Render_NoContainers exercises the "no containers found" branch.
func TestDisplay_Render_NoContainers(t *testing.T) {
	exec := &mockExecutor{
		responses: map[string][]byte{
			"podman ps": []byte(`[]`),
		},
	}
	c := NewCollectorWithExecutor("podman", nil, exec)

	config := DefaultDisplayConfig()
	config.ShowStopped = true
	var buf bytes.Buffer
	d := NewDisplayWithWriter(c, config, &buf)

	d.render(context.Background())

	output := buf.String()
	assert.Contains(t, output, "No containers found")
}

// TestDisplay_Render_WithFilter exercises filter during render.
func TestDisplay_Render_WithFilter(t *testing.T) {
	psOutput := `[{"Id":"flt1","Names":["/web-server"],"Image":"nginx:latest","Created":"2024-01-01T00:00:00Z","State":{"Status":"running","StartedAt":"2024-01-01T00:00:00Z"}},{"Id":"flt2","Names":["/db-server"],"Image":"postgres","Created":"2024-01-01T00:00:00Z","State":{"Status":"running","StartedAt":"2024-01-01T00:00:00Z"}}]`
	statsOutput := `{"CPUPerc":"5.0%","MemUsage":"50MiB / 1GiB","MemPerc":"5.0%","NetIO":"0B / 0B","BlockIO":"0B / 0B","PIDs":"1"}`

	exec := &mockExecutor{
		responses: map[string][]byte{
			"podman ps":    []byte(psOutput),
			"podman stats": []byte(statsOutput),
		},
	}
	c := NewCollectorWithExecutor("podman", nil, exec)

	var buf bytes.Buffer
	d := NewDisplayWithWriter(c, DefaultDisplayConfig(), &buf)
	d.SetFilterName("web")

	d.render(context.Background())

	output := buf.String()
	assert.Contains(t, output, "web-server")
}
