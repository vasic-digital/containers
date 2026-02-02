package lifecycle

import (
	"context"
	"fmt"
	"sync"
	"time"

	"digital.vasic.containers/pkg/compose"
	"digital.vasic.containers/pkg/health"
)

// LifecycleManager controls the full lifecycle of registered
// services including lazy boot, idle shutdown, and concurrency
// limiting.
type LifecycleManager interface {
	// Register adds a service specification to the manager.
	Register(spec ServiceSpec) error

	// Start starts the named service and waits for it to become
	// healthy.
	Start(ctx context.Context, name string) error

	// Stop stops the named service.
	Stop(ctx context.Context, name string) error

	// Acquire obtains a lease on the service, starting it if
	// needed (lazy boot). The returned ReleaseFunc must be called
	// when the caller is finished.
	Acquire(ctx context.Context, name string) (ReleaseFunc, error)

	// Status returns the current lifecycle status of a service.
	Status(name string) (ServiceLifecycleStatus, error)

	// Shutdown gracefully stops all managed services.
	Shutdown(ctx context.Context) error
}

// serviceEntry holds runtime state for a single managed service.
type serviceEntry struct {
	spec       ServiceSpec
	state      string
	healthy    bool
	lastStart  time.Time
	lastStop   time.Time
	lastAcq    time.Time
	semaphore  *ConcurrencySemaphore
	idleCtrl   *IdleShutdown
	lazyBooter *LazyBooter
}

// DefaultManager is the standard LifecycleManager implementation.
// It uses a ComposeOrchestrator to start/stop containers and a
// HealthChecker to verify readiness.
type DefaultManager struct {
	mu           sync.Mutex
	services     map[string]*serviceEntry
	orchestrator compose.ComposeOrchestrator
	checker      health.HealthChecker
}

// NewDefaultManager creates a DefaultManager backed by the given
// orchestrator and health checker.
func NewDefaultManager(
	orch compose.ComposeOrchestrator,
	hc health.HealthChecker,
) *DefaultManager {
	return &DefaultManager{
		services:     make(map[string]*serviceEntry),
		orchestrator: orch,
		checker:      hc,
	}
}

// Register adds a service specification. Returns an error if a
// service with the same name is already registered.
func (m *DefaultManager) Register(spec ServiceSpec) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if spec.Name == "" {
		return fmt.Errorf("lifecycle: service name is required")
	}
	if _, exists := m.services[spec.Name]; exists {
		return fmt.Errorf(
			"lifecycle: service %q already registered", spec.Name,
		)
	}

	entry := &serviceEntry{
		spec:  spec,
		state: "stopped",
	}

	if spec.MaxConcurrent > 0 {
		entry.semaphore = NewConcurrencySemaphore(
			spec.MaxConcurrent,
		)
	}

	m.services[spec.Name] = entry
	return nil
}

// Start starts the named service via compose and waits for health.
func (m *DefaultManager) Start(
	ctx context.Context,
	name string,
) error {
	m.mu.Lock()
	entry, ok := m.services[name]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf(
			"lifecycle: service %q not found", name,
		)
	}

	if entry.state == "running" {
		m.mu.Unlock()
		return nil
	}

	// Start dependencies first.
	deps := entry.spec.Dependencies
	m.mu.Unlock()

	for _, dep := range deps {
		if err := m.Start(ctx, dep); err != nil {
			return fmt.Errorf(
				"lifecycle: dependency %q for %q: %w",
				dep, name, err,
			)
		}
	}

	m.mu.Lock()
	entry.state = "starting"
	m.mu.Unlock()

	// Start via compose.
	if m.orchestrator != nil && entry.spec.ComposeFile != "" {
		project := compose.ComposeProject{
			File:    entry.spec.ComposeFile,
			Profile: entry.spec.Profile,
		}
		if err := m.orchestrator.Up(ctx, project); err != nil {
			m.mu.Lock()
			entry.state = "stopped"
			m.mu.Unlock()
			return fmt.Errorf(
				"lifecycle: start %q: %w", name, err,
			)
		}
	}

	// Health check.
	if m.checker != nil {
		result := m.checker.Check(ctx, entry.spec.HealthTarget)
		m.mu.Lock()
		entry.healthy = result.Healthy
		m.mu.Unlock()
		if !result.Healthy && result.Error != "" {
			m.mu.Lock()
			entry.state = "stopped"
			m.mu.Unlock()
			return fmt.Errorf(
				"lifecycle: health check %q: %s",
				name, result.Error,
			)
		}
	}

	m.mu.Lock()
	entry.state = "running"
	entry.lastStart = time.Now()
	m.mu.Unlock()

	// Start idle shutdown monitor if configured.
	if entry.spec.IdleTimeout > 0 {
		m.mu.Lock()
		entry.idleCtrl = NewIdleShutdown(
			entry.spec.IdleTimeout,
			func() {
				_ = m.Stop(context.Background(), name)
			},
		)
		m.mu.Unlock()
		entry.idleCtrl.Start()
	}

	return nil
}

