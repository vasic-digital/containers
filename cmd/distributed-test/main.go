package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"digital.vasic.containers/pkg/distribution"
	"digital.vasic.containers/pkg/envconfig"
	"digital.vasic.containers/pkg/logging"
	"digital.vasic.containers/pkg/remote"
	"digital.vasic.containers/pkg/runtime"
	"digital.vasic.containers/pkg/scheduler"
)

const (
	exitSuccess  = 0
	exitFailures = 1
	exitError    = 2
)

var (
	flagEnvFile        = flag.String("env", "containers/.env", "Path to distribution config .env file")
	flagProjectRoot    = flag.String("root", ".", "Project root directory")
	flagOutput         = flag.String("output", "reports/distributed-test", "Output directory for results")
	flagTimeout        = flag.Duration("timeout", 30*time.Minute, "Overall timeout")
	flagVerbose        = flag.Bool("verbose", false, "Enable verbose logging")
	flagSkipGo         = flag.Bool("skip-go", false, "Skip Go backend tests")
	flagSkipJS         = flag.Bool("skip-js", false, "Skip JavaScript/TypeScript tests")
	flagSkipAndroid    = flag.Bool("skip-android", false, "Skip Android tests")
	flagSkipChallenges = flag.Bool("skip-challenges", false, "Skip challenge execution")
	flagPlatform       = flag.String("platform", "api", "Challenge platform (api, web, desktop, android, tv, all)")
	flagDeployOnly     = flag.Bool("deploy-only", false, "Only deploy infrastructure, don't run tests")
	flagLocalOnly      = flag.Bool("local", false, "Force local execution (disable remote distribution)")
	// --go-api-dir / --js-projects / --android-projects are how
	// the CLI stays project-agnostic. Previous versions hardcoded
	// Catalogizer directory names — now each tier of tests is
	// driven entirely by caller configuration so any project can
	// use this runner.
	flagGoAPIDir        = flag.String("go-api-dir", "", "Directory (relative to --root) holding the Go API module to test. Empty = skip Go tests.")
	flagJSProjects      = flag.String("js-projects", "", "Comma-separated list of JS/TS project directories (relative to --root) to test. Empty = skip JS tests.")
	flagAndroidProjects = flag.String("android-projects", "", "Comma-separated list of Android project directories (relative to --root) to test. Empty = skip Android tests.")
)

type TestResult struct {
	Name     string        `json:"name"`
	Platform string        `json:"platform"`
	Status   string        `json:"status"`
	Duration time.Duration `json:"duration"`
	Output   string        `json:"output,omitempty"`
	Error    string        `json:"error,omitempty"`
	Host     string        `json:"host,omitempty"`
	Remote   bool          `json:"remote"`
}

type TestSummary struct {
	Timestamp    time.Time     `json:"timestamp"`
	TotalTests   int           `json:"total_tests"`
	Passed       int           `json:"passed"`
	Failed       int           `json:"failed"`
	Skipped      int           `json:"skipped"`
	Duration     time.Duration `json:"duration"`
	Distribution DistInfo      `json:"distribution"`
	Results      []TestResult  `json:"results"`
}

type DistInfo struct {
	Enabled     bool     `json:"enabled"`
	Scheduler   string   `json:"scheduler"`
	Hosts       []string `json:"hosts"`
	LocalCount  int      `json:"local_count"`
	RemoteCount int      `json:"remote_count"`
}

type cliLogger struct {
	verbose bool
	prefix  string
}

func (l *cliLogger) Info(msg string, args ...any) {
	fmt.Printf("[%sINFO]  %s", l.prefix, msg)
	l.printArgs(args)
	fmt.Println()
}

func (l *cliLogger) Warn(msg string, args ...any) {
	fmt.Printf("[%sWARN]  %s", l.prefix, msg)
	l.printArgs(args)
	fmt.Println()
}

func (l *cliLogger) Error(msg string, args ...any) {
	fmt.Fprintf(os.Stderr, "[%sERROR] %s", l.prefix, msg)
	l.printArgs(args)
	fmt.Fprintln(os.Stderr)
}

