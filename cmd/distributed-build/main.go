package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"digital.vasic.containers/internal/buildpkg"
	"digital.vasic.containers/pkg/envconfig"
	"digital.vasic.containers/pkg/remote"
	"digital.vasic.containers/pkg/scheduler"
)

// convertBuildEnvVars converts BUILD_* environment variables to
// CONTAINERS_REMOTE_* equivalents for backward compatibility.
func convertBuildEnvVars() {
	for _, env := range os.Environ() {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) != 2 {
			continue
		}
		key := parts[0]
		value := parts[1]
		if !strings.HasPrefix(key, "BUILD_") {
			continue
		}
		// Mapping from BUILD_ to CONTAINERS_REMOTE_
		var newKey string
		switch {
		case key == "BUILD_DISTRIBUTED":
			newKey = "CONTAINERS_REMOTE_ENABLED"
		case key == "BUILD_SCHEDULER_STRATEGY":
			newKey = "CONTAINERS_REMOTE_SCHEDULER"
		case strings.HasPrefix(key, "BUILD_HOST_") && strings.HasSuffix(key, "_NAME"):
			newKey = strings.Replace(key, "BUILD_HOST_", "CONTAINERS_REMOTE_HOST_", 1)
		case strings.HasPrefix(key, "BUILD_HOST_") && strings.HasSuffix(key, "_ADDRESS"):
			newKey = strings.Replace(key, "BUILD_HOST_", "CONTAINERS_REMOTE_HOST_", 1)
		case strings.HasPrefix(key, "BUILD_HOST_") && strings.HasSuffix(key, "_USER"):
			newKey = strings.Replace(key, "BUILD_HOST_", "CONTAINERS_REMOTE_HOST_", 1)
		case strings.HasPrefix(key, "BUILD_HOST_") && strings.HasSuffix(key, "_KEY_PATH"):
			// Convert KEY_PATH to KEY
			base := strings.TrimSuffix(key, "_KEY_PATH")
			base = strings.TrimPrefix(base, "BUILD_HOST_")
			newKey = "CONTAINERS_REMOTE_HOST_" + base + "_KEY"
		case strings.HasPrefix(key, "BUILD_HOST_") && strings.HasSuffix(key, "_RUNTIME"):
			newKey = strings.Replace(key, "BUILD_HOST_", "CONTAINERS_REMOTE_HOST_", 1)
		case strings.HasPrefix(key, "BUILD_HOST_") && strings.HasSuffix(key, "_LABELS"):
			newKey = strings.Replace(key, "BUILD_HOST_", "CONTAINERS_REMOTE_HOST_", 1)
		default:
			// Unknown BUILD_ variable, ignore
			continue
		}
		// Only set if not already defined
		if os.Getenv(newKey) == "" {
			os.Setenv(newKey, value)
		}
	}
}

