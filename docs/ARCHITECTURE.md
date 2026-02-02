# Containers Module - Architecture

## Design Philosophy

The Containers module provides a **generic, runtime-agnostic** abstraction for container orchestration. It is designed to be used as a library by any Go application that needs to manage containerized services.

## Package Dependency Graph

```
boot.BootManager (top-level orchestrator)
│
├── compose.ComposeOrchestrator
│   └── internal/exec (shell commands)
│
├── health.HealthChecker
│   ├── health.TCPCheck
│   ├── health.HTTPCheck
│   ├── health.GRPCCheck
│   └── health.CustomCheck
│
├── discovery.Discoverer
│   ├── discovery.TCPDiscoverer
│   └── discovery.DNSDiscoverer
│
├── event.EventBus
│
├── metrics.MetricsCollector
│   ├── metrics.PrometheusCollector
│   └── metrics.NoopCollector
│
├── logging.Logger
│   ├── logging.SlogAdapter
│   └── logging.NoopLogger
│
└── endpoint.ServiceEndpoint (configuration)

lifecycle.LifecycleManager (advanced)
├── lifecycle.LazyBooter
├── lifecycle.IdleShutdown
└── lifecycle.ConcurrencySemaphore

runtime.ContainerRuntime (low-level)
├── runtime.DockerRuntime
├── runtime.PodmanRuntime
└── runtime.KubernetesRuntime
```

## Design Patterns

### Strategy Pattern
- `ContainerRuntime` interface with Docker/Podman/K8s implementations
- `HealthChecker` with TCP/HTTP/gRPC/Custom check strategies
- `Discoverer` with TCP/DNS discovery strategies

### Observer Pattern
- `EventBus` publishes lifecycle events (started, stopped, health changed)
- Subscribers filter by event type or source

### Factory Pattern
- `runtime.AutoDetect()` creates the appropriate runtime
- `health.NewDefaultChecker()` creates a checker with all strategies

### Builder Pattern
- `endpoint.NewEndpoint().WithHost().WithPort().Build()`
- Fluent API for constructing ServiceEndpoint configurations

### Decorator Pattern
- `health.RetryPolicy` wraps any health check with retry logic
- Logging wrappers can be applied to any interface

### Functional Options
- `boot.NewBootManager(endpoints, WithRuntime(), WithLogger(), ...)`
- Extensible configuration without breaking changes

## Boot Sequence

1. **Discovery Phase**: Probe endpoints with discovery enabled
2. **Grouping Phase**: Group services by compose file + profile
3. **Start Phase**: Start each compose group with `docker compose up -d`
4. **Health Check Phase**: Check all enabled services
5. **Summary Phase**: Aggregate results, fail if required services are unhealthy

## Thread Safety

- All registries use `sync.RWMutex`
- EventBus uses channel-based delivery
- LifecycleManager coordinates via mutexes and semaphores
- Health checks are stateless and safe for concurrent use
