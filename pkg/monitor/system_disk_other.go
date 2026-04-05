//go:build !linux

package monitor

import "os"

// collectDiskLinuxFromPath is a no-op on non-Linux platforms.
// Real disk stats require syscall.Statfs which is Linux-specific.
func (c *DefaultSystemCollector) collectDiskLinuxFromPath(
	res *SystemResources,
	path string,
) {
	// Verify the path is accessible; leave disk stats at zero.
	info, err := os.Stat(path)
	if err != nil || info == nil {
		return
	}
}