func (l *cliLogger) Debug(msg string, args ...any) {
	if l.verbose {
		fmt.Printf("[%sDEBUG] %s", l.prefix, msg)
		l.printArgs(args)
		fmt.Println()
	}
}

func (l *cliLogger) printArgs(args []any) {
	if len(args) == 0 {
		return
	}
	if len(args)%2 == 0 {
		for i := 0; i < len(args); i += 2 {
			fmt.Printf(" %v=%v", args[i], args[i+1])
		}
	} else {
		for _, a := range args {
			fmt.Printf(" %v", a)
		}
	}
}

type DistributedTestRunner struct {
	logger       *cliLogger
	projectRoot  string
	outputDir    string
	distributor  *distribution.DefaultDistributor
	hostManager  remote.HostManager
	sshExecutor  *remote.SSHExecutor
	distConfig   *envconfig.DistributionConfig
	localRuntime runtime.ContainerRuntime
	results      []TestResult
	mu           sync.Mutex
}

func main() {
	os.Exit(run())
}

func run() int {
	flag.Parse()

	logger := &cliLogger{verbose: *flagVerbose, prefix: "DTEST "}

	logger.Info("Distributed Test Runner starting")
	logger.Info("Configuration",
		"env", *flagEnvFile,
		"root", *flagProjectRoot,
		"output", *flagOutput,
		"timeout", *flagTimeout,
		"platform", *flagPlatform,
	)

	projectRoot, err := filepath.Abs(*flagProjectRoot)
	if err != nil {
		logger.Error("resolve project root", "error", err)
		return exitError
	}

	outputDir := filepath.Join(projectRoot, *flagOutput)
	if err := os.MkdirAll(outputDir, 0o755); err != nil {
		logger.Error("create output dir", "error", err)
		return exitError
	}

	ctx, cancel := context.WithTimeout(context.Background(), *flagTimeout)
	defer cancel()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		select {
		case sig := <-sigCh:
			logger.Warn("received signal, shutting down", "signal", sig)
			cancel()
		case <-ctx.Done():
		}
	}()

	runner, err := NewDistributedTestRunner(logger, projectRoot, outputDir)
	if err != nil {
		logger.Error("create runner", "error", err)
		return exitError
	}

	if *flagDeployOnly {
		logger.Info("deploy-only mode: deploying infrastructure")
		if err := runner.DeployInfrastructure(ctx); err != nil {
			logger.Error("deploy infrastructure", "error", err)
			return exitError
		}
		logger.Info("infrastructure deployed successfully")
		return exitSuccess
	}

	summary, err := runner.RunAll(ctx)
	if err != nil {
		logger.Error("run tests", "error", err)
	}

	if err := runner.GenerateReport(summary, outputDir); err != nil {
		logger.Warn("generate report", "error", err)
	}

	printSummary(summary, logger)

	if summary.Failed > 0 {
		return exitFailures
	}
	if err != nil {
		return exitError
	}
	return exitSuccess
}

func NewDistributedTestRunner(logger *cliLogger, projectRoot, outputDir string) (*DistributedTestRunner, error) {
	runner := &DistributedTestRunner{
		logger:      logger,
		projectRoot: projectRoot,
		outputDir:   outputDir,
		results:     make([]TestResult, 0),
	}

	envPath := filepath.Join(projectRoot, *flagEnvFile)
	if _, err := os.Stat(envPath); os.IsNotExist(err) {
		logger.Warn("distribution config not found, running locally", "path", envPath)
		runner.distConfig = &envconfig.DistributionConfig{Enabled: false}
	} else {
		cfg, err := envconfig.LoadFromFile(envPath)
		if err != nil {
			return nil, fmt.Errorf("load config: %w", err)
		}
		if *flagLocalOnly {
			cfg.Enabled = false
			logger.Info("local-only mode: remote distribution disabled")
		}
		runner.distConfig = cfg
	}

	if runner.distConfig.Enabled && len(runner.distConfig.Hosts) > 0 {
		logger.Info("remote distribution enabled",
			"scheduler", runner.distConfig.Scheduler,
			"hosts", len(runner.distConfig.Hosts),
		)

		sshExec, err := remote.NewSSHExecutor(logging.NopLogger{})
		if err != nil {
			logger.Warn("SSH executor creation failed, falling back to local", "error", err)
			runner.distConfig.Enabled = false
		} else {
			runner.sshExecutor = sshExec

			hm := remote.NewHostManager(sshExec, logging.NopLogger{})
			for _, h := range runner.distConfig.ToRemoteHosts() {
				hm.AddHost(h)
				logger.Info("added remote host", "name", h.Name, "address", h.Address)
			}
			runner.hostManager = hm

			detectCtx, detectCancel := context.WithTimeout(context.Background(), 10*time.Second)
			localRt, err := runtime.AutoDetect(detectCtx)
			detectCancel()
			if err != nil {
				logger.Warn("local runtime detection failed", "error", err)
			} else {
				runner.localRuntime = localRt
				logger.Info("detected local runtime", "runtime", localRt.Name())
			}

			sched := createScheduler(runner.distConfig.Scheduler, hm, logging.NopLogger{})
			runner.distributor = distribution.NewDistributor(
				distribution.WithScheduler(sched),
				distribution.WithExecutor(sshExec),
				distribution.WithHostManager(hm),
				distribution.WithLocalRuntime(localRt),
				distribution.WithLogger(logging.NopLogger{}),
			)
		}
	}

	return runner, nil
}