func main() {
	projectDir := flag.String("project", ".", "Path to Catalogizer project root")
	envFile := flag.String("env", ".env", "Path to .env file with host configuration")
	component := flag.String("component", "", "Build single component (default: all)")
	version := flag.String("version", "", "Version string (default: auto-detect)")
	skipTests := flag.Bool("skip-tests", false, "Skip test execution")
	dryRun := flag.Bool("dry-run", false, "Show plan without executing")
	timeoutMin := flag.Int("timeout", 30, "Build timeout in minutes")
	schedStrategy := flag.String("strategy", "resource_aware", "Scheduling strategy")
	flag.Parse()

	absProject, err := filepath.Abs(*projectDir)
	if err != nil {
		log.Fatalf("resolve project path: %v", err)
	}

	remoteDir := fmt.Sprintf("/tmp/catalogizer-build-%d", time.Now().UnixMilli())

	cfg, err := loadConfig(*envFile)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if len(cfg.Hosts) == 0 {
		log.Fatal("no remote hosts configured — set CONTAINERS_REMOTE_HOST_* env vars or use --env flag")
	}
	log.Printf("Loaded %d hosts:", len(cfg.Hosts))
	for i, h := range cfg.Hosts {
		log.Printf("  Host %d: %s (%s) runtime=%s labels=%v", i+1, h.Name, h.Address, h.Runtime, h.Labels)
	}

	sshExec, err := remote.NewSSHExecutor(nil)
	if err != nil {
		log.Fatalf("create SSH executor: %v", err)
	}
	defer sshExec.Close()

	hostMgr := remote.NewHostManager(sshExec, nil)

	for _, h := range cfg.ToRemoteHosts() {
		if err := hostMgr.AddHost(h); err != nil {
			log.Printf("warning: failed to register host %s: %v", h.Name, err)
		}
	}

	strategy := scheduler.PlacementStrategy(*schedStrategy)
	sched := scheduler.NewScheduler(hostMgr, nil, scheduler.WithStrategy(strategy))

	planner := buildpkg.NewPlannerWithScheduler(hostMgr, sched)

	ctx := context.Background()

	var plan *buildpkg.BuildPlan
	if *component != "" {
		plan, err = planner.PlanSingle(ctx, *component)
	} else {
		plan, err = planner.PlanAll(ctx)
	}
	if err != nil {
		log.Fatalf("create build plan: %v", err)
	}

	fmt.Println("=== Distributed Build Plan ===")
	for _, a := range plan.Assignments {
		fmt.Printf("  %s -> %s\n", a.Component.Name, a.Host)
	}

	if *dryRun {
		fmt.Println("(dry run)")
		return
	}

	buildExec := buildpkg.NewBuildExecutor(sshExec, absProject, remoteDir).
		WithBuildTimeout(time.Duration(*timeoutMin) * time.Minute)
	artifactColl := buildpkg.NewArtifactCollector(sshExec, absProject, remoteDir)

	syncedHosts := make(map[string]bool)
	for _, a := range plan.RemoteAssignments() {
		if syncedHosts[a.Host] {
			continue
		}
		host, err := hostMgr.GetHost(a.Host)
		if err != nil {
			log.Fatalf("get host %s: %v", a.Host, err)
		}
		fmt.Printf("Syncing source to %s...\n", a.Host)
		if err := buildExec.SyncSource(ctx, *host); err != nil {
			log.Fatalf("sync source to %s: %v", a.Host, err)
		}
		syncedHosts[a.Host] = true
	}

	for _, a := range plan.LocalAssignments() {
		fmt.Printf("  %s -> local (local builds handled by shell pipeline)\n", a.Component.Name)
	}

	var results []buildpkg.BuildResult
	for _, a := range plan.RemoteAssignments() {
		host, err := hostMgr.GetHost(a.Host)
		if err != nil {
			log.Fatalf("get host %s: %v", a.Host, err)
		}
		fmt.Printf("Building %s on %s...\n", a.Component.Name, a.Host)
		result, err := buildExec.LaunchRemoteBuild(ctx, *host, a.Component.Name, *version, *skipTests)
		if err != nil {
			log.Printf("build %s on %s failed: %v", a.Component.Name, a.Host, err)
			results = append(results, *result)
			continue
		}
		fmt.Printf("  %s: %s (%.1fs)\n", a.Component.Name, result.Status, result.Duration.Seconds())
		results = append(results, *result)

		if result.IsSuccess() {
			artifacts, err := artifactColl.DiscoverArtifacts(ctx, *host, a.Component.Name, *version)
			if err != nil {
				log.Printf("discover artifacts for %s on %s: %v", a.Component.Name, a.Host, err)
				continue
			}
			if len(artifacts) > 0 {
				if err := artifactColl.CollectArtifacts(ctx, *host, artifacts); err != nil {
					log.Printf("collect artifacts for %s on %s: %v", a.Component.Name, a.Host, err)
				} else {
					fmt.Printf("  collected %d artifact(s) for %s\n", len(artifacts), a.Component.Name)
				}
			}
		}
	}

	fmt.Println("\n=== Build Results ===")
	hasFailures := false
	for _, r := range results {
		status := string(r.Status)
		if r.Error != "" {
			status = fmt.Sprintf("%s (%s)", r.Status, r.Error)
		}
		fmt.Printf("  %-30s %s %.1fs\n", r.Component, status, r.Duration.Seconds())
		if r.IsFailure() {
			hasFailures = true
		}
	}

	for hostName := range syncedHosts {
		host, err := hostMgr.GetHost(hostName)
		if err != nil {
			continue
		}
		_, _ = sshExec.Execute(ctx, *host, fmt.Sprintf("rm -rf %s", remoteDir))
	}

	if hasFailures {
		os.Exit(1)
	}
}

func loadConfig(envFile string) (*envconfig.DistributionConfig, error) {
	// First, convert any existing BUILD_* environment variables
	convertBuildEnvVars()

	// If env file exists, load it and convert any BUILD_* variables from file
	if _, err := os.Stat(envFile); err == nil {
		log.Printf("Loading environment from %s", envFile)
		// Load .env file into environment variables
		if err := envconfig.LoadDotEnv(envFile); err != nil {
			return nil, fmt.Errorf("load %s: %w", envFile, err)
		}
		// Convert any BUILD_* variables that were loaded from the file
		convertBuildEnvVars()
	} else {
		log.Printf("Environment file %s does not exist: %v", envFile, err)
	}

	// Load configuration from environment variables
	cfg := envconfig.LoadFromEnv()
	if len(cfg.Hosts) == 0 {
		return nil, fmt.Errorf("no remote hosts configured — set CONTAINERS_REMOTE_HOST_* env vars or use --env flag")
	}
	return cfg, nil
}
