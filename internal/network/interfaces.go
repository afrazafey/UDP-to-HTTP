package network

import (
	"fmt"
	"net"
	"net/netip"
	"sort"
)

type Interface struct {
	Index     int      `json:"index"`
	Name      string   `json:"name"`
	MTU       int      `json:"mtu"`
	Up        bool     `json:"up"`
	Loopback  bool     `json:"loopback"`
	Multicast bool     `json:"multicast"`
	IPv4      []string `json:"ipv4"`
	MAC       string   `json:"mac"`
}

// Discover uses the OS network API, including Linux interfaces without shell commands or root.
func Discover() ([]Interface, error) {
	return discover(net.Interfaces, func(i net.Interface) ([]net.Addr, error) { return i.Addrs() })
}
func discover(list func() ([]net.Interface, error), addrs func(net.Interface) ([]net.Addr, error)) ([]Interface, error) {
	items, err := list()
	if err != nil {
		return nil, fmt.Errorf("list network interfaces: %w", err)
	}
	result := make([]Interface, 0, len(items))
	for _, item := range items {
		a, err := addrs(item)
		if err != nil {
			return nil, fmt.Errorf("addresses for %s: %w", item.Name, err)
		}
		v := Interface{Index: item.Index, Name: item.Name, MTU: item.MTU, Up: item.Flags&net.FlagUp != 0, Loopback: item.Flags&net.FlagLoopback != 0, Multicast: item.Flags&net.FlagMulticast != 0, IPv4: []string{}}
		v.MAC = item.HardwareAddr.String()
		for _, addr := range a {
			p, err := netip.ParsePrefix(addr.String())
			if err == nil && p.Addr().Is4() {
				v.IPv4 = append(v.IPv4, p.Addr().String())
			}
		}
		sort.Strings(v.IPv4)
		result = append(result, v)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Index < result[j].Index })
	return result, nil
}

// Resolve checks configured names/addresses against real interfaces. Empty means unselected.
func Resolve(value string, items []Interface, requireMulticast bool) (*Interface, error) {
	if value == "" {
		return nil, nil
	}
	for _, item := range items {
		match := item.Name == value
		for _, ip := range item.IPv4 {
			match = match || ip == value
		}
		if match {
			if !item.Up || len(item.IPv4) == 0 {
				return nil, fmt.Errorf("interface %s is down or has no IPv4 address", item.Name)
			}
			if requireMulticast && !item.Multicast {
				return nil, fmt.Errorf("interface %s does not support multicast", item.Name)
			}
			return &item, nil
		}
	}
	return nil, fmt.Errorf("interface or local IPv4 address %q does not exist", value)
}
