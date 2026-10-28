//go:build windows

package app

import (
	"syscall"
	"unsafe"
)

func diskSpace(path string) (free, total uint64, err error) {
	p, e := syscall.UTF16PtrFromString(path)
	if e != nil {
		return 0, 0, e
	}
	var allFree uint64
	proc := syscall.NewLazyDLL("kernel32.dll").NewProc("GetDiskFreeSpaceExW")
	r, _, e := proc.Call(uintptr(unsafe.Pointer(p)), uintptr(unsafe.Pointer(&free)), uintptr(unsafe.Pointer(&total)), uintptr(unsafe.Pointer(&allFree)))
	if r == 0 {
		err = e
	}
	return
}
func hostMemory() (uint64, uint64)  { return 0, 0 }
func cpuCounters() (uint64, uint64) { return 0, 0 }
