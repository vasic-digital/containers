# GPU-Aware Scheduling

Added 2026-04-17 as part of the OpenClaw Ultimate (OCU) foundation wave.

## What's new

- `remote.HostResources.GPU []GPUDevice` — per-host inventory populated by
  `ProbeGPU` or from env labels.
- `scheduler.ContainerRequirements.GPU *GPURequirement` — optional; nil
  preserves all prior behaviour.
- `scheduler.StrategyGPUAffinity` — new placement strategy that only
  considers GPU-bearing hosts.
- `health.GPUHealthCheck` — VRAM-floor probe.
- `remote.ProbeGPU` — read-only SSH probe (nvidia-smi + rocm-smi +
  docker nvidia runtime); no sudo.

## Thinker.local example

```bash
# .env
CONTAINERS_REMOTE_ENABLED=true
CONTAINERS_REMOTE_HOST_1_NAME=thinker
CONTAINERS_REMOTE_HOST_1_ADDRESS=thinker.local
CONTAINERS_REMOTE_HOST_1_USER=milosvasic
CONTAINERS_REMOTE_HOST_1_LABELS=gpu=true,gpu_vendor=nvidia,gpu_model=rtx3060,cuda=12.2,nvenc=true,vulkan=true
CONTAINERS_REMOTE_HOST_1_GPU_AUTOPROBE=true
```

```go
req := scheduler.ContainerRequirements{
    Name:  "cuda-opencv",
    Image: "ghcr.io/vasic-digital/ocu-cuda-sidecar:latest",
    GPU: &scheduler.GPURequirement{
        Count:        1,
        MinVRAMMB:    2048,
        Vendor:       "nvidia",
        MinCompute:   "8.0",
        Capabilities: []string{"cuda", "nvenc"},
    },
}

sched := scheduler.NewScheduler(hm, logger,
    scheduler.WithStrategy(scheduler.StrategyGPUAffinity))
dist := distribution.NewDistributor(
    distribution.WithScheduler(sched),
    distribution.WithExecutor(sshExec),
    distribution.WithHostManager(hm),
    distribution.WithLocalRuntime(localRT),
)
summary, err := dist.Distribute(ctx, []scheduler.ContainerRequirements{req})
```

## Backward compatibility

- `HostResources.GPU == nil` behaves identically to before.
- `ContainerRequirements.GPU == nil` means "no GPU needed"; existing
  strategies (`resource_aware`, `round_robin`, `affinity`, `spread`,
  `bin_pack`) score such requirements exactly as today.
- `Options.GPUWeight == 0` (default) means GPU score contributes 0 to
  total — i.e. the new code is inert until callers opt in.
