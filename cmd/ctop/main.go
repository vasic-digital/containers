package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"digital.vasic.containers/pkg/ctop"
	"digital.vasic.containers/pkg/envconfig"
	"digital.vasic.containers/pkg/logging"
	"digital.vasic.containers/pkg/remote"
)

var (
	flagOnce    = flag.Bool("once", false, "Print once and exit (no TUI)")
	flagJSON    = flag.Bool("json", false, "Output as JSON")
	flagHost    = flag.String("host", "", "Filter by host name")
	flagName    = flag.String("name", "", "Filter by container name")
	flagSort    = flag.String("sort", "cpu", "Sort by: cpu, mem, name, state, uptime, runtime, host")
	flagOrder   = flag.String("order", "desc", "Sort order: asc, desc")
	flagAll     = flag.Bool("all", false, "Show stopped containers")
	flagRefresh = flag.Int("refresh", 1000, "Refresh rate in milliseconds")
	flagNoColor = flag.Bool("no-color", false, "Disable colors")
	flagRuntime = flag.String("runtime", "", "Container runtime (auto-detected if empty)")
	flagHelp    = flag.Bool("help", false, "Show help")
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: ctop [options]\n\n")
		fmt.Fprintf(os.Stderr, "Container monitoring with top/htop-style display.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  ctop                  # Interactive TUI\n")
		fmt.Fprintf(os.Stderr, "  ctop --once           # One-time snapshot\n")
		fmt.Fprintf(os.Stderr, "  ctop --json           # JSON output\n")
		fmt.Fprintf(os.Stderr, "  ctop --host thinker   # Filter by host\n")
		fmt.Fprintf(os.Stderr, "  ctop --sort mem       # Sort by memory\n")
		fmt.Fprintf(os.Stderr, "  ctop --all            # Show stopped containers\n")
	}

	flag.Parse()

	if *flagHelp {
		flag.Usage()
		os.Exit(0)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	cfg := parseConfig()
	display := setupDisplay(cfg)

	if *flagOnce || *flagJSON {
		runOnce(ctx, display, *flagJSON)
	} else {
		runInteractive(ctx, display)
	}
}

type config struct {
	runtime    string
	sortBy     ctop.SortField
	sortOrder  ctop.SortOrder
	displayCfg ctop.DisplayConfig
}

func parseConfig() *config {
	cfg := &config{
		runtime: *flagRuntime,
	}

	sortMap := map[string]ctop.SortField{
		"cpu":     ctop.SortByCPU,
		"mem":     ctop.SortByMemory,
		"name":    ctop.SortByName,
		"state":   ctop.SortByState,
		"uptime":  ctop.SortByUptime,
		"runtime": ctop.SortByRuntime,
		"host":    ctop.SortByHost,
	}

	if sort, ok := sortMap[*flagSort]; ok {
		cfg.sortBy = sort
	} else {
		cfg.sortBy = ctop.SortByCPU
	}

	if *flagOrder == "asc" {
		cfg.sortOrder = ctop.SortAsc
	} else {
		cfg.sortOrder = ctop.SortDesc
	}

	cfg.displayCfg = ctop.DisplayConfig{
		SortBy:      cfg.sortBy,
		SortOrder:   cfg.sortOrder,
		ShowStopped: *flagAll,
		FilterHost:  *flagHost,
		FilterName:  *flagName,
		RefreshRate: *flagRefresh,
		NoColor:     *flagNoColor,
		Compact:     false,
	}

	return cfg
}

func setupDisplay(cfg *config) *ctop.Display {
	logger := logging.NopLogger{}

	var hostManager remote.HostManager
	var sshExecutor *remote.SSHExecutor

	envCfg := envconfig.LoadFromEnv()
	if envCfg.Enabled && len(envCfg.Hosts) > 0 {
		hosts := envCfg.ToRemoteHosts()

		sshExec, err := remote.NewSSHExecutor(logger)
		if err == nil {
			sshExecutor = sshExec
			hm := remote.NewHostManager(sshExecutor, logger)
			for _, h := range hosts {
				hm.AddHost(h)
			}
			hostManager = hm
		}
	}

	var collector *ctop.Collector
	if sshExecutor != nil && hostManager != nil {
		collector = ctop.NewCollectorWithSSH(cfg.runtime, hostManager, sshExecutor)
	} else if hostManager != nil {
		collector = ctop.NewCollector(cfg.runtime, hostManager)
	} else {
		collector = ctop.NewCollector(cfg.runtime, nil)
	}

	display := ctop.NewDisplay(collector, cfg.displayCfg)

	if *flagHost != "" {
		display.SetFilterHost(*flagHost)
	}
	if *flagName != "" {
		display.SetFilterName(*flagName)
	}
	display.SetSortBy(cfg.sortBy)
	display.SetSortOrder(cfg.sortOrder)

	return display
}

func runOnce(ctx context.Context, display *ctop.Display, jsonOutput bool) {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	var output string
	var err error

	if jsonOutput {
		output, err = display.RenderJSON(ctx)
	} else {
		output, err = display.RenderSnapshot(ctx)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println(output)
}

func runInteractive(ctx context.Context, display *ctop.Display) {
	if err := display.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
}
