# AGENTS.md - Containers Module

## Module Overview

`digital.vasic.containers` is a generic, reusable Go module for container orchestration, health checking, lifecycle management, and service discovery. It provides a unified interface for Docker, Podman, and Kubernetes runtimes with advanced features like lazy booting, idle shutdown, semaphore-based control, and resource monitoring.

**Module path**: `digital.vasic.containers`
**Go version**: 1.24+
**Dependencies**: Container runtime clients (Docker/Podman/K8s), Prometheus client, minimal external dependencies

## Package Responsibilities

| Package | Path | Responsibility |
|---------|------|----------------|
| `runtime` | `pkg/runtime/` | Container runtime abstraction layer: `ContainerRuntime` interface with implementations for Docker, Podman, Kubernetes. Auto-detection of available runtime. Runtime-agnostic container operations (start, stop, inspect, remove). |
| `compose` | `pkg/compose/` | Docker Compose orchestration: `ComposeOrchestrator` interface for compose file operations (up, down, restart). Support for profiles and service filtering. Multi-file composition and variable substitution. |
| `health` | `pkg/health/` | Health checking dispatcher: Multiple strategies (TCP, HTTP, gRPC, custom script). Retry policies with exponential backoff. Configurable timeouts and thresholds. Health status aggregation. |
| `endpoint` | `pkg/endpoint/` | Service endpoint configuration: `Endpoint` struct (host, port, path, scheme). Builder pattern for endpoint construction. Endpoint validation and URL generation. |
| `lifecycle` | `pkg/lifecycle/` | Advanced lifecycle management: Lazy boot (on-demand container startup). Idle shutdown (resource optimization). Semaphore-based parallelism control. Graceful shutdown sequences. |
| `monitor` | `pkg/monitor/` | Resource monitoring: System resource tracking (CPU, memory, disk). Per-container resource usage. Threshold-based alerting. Prometheus metrics export. |
| `event` | `pkg/event/` | Lifecycle event bus: Publish/subscribe for container lifecycle events. Event types: `ContainerStarted`, `ContainerStopped`, `HealthCheckFailed`, etc. Hook system for custom actions. |
| `discovery` | `pkg/discovery/` | Service discovery: TCP port scanning for service detection. DNS-based discovery. mDNS support for local network. Multi-strategy discovery with fallback. |
| `logging` | `pkg/logging/` | Logging abstraction: Bring-your-own-logger interface. Adapters for popular loggers (logrus, zap, zerolog). Structured logging support. |
| `metrics` | `pkg/metrics/` | Metrics collection: Prometheus-compatible metrics. Container lifecycle metrics. Health check metrics. Resource utilization metrics. |
| `boot` | `pkg/boot/` | High-level orchestration: `BootManager` composing all packages. One-line service initialization. Coordinated health checking and lifecycle management. Configuration validation. |

## Dependency Graph

```
boot  --->  runtime
boot  --->  compose  --->  runtime
boot  --->  health  --->  endpoint
boot  --->  lifecycle  --->  runtime, event
boot  --->  monitor  --->  runtime
boot  --->  discovery  --->  endpoint
boot  --->  event
boot  --->  logging
boot  --->  metrics
```

`runtime` and `endpoint` are leaf packages. `boot` is the integration layer depending on all other packages.

## Key Files

| File | Purpose |
|------|---------|
| `pkg/runtime/runtime.go` | ContainerRuntime interface and implementations |
| `pkg/runtime/docker.go` | Docker client implementation |
| `pkg/runtime/podman.go` | Podman client implementation |
| `pkg/runtime/kubernetes.go` | Kubernetes client implementation |
| `pkg/runtime/autodetect.go` | Runtime auto-detection logic |
| `pkg/compose/compose.go` | ComposeOrchestrator interface |
| `pkg/compose/docker_compose.go` | Docker Compose implementation |
| `pkg/health/health.go` | HealthChecker interface and dispatcher |
| `pkg/health/tcp.go` | TCP health check implementation |
| `pkg/health/http.go` | HTTP health check implementation |
| `pkg/health/grpc.go` | gRPC health check implementation |
| `pkg/lifecycle/lifecycle.go` | LifecycleManager interface |
| `pkg/lifecycle/lazy_boot.go` | Lazy boot implementation |
| `pkg/lifecycle/idle_shutdown.go` | Idle shutdown implementation |
| `pkg/boot/boot_manager.go` | BootManager main orchestration logic |
| `pkg/boot/config.go` | Configuration structures and validation |
| `go.mod` | Module definition and dependencies |
| `CLAUDE.md` | AI coding assistant instructions |
| `README.md` | User-facing documentation with quick start |

## Agent Coordination Guide

### Division of Work

When multiple agents work on this module simultaneously, divide work by package boundary:

1. **Runtime Agent** -- Owns `pkg/runtime/`. Changes to runtime interface affect compose, lifecycle, and monitor packages. Must coordinate before modifying `ContainerRuntime` interface.
2. **Health Agent** -- Owns `pkg/health/`. New health check strategies can be added independently. Changes to `HealthChecker` interface require boot package updates.
3. **Lifecycle Agent** -- Owns `pkg/lifecycle/`. Complex lifecycle logic. Coordinates with runtime and event agents for state management.
4. **Boot Agent** -- Owns `pkg/boot/`. Integration layer. Requires testing against all package combinations.
5. **Discovery Agent** -- Owns `pkg/discovery/`. Independent service discovery logic. Can work in parallel with other agents.
6. **Monitor Agent** -- Owns `pkg/monitor/`. Resource tracking. Can work independently but coordinates with runtime for container metrics.

### Coordination Rules

