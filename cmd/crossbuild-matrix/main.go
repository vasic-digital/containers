// SPDX-License-Identifier: Apache-2.0

// cmd/crossbuild-matrix — unified multi-platform build orchestrator.
//
// Orchestrates cross-platform artifact builds across all supported
// targets (linux, windows, web/wasm, macos, ios) in a single command.
// Each target is delegated to the corresponding pkg/crossbuild or
// pkg/vm backend; the operator never needs to invoke backends directly.
//
// # Usage
//
//	crossbuild-matrix \
//	  --targets linux,windows,web,macos,ios \
//	  --project /absolute/path/to/consumer-project \
//	  --output /absolute/path/to/output-dir \
//	  [--linux-task  "./gradlew :desktopApp:packageReleaseDeb"] \
//	  [--windows-task "./gradlew :desktopApp:packageReleaseMsi"] \
//	  [--web-task    "./gradlew :webApp:wasmJsBrowserDistribution"] \
//	  [--macos-image  "ghcr.io/example/macos-sonoma-jdk21:latest"] \
//	  [--macos-task   "./gradlew :desktopApp:packageDmg"] \
//	  [--ios-scheme   "MyApp"] \
//	  [--timeout-minutes 60]
//
// # Exit codes
//
//	0 — every requested target succeeded
//	1 — at least one target failed (summary printed to stderr)
//	2 — invalid CLI arguments
//
// # Constraints documented honestly (CONST-039)
//
//	linux   — requires container image provisioned per
//	          docs/crossbuild/linux-image-provisioning.md
//	          (SKIP-OK: #crossbuild-linux-image-provisioning)
//	windows — requires Wine container image provisioned per
//	          docs/crossbuild/windows-image-provisioning.md
//	          (SKIP-OK: #crossbuild-windows-image-provisioning)
//	web     — requires web-wasm container image provisioned per
//	          docs/crossbuild/web-wasm-image-provisioning.md; also
//	          requires consumer project KGP > 2.1.x
//	          (SKIP-OK: #crossbuild-web-wasm-image-provisioning /
//	           #wasmjs-production-distribution-gap)
//	macos   — requires macOS Apple Silicon host + Tart installed
//	          (SKIP-OK: #tart-requires-macos-apple-silicon)
//	ios     — requires macOS host + Xcode installed
//	          (SKIP-OK: #ios-build-requires-xcode-macos)
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"digital.vasic.containers/pkg/crossbuild"
	"digital.vasic.containers/pkg/vm/ios"
	"digital.vasic.containers/pkg/vm/macos"
)

func main() {
	os.Exit(run())
}

