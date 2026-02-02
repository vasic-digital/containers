# Containers Module - User Guide

## Installation

```bash
go get digital.vasic.containers
```

## Basic Usage

### Container Runtime

```go
import "digital.vasic.containers/pkg/runtime"

ctx := context.Background()

// Auto-detect available runtime
rt, err := runtime.AutoDetect(ctx)

// Or use specific runtime
docker := runtime.NewDockerRuntime()
podman := runtime.NewPodmanRuntime()

// Check version
version, _ := rt.Version(ctx)

// List containers
containers, _ := rt.List(ctx, runtime.ListFilter{All: true})

// Get container status
status, _ := rt.Status(ctx, "my-container")

// Execute command in container
result, _ := rt.Exec(ctx, "my-container", []string{"echo", "hello"})
```

### Health Checking

```go
import "digital.vasic.containers/pkg/health"

checker := health.NewDefaultChecker()

// TCP health check
result := checker.Check(ctx, health.HealthTarget{
    Name: "postgres",
    Host: "localhost",
    Port: "5432",
    Type: health.HealthTCP,
    Timeout: 5 * time.Second,
})

// HTTP health check
result = checker.Check(ctx, health.HealthTarget{
    Name: "api",
    Host: "localhost",
    Port: "8080",
    Type: health.HealthHTTP,
    Path: "/health",
})

// With retry
policy := health.RetryPolicy{
    MaxRetries:    5,
    Delay:         2 * time.Second,
    BackoffFactor: 1.5,
}
result = health.CheckWithRetry(ctx, checker, target, policy)
```

### Compose Orchestration

```go
import "digital.vasic.containers/pkg/compose"

orch := compose.NewDefaultOrchestrator()

project := compose.ComposeProject{
    Name:    "myapp",
    File:    "docker-compose.yml",
    Profile: "default",
    Services: []string{"postgres", "redis"},
}

// Start services
err := orch.Up(ctx, project, compose.WithDetach(true))

// Check status
statuses, _ := orch.Status(ctx, project)

// Stop services
err = orch.Down(ctx, project)
```

### Service Endpoints (Builder)

```go
import "digital.vasic.containers/pkg/endpoint"

ep := endpoint.NewEndpoint().
    WithHost("localhost").
    WithPort("5432").
    WithEnabled(true).
    WithRequired(true).
    WithHealthType("tcp").
    WithTimeout(10 * time.Second).
    WithRetryCount(5).
    WithComposeFile("docker-compose.yml").
    WithServiceName("postgres").
    Build()
```

### Boot Manager

```go
import "digital.vasic.containers/pkg/boot"

endpoints := map[string]endpoint.ServiceEndpoint{
    "postgres": pgEndpoint,
    "redis":    redisEndpoint,
}

mgr := boot.NewBootManager(endpoints,
    boot.WithLogger(myLogger),
    boot.WithMetrics(myMetrics),
    boot.WithEventBus(myEventBus),
    boot.WithProjectDir("/path/to/project"),
)

summary, err := mgr.BootAll(ctx)
if err != nil {
    // Required service failed
}

// Later...
err = mgr.Shutdown(ctx)
```

### Lifecycle Management

```go
import "digital.vasic.containers/pkg/lifecycle"

lm := lifecycle.NewDefaultManager(orchestrator, checker)

// Register service with lazy boot
lm.Register(lifecycle.ServiceSpec{
    Name:          "postgres",
    LazyBoot:      true,
    IdleTimeout:   30 * time.Minute,
    MaxConcurrent: 10,
    Priority:      1,
    HealthTarget:  pgHealthTarget,
})

// Acquire (starts if lazy and not running)
release, err := lm.Acquire(ctx, "postgres")
defer release()

// Use the service...
```

### Event System

```go
import "digital.vasic.containers/pkg/event"

bus := event.NewEventBus(100)

// Subscribe to events
id := bus.Subscribe(
    event.EventFilter{Types: []event.EventType{event.EventContainerStarted}},
    func(ctx context.Context, e event.Event) {
        fmt.Printf("Container started: %s\n", e.Name)
    },
)

// Later...
bus.Unsubscribe(id)
```

### Resource Monitoring

```go
import "digital.vasic.containers/pkg/monitor"

mon := monitor.NewDefaultMonitor(rt)
mon.SetThreshold(monitor.ThresholdRule{
    Metric:    "cpu_percent",
    Threshold: 90.0,
    Operator:  ">",
    Action: func(snap *monitor.ResourceSnapshot) {
        fmt.Println("CPU usage too high!")
    },
})

mon.Start(ctx, 10 * time.Second)
defer mon.Stop()

snap, _ := mon.Snapshot()
```
