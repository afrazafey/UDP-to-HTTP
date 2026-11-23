//go:build linux

package operations

import (
	"os"
	"strconv"
	"strings"

	"golang.org/x/sys/unix"
)

func processUsage() (*float64, *uint64) {
	var cpu *float64
	var rss *uint64
	var usage unix.Rusage
	if unix.Getrusage(unix.RUSAGE_SELF, &usage) == nil {
		value := float64(usage.Utime.Sec+usage.Stime.Sec) + float64(usage.Utime.Usec+usage.Stime.Usec)/1e6
		cpu = &value
	}
	if data, err := os.ReadFile("/proc/self/statm"); err == nil {
		fields := strings.Fields(string(data))
		if len(fields) > 1 {
			if pages, e := strconv.ParseUint(fields[1], 10, 64); e == nil {
				value := pages * uint64(os.Getpagesize())
				rss = &value
			}
		}
	}
	return cpu, rss
}
