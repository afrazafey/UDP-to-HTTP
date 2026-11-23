//go:build windows

package operations

import (
	"golang.org/x/sys/windows"
	"unsafe"
)

var processMemoryInfo = windows.NewLazySystemDLL("kernel32.dll").NewProc("K32GetProcessMemoryInfo")

type memoryCounters struct {
	Size, PageFaults                                                                                             uint32
	PeakWorkingSet, WorkingSet, PeakPagedPool, PagedPool, PeakNonPagedPool, NonPagedPool, PageFile, PeakPageFile uintptr
}

func processUsage() (*float64, *uint64) {
	var cpu *float64
	var rss *uint64
	var created, exited, kernel, user windows.Filetime
	if err := windows.GetProcessTimes(windows.CurrentProcess(), &created, &exited, &kernel, &user); err == nil {
		k := uint64(kernel.HighDateTime)<<32 | uint64(kernel.LowDateTime)
		u := uint64(user.HighDateTime)<<32 | uint64(user.LowDateTime)
		seconds := float64(k+u) / 1e7
		cpu = &seconds
	}
	memory := memoryCounters{}
	memory.Size = uint32(unsafe.Sizeof(memory))
	if processMemoryInfo.Find() == nil {
		ok, _, _ := processMemoryInfo.Call(uintptr(windows.CurrentProcess()), uintptr(unsafe.Pointer(&memory)), uintptr(memory.Size))
		if ok != 0 {
			value := uint64(memory.WorkingSet)
			rss = &value
		}
	}
	return cpu, rss
}