// Stop stops the named service via compose.
func (m *DefaultManager) Stop(
	ctx context.Context,
	name string,
) error {
	m.mu.Lock()
	entry, ok := m.services[name]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf(
			"lifecycle: service %q not found", name,
		)
	}

	if entry.state == "stopped" {
		m.mu.Unlock()
		return nil
	}

	entry.state = "stopping"
	m.mu.Unlock()

	// Stop idle controller.
	if entry.idleCtrl != nil {
		entry.idleCtrl.Stop()
	}

	// Stop via compose.
	if m.orchestrator != nil && entry.spec.ComposeFile != "" {
		project := compose.ComposeProject{
			File:    entry.spec.ComposeFile,
			Profile: entry.spec.Profile,
		}
		if err := m.orchestrator.Down(ctx, project); err != nil {
			m.mu.Lock()
			entry.state = "running"
			m.mu.Unlock()
			return fmt.Errorf(
				"lifecycle: stop %q: %w", name, err,
			)
		}
	}

	m.mu.Lock()
	entry.state = "stopped"
	entry.healthy = false
	entry.lastStop = time.Now()
	m.mu.Unlock()

	return nil
}

// Acquire obtains a lease on the named service. If the service is
// configured for lazy boot it will be started on first Acquire.
func (m *DefaultManager) Acquire(
	ctx context.Context,
	name string,
) (ReleaseFunc, error) {
	m.mu.Lock()
	entry, ok := m.services[name]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf(
			"lifecycle: service %q not found", name,
		)
	}
	m.mu.Unlock()

	// Lazy boot: start on first acquire if not running.
	if entry.spec.LazyBoot {
		m.mu.Lock()
		if entry.lazyBooter == nil {
			entry.lazyBooter = NewLazyBooter(func() error {
				return m.Start(ctx, name)
			})
		}
		lb := entry.lazyBooter
		m.mu.Unlock()

		if err := lb.EnsureStarted(); err != nil {
			return nil, fmt.Errorf(
				"lifecycle: lazy boot %q: %w", name, err,
			)
		}
	} else {
		m.mu.Lock()
		if entry.state != "running" {
			m.mu.Unlock()
			return nil, fmt.Errorf(
				"lifecycle: service %q is not running", name,
			)
		}
		m.mu.Unlock()
	}

	// Acquire semaphore slot.
	if entry.semaphore != nil {
		if err := entry.semaphore.Acquire(ctx); err != nil {
			return nil, fmt.Errorf(
				"lifecycle: semaphore %q: %w", name, err,
			)
		}
	}

	// Reset idle timer.
	if entry.idleCtrl != nil {
		entry.idleCtrl.Touch()
	}

	m.mu.Lock()
	entry.lastAcq = time.Now()
	m.mu.Unlock()

	released := false
	return func() {
		if released {
			return
		}
		released = true
		if entry.semaphore != nil {
			entry.semaphore.Release()
		}
		if entry.idleCtrl != nil {
			entry.idleCtrl.Touch()
		}
	}, nil
}

// Status returns the current lifecycle status of the named service.
func (m *DefaultManager) Status(
	name string,
) (ServiceLifecycleStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry, ok := m.services[name]
	if !ok {
		return ServiceLifecycleStatus{}, fmt.Errorf(
			"lifecycle: service %q not found", name,
		)
	}

	activeUsers := 0
	if entry.semaphore != nil {
		activeUsers = entry.semaphore.ActiveCount()
	}

	return ServiceLifecycleStatus{
		Name:         name,
		State:        entry.state,
		Healthy:      entry.healthy,
		ActiveUsers:  activeUsers,
		LastStarted:  entry.lastStart,
		LastStopped:  entry.lastStop,
		LastAcquired: entry.lastAcq,
	}, nil
}

// Shutdown stops all managed services in reverse priority order.
func (m *DefaultManager) Shutdown(ctx context.Context) error {
	m.mu.Lock()
	names := make([]string, 0, len(m.services))
	for name := range m.services {
		names = append(names, name)
	}
	m.mu.Unlock()

	var firstErr error
	for _, name := range names {
		if err := m.Stop(ctx, name); err != nil && firstErr == nil {
			firstErr = err
		}
	}
	return firstErr
}
