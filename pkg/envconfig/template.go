package envconfig

// GenerateEnvExample returns a fully documented .env.example
// template for remote distribution configuration.
func GenerateEnvExample() string {
	return `# Containers Module — Remote Distribution Configuration
# =====================================================
#
# Enable remote container distribution across multiple hosts.
# All values below are loaded via environment variables or this
# .env file.

# --- Global Settings ---

# Enable/disable remote distribution (default: false).
CONTAINERS_REMOTE_ENABLED=false

# Default SSH user for all remote hosts.
CONTAINERS_REMOTE_DEFAULT_SSH_USER=deploy

# Default SSH private key path.
CONTAINERS_REMOTE_DEFAULT_SSH_KEY=~/.ssh/id_rsa

# Default container runtime on remote hosts (docker/podman).
CONTAINERS_REMOTE_DEFAULT_RUNTIME=docker

# Scheduling strategy: resource_aware, round_robin, affinity,
# spread, bin_pack (default: resource_aware).
CONTAINERS_REMOTE_SCHEDULER=resource_aware

# Local port range for SSH tunnels (default: 20000-30000).
CONTAINERS_REMOTE_PORT_RANGE_START=20000
CONTAINERS_REMOTE_PORT_RANGE_END=30000

# Volume mount type: sshfs, nfs, rsync (default: sshfs).
CONTAINERS_REMOTE_VOLUME_TYPE=sshfs

# --- Per-Host Configuration ---
#
# Add hosts by incrementing the number (1, 2, 3, ...).
# Stop reading when a NAME is empty or missing.

# Host 1
CONTAINERS_REMOTE_HOST_1_NAME=gpu-server-1
CONTAINERS_REMOTE_HOST_1_ADDRESS=192.168.1.100
CONTAINERS_REMOTE_HOST_1_PORT=22
CONTAINERS_REMOTE_HOST_1_USER=deploy
CONTAINERS_REMOTE_HOST_1_KEY=~/.ssh/gpu_key
CONTAINERS_REMOTE_HOST_1_RUNTIME=docker
CONTAINERS_REMOTE_HOST_1_LABELS=gpu=true,arch=amd64

# Host 2
# CONTAINERS_REMOTE_HOST_2_NAME=cpu-server-1
# CONTAINERS_REMOTE_HOST_2_ADDRESS=192.168.1.101
# CONTAINERS_REMOTE_HOST_2_PORT=22
# CONTAINERS_REMOTE_HOST_2_USER=
# CONTAINERS_REMOTE_HOST_2_KEY=
# CONTAINERS_REMOTE_HOST_2_RUNTIME=
# CONTAINERS_REMOTE_HOST_2_LABELS=arch=arm64

# Host 3
# CONTAINERS_REMOTE_HOST_3_NAME=storage-server
# CONTAINERS_REMOTE_HOST_3_ADDRESS=192.168.1.102
# CONTAINERS_REMOTE_HOST_3_PORT=2222
# CONTAINERS_REMOTE_HOST_3_USER=admin
# CONTAINERS_REMOTE_HOST_3_KEY=~/.ssh/storage_key
# CONTAINERS_REMOTE_HOST_3_RUNTIME=podman
# CONTAINERS_REMOTE_HOST_3_LABELS=storage=true,ssd=true
`
}
