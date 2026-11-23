//go:build !linux

package network

func interfaceCounters(string) *ErrorCounters { return nil }