func run() int {
	targetsFlag := flag.String("targets", "linux,windows,web,macos,ios",
		"Comma-separated list of targets to build. Valid values: linux,windows,web,macos,ios")
	projectFlag := flag.String("project", "",
		"Absolute path to the consumer project root (required)")
	outputFlag := flag.String("output", "",
		"Absolute path to the output directory for produced artifacts (required)")
	linuxTaskFlag := flag.String("linux-task", "./gradlew :desktopApp:packageReleaseDeb",
		"Gradle task to run for Linux .deb build")
	linuxSubpathFlag := flag.String("linux-subpath",
		"desktopApp/build/compose/binaries/main-release/deb",
		"OutputSubpath (relative to project dir) for Linux artifact")
	windowsTaskFlag := flag.String("windows-task", "./gradlew :desktopApp:packageReleaseMsi",
		"Gradle task to run for Windows .msi build")
	windowsSubpathFlag := flag.String("windows-subpath",
		"desktopApp/build/compose/binaries/main-release/msi",
		"OutputSubpath (relative to project dir) for Windows artifact")
	webTaskFlag := flag.String("web-task", "./gradlew :webApp:wasmJsBrowserDistribution",
		"Gradle task to run for Web/Wasm distribution build")
	webSubpathFlag := flag.String("web-subpath",
		"webApp/build/dist/wasmJs/productionExecutable/index.html",
		"OutputSubpath (relative to project dir) for primary Wasm artifact")
	macosImageFlag := flag.String("macos-image", "",
		"Tart VM image to use for macOS build (e.g. ghcr.io/cirruslabs/macos-sonoma-xcode:latest)")
	macosTaskFlag := flag.String("macos-task", "./gradlew :desktopApp:packageDmg",
		"Shell command to run inside the macOS Tart VM")
	iosProjectFlag := flag.String("ios-project", "",
		"Absolute path to the Xcode project file (.xcodeproj or .xcworkspace) for iOS build")
	iosSchemeFlag := flag.String("ios-scheme", "",
		"Xcode scheme for iOS build")
	iosExportPathFlag := flag.String("ios-export", "",
		"Directory where xcodebuild -exportArchive writes the .ipa")
	timeoutFlag := flag.Int("timeout-minutes", 60,
		"Per-target timeout in minutes")
	flag.Parse()

	if *projectFlag == "" {
		fmt.Fprintln(os.Stderr, "crossbuild-matrix: --project is required")
		flag.Usage()
		return 2
	}
	if *outputFlag == "" {
		fmt.Fprintln(os.Stderr, "crossbuild-matrix: --output is required")
		flag.Usage()
		return 2
	}

	targets := parseCSV(*targetsFlag)
	if len(targets) == 0 {
		fmt.Fprintln(os.Stderr, "crossbuild-matrix: --targets must be non-empty")
		return 2
	}

	if err := os.MkdirAll(*outputFlag, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "crossbuild-matrix: cannot create output dir %s: %v\n", *outputFlag, err)
		return 2
	}

	timeout := time.Duration(*timeoutFlag) * time.Minute
	ctx := context.Background()

	type targetResult struct {
		target string
		ok     bool
		detail string
	}
	var results []targetResult

	for _, t := range targets {
		fmt.Printf("\n=== crossbuild-matrix: target=%s ===\n", t)
		var ok bool
		var detail string

		switch strings.ToLower(strings.TrimSpace(t)) {
		case "linux":
			ok, detail = buildLinux(ctx, *projectFlag, *linuxTaskFlag, *linuxSubpathFlag, *outputFlag, timeout)
		case "windows":
			ok, detail = buildWindows(ctx, *projectFlag, *windowsTaskFlag, *windowsSubpathFlag, *outputFlag, timeout)
		case "web", "wasm", "web-wasm":
			ok, detail = buildWeb(ctx, *projectFlag, *webTaskFlag, *webSubpathFlag, *outputFlag, timeout)
		case "macos", "darwin":
			ok, detail = buildMacOS(ctx, *macosImageFlag, *macosTaskFlag, *projectFlag, timeout)
		case "ios":
			ok, detail = buildIOS(ctx, *iosProjectFlag, *iosSchemeFlag, *iosExportPathFlag, *outputFlag, timeout)
		default:
			ok = false
			detail = fmt.Sprintf("unknown target %q; valid: linux,windows,web,macos,ios", t)
		}

		results = append(results, targetResult{target: t, ok: ok, detail: detail})
		if ok {
			fmt.Printf("  PASS: %s\n", detail)
		} else {
			fmt.Printf("  FAIL: %s\n", detail)
		}
	}

	// Summary report.
	fmt.Printf("\n=== crossbuild-matrix summary ===\n")
	anyFail := false
	for _, r := range results {
		status := "PASS"
		if !r.ok {
			status = "FAIL"
			anyFail = true
		}
		fmt.Printf("  %-12s %s  %s\n", r.target, status, r.detail)
	}

	if anyFail {
		fmt.Fprintln(os.Stderr, "\ncrossbuild-matrix: one or more targets failed (see above)")
		return 1
	}
	fmt.Println("\ncrossbuild-matrix: all targets succeeded")
	return 0
}

