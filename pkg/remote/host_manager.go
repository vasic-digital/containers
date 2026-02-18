package remote

import (
	"context"
	"fmt"
	"sync"

	"digital.vasic.containers/pkg/logging"
)

// HostManager defines the interface for managing a registry of
// remote hosts.
type HostManager interface {
	// AddHost registers a remote host.
	AddHost(host RemoteHost) error
	// RemoveHost removes a host by name.
	RemoveHost(name string) error
	// GetHost retrieves a host by name.
	GetHost(name string) (*RemoteHost, error)
	// ListHosts returns all registered hosts.
	ListHosts() []RemoteHost
	// ProbeHost collects resources from a specific host.
	ProbeHost(ctx context.Context, name string) (*HostResources, error)
	// ProbeAll collects resources from all hosts.
	ProbeAll(ctx context.Context) map[string]*HostResources
	// HostState returns the current state of a host.
	HostState(name string) HostState
}

// DefaultHostManager implements HostManager with an in-memory
// host registry.
type DefaultHostManager struct {
	mu       sync.RWMutex
	hosts    map[string]RemoteHost
	states   map[string]HostState
	executor RemoteExecutor
	prober   *Prober
	logger   logging.Logger
}

// NewHostManager creates a DefaultHostManager.
func NewHostManager(
	executor RemoteExecutor,
	logger logging.Logger,
) *DefaultHostManager {
	if logger == nil {
		logger = logging.NopLogger{}
	}
	return &DefaultHostManager{
		hosts:    make(map[string]RemoteHost),
		states:   make(map[string]HostState),
		executor: executor,
		prober:   NewProber(executor),
		logger:   logger,
	}
}

// AddHost registers a remote host.
func (m *DefaultHostManager) AddHost(host RemoteHost) error {
	if host.Name == "" {
		return fmt.Errorf("host name cannot be empty")
	}
	if host.Address == "" {
		return fmt.Errorf("host address cannot be empty")
	}
	if host.User == "" {
		return fmt.Errorf("host user cannot be empty")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.hosts[host.Name]; exists {
		return fmt.Errorf("host %q already registered", host.Name)
	}

	m.hosts[host.Name] = host
	m.states[host.Name] = HostUnknown
	m.logger.Info("registered host %s (%s@%s:%d)",
		host.Name, host.User, host.Address, host.SSHPort(),
	)
	return nil
}

// RemoveHost removes a host from the registry.
func (m *DefaultHostManager) RemoveHost(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.hosts[name]; !exists {
		return fmt.Errorf("host %q not found", name)
	}

	delete(m.hosts, name)
	delete(m.states, name)
	m.logger.Info("removed host %s", name)
	return nil
}

// GetHost retrieves a host by name.
func (m *DefaultHostManager) GetHost(
	name string,
) (*RemoteHost, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	host, ok := m.hosts[name]
	if !ok {
		return nil, fmt.Errorf("host %q not found", name)
	}
	return &host, nil
}

// ListHosts returns all registered hosts.
func (m *DefaultHostManager) ListHosts() []RemoteHost {
	m.mu.RLock()
	defer m.mu.RUnlock()

	hosts := make([]RemoteHost, 0, len(m.hosts))
	for _, h := range m.hosts {
		hosts = append(hosts, h)
	}
	return hosts
}

// ProbeHost collects resource information from a specific host.
func (m *DefaultHostManager) ProbeHost(
	ctx context.Context, name string,
) (*HostResources, error) {
	m.mu.RLock()
	host, ok := m.hosts[name]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("host %q not found", name)
	}

	if !m.executor.IsReachable(ctx, host) {
		m.setState(name, HostOffline)
		return nil, fmt.Errorf("host %q is unreachable", name)
	}

	resources, err := m.prober.Probe(ctx, host)
	if err != nil {
		m.setState(name, HostDegraded)
		return nil, fmt.Errorf("probe %s: %w", name, err)
	}

	if resources.CPUPercent > 90 || resources.MemoryPercent > 90 {
		m.setState(name, HostDegraded)
	} else {
		m.setState(name, HostOnline)
	}

	return resources, nil
}

// ProbeAll collects resources from all registered hosts concurrently.
func (m *DefaultHostManager) ProbeAll(
	ctx context.Context,
) map[string]*HostResources {
	m.mu.RLock()
	hosts := make([]RemoteHost, 0, len(m.hosts))
	for _, h := range m.hosts {
		hosts = append(hosts, h)
	}
	m.mu.RUnlock()

	results := make(map[string]*HostResources)
	var mu sync.Mutex
	var wg sync.WaitGroup

	for _, host := range hosts {
		wg.Add(1)
		go func(h RemoteHost) {
			defer wg.Done()
			res, err := m.ProbeHost(ctx, h.Name)
			if err != nil {
				m.logger.Warn("probe %s failed: %v", h.Name, err)
				return
			}
			mu.Lock()
			results[h.Name] = res
			mu.Unlock()
		}(host)
	}

	wg.Wait()
	return results
}

// HostState returns the current state of a host.
func (m *DefaultHostManager) HostState(name string) HostState {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, ok := m.states[name]
	if !ok {
		return HostUnknown
	}
	return state
}

func (m *DefaultHostManager) setState(
	name string, state HostState,
) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.states[name] = state
}
