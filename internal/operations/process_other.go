//go:build !linux && !windows

package operations

func processUsage() (*float64, *uint64) { return nil, nil }
