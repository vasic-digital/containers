# CLAUDE.md - Containers Module

## Overview

`digital.vasic.containers` is a generic, reusable Go module for container orchestration, health checking, lifecycle management, and service discovery. It supports Docker, Podman, and Kubernetes runtimes.

**Module**: `digital.vasic.containers` (Go 1.24+)

## Build & Test

```bash
go build ./...
go test ./... -count=1 -race
go test ./... -short              # Unit tests only
go test -tags=integration ./...   # Integration tests (requires Docker)
go test -bench=. ./tests/benchmark/
```

## Code Style

- Standard Go conventions, `gofmt` formatting
- Imports grouped: stdlib, third-party, internal (blank line separated)
- Line length ≤ 100 chars
- Naming: `camelCase` private, `PascalCase` exported, acronyms all-caps
- Errors: always check, wrap with `fmt.Errorf("...: %w", err)`
- Tests: table-driven, `testify`, naming `Test<Struct>_<Method>_<Scenario>`

## Package Structure

| Package | Purpose |
|---------|---------|
| `pkg/runtime` | Container runtime abstraction (Docker/Podman/K8s) |
| `pkg/compose` | Docker Compose orchestration |
| `pkg/health` | Health checking (TCP/HTTP/gRPC/Custom) |
| `pkg/endpoint` | Service endpoint configuration |
| `pkg/lifecycle` | Advanced lifecycle (lazy boot, idle shutdown, semaphores) |
| `pkg/monitor` | Resource monitoring (CPU/memory/disk) |
| `pkg/event` | Event bus for lifecycle hooks |
| `pkg/discovery` | Service discovery (TCP/DNS) |
| `pkg/logging` | Logging abstraction (bring your own) |
| `pkg/metrics` | Prometheus-compatible metrics |
| `pkg/boot` | High-level BootManager composing everything |

## Key Interfaces

- `runtime.ContainerRuntime` — Container operations
- `compose.ComposeOrchestrator` — Compose file operations
- `health.HealthChecker` — Health check dispatch
- `lifecycle.LifecycleManager` — Service lifecycle with lazy boot
- `monitor.ResourceMonitor` — System/container resource monitoring
- `event.EventBus` — Publish/subscribe for lifecycle events
- `discovery.Discoverer` — Service discovery
- `logging.Logger` — Logging abstraction
- `metrics.MetricsCollector` — Metrics collection

## Design Patterns

- **Strategy**: ContainerRuntime (Docker/Podman/K8s), HealthChecker (TCP/HTTP/gRPC)
- **Observer**: EventBus for lifecycle events
- **Factory**: `runtime.AutoDetect()`, `health.NewDefaultChecker()`
- **Builder**: `endpoint.NewEndpoint().WithHost().WithPort().Build()`
- **Decorator**: RetryPolicy wraps HealthChecker
- **Functional Options**: `boot.WithRuntime()`, `boot.WithLogger()`, etc.

## Commit Style

Conventional Commits: `feat(runtime): add Kubernetes support`
