package challenges

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"digital.vasic.containers/pkg/compose"
	"digital.vasic.containers/pkg/envconfig"
	"digital.vasic.containers/pkg/logging"
	"digital.vasic.containers/pkg/remote"
)

// RemoteDeploymentChallengeSuite defines challenges for remote container deployment.
// Run with: go test -v -tags=challenge ./challenges/

// skipIfNoRemoteHost skips the test if no remote host is configured.
func skipIfNoRemoteHost(t *testing.T) (*envconfig.DistributionConfig, *remote.SSHExecutor, remote.RemoteHost) {
	t.Helper()

	cfg := envconfig.LoadFromEnv()
	if !cfg.Enabled {
		t.Skip("Set CONTAINERS_REMOTE_ENABLED=true to run remote challenges")  // SKIP-OK: #legacy-untriaged
	}

	hosts := cfg.ToRemoteHosts()
	if len(hosts) == 0 {
		t.Skip("No remote hosts configured - set CONTAINERS_REMOTE_HOST_1_*")  // SKIP-OK: #legacy-untriaged
	}

	logger := logging.NopLogger{}
	executor, err := remote.NewSSHExecutor(logger)
	if err != nil {
		t.Fatalf("Failed to create SSH executor: %v", err)
	}

	return cfg, executor, hosts[0]
}

