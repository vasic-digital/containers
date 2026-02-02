package runtime

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// PodmanRuntime implements ContainerRuntime using the podman CLI.
// Podman is largely command-compatible with Docker, so this
// implementation follows the same patterns with podman-specific
// adjustments.
type PodmanRuntime struct {
	binary   string
	executor CommandExecutor
}

// NewPodmanRuntime creates a PodmanRuntime with the real podman binary.
func NewPodmanRuntime() *PodmanRuntime {
	return &PodmanRuntime{
		binary:   "podman",
		executor: &defaultExecutor{},
	}
}

// NewPodmanRuntimeWithExecutor creates a PodmanRuntime with a custom
// executor, primarily for testing.
func NewPodmanRuntimeWithExecutor(exec CommandExecutor) *PodmanRuntime {
	return &PodmanRuntime{
		binary:   "podman",
		executor: exec,
	}
}

func (p *PodmanRuntime) Name() string {
	return "podman"
}

func (p *PodmanRuntime) Version(
	ctx context.Context,
) (string, error) {
	out, err := p.executor.Execute(
		ctx, p.binary, "version", "--format", "{{.Server.Version}}",
	)
	if err != nil {
		// Podman rootless may only have client version.
		out, err = p.executor.Execute(
			ctx, p.binary, "version", "--format",
			"{{.Client.Version}}",
		)
		if err != nil {
			return "", fmt.Errorf("podman version: %w", err)
		}
	}
	return strings.TrimSpace(string(out)), nil
}

func (p *PodmanRuntime) IsAvailable(ctx context.Context) bool {
	_, err := p.executor.Execute(ctx, p.binary, "info")
	return err == nil
}

func (p *PodmanRuntime) Start(
	ctx context.Context, id string, opts ...StartOption,
) error {
	_ = applyStartOptions(opts)
	args := []string{"start", id}
	_, err := p.executor.Execute(ctx, p.binary, args...)
	if err != nil {
		return fmt.Errorf("podman start %s: %w", id, err)
	}
	return nil
}

func (p *PodmanRuntime) Stop(
	ctx context.Context, id string, opts ...StopOption,
) error {
	o := applyStopOptions(opts)
	timeoutSec := int(o.Timeout.Seconds())
	args := []string{"stop", "-t", strconv.Itoa(timeoutSec), id}
	_, err := p.executor.Execute(ctx, p.binary, args...)
	if err != nil {
		return fmt.Errorf("podman stop %s: %w", id, err)
	}
	return nil
}

func (p *PodmanRuntime) Remove(
	ctx context.Context, id string, opts ...RemoveOption,
) error {
	o := applyRemoveOptions(opts)
	args := []string{"rm"}
	if o.Force {
		args = append(args, "-f")
	}
	if o.Volumes {
		args = append(args, "-v")
	}
	args = append(args, id)
	_, err := p.executor.Execute(ctx, p.binary, args...)
	if err != nil {
		return fmt.Errorf("podman rm %s: %w", id, err)
	}
	return nil
}

func (p *PodmanRuntime) Status(
	ctx context.Context, id string,
) (*ContainerStatus, error) {
	out, err := p.executor.Execute(
		ctx, p.binary, "inspect", "--format", "json", id,
	)
	if err != nil {
		return nil, fmt.Errorf("podman inspect %s: %w", id, err)
	}
	return parseDockerInspectStatus(out)
}

func (p *PodmanRuntime) List(
	ctx context.Context, filter ListFilter,
) ([]ContainerInfo, error) {
	args := []string{"ps", "--format", "json", "--no-trunc"}
	if filter.All {
		args = append(args, "-a")
	}
	for k, v := range filter.Labels {
		args = append(args,
			"--filter", fmt.Sprintf("label=%s=%s", k, v),
		)
	}
	for _, name := range filter.Names {
		args = append(args, "--filter", fmt.Sprintf("name=%s", name))
	}
	for _, status := range filter.Status {
		args = append(args,
			"--filter", fmt.Sprintf("status=%s", status),
		)
	}

	out, err := p.executor.Execute(ctx, p.binary, args...)
	if err != nil {
		return nil, fmt.Errorf("podman ps: %w", err)
	}
	return parsePodmanPSOutput(out)
}

