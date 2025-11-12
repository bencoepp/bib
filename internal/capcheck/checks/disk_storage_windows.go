//go:build windows

package checks

import (
	"syscall"
	"unsafe"
)

func statFS(path string) (fsStats, error) {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	getDiskFreeSpaceExW := kernel32.NewProc("GetDiskFreeSpaceExW")
	lpDirectoryName, _ := syscall.UTF16PtrFromString(path)
	var freeAvail, totalBytes, totalFreeBytes int64
	r1, _, err := getDiskFreeSpaceExW.Call(
		uintptr(unsafe.Pointer(lpDirectoryName)),
		uintptr(unsafe.Pointer(&freeAvail)),
		uintptr(unsafe.Pointer(&totalBytes)),
		uintptr(unsafe.Pointer(&totalFreeBytes)),
	)
	if r1 == 0 {
		return fsStats{}, err
	}
	return fsStats{
		Free:  uint64(totalFreeBytes),
		Total: uint64(totalBytes),
	}, nil
}