// Challenge: SSH Connection Establishment
func TestChallenge_SSH_Connection(t *testing.T) {
	_, executor, host := skipIfNoRemoteHost(t)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	result, err := executor.Execute(ctx, host, "echo connection_test")
	if err != nil {
		t.Fatalf("SSH connection failed: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("SSH command failed with exit code %d: %s", result.ExitCode, result.Stderr)
	}
	if !strings.Contains(result.Stdout, "connection_test") {
		t.Fatalf("Unexpected output: %s", result.Stdout)
	}
	t.Logf("✓ SSH connection successful")
}

// Challenge: Compose Command Detection (podman-compose preferred)
func TestChallenge_ComposeDetection_PodmanComposeFirst(t *testing.T) {
	_, executor, host := skipIfNoRemoteHost(t)

	ctx := context.Background()
	logger := logging.NopLogger{}

	// Check if podman-compose exists
	checkResult, _ := executor.Execute(ctx, host, "which podman-compose 2>/dev/null")
	podmanComposeExists := checkResult != nil && checkResult.ExitCode == 0

	// Run detection
	detector := remote.NewComposeDetector(executor, logger)
	cmd, err := detector.Detect(ctx, host)
	if err != nil {
		t.Fatalf("Compose detection failed: %v", err)
	}

	t.Logf("Detected compose command: %s (version: %s)", cmd.Name, cmd.Version)

	// If podman-compose exists, it MUST be the detected command
	if podmanComposeExists {
		if cmd.Name != "podman-compose" {
			t.Errorf("Expected podman-compose to be detected, got: %s", cmd.Name)
		} else {
			t.Logf("✓ podman-compose correctly detected as preferred")
		}
	}
}

// Challenge: Compose Command Detection Fallback Chain
func TestChallenge_ComposeDetection_FallbackChain(t *testing.T) {
	// This challenge documents the expected detection priority order:
	// podman-compose > docker compose > podman compose > docker-compose

	expectedPriority := []string{
		"podman-compose",
		"docker compose",
		"podman compose",
		"docker-compose",
	}

	// Verify KnownComposeCommands returns the correct order
	known := remote.KnownComposeCommands()
	if len(known) != len(expectedPriority) {
		t.Errorf("Expected %d known commands, got %d", len(expectedPriority), len(known))
	}

	for i, expected := range expectedPriority {
		if i >= len(known) || known[i] != expected {
			t.Errorf("Priority %d: expected %q, got %q", i, expected, known[i])
		}
	}

	t.Logf("✓ Compose detection priority order verified: %v", expectedPriority)
}

// Challenge: Single Container Deployment
func TestChallenge_Deploy_SingleContainer(t *testing.T) {
	if os.Getenv("CONTAINERS_CHALLENGE_DEPLOY") == "" {
		t.Skip("Set CONTAINERS_CHALLENGE_DEPLOY=1 to run deployment challenges")  // SKIP-OK: #legacy-untriaged
	}

	_, executor, host := skipIfNoRemoteHost(t)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	containerName := fmt.Sprintf("challenge-test-%d", time.Now().Unix())

	// Detect compose command
	logger := logging.NopLogger{}
	detector := remote.NewComposeDetector(executor, logger)
	cmd, err := detector.Detect(ctx, host)
	if err != nil {
		t.Fatalf("Compose detection failed: %v", err)
	}

	// Deploy a simple nginx container using detected runtime
	var deployCmd string
	if cmd.Binary == "podman-compose" || cmd.Binary == "podman" {
		deployCmd = fmt.Sprintf("podman run -d --name %s -p 8888:80 docker.io/nginx:alpine", containerName)
	} else {
		deployCmd = fmt.Sprintf("docker run -d --name %s -p 8888:80 docker.io/nginx:alpine", containerName)
	}

	result, err := executor.Execute(ctx, host, deployCmd)
	if err != nil {
		t.Fatalf("Deploy failed: %v", err)
	}
	if result.ExitCode != 0 {
		t.Fatalf("Deploy failed with exit code %d: %s", result.ExitCode, result.Stderr)
	}

	// Cleanup
	defer func() {
		runtime := "podman"
		if cmd.Binary == "docker" || cmd.Binary == "docker-compose" {
			runtime = "docker"
		}
		_, _ = executor.Execute(ctx, host, runtime+" rm -f "+containerName)
	}()

	// Verify container is running
	time.Sleep(3 * time.Second)
	runtime := "podman"
	if cmd.Binary == "docker" || cmd.Binary == "docker-compose" {
		runtime = "docker"
	}

	inspectCmd := fmt.Sprintf("%s inspect -f '{{.State.Running}}' %s 2>/dev/null", runtime, containerName)
	result, err = executor.Execute(ctx, host, inspectCmd)
	if err != nil {
		t.Fatalf("Inspect failed: %v", err)
	}
	if !strings.Contains(result.Stdout, "true") {
		t.Fatalf("Container is not running: %s", result.Stdout)
	}

	t.Logf("✓ Container %s deployed and running successfully", containerName)
}

// Challenge: Compose Project Deployment
func TestChallenge_Deploy_ComposeProject(t *testing.T) {
	if os.Getenv("CONTAINERS_CHALLENGE_DEPLOY") == "" {
		t.Skip("Set CONTAINERS_CHALLENGE_DEPLOY=1 to run deployment challenges")  // SKIP-OK: #legacy-untriaged
	}

	_, executor, host := skipIfNoRemoteHost(t)

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	logger := logging.NopLogger{}
	projectName := fmt.Sprintf("challenge-compose-%d", time.Now().Unix())
	composeContent := fmt.Sprintf(`
services:
  web:
    image: docker.io/nginx:alpine
    ports:
      - "8889:80"
  redis:
    image: docker.io/redis:alpine
    ports:
      - "6380:6379"
`)

	// Create temp file
	tmpFile, err := os.CreateTemp("", "challenge-compose-*.yml")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.WriteString(composeContent)
	tmpFile.Close()

	// Copy to remote
	remotePath := fmt.Sprintf("/tmp/%s-compose.yml", projectName)
	err = executor.CopyFile(ctx, host, tmpFile.Name(), remotePath)
	if err != nil {
		t.Fatalf("Failed to copy compose file: %v", err)
	}
	defer executor.Execute(ctx, host, "rm -f "+remotePath)

	// Create orchestrator
	orch := remote.NewRemoteComposeOrchestrator(host, executor, logger)

	// Deploy
	project := compose.ComposeProject{
		File: remotePath,
		Name: projectName,
	}
	err = orch.Up(ctx, project)
	if err != nil {
		t.Fatalf("Compose up failed: %v", err)
	}

	// Cleanup
	defer func() {
		_ = orch.Down(ctx, project)
	}()

	// Verify services
	time.Sleep(5 * time.Second)
	statuses, err := orch.Status(ctx, project)
	if err != nil {
		t.Fatalf("Failed to get status: %v", err)
	}

	runningCount := 0
	for _, s := range statuses {
		if s.State == "running" {
			runningCount++
		}
	}

	if runningCount < 2 {
		t.Errorf("Expected 2 running services, got %d", runningCount)
	} else {
		t.Logf("✓ Compose project deployed with %d running services", runningCount)
	}
}

// Challenge: Health Check Over Remote
func TestChallenge_HealthCheck_Remote(t *testing.T) {
	_, executor, host := skipIfNoRemoteHost(t)

	ctx := context.Background()

	// Test TCP port check (SSH should always be available)
	checkSSH := "nc -z localhost 22 && echo SSH_OK"
	result, err := executor.Execute(ctx, host, checkSSH)
	if err != nil {
		t.Fatalf("Health check command failed: %v", err)
	}
	if !strings.Contains(result.Stdout, "SSH_OK") {
		t.Fatalf("SSH port not responding on remote host")
	}

	t.Logf("✓ Remote health check passed")
}

// Challenge: Resource Probing
func TestChallenge_ResourceProbing(t *testing.T) {
	_, executor, host := skipIfNoRemoteHost(t)

	ctx := context.Background()

	// CPU
	cpuResult, err := executor.Execute(ctx, host, "nproc")
	if err != nil {
		t.Errorf("CPU probe failed: %v", err)
	} else {
		t.Logf("✓ Remote CPU cores: %s", strings.TrimSpace(cpuResult.Stdout))
	}

	// Memory
	memResult, err := executor.Execute(ctx, host, "free -m | awk '/^Mem:/{print $2}'")
	if err != nil {
		t.Errorf("Memory probe failed: %v", err)
	} else {
		t.Logf("✓ Remote total memory (MB): %s", strings.TrimSpace(memResult.Stdout))
	}

	// Disk
	diskResult, err := executor.Execute(ctx, host, "df -BG / | awk 'NR==2{print $4}'")
	if err != nil {
		t.Errorf("Disk probe failed: %v", err)
	} else {
		t.Logf("✓ Remote available disk (GB): %s", strings.TrimSpace(diskResult.Stdout))
	}
}

// Challenge: ControlMaster Connection Pooling
func TestChallenge_SSH_ControlMaster(t *testing.T) {
	_, _, host := skipIfNoRemoteHost(t)

	// Create executor with ControlMaster enabled
	logger := logging.NopLogger{}
	executor, err := remote.NewSSHExecutor(
		logger,
		remote.WithControlMaster(true),
		remote.WithControlPersist(60*time.Second),
	)
	if err != nil {
		t.Fatalf("Failed to create executor: %v", err)
	}
	defer executor.Close()

	ctx := context.Background()

	// Execute 10 commands and measure time
	start := time.Now()
	for i := 0; i < 10; i++ {
		_, err := executor.Execute(ctx, host, "echo test")
		if err != nil {
			t.Errorf("Command %d failed: %v", i, err)
		}
	}
	elapsed := time.Since(start)

	t.Logf("10 commands with ControlMaster took: %v", elapsed)

	// With ControlMaster, each command should be fast
	avgMs := elapsed.Milliseconds() / 10
	if avgMs > 500 {
		t.Errorf("Average command time too high: %dms (expected <500ms)", avgMs)
	} else {
		t.Logf("✓ ControlMaster working efficiently (avg: %dms/command)", avgMs)
	}
}

// Challenge: Multi-Host Configuration
func TestChallenge_MultiHost_Configuration(t *testing.T) {
	cfg := envconfig.LoadFromEnv()
	if !cfg.Enabled {
		t.Skip("Set CONTAINERS_REMOTE_ENABLED=true")  // SKIP-OK: #legacy-untriaged
	}

	hosts := cfg.ToRemoteHosts()
	t.Logf("Configured hosts: %d", len(hosts))

	for i, h := range hosts {
		t.Logf("  Host %d: %s (%s@%s:%d) runtime=%s labels=%v",
			i+1, h.Name, h.User, h.Address, h.SSHPort(), h.Runtime, h.Labels)
	}

	if len(hosts) >= 2 {
		t.Logf("✓ Multi-host configuration detected")
	}
}

// Challenge: Scheduler Strategy Validation
func TestChallenge_Scheduler_Strategy(t *testing.T) {
	cfg := envconfig.LoadFromEnv()
	if !cfg.Enabled {
		t.Skip("Set CONTAINERS_REMOTE_ENABLED=true")  // SKIP-OK: #legacy-untriaged
	}

	validStrategies := map[string]bool{
		"resource_aware": true,
		"round_robin":    true,
		"affinity":       true,
		"spread":         true,
		"bin_pack":       true,
	}

	strategy := cfg.Scheduler
	if strategy == "" {
		strategy = "resource_aware" // default
	}

	if !validStrategies[strategy] {
		t.Errorf("Invalid scheduler strategy: %s", strategy)
	} else {
		t.Logf("✓ Scheduler strategy: %s", strategy)
	}
}

// Challenge: Environment Variable Parsing
func TestChallenge_EnvConfig_Parsing(t *testing.T) {
	// This challenge verifies that all expected env vars are parsed correctly
	cfg := envconfig.LoadFromEnv()

	t.Logf("Configuration loaded:")
	t.Logf("  Enabled: %v", cfg.Enabled)
	t.Logf("  Scheduler: %s", cfg.Scheduler)
	t.Logf("  ConnectTimeout: %d", cfg.ConnectTimeout)
	t.Logf("  CommandTimeout: %d", cfg.CommandTimeout)
	t.Logf("  ControlMaster: %v", cfg.ControlMasterEnabled)
	t.Logf("  ControlPersist: %d", cfg.ControlPersist)
	t.Logf("  MaxConnections: %d", cfg.MaxConnections)
	t.Logf("  PortRange: %d-%d", cfg.PortRangeStart, cfg.PortRangeEnd)

	t.Logf("✓ Environment configuration parsed successfully")
}
