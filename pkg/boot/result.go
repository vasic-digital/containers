package boot

import (
	"fmt"
	"strings"
	"time"
)

// BootResult captures the outcome of booting a single service.
type BootResult struct {
	// Name is the service identifier.
	Name string
	// Status is the outcome: "started", "remote", "discovered",
	// "failed", "skipped".
	Status string
	// Duration is how long the boot took.
	Duration time.Duration
	// Error holds the error when Status is "failed".
	Error error
}

// BootSummary aggregates boot results for all services.
type BootSummary struct {
	// Results holds per-service boot outcomes keyed by name.
	Results map[string]*BootResult
	// Started is the count of locally started services.
	Started int
	// Remote is the count of remote services verified.
	Remote int
	// Discovered is the count of discovered services.
	Discovered int
	// Failed is the count of services that failed to boot.
	Failed int
	// Skipped is the count of disabled services.
	Skipped int
	// TotalDuration is the wall-clock time for the entire boot.
	TotalDuration time.Duration
}

// String returns a human-readable summary.
func (s *BootSummary) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Boot summary: %d started, %d remote, ",
		s.Started, s.Remote)
	fmt.Fprintf(&b, "%d discovered, %d failed, %d skipped ",
		s.Discovered, s.Failed, s.Skipped)
	fmt.Fprintf(&b, "(total: %s)", s.TotalDuration.Round(
		time.Millisecond))
	return b.String()
}

// HasFailures reports whether any required service failed.
func (s *BootSummary) HasFailures() bool {
	return s.Failed > 0
}
