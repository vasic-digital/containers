package compose

// ServiceStatus holds the current state of a compose-managed service.
type ServiceStatus struct {
	// Name is the service name as defined in the compose file.
	Name string
	// State is the container state (e.g., "running", "exited").
	State string
	// Health is the health status (e.g., "healthy", "unhealthy", "none").
	Health string
	// Ports lists the published port mappings.
	Ports []string
	// ExitCode is the exit code of the container process, if stopped.
	ExitCode int
}

// ComposeProject identifies a compose deployment by its file and
// optional profile.
type ComposeProject struct {
	// Name is a human-readable project name passed to --project-name.
	Name string
	// File is the path to the compose file.
	File string
	// Profile selects a compose profile (--profile).
	Profile string
	// Services limits the operation to the listed services. An empty
	// slice means all services in the project.
	Services []string
}
