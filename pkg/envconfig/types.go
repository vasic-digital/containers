package envconfig

import "digital.vasic.containers/pkg/remote"

// DistributionConfig holds the full remote distribution
// configuration loaded from environment or file.
type DistributionConfig struct {
	// Enabled turns remote distribution on/off.
	Enabled bool
	// DefaultUser is the default SSH user for all hosts.
	DefaultUser string
	// DefaultKeyPath is the default SSH key path.
	DefaultKeyPath string
	// DefaultPassword is the default SSH password for bootstrap.
	DefaultPassword string
	// DefaultRuntime is the default container runtime.
	DefaultRuntime string
	// Scheduler is the scheduling strategy name.
	Scheduler string
	// PortRangeStart is the start of the tunnel port range.
	PortRangeStart int
	// PortRangeEnd is the end of the tunnel port range.
	PortRangeEnd int
	// VolumeType is the default volume mount type.
	VolumeType string
	// ConnectTimeout is the SSH connection timeout in seconds.
	ConnectTimeout int
	// CommandTimeout is the SSH command timeout in seconds.
	CommandTimeout int
	// ControlMasterEnabled enables SSH ControlMaster pooling.
	ControlMasterEnabled bool
	// ControlPersist is the ControlMaster persist time in
	// seconds.
	ControlPersist int
	// MaxConnections is the max concurrent SSH connections per
	// host.
	MaxConnections int
	// Hosts is the list of remote host configurations.
	Hosts []RemoteEndpointConfig
}

// RemoteEndpointConfig holds per-host configuration.
type RemoteEndpointConfig struct {
	// Name is the host identifier.
	Name string
	// Address is the hostname or IP.
	Address string
	// Port is the SSH port.
	Port int
	// User is the SSH user (overrides default).
	User string
	// KeyPath is the SSH key (overrides default).
	KeyPath string
	// Password is the SSH password (for bootstrap, overrides
	// default).
	Password string
	// Runtime is the container runtime (overrides default).
	Runtime string
	// Labels are scheduling labels.
	Labels map[string]string
}

// ToRemoteHosts converts the config to a slice of RemoteHost
// suitable for HostManager.AddHost().
func (c *DistributionConfig) ToRemoteHosts() []remote.RemoteHost {
	hosts := make([]remote.RemoteHost, 0, len(c.Hosts))
	for _, h := range c.Hosts {
		host := remote.RemoteHost{
			Name:    h.Name,
			Address: h.Address,
			Port:    h.Port,
			User:    h.User,
			KeyPath: h.KeyPath,
			Password: h.Password,
			Runtime: h.Runtime,
			Auth:    remote.AuthSSHKey,
			Labels:  h.Labels,
		}
		if host.User == "" {
			host.User = c.DefaultUser
		}
		if host.KeyPath == "" {
			host.KeyPath = c.DefaultKeyPath
		}
		if host.Password == "" {
			host.Password = c.DefaultPassword
		}
		if host.Runtime == "" {
			host.Runtime = c.DefaultRuntime
		}
		if host.Port == 0 {
			host.Port = 22
		}
		if host.Password != "" {
			host.Auth = remote.AuthPassword
		}
		hosts = append(hosts, host)
	}
	return hosts
}
