package connection

import (
	"time"
)

// ConnectionType identifies the type of remote connection.
type ConnectionType string

const (
	TypeSSH            ConnectionType = "ssh"
	TypeSSHAgent       ConnectionType = "ssh_agent"
	TypeSSHCertificate ConnectionType = "ssh_certificate"
	TypeSSHBastion     ConnectionType = "ssh_bastion"
	TypeWinRM          ConnectionType = "winrm"
	TypeAnsible        ConnectionType = "ansible"
	TypeDocker         ConnectionType = "docker"
	TypeKubernetes     ConnectionType = "kubernetes"
	TypeAWSSSM         ConnectionType = "aws_ssm"
	TypeAzureSerial    ConnectionType = "azure_serial"
	TypeGCPOSLogin     ConnectionType = "gcp_os_login"
	TypeLocal          ConnectionType = "local"
)

// AuthType identifies the authentication method.
type AuthType string

const (
	AuthPassword   AuthType = "password"
	AuthKey        AuthType = "key"
	AuthAgent      AuthType = "agent"
	AuthCertificate AuthType = "certificate"
	AuthIAM        AuthType = "iam"
	AuthToken      AuthType = "token"
)

// ConnectionStatus represents the state of a connection.
type ConnectionStatus string

const (
	StatusDisconnected ConnectionStatus = "disconnected"
	StatusConnecting   ConnectionStatus = "connecting"
	StatusConnected    ConnectionStatus = "connected"
	StatusError        ConnectionStatus = "error"
	StatusReconnecting ConnectionStatus = "reconnecting"
)

// ConnectionConfig holds configuration for a remote connection.
type ConnectionConfig struct {
	Type     ConnectionType         `json:"type"`
	Host     string                 `json:"host"`
	Port     int                    `json:"port"`
	Username string                 `json:"username"`
	Password string                 `json:"password,omitempty"`
	KeyPath  string                 `json:"key_path,omitempty"`
	Options  map[string]interface{} `json:"options,omitempty"`
	
	SSHAgent      *SSHAgentConfig      `json:"ssh_agent,omitempty"`
	SSHCert       *SSHCertificateConfig `json:"ssh_certificate,omitempty"`
	SSHBastion    *SSHBastionConfig    `json:"bastion,omitempty"`
	WinRM         *WinRMConfig         `json:"winrm,omitempty"`
	Ansible       *AnsibleConfig       `json:"ansible,omitempty"`
	Docker        *DockerConfig        `json:"docker,omitempty"`
	Kubernetes    *KubernetesConfig    `json:"kubernetes,omitempty"`
	AWSSSM        *AWSSSMConfig        `json:"aws_ssm,omitempty"`
	AzureSerial   *AzureSerialConfig   `json:"azure_serial,omitempty"`
	GCPOSLogin    *GCPOSLoginConfig    `json:"gcp_os_login,omitempty"`
	Local         *LocalConfig         `json:"local,omitempty"`
	
	Timeout    time.Duration `json:"timeout"`
	Retries    int           `json:"retries"`
	MaxRetries int           `json:"max_retries"`
}

// SSHAgentConfig holds SSH agent configuration.
type SSHAgentConfig struct {
	SocketPath   string `json:"socket_path"`
	ForwardAgent bool   `json:"forward_agent"`
}

// SSHCertificateConfig holds SSH certificate configuration.
type SSHCertificateConfig struct {
	CertificatePath string `json:"certificate_path"`
	KeyPath         string `json:"key_path"`
	CAPublicKeyPath string `json:"ca_public_key_path"`
}

// SSHBastionConfig holds bastion/jump host configuration.
type SSHBastionConfig struct {
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password,omitempty"`
	KeyPath  string `json:"key_path,omitempty"`
}

// WinRMConfig holds WinRM configuration.
type WinRMConfig struct {
	Transport  string `json:"transport"`
	AuthType   string `json:"auth_type"`
	VerifySSL  bool   `json:"verify_ssl"`
	CertPath   string `json:"cert_path,omitempty"`
	KeyPath    string `json:"key_path,omitempty"`
}

// AnsibleConfig holds Ansible configuration.
type AnsibleConfig struct {
	InventoryPath string            `json:"inventory_path"`
	PlaybookPath  string            `json:"playbook_path"`
	Variables     map[string]string `json:"variables,omitempty"`
}

// DockerConfig holds Docker connection configuration.
type DockerConfig struct {
	Host          string   `json:"host"`
	ContainerName string   `json:"container_name"`
	Image         string   `json:"image"`
	Network       string   `json:"network,omitempty"`
	Volumes       []string `json:"volumes,omitempty"`
}

// KubernetesConfig holds Kubernetes connection configuration.
type KubernetesConfig struct {
	KubeconfigPath string `json:"kubeconfig_path"`
	Namespace      string `json:"namespace"`
	PodSelector    string `json:"pod_selector"`
	Container      string `json:"container,omitempty"`
	Context        string `json:"context,omitempty"`
}

// AWSSSMConfig holds AWS Systems Manager configuration.
type AWSSSMConfig struct {
	InstanceID     string            `json:"instance_id"`
	Region         string            `json:"region"`
	Profile        string            `json:"profile,omitempty"`
	SessionOptions map[string]string `json:"session_options,omitempty"`
}

// AzureSerialConfig holds Azure Serial Console configuration.
type AzureSerialConfig struct {
	SubscriptionID string `json:"subscription_id"`
	ResourceGroup  string `json:"resource_group"`
	VMName         string `json:"vm_name"`
	TenantID       string `json:"tenant_id"`
	ClientID       string `json:"client_id"`
	ClientSecret   string `json:"client_secret"`
}

// GCPOSLoginConfig holds GCP OS Login configuration.
type GCPOSLoginConfig struct {
	Project           string `json:"project"`
	Zone              string `json:"zone"`
	Instance          string `json:"instance"`
	ServiceAccountKey string `json:"service_account_key,omitempty"`
}

// LocalConfig holds local execution configuration.
type LocalConfig struct {
	WorkingDirectory string `json:"working_directory"`
	User             string `json:"user,omitempty"`
}

// ExecutionResult holds the result of a command execution.
type ExecutionResult struct {
	Success    bool
	Output     string
	ErrorOutput string
	ExitCode   int
	Duration   time.Duration
}

// TransferResult holds the result of a file transfer.
type TransferResult struct {
	Success         bool
	BytesTransferred int64
	Message         string
	Duration        time.Duration
}

// ConnectionMetadata holds connection metadata.
type ConnectionMetadata struct {
	Type     ConnectionType
	Host     string
	Port     int
	Username string
	Options  map[string]string
}

// ConnectionHealth holds connection health information.
type ConnectionHealth struct {
	IsHealthy   bool
	LatencyMs   int64
	LastChecked time.Time
	Message     string
}

// ConnectionResult holds the result of a connection attempt.
type ConnectionResult struct {
	Success   bool
	Message   string
	Error     error
	Connected bool
}
