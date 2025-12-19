//go:build !windows

package sqlite

import "syscall"

// getAvailableSpace returns the available disk space in bytes for the given path.
func getAvailableSpace(path string) int64 {
	var stat syscall.Statfs_t
	if err := syscall.Statfs(path, &stat); err != nil {
		return 0
	}
	return int64(stat.Bavail) * int64(stat.Bsize)
}
