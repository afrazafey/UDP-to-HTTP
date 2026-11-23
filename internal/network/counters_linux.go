//go:build linux

package network

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func interfaceCounters(name string) *ErrorCounters {
	if name == "" || strings.ContainsAny(name, "/\\") || name == ".." {
		return nil
	}
	values := []uint64{}
	for _, key := range []string{"rx_errors", "tx_errors", "rx_dropped", "tx_dropped"} {
		data, err := os.ReadFile(filepath.Join("/sys/class/net", name, "statistics", key))
		if err != nil {
			return nil
		}
		n, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
		if err != nil {
			return nil
		}
		values = append(values, n)
	}
	return &ErrorCounters{values[0], values[1], values[2], values[3]}
}
