# digital.vasic.containers

A generic, reusable Go module for container orchestration, health checking, lifecycle management, and service discovery. Supports Docker, Podman, and Kubernetes runtimes.

## Installation

```bash
go get digital.vasic.containers
```

## Quick Start

```go
package main

import (
    "context"
    "fmt"
    "log"

    "digital.vasic.containers/pkg/boot"
    "digital.vasic.containers/pkg/endpoint"
    "digital.vasic.containers/pkg/health"
    "digital.vasic.containers/pkg/logging"
    "digital.vasic.containers/pkg/runtime"
)

func main() {
    ctx := context.Background()

    // Auto-detect container runtime (Docker or Podman)
    rt, err := runtime.AutoDetect(ctx)
    if err != nil {
        log.Fatal(err)
    }
    fmt.Printf("Using runtime: %s\n", rt.Name())

    // Define service endpoints
    endpoints := map[string]endpoint.ServiceEndpoint{
        "postgres": endpoint.NewEndpoint().
            WithHost("localhost").WithPort("5432").
            WithHealthType("tcp").WithRequired(true).
            WithComposeFile("docker-compose.yml").
            WithServiceName("postgres").
            Build(),
        "redis": endpoint.NewEndpoint().
            WithHost("localhost").WithPort("6379").
            WithHealthType("tcp").WithRequired(true).
            WithComposeFile("docker-compose.yml").
            WithServiceName("redis").
            Build(),
    }

    // Boot all services
    mgr := boot.NewBootManager(endpoints,
        boot.WithRuntime(rt),
        boot.WithLogger(logging.NewSlogAdapter()),
    )

    summary, err := mgr.BootAll(ctx)
    if err != nil {
        log.Fatalf("Boot failed: %v", err)
    }

    fmt.Printf("Started: %d, Failed: %d\n",
        summary.Started, summary.Failed)
}
```

## Features

- **Multi-runtime support**: Docker, Podman, Kubernetes
- **Auto-detection**: Automatically finds available container runtime
- **Health checking**: TCP, HTTP, gRPC, and custom health checks with retry
- **Compose orchestration**: Batch operations grouped by compose file/profile
- **Lifecycle management**: Lazy boot, idle shutdown, concurrency semaphores
- **Resource monitoring**: System and per-container CPU/memory/disk
- **Event system**: Publish/subscribe for lifecycle events
- **Service discovery**: TCP port probe and DNS-based discovery
- **Prometheus metrics**: Built-in metrics collection
- **Pluggable logging**: Bring your own logger (slog adapter included)

## Architecture

```
boot.BootManager
├── compose.ComposeOrchestrator  (Docker Compose operations)
├── health.HealthChecker         (TCP/HTTP/gRPC checks)
├── discovery.Discoverer         (Service discovery)
├── event.EventBus               (Lifecycle events)
├── metrics.MetricsCollector     (Prometheus metrics)
└── logging.Logger               (Pluggable logging)

lifecycle.LifecycleManager
├── LazyBooter                   (Start on first Acquire)
├── IdleShutdown                 (Stop after inactivity)
└── ConcurrencySemaphore         (Limit parallel users)

runtime.ContainerRuntime
├── DockerRuntime
├── PodmanRuntime
└── KubernetesRuntime
```

## License

MIT