// podmanPSJSON models podman ps --format json output.
// Podman returns a JSON array unlike Docker's newline-delimited JSON.
type podmanPSJSON struct {
	ID      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	ImageID string            `json:"ImageID"`
	State   string            `json:"State"`
	Status  string            `json:"Status"`
	Created interface{}       `json:"Created"`
	Labels  map[string]string `json:"Labels"`
}

func parsePodmanPSOutput(data []byte) ([]ContainerInfo, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" || trimmed == "[]" {
		return nil, nil
	}

	var items []podmanPSJSON
	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		return nil, fmt.Errorf("parsing podman ps output: %w", err)
	}

	var containers []ContainerInfo
	for _, item := range items {
		name := ""
		if len(item.Names) > 0 {
			name = item.Names[0]
		}
		labels := item.Labels
		if labels == nil {
			labels = make(map[string]string)
		}
		containers = append(containers, ContainerInfo{
			ID:      item.ID,
			Name:    name,
			Image:   item.Image,
			ImageID: item.ImageID,
			State:   mapContainerState(item.State),
			Status:  item.Status,
			Labels:  labels,
		})
	}
	return containers, nil
}

func (p *PodmanRuntime) Stats(
	ctx context.Context, id string,
) (*ContainerStats, error) {
	out, err := p.executor.Execute(
		ctx, p.binary, "stats", "--no-stream", "--format", "json", id,
	)
	if err != nil {
		return nil, fmt.Errorf("podman stats %s: %w", id, err)
	}
	return parsePodmanStats(out)
}

// podmanStatsJSON models podman stats --format json output.
type podmanStatsJSON struct {
	CPUPercent float64 `json:"cpu_percent"`
	MemPerc   float64 `json:"mem_percent"`
	MemUsage  uint64  `json:"mem_usage"`
	MemLimit  uint64  `json:"mem_limit"`
	NetInput  uint64  `json:"net_input"`
	NetOutput uint64  `json:"net_output"`
	BlockIn   uint64  `json:"block_input"`
	BlockOut  uint64  `json:"block_output"`
	PIDs      int     `json:"pids"`
}

func parsePodmanStats(data []byte) (*ContainerStats, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, fmt.Errorf("empty stats output")
	}

	var items []podmanStatsJSON
	if err := json.Unmarshal([]byte(trimmed), &items); err != nil {
		// Fall back to docker-style string parsing.
		return parseDockerStats(data)
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("no stats data returned")
	}

	s := items[0]
	return &ContainerStats{
		CPUPercent:    s.CPUPercent,
		MemoryPercent: s.MemPerc,
		MemoryUsage:   s.MemUsage,
		MemoryLimit:   s.MemLimit,
		NetworkRx:     s.NetInput,
		NetworkTx:     s.NetOutput,
		BlockRead:     s.BlockIn,
		BlockWrite:    s.BlockOut,
		PIDs:          s.PIDs,
	}, nil
}

func (p *PodmanRuntime) Exec(
	ctx context.Context, id string, cmd []string,
) (*ExecResult, error) {
	args := append([]string{"exec", id}, cmd...)
	stdout, stderr, exitCode, err := p.executor.ExecuteWithStderr(
		ctx, p.binary, args...,
	)
	if err != nil {
		return nil, fmt.Errorf("podman exec %s: %w", id, err)
	}
	return &ExecResult{
		ExitCode: exitCode,
		Stdout:   string(stdout),
		Stderr:   string(stderr),
	}, nil
}

func (p *PodmanRuntime) Logs(
	ctx context.Context, id string, opts ...LogOption,
) (io.ReadCloser, error) {
	o := applyLogOptions(opts)
	args := []string{"logs"}
	if o.Follow {
		args = append(args, "-f")
	}
	if o.Since != "" {
		args = append(args, "--since", o.Since)
	}
	if o.Until != "" {
		args = append(args, "--until", o.Until)
	}
	if o.Tail != "" {
		args = append(args, "--tail", o.Tail)
	}
	args = append(args, id)

	rc, err := p.executor.ExecuteStream(ctx, p.binary, args...)
	if err != nil {
		return nil, fmt.Errorf("podman logs %s: %w", id, err)
	}
	return rc, nil
}
