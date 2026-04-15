package buildpkg

import (
	"context"
	"testing"

	"digital.vasic.containers/pkg/remote"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type stubHostManager struct {
	hosts     []remote.RemoteHost
	resources map[string]*remote.HostResources
}

func newStubHostManager() *stubHostManager {
	return &stubHostManager{
		resources: make(map[string]*remote.HostResources),
	}
}

func (s *stubHostManager) AddHost(host remote.RemoteHost) error {
	s.hosts = append(s.hosts, host)
	return nil
}

func (s *stubHostManager) RemoveHost(name string) error {
	for i, h := range s.hosts {
		if h.Name == name {
			s.hosts = append(s.hosts[:i], s.hosts[i+1:]...)
			break
		}
	}
	delete(s.resources, name)
	return nil
}

func (s *stubHostManager) GetHost(name string) (*remote.RemoteHost, error) {
	for _, h := range s.hosts {
		if h.Name == name {
			return &h, nil
		}
	}
	return nil, nil
}

func (s *stubHostManager) ListHosts() []remote.RemoteHost {
	return s.hosts
}

func (s *stubHostManager) ProbeHost(_ context.Context, name string) (*remote.HostResources, error) {
	if r, ok := s.resources[name]; ok {
		return r, nil
	}
	return nil, nil
}

func (s *stubHostManager) ProbeAll(_ context.Context) map[string]*remote.HostResources {
	return s.resources
}

func (s *stubHostManager) HostState(name string) remote.HostState {
	if _, ok := s.resources[name]; ok {
		return remote.HostOnline
	}
	return remote.HostUnknown
}

func setupMultiHostPlanner() *Planner {
	hm := newStubHostManager()

	hm.resources["local"] = &remote.HostResources{
		Host:          "local",
		CPUCores:      8,
		CPUPercent:    20,
		MemoryTotalMB: 16384,
		MemoryPercent: 30,
		DiskTotalMB:   100000,
		DiskPercent:   40,
	}

	hm.resources["thinker"] = &remote.HostResources{
		Host:          "thinker",
		CPUCores:      16,
		CPUPercent:    10,
		MemoryTotalMB: 32768,
		MemoryPercent: 20,
		DiskTotalMB:   500000,
		DiskPercent:   30,
	}
	hm.hosts = append(hm.hosts, remote.RemoteHost{
		Name:    "thinker",
		Address: "thinker.local",
		Labels:  map[string]string{},
	})

	hm.resources["amber"] = &remote.HostResources{
		Host:          "amber",
		CPUCores:      12,
		CPUPercent:    15,
		MemoryTotalMB: 24576,
		MemoryPercent: 25,
		DiskTotalMB:   300000,
		DiskPercent:   35,
	}
	hm.hosts = append(hm.hosts, remote.RemoteHost{
		Name:    "amber",
		Address: "amber.local",
		Labels:  map[string]string{},
	})

	return NewPlanner(hm)
}

func TestPlanner_PlanAll(t *testing.T) {
	p := setupMultiHostPlanner()

	plan, err := p.PlanAll(context.Background())
	require.NoError(t, err)
	require.NotNil(t, plan)

	assert.Len(t, plan.Assignments, 7)

	for _, a := range plan.Assignments {
		assert.NotEmpty(t, a.Host,
			"component %s has no host assigned", a.Component.Name)
	}
}

func TestPlanner_PlanSingle(t *testing.T) {
	hm := newStubHostManager()
	hm.resources["local"] = &remote.HostResources{
		Host:          "local",
		CPUCores:      8,
		CPUPercent:    20,
		MemoryTotalMB: 16384,
		MemoryPercent: 30,
		DiskTotalMB:   100000,
		DiskPercent:   40,
	}

	p := NewPlanner(hm)

	plan, err := p.PlanSingle(context.Background(), "catalog-api")
	require.NoError(t, err)
	require.NotNil(t, plan)

	assert.Len(t, plan.Assignments, 1)
	assert.Equal(t, "catalog-api", plan.Assignments[0].Component.Name)
}

func TestPlanner_PlanSingleUnknownComponent(t *testing.T) {
	hm := newStubHostManager()
	p := NewPlanner(hm)

	_, err := p.PlanSingle(context.Background(), "nonexistent")
	assert.Error(t, err)
}
