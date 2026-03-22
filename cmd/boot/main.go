package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"digital.vasic.containers/pkg/boot"
	"digital.vasic.containers/pkg/distribution"
	"digital.vasic.containers/pkg/endpoint"
	"digital.vasic.containers/pkg/envconfig"
	"digital.vasic.containers/pkg/logging"
	"digital.vasic.containers/pkg/remote"
	"digital.vasic.containers/pkg/runtime"
	"digital.vasic.containers/pkg/scheduler"
)

var (
	flagEnvFile = flag.String("env", "", "Path to .env file (default: ./pkg/envconfig/.env)")
	flagProject = flag.String("project", "", "Path to project directory")
	flagTimeout = flag.Duration("timeout", 5*time.Minute, "Boot timeout")
	flagHelp    = flag.Bool("help", false, "Show help")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: boot [options]\n\n")
		fmt.Fprintf(os.Stderr, "Boot services using the Containers module.\n")
		fmt.Fprintf(os.Stderr, "Distributes containers to remote hosts based on .env configuration.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  boot                           # Boot with default .env\n")
		fmt.Fprintf(os.Stderr, "  boot --env /path/to/.env      # Custom env file\n")
		fmt.Fprintf(os.Stderr, "  boot --project /my/project     # Custom project path\n")
	}

	flag.Parse()

	if *flagHelp {
		flag.Usage()
		os.Exit(0)
	}

	envFile := *flagEnvFile
	if envFile == "" {
		locations := []string{
			"../../../tools/containers/.env",
			"../../.env",
			"../.env",
			"./.env",
		}
		for _, loc := range locations {
			if _, err := os.Stat(loc); err == nil {
				envFile = loc
				break
			}
		}
	}

	projectDir := *flagProject
	if projectDir == "" {
		projectDir = "../../../"
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("\nShutting down...")
		cancel()
	}()

	if err := runBoot(ctx, envFile, projectDir); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}

type distributorAdapter struct {
	*distribution.DefaultDistributor
}

func (d *distributorAdapter) DistributeEndpoints(ctx context.Context, names []string) (int, error) {
	return 0, nil
}

func runBoot(ctx context.Context, envFile, projectDir string) error {
	logger := logging.NewStdLogger("boot")
	logger.Info("Starting boot process...")

	var cfg *envconfig.DistributionConfig
	var err error

	if envFile != "" {
		cfg, err = envconfig.LoadFromFile(envFile)
		if err != nil {
			return fmt.Errorf("load env config: %w", err)
		}
	} else {
		cfg = envconfig.LoadFromEnv()
		logger.Info("No .env file found, using local mode")
	}

	logger.Info("Configuration loaded: remote=%v, hosts=%d, scheduler=%s",
		cfg.Enabled, len(cfg.Hosts), cfg.Scheduler)

	rt, err := runtime.AutoDetect(ctx)
	if err != nil {
		return fmt.Errorf("auto-detect runtime: %w", err)
	}
	logger.Info("Using runtime: %s", rt.Name())

	exec, err := remote.NewSSHExecutor(logger)
	if err != nil {
		return fmt.Errorf("create SSH executor: %w", err)
	}

	hostManager := remote.NewHostManager(exec, logger)
	for _, host := range cfg.ToRemoteHosts() {
		if err := hostManager.AddHost(host); err != nil {
			logger.Warn("Failed to add host %s: %v", host.Name, err)
			continue
		}
		logger.Info("Registered remote host: %s (%s)", host.Name, host.Address)
	}

	sched := scheduler.NewScheduler(hostManager, logger)

	var distributor boot.Distributor
	if cfg.Enabled && len(cfg.Hosts) > 0 {
		defaultDist := distribution.NewDistributor(
			distribution.WithScheduler(sched),
			distribution.WithHostManager(hostManager),
			distribution.WithLogger(logger),
		)
		distributor = &distributorAdapter{DefaultDistributor: defaultDist}
		logger.Info("Remote distribution enabled with scheduler: %s", cfg.Scheduler)
	}

	endpoints := map[string]endpoint.ServiceEndpoint{
		"helixagent": {
			Host:    "localhost",
			Port:    "7061",
			Enabled: true,
		},
	}

	bm := boot.NewBootManager(
		endpoints,
		boot.WithRuntime(rt),
		boot.WithLogger(logger),
		boot.WithProjectDir(projectDir),
		boot.WithDistributor(distributor),
		boot.WithHostManager(hostManager),
		boot.WithScheduler(sched),
	)

	logger.Info("Booting services...")
	bootCtx, bootCancel := context.WithTimeout(ctx, *flagTimeout)
	defer bootCancel()

	result, err := bm.BootAll(bootCtx)
	if err != nil {
		logger.Error("Boot failed: %v", err)
		return err
	}

	logger.Info("Boot completed: %d services processed", len(result.Results))
	for name, res := range result.Results {
		logger.Info("  - %s: %s (duration=%s)", name, res.Status, res.Duration)
	}

	return nil
}
