//go:build !windows

package checks

import (
	"syscall"
)

func statFS(path string) (fsStats, error) {
	var s syscall.Statfs_t
	err := syscall.Statfs(path, &s)
	if err != nil {
		return fsStats{}, err
	}
	return fsStats{
		Free:  uint64(s.Bavail) * uint64(s.Bsize),
		Total: uint64(s.Blocks) * uint64(s.Bsize),
	}, nil
}