func createScheduler(strategy string, hm remote.HostManager, logger logging.Logger) scheduler.Scheduler {
	var strategyEnum scheduler.PlacementStrategy
	switch strategy {
	case "round_robin":
		strategyEnum = scheduler.StrategyRoundRobin
	case "affinity":
		strategyEnum = scheduler.StrategyAffinity
	case "spread":
		strategyEnum = scheduler.StrategySpread
	case "bin_pack":
		strategyEnum = scheduler.StrategyBinPack
	default:
		strategyEnum = scheduler.StrategyResourceAware
	}
	return scheduler.NewScheduler(hm, logger, scheduler.WithStrategy(strategyEnum))
}

func (r *DistributedTestRunner) DeployInfrastructure(ctx context.Context) error {
	if !r.distConfig.Enabled || r.distributor == nil {
		r.logger.Info("running locally, no remote deployment needed")
		return nil
	}

	reqs := []scheduler.ContainerRequirements{
		{Name: "postgres", Image: "docker.io/library/postgres:15-alpine", MemoryMB: 1024, CPUCores: 1.0},
		{Name: "redis", Image: "docker.io/library/redis:7-alpine", MemoryMB: 512, CPUCores: 0.5},
	}

	r.logger.Info("distributing test infrastructure", "services", len(reqs))
	summary, err := r.distributor.Distribute(ctx, reqs)
	if err != nil {
		return fmt.Errorf("distribute: %w", err)
	}

	r.logger.Info("distribution complete",
		"local", summary.LocalContainers,
		"remote", summary.RemoteContainers,
		"failed", summary.FailedContainers,
		"duration", summary.Duration,
	)

	if summary.FailedContainers > 0 {
		for _, c := range summary.Containers {
			if c.State == distribution.StateFailed {
				r.logger.Error("container failed", "name", c.Requirement.Name, "error", c.Error)
			}
		}
		return fmt.Errorf("%d containers failed to deploy", summary.FailedContainers)
	}

	time.Sleep(5 * time.Second)

	return nil
}

func (r *DistributedTestRunner) RunAll(ctx context.Context) (*TestSummary, error) {
	start := time.Now()
	summary := &TestSummary{
		Timestamp: time.Now(),
		Distribution: DistInfo{
			Enabled:   r.distConfig.Enabled,
			Scheduler: r.distConfig.Scheduler,
		},
		Results: make([]TestResult, 0),
	}

	for _, h := range r.distConfig.Hosts {
		summary.Distribution.Hosts = append(summary.Distribution.Hosts, h.Name)
	}

	if err := r.DeployInfrastructure(ctx); err != nil {
		r.logger.Error("infrastructure deployment failed", "error", err)
	}

	var wg sync.WaitGroup
	var runErr error

	if !*flagSkipGo {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := r.runGoTests(ctx)
			r.addResult(result)
		}()
	}

	if !*flagSkipJS {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, result := range r.runJSTests(ctx) {
				r.addResult(result)
			}
		}()
	}

	if !*flagSkipAndroid {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for _, result := range r.runAndroidTests(ctx) {
				r.addResult(result)
			}
		}()
	}

	if !*flagSkipChallenges {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result := r.runChallenges(ctx)
			r.addResult(result)
		}()
	}

	wg.Wait()

	r.mu.Lock()
	summary.Results = r.results
	for _, res := range r.results {
		summary.TotalTests++
		switch res.Status {
		case "passed":
			summary.Passed++
		case "failed":
			summary.Failed++
		case "skipped":
			summary.Skipped++
		}
		if res.Remote {
			summary.Distribution.RemoteCount++
		} else {
			summary.Distribution.LocalCount++
		}
	}
	r.mu.Unlock()

	summary.Duration = time.Since(start)
	return summary, runErr
}