- **Runtime interface changes** require all agents to update. The `ContainerRuntime` interface is the shared contract.
- **Health checker** and **discovery** packages are independent and can be modified in parallel.
- **Boot package** integrates all packages. Any interface change in sub-packages requires corresponding boot updates.
- **Lifecycle** and **event** packages are tightly coupled. Coordinate changes to event types and lifecycle states.
- **Test isolation**: Each package has its own `_test.go` files. Boot tests import all packages for integration scenarios.
- **No circular dependencies**: The dependency graph is strictly acyclic. Never import `boot` from sub-packages.

### Safe Parallel Changes

These changes can be made simultaneously without coordination:
- Adding a new runtime implementation (e.g., LXC) to `pkg/runtime/`
- Adding a new health check strategy to `pkg/health/`
- Adding new discovery mechanisms to `pkg/discovery/`
- Adding new event types to `pkg/event/`
- Adding new tests to any package
- Updating documentation

### Changes Requiring Coordination

- Modifying the `ContainerRuntime` interface
- Changing `HealthChecker` interface signature
- Modifying lifecycle state machine
- Adding new configuration fields to `boot.Config`
- Changing event types used across packages
- Modifying metrics schema

## Build and Test Commands

```bash
# Build all packages
go build ./...

# Run all tests with race detection
go test ./... -count=1 -race

# Run unit tests only (short mode)
go test ./... -short

# Run integration tests (requires Docker/Podman)
go test -tags=integration ./...

# Run benchmarks
go test -bench=. ./tests/benchmark/

# Run a specific test
go test -v -run TestBootManager_Start ./pkg/boot/

# Format code
gofmt -w .

# Vet code
go vet ./...
```

## Commit Conventions

Follow Conventional Commits with package scope:

```
feat(runtime): add LXC runtime support
feat(health): add Redis health check strategy
feat(lifecycle): implement graceful shutdown with timeout
fix(boot): prevent race condition in parallel health checks
fix(compose): handle profile selection correctly
test(runtime): add Docker client integration tests
docs(containers): update API reference with lifecycle examples
refactor(health): extract retry logic to separate package
```

## Thread Safety Notes

- **BootManager** is fully thread-safe. All public methods use `sync.RWMutex` for state protection.
- **Runtime implementations** use per-client locking for API calls.
- **HealthChecker** executes health checks concurrently but uses mutexes for result aggregation.
- **LifecycleManager** uses atomic operations for state transitions.
- **EventBus** (from `pkg/event/`) is thread-safe with internal locking.
- **MetricsCollector** uses `sync.Map` for concurrent metric updates.

## Configuration Example

```go
package main

import (
    "digital.vasic.containers/pkg/boot"
    "digital.vasic.containers/pkg/runtime"
    "digital.vasic.containers/pkg/logging"
)

func main() {
    // Auto-detect runtime
    rt, _ := runtime.AutoDetect()

    // Create logger
    logger := logging.NewDefaultLogger()

    // Create boot manager with functional options
    manager := boot.NewBootManager(
        boot.WithRuntime(rt),
        boot.WithLogger(logger),
        boot.WithHealthCheckRetries(3),
        boot.WithParallelStartup(true),
        boot.WithLazyBoot(true),
    )

    // Add services
    manager.AddService("postgresql", boot.ServiceConfig{
        ComposeFile: "docker-compose.yml",
        ServiceName: "postgres",
        HealthCheck: boot.TCPCheck("localhost", 5432),
        Required:    true,
    })

    // Start all services
    manager.Start(ctx)
}
```

## Runtime Detection Logic

```go
// AutoDetect tries: Docker -> Podman -> Kubernetes (in that order)
func AutoDetect() (ContainerRuntime, error) {
    // 1. Try Docker
    if dockerAvailable() {
        return NewDockerRuntime()
    }

    // 2. Try Podman
    if podmanAvailable() {
        return NewPodmanRuntime()
    }

    // 3. Try Kubernetes
    if kubernetesAvailable() {
        return NewKubernetesRuntime()
    }

    return nil, ErrNoRuntimeAvailable
}
```

## Health Check Strategies

| Strategy | Use Case | Configuration |
|----------|----------|---------------|
| TCP | Database, cache, message queue | Host, port, timeout |
| HTTP | REST APIs, web services | URL, expected status code, timeout |
| gRPC | gRPC services with health check protocol | Host, port, service name |
| Custom | Custom health logic | Script path or function |

## Lifecycle States

```
UNSTARTED -> STARTING -> STARTED -> STOPPING -> STOPPED
                  |                     |
                  +---> FAILED <--------+
```

## Best Practices

### 1. Always Use Auto-Detection
```go
// Good
runtime, err := runtime.AutoDetect()

// Bad
runtime := runtime.NewDockerRuntime()  // Hardcoded
```

### 2. Configure Health Checks for All Services
```go
// Good
manager.AddService("redis", boot.ServiceConfig{
    HealthCheck: boot.TCPCheck("localhost", 6379),
})

// Bad - no health check
manager.AddService("redis", boot.ServiceConfig{})
```

### 3. Mark Critical Services as Required
```go
// Database is critical
manager.AddService("postgres", boot.ServiceConfig{
    Required: true,  // Fail fast if unavailable
})

// Optional service
manager.AddService("optional-cache", boot.ServiceConfig{
    Required: false,  // Continue if unavailable
})
```

### 4. Use Lazy Boot for Optional Services
```go
manager := boot.NewBootManager(
    boot.WithLazyBoot(true),  // Start services on-demand
)
```

### 5. Monitor Resource Usage
```go
monitor := monitor.NewResourceMonitor(runtime)
metrics := monitor.GetContainerMetrics("postgres")
if metrics.MemoryPercent > 90.0 {
    logger.Warn("High memory usage detected")
}
```

---

**Last Updated**: February 10, 2026
**Version**: 1.0.0
**Status**: ✅ Production Ready
