package remote

import (
	"context"
	"fmt"
	"strings"
)

// probeNvidiaCmd is the nvidia-smi query used by ProbeGPU. Exported
// for tests so they can stub the same key in a fake executor.
const probeNvidiaCmd = "nvidia-smi --query-gpu=index,name,driver_version,memory.total,memory.free,utilization.gpu,compute_cap --format=csv,noheader,nounits 2>/dev/null || true"

// probeRocmCmd runs rocm-smi in its default text mode.
const probeRocmCmd = "rocm-smi --showproductname --showmeminfo vram 2>/dev/null || true"

// probeRuntimeCmd detects whether docker has the nvidia runtime
// registered. Caller runs this only if at least one NVIDIA GPU was
// found.
const probeRuntimeCmd = "docker info --format '{{.Runtimes}}' 2>/dev/null || true"

// ProbeGPU runs a small set of read-only probe commands over SSH
// and returns the host's GPU inventory. Works without sudo.
//
// The function tolerates any single probe failing: an nvidia-smi
// failure does not abort the rocm-smi probe, and vice versa. A host
// with no probe tools installed returns an empty slice + nil error.
func ProbeGPU(ctx context.Context, exec RemoteExecutor, host RemoteHost) ([]GPUDevice, error) {
	if exec == nil {
		return nil, fmt.Errorf("probe_gpu: executor is nil")
	}

	var out []GPUDevice

	if r, err := exec.Execute(ctx, host, probeNvidiaCmd); err == nil && r.ExitCode == 0 && strings.TrimSpace(r.Stdout) != "" {
		devs, perr := ParseNvidiaSmi(r.Stdout)
		if perr != nil {
			return nil, fmt.Errorf("probe_gpu: parse nvidia-smi: %w", perr)
		}
		out = append(out, devs...)
	}

	if r, err := exec.Execute(ctx, host, probeRocmCmd); err == nil && r.ExitCode == 0 && strings.TrimSpace(r.Stdout) != "" {
		devs, perr := ParseRocmSmi(r.Stdout)
		if perr != nil {
			return nil, fmt.Errorf("probe_gpu: parse rocm-smi: %w", perr)
		}
		out = append(out, devs...)
	}

	// If we saw NVIDIA GPUs, probe for nvidia docker runtime.
	hasNvidia := false
	for _, d := range out {
		if d.Vendor == "nvidia" {
			hasNvidia = true
			break
		}
	}
	if hasNvidia {
		if r, err := exec.Execute(ctx, host, probeRuntimeCmd); err == nil && r.ExitCode == 0 {
			if strings.Contains(strings.ToLower(r.Stdout), "nvidia") {
				for i := range out {
					if out[i].Vendor == "nvidia" {
						out[i].NVIDIARuntime = true
					}
				}
			}
		}
	}

	return out, nil
}