func (r *DistributedTestRunner) addResult(result TestResult) {
	r.mu.Lock()
	r.results = append(r.results, result)
	r.mu.Unlock()
}

func (r *DistributedTestRunner) runGoTests(ctx context.Context) TestResult {
	result := TestResult{
		Name:     "go-backend",
		Platform: "api",
		Status:   "passed",
	}
	start := time.Now()

	r.logger.Info("running Go backend tests")

	if *flagGoAPIDir == "" {
		result.Status = "skipped"
		result.Error = "no --go-api-dir supplied; skipping Go tests"
		return result
	}
	apiDir := filepath.Join(r.projectRoot, *flagGoAPIDir)
	cmd := exec.CommandContext(ctx, "go", "test", "./...", "-v", "-count=1", "-timeout=10m")
	cmd.Dir = apiDir
	cmd.Env = append(os.Environ(),
		"GOMAXPROCS=3",
		"CGO_ENABLED=1",
	)

	output, err := cmd.CombinedOutput()
	result.Duration = time.Since(start)
	result.Output = string(output)

	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		r.logger.Error("Go tests failed", "error", err)
	} else {
		r.logger.Info("Go tests passed", "duration", result.Duration)
	}

	return result
}

func (r *DistributedTestRunner) runJSTests(ctx context.Context) []TestResult {
	projects := splitAndTrim(*flagJSProjects)
	if len(projects) == 0 {
		return nil
	}
	var results []TestResult

	for _, project := range projects {
		result := TestResult{
			Name:     fmt.Sprintf("js-%s", project),
			Platform: "web",
			Status:   "skipped",
		}

		projectDir := filepath.Join(r.projectRoot, project)
		if _, err := os.Stat(filepath.Join(projectDir, "package.json")); os.IsNotExist(err) {
			r.logger.Debug("skipping project, no package.json", "project", project)
			results = append(results, result)
			continue
		}

		start := time.Now()
		r.logger.Info("running JS tests", "project", project)

		if _, err := os.Stat(filepath.Join(projectDir, "node_modules")); os.IsNotExist(err) {
			installCmd := exec.CommandContext(ctx, "npm", "install", "--silent")
			installCmd.Dir = projectDir
			if err := installCmd.Run(); err != nil {
				result.Status = "failed"
				result.Error = fmt.Sprintf("npm install: %v", err)
				result.Duration = time.Since(start)
				results = append(results, result)
				continue
			}
		}

		cmd := exec.CommandContext(ctx, "npm", "run", "test", "--", "--run")
		cmd.Dir = projectDir
		output, err := cmd.CombinedOutput()

		result.Duration = time.Since(start)
		result.Output = string(output)

		if err != nil {
			if strings.Contains(string(output), "no test specified") {
				result.Status = "skipped"
				r.logger.Debug("no tests configured", "project", project)
			} else {
				result.Status = "failed"
				result.Error = err.Error()
				r.logger.Error("JS tests failed", "project", project, "error", err)
			}
		} else {
			result.Status = "passed"
			r.logger.Info("JS tests passed", "project", project, "duration", result.Duration)
		}

		results = append(results, result)
	}

	return results
}

