//go:build windows

package postgres

import (
	"syscall"
	"unsafe"
)

var (
	kernel32            = syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceExW = kernel32.NewProc("GetDiskFreeSpaceExW")
)

// getAvailableSpace returns the available disk space in bytes for the given path.
func getAvailableSpace(path string) int64 {
	if path == "" {
		return 0
	}

	pathPtr, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0
	}

	var freeBytesAvailable, totalBytes, totalFreeBytes uint64
	ret, _, _ := getDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(&freeBytesAvailable)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)
	if ret == 0 {
		return 0
	}

	return int64(freeBytesAvailable)
}
