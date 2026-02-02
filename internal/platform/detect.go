package platform

import "runtime"

// IsLinux reports whether the current platform is Linux.
func IsLinux() bool {
	return runtime.GOOS == "linux"
}

// IsDarwin reports whether the current platform is macOS.
func IsDarwin() bool {
	return runtime.GOOS == "darwin"
}

// IsWindows reports whether the current platform is Windows.
func IsWindows() bool {
	return runtime.GOOS == "windows"
}