func (r *DistributedTestRunner) runAndroidTests(ctx context.Context) []TestResult {
	projects := splitAndTrim(*flagAndroidProjects)
	if len(projects) == 0 {
		return nil
	}
	var results []TestResult

	for _, project := range projects {
		result := TestResult{
			Name:     fmt.Sprintf("android-%s", project),
			Platform: "android",
			Status:   "skipped",
		}

		projectDir := filepath.Join(r.projectRoot, project)
		gradlew := filepath.Join(projectDir, "gradlew")
		if _, err := os.Stat(gradlew); os.IsNotExist(err) {
			r.logger.Debug("skipping project, no gradlew", "project", project)
			results = append(results, result)
			continue
		}

		start := time.Now()
		r.logger.Info("running Android tests", "project", project)

		cmd := exec.CommandContext(ctx, "./gradlew", "testDebugUnitTest", "--no-daemon")
		cmd.Dir = projectDir
		output, err := cmd.CombinedOutput()

		result.Duration = time.Since(start)
		result.Output = string(output)

		if err != nil {
			result.Status = "failed"
			result.Error = err.Error()
			r.logger.Error("Android tests failed", "project", project, "error", err)
		} else {
			result.Status = "passed"
			r.logger.Info("Android tests passed", "project", project, "duration", result.Duration)
		}

		results = append(results, result)
	}

	return results
}

func (r *DistributedTestRunner) runChallenges(ctx context.Context) TestResult {
	result := TestResult{
		Name:     "challenges",
		Platform: *flagPlatform,
		Status:   "passed",
	}
	start := time.Now()

	r.logger.Info("running challenges", "platform", *flagPlatform)

	runnerDir := filepath.Join(r.projectRoot, "Challenges", "cmd", "userflow-runner")
	if _, err := os.Stat(runnerDir); os.IsNotExist(err) {
		result.Status = "skipped"
		result.Error = "challenge runner not found"
		r.logger.Warn("skipping challenges, runner not found")
		return result
	}

	args := []string{
		"run", ".",
		"--platform", *flagPlatform,
		"--report", "json",
		"--output", filepath.Join(r.outputDir, "challenges"),
		"--root", r.projectRoot,
		"--timeout", "20m",
	}
	if *flagVerbose {
		args = append(args, "--verbose")
	}

	cmd := exec.CommandContext(ctx, "go", args...)
	cmd.Dir = runnerDir
	cmd.Env = append(os.Environ(),
		"GOMAXPROCS=2",
		"CGO_ENABLED=1",
	)

	output, err := cmd.CombinedOutput()
	result.Duration = time.Since(start)
	result.Output = string(output)

	if err != nil {
		result.Status = "failed"
		result.Error = err.Error()
		r.logger.Error("Challenges failed", "error", err)
	} else {
		r.logger.Info("Challenges passed", "duration", result.Duration)
	}

	return result
}

func (r *DistributedTestRunner) GenerateReport(summary *TestSummary, outputDir string) error {
	reportPath := filepath.Join(outputDir, "test-report.json")
	data, err := json.MarshalIndent(summary, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}
	if err := os.WriteFile(reportPath, data, 0o644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}

	htmlReport := generateHTMLReport(summary)
	htmlPath := filepath.Join(outputDir, "test-report.html")
	if err := os.WriteFile(htmlPath, []byte(htmlReport), 0o644); err != nil {
		return fmt.Errorf("write HTML report: %w", err)
	}

	r.logger.Info("reports generated", "json", reportPath, "html", htmlPath)
	return nil
}

