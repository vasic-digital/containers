package connection

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestConnectionType_Values(t *testing.T) {
	types := []ConnectionType{
		TypeSSH,
		TypeSSHAgent,
		TypeSSHCertificate,
		TypeSSHBastion,
		TypeWinRM,
		TypeAnsible,
		TypeDocker,
		TypeKubernetes,
		TypeAWSSSM,
		TypeAzureSerial,
		TypeGCPOSLogin,
		TypeLocal,
	}
	for _, ct := range types {
		assert.NotEmpty(t, string(ct))
	}
}

func TestAuthType_Values(t *testing.T) {
	authTypes := []AuthType{
		AuthPassword,
		AuthKey,
		AuthAgent,
		AuthCertificate,
		AuthIAM,
		AuthToken,
	}
	for _, at := range authTypes {
		assert.NotEmpty(t, string(at))
	}
}

func TestConnectionStatus_Values(t *testing.T) {
	statuses := []ConnectionStatus{
		StatusDisconnected,
		StatusConnecting,
		StatusConnected,
		StatusError,
		StatusReconnecting,
	}
	for _, s := range statuses {
		assert.NotEmpty(t, string(s))
	}
}

func TestConnectionConfig_Defaults(t *testing.T) {
	cfg := ConnectionConfig{
		Type:     TypeSSH,
		Host:     "localhost",
		Port:     22,
		Username: "user",
	}
	assert.Equal(t, TypeSSH, cfg.Type)
	assert.Equal(t, "localhost", cfg.Host)
	assert.Equal(t, 22, cfg.Port)
}

func TestSSHAgentConfig(t *testing.T) {
	cfg := SSHAgentConfig{
		SocketPath:   "/run/user/1000/keyring/ssh",
		ForwardAgent: true,
	}
	assert.Equal(t, "/run/user/1000/keyring/ssh", cfg.SocketPath)
	assert.True(t, cfg.ForwardAgent)
}

func TestSSHCertificateConfig(t *testing.T) {
	cfg := SSHCertificateConfig{
		CertificatePath: "/path/to/cert.pub",
		KeyPath:         "/path/to/key",
		CAPublicKeyPath: "/path/to/ca.pub",
	}
	assert.Equal(t, "/path/to/cert.pub", cfg.CertificatePath)
}

func TestSSHBastionConfig(t *testing.T) {
	cfg := SSHBastionConfig{
		Host:     "bastion.example.com",
		Port:     22,
		Username: "jumpuser",
	}
	assert.Equal(t, "bastion.example.com", cfg.Host)
	assert.Equal(t, 22, cfg.Port)
}

func TestWinRMConfig(t *testing.T) {
	cfg := WinRMConfig{
		Transport: "https",
		AuthType:  "basic",
		VerifySSL: true,
	}
	assert.Equal(t, "https", cfg.Transport)
	assert.True(t, cfg.VerifySSL)
}

func TestDockerConfig(t *testing.T) {
	cfg := DockerConfig{
		Host:          "unix:///var/run/docker.sock",
		ContainerName: "test-container",
		Image:         "nginx:latest",
	}
	assert.Equal(t, "unix:///var/run/docker.sock", cfg.Host)
	assert.Equal(t, "test-container", cfg.ContainerName)
}

func TestKubernetesConfig(t *testing.T) {
	cfg := KubernetesConfig{
		KubeconfigPath: "/home/user/.kube/config",
		Namespace:      "default",
		PodSelector:    "app=nginx",
		Container:      "nginx",
	}
	assert.Equal(t, "default", cfg.Namespace)
	assert.Equal(t, "app=nginx", cfg.PodSelector)
}

func TestAWSSSMConfig(t *testing.T) {
	cfg := AWSSSMConfig{
		InstanceID: "i-1234567890abcdef0",
		Region:     "us-east-1",
		Profile:    "default",
	}
	assert.Equal(t, "i-1234567890abcdef0", cfg.InstanceID)
	assert.Equal(t, "us-east-1", cfg.Region)
}

func TestAzureSerialConfig(t *testing.T) {
	cfg := AzureSerialConfig{
		SubscriptionID: "sub-123",
		ResourceGroup:  "rg-test",
		VMName:         "vm-test",
	}
	assert.Equal(t, "sub-123", cfg.SubscriptionID)
	assert.Equal(t, "rg-test", cfg.ResourceGroup)
}

func TestGCPOSLoginConfig(t *testing.T) {
	cfg := GCPOSLoginConfig{
		Project:  "my-project",
		Zone:     "us-central1-a",
		Instance: "instance-1",
	}
	assert.Equal(t, "my-project", cfg.Project)
	assert.Equal(t, "us-central1-a", cfg.Zone)
}

func TestLocalConfig(t *testing.T) {
	cfg := LocalConfig{
		WorkingDirectory: "/opt/app",
		User:             "appuser",
	}
	assert.Equal(t, "/opt/app", cfg.WorkingDirectory)
}

func TestExecutionResult(t *testing.T) {
	result := ExecutionResult{
		Success:     true,
		Output:      "output",
		ErrorOutput: "",
		ExitCode:    0,
		Duration:    time.Second,
	}
	assert.True(t, result.Success)
	assert.Equal(t, 0, result.ExitCode)
}

func TestTransferResult(t *testing.T) {
	result := TransferResult{
		Success:          true,
		BytesTransferred: 1024,
		Message:          "Transfer complete",
		Duration:         time.Second,
	}
	assert.True(t, result.Success)
	assert.Equal(t, int64(1024), result.BytesTransferred)
}

func TestConnectionMetadata(t *testing.T) {
	meta := ConnectionMetadata{
		Type:     TypeSSH,
		Host:     "example.com",
		Port:     22,
		Username: "user",
		Options:  map[string]string{"key": "value"},
	}
	assert.Equal(t, TypeSSH, meta.Type)
	assert.Equal(t, "example.com", meta.Host)
}

func TestConnectionHealth(t *testing.T) {
	health := ConnectionHealth{
		IsHealthy:   true,
		LatencyMs:   50,
		LastChecked: time.Now(),
		Message:     "Connection OK",
	}
	assert.True(t, health.IsHealthy)
	assert.Equal(t, int64(50), health.LatencyMs)
}

func TestConnectionResult(t *testing.T) {
	result := ConnectionResult{
		Success:   true,
		Message:   "Connected",
		Connected: true,
	}
	assert.True(t, result.Success)
	assert.True(t, result.Connected)
}