func buildLinux(ctx context.Context, projectDir, task, subpath, outputDir string, timeout time.Duration) (bool, string) {
	sel := crossbuild.NewSelectorForHost(runtime.GOOS, runtime.GOARCH)
	sel.Register(crossbuild.NewLinuxContainerBackend("", ""))
	result := sel.Build(ctx, crossbuild.BuildRequest{
		Target:        crossbuild.Target{OS: "linux", Arch: "amd64"},
		SourceDir:     projectDir,
		BuildCommand:  task,
		OutputSubpath: subpath,
		HostOutputDir: filepath.Join(outputDir, "linux"),
		Timeout:       timeout,
	})
	if result.Error != nil {
		return false, fmt.Sprintf("linux build failed: %v", result.Error)
	}
	return true, fmt.Sprintf("artifact: %s (%d bytes)", result.ArtifactPath, result.ArtifactSize)
}

func buildWindows(ctx context.Context, projectDir, task, subpath, outputDir string, timeout time.Duration) (bool, string) {
	sel := crossbuild.NewSelectorForHost("linux", "amd64") // Wine requires Linux host
	sel.Register(crossbuild.NewWineContainerBackend(""))
	result := sel.Build(ctx, crossbuild.BuildRequest{
		Target:        crossbuild.Target{OS: "windows", Arch: "amd64"},
		SourceDir:     projectDir,
		BuildCommand:  task,
		OutputSubpath: subpath,
		HostOutputDir: filepath.Join(outputDir, "windows"),
		Timeout:       timeout,
	})
	if result.Error != nil {
		return false, fmt.Sprintf("windows build failed: %v", result.Error)
	}
	return true, fmt.Sprintf("artifact: %s (%d bytes)", result.ArtifactPath, result.ArtifactSize)
}

func buildWeb(ctx context.Context, projectDir, task, subpath, outputDir string, timeout time.Duration) (bool, string) {
	sel := crossbuild.NewSelector()
	sel.Register(crossbuild.NewWebWasmContainerBackend("", nil))
	result := sel.Build(ctx, crossbuild.BuildRequest{
		Target:        crossbuild.Target{OS: "js", Arch: "wasm"},
		SourceDir:     projectDir,
		BuildCommand:  task,
		OutputSubpath: subpath,
		HostOutputDir: filepath.Join(outputDir, "web"),
		Timeout:       timeout,
	})
	if result.Error != nil {
		return false, fmt.Sprintf("web/wasm build failed: %v", result.Error)
	}
	return true, fmt.Sprintf("artifact: %s (%d bytes)", result.ArtifactPath, result.ArtifactSize)
}

func buildMacOS(ctx context.Context, image, command, mountDir string, timeout time.Duration) (bool, string) {
	if image == "" {
		return false, "macOS build skipped: --macos-image is required " +
			"(SKIP-OK: #tart-requires-macos-apple-silicon)"
	}
	b := macos.NewMacOSBuilder()
	result := b.RunInVM(ctx, macos.VMRunRequest{
		Image:    image,
		Command:  command,
		MountDir: mountDir,
		Timeout:  timeout,
	})
	if result.Error != nil {
		return false, fmt.Sprintf("macOS build failed: %v", result.Error)
	}
	return true, fmt.Sprintf("VM=%s exit=%d stdout=%d bytes",
		result.VMName, result.ExitCode, len(result.Stdout))
}

func buildIOS(ctx context.Context, projectFile, scheme, exportPath, outputDir string, timeout time.Duration) (bool, string) {
	if scheme == "" {
		return false, "iOS build skipped: --ios-scheme is required " +
			"(SKIP-OK: #ios-build-requires-xcode-macos)"
	}
	if projectFile == "" {
		return false, "iOS build skipped: --ios-project is required " +
			"(SKIP-OK: #ios-build-requires-xcode-macos)"
	}
	ep := exportPath
	if ep == "" {
		ep = filepath.Join(outputDir, "ios")
	}
	b := ios.NewIOSBuilder()
	result := b.BuildIPA(ctx, ios.BuildIPARequest{
		ProjectDir: projectFile,
		Scheme:     scheme,
		ExportPath: ep,
		Timeout:    timeout,
	})
	if result.Error != nil {
		return false, fmt.Sprintf("iOS build failed: %v", result.Error)
	}
	return true, fmt.Sprintf("ipa: %s", result.IPAPath)
}

func parseCSV(s string) []string {
	parts := strings.Split(s, ",")
	var out []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