func generateHTMLReport(summary *TestSummary) string {
	var resultsHTML string
	for _, r := range summary.Results {
		statusClass := "status-pass"
		if r.Status == "failed" {
			statusClass = "status-fail"
		} else if r.Status == "skipped" {
			statusClass = "status-skip"
		}
		resultsHTML += fmt.Sprintf(`
			<tr>
				<td>%s</td>
				<td>%s</td>
				<td class="%s">%s</td>
				<td>%s</td>
				<td>%s</td>
			</tr>`, r.Name, r.Platform, statusClass, strings.ToUpper(r.Status), r.Duration.Round(time.Millisecond), r.Host)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
	<title>Distributed Test Report - %s</title>
	<style>
		body { font-family: system-ui, sans-serif; margin: 40px; background: #f5f5f5; }
		.container { max-width: 1200px; margin: 0 auto; background: white; padding: 30px; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
		h1 { color: #333; border-bottom: 2px solid #4CAF50; padding-bottom: 10px; }
		.metrics { display: grid; grid-template-columns: repeat(5, 1fr); gap: 20px; margin: 20px 0; }
		.metric { text-align: center; padding: 20px; background: #f9f9f9; border-radius: 8px; }
		.metric h3 { margin: 0; color: #666; font-size: 14px; }
		.metric p { margin: 10px 0 0; font-size: 28px; font-weight: bold; }
		.pass { color: #4CAF50; }
		.fail { color: #f44336; }
		.skip { color: #ff9800; }
		table { width: 100%%; border-collapse: collapse; margin-top: 20px; }
		th, td { padding: 12px; text-align: left; border-bottom: 1px solid #ddd; }
		th { background: #f5f5f5; font-weight: 600; }
		.status-pass { color: #4CAF50; font-weight: bold; }
		.status-fail { color: #f44336; font-weight: bold; }
		.status-skip { color: #ff9800; }
		.dist-info { background: #e3f2fd; padding: 15px; border-radius: 8px; margin: 20px 0; }
	</style>
</head>
<body>
	<div class="container">
		<h1>Distributed Test Report</h1>
		<p>Generated: %s | Duration: %s</p>
		
		<div class="dist-info">
			<strong>Distribution:</strong> Enabled=%v | Scheduler=%s | Hosts=%s | Local=%d | Remote=%d
		</div>
		
		<div class="metrics">
			<div class="metric"><h3>Total</h3><p>%d</p></div>
			<div class="metric"><h3>Passed</h3><p class="pass">%d</p></div>
			<div class="metric"><h3>Failed</h3><p class="fail">%d</p></div>
			<div class="metric"><h3>Skipped</h3><p class="skip">%d</p></div>
			<div class="metric"><h3>Duration</h3><p>%s</p></div>
		</div>
		
		<h2>Results</h2>
		<table>
			<thead><tr><th>Test</th><th>Platform</th><th>Status</th><th>Duration</th><th>Host</th></tr></thead>
			<tbody>%s</tbody>
		</table>
	</div>
</body>
</html>`,
		summary.Timestamp.Format("2006-01-02 15:04:05"),
		summary.Timestamp.Format("15:04:05"),
		summary.Duration.Round(time.Second),
		summary.Distribution.Enabled,
		summary.Distribution.Scheduler,
		strings.Join(summary.Distribution.Hosts, ", "),
		summary.Distribution.LocalCount,
		summary.Distribution.RemoteCount,
		summary.TotalTests,
		summary.Passed,
		summary.Failed,
		summary.Skipped,
		summary.Duration.Round(time.Second),
		resultsHTML,
	)
}

func printSummary(summary *TestSummary, logger *cliLogger) {
	fmt.Println()
	fmt.Println("========================================")
	fmt.Println("  Distributed Test Runner - Summary")
	fmt.Println("========================================")
	fmt.Printf("  Total:      %d tests\n", summary.TotalTests)
	fmt.Printf("  Passed:     %d\n", summary.Passed)
	fmt.Printf("  Failed:     %d\n", summary.Failed)
	fmt.Printf("  Skipped:    %d\n", summary.Skipped)
	fmt.Printf("  Duration:   %s\n", summary.Duration.Round(time.Second))
	fmt.Println()
	fmt.Printf("  Distribution: enabled=%v hosts=%v\n",
		summary.Distribution.Enabled,
		summary.Distribution.Hosts,
	)
	fmt.Println("========================================")

	if summary.Failed > 0 {
		fmt.Println()
		fmt.Println("Failed Tests:")
		for _, r := range summary.Results {
			if r.Status == "failed" {
				fmt.Printf("  - %s: %s\n", r.Name, r.Error)
			}
		}
	}
}

// splitAndTrim splits a comma-separated flag value, trims whitespace
// from each entry, and discards empty entries. Used by the project-
// layout flags (--js-projects, --android-projects, …) so the CLI
// stays project-agnostic.
func splitAndTrim(s string) []string {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := parts[:0]
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
