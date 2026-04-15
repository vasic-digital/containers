//go:build linux

package monitor

import "syscall"

// collectDiskLinuxFromPath reads disk stats from the specified path
// using syscall.Statfs. Separated for testability.
func (c *DefaultSystemCollector) collectDiskLinuxFromPath(
	res *SystemResources,
	path string,
) {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return
	}
	// Total and free in bytes using the filesystem block size.
	res.DiskTotal = stat.Blocks * uint64(stat.Bsize)
	free := stat.Bfree * uint64(stat.Bsize)
	res.DiskUsed = res.DiskTotal - free
	if res.DiskTotal > 0 {
		res.DiskPercent = float64(res.DiskUsed) / float64(res.DiskTotal) * 100
	}
}
