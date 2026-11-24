// Package netpolicy normalizes and compiles IP access lists.
package netpolicy

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// List accepts a scalar or an array on input and is serialized as an array.
type List []string

func (l List) MarshalJSON() ([]byte, error) {
	if l == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]string(l))
}

func (l *List) UnmarshalJSON(data []byte) error {
	if len(data) > 0 && data[0] == '"' {
		var text string
		if err := json.Unmarshal(data, &text); err != nil {
			return err
		}
		*l = List{text}
		return nil
	}
	var entries []*string
	if err := json.Unmarshal(data, &entries); err != nil {
		return err
	}
	values := make(List, 0, len(entries))
	for index, entry := range entries {
		if entry == nil {
			return fmt.Errorf("entry %d must be an IP string", index+1)
		}
		values = append(values, *entry)
	}
	*l = values
	return nil
}

func (l *List) UnmarshalYAML(n *yaml.Node) error {
	if n.Kind == yaml.ScalarNode && n.Tag == "!!str" {
		*l = List{n.Value}
		return nil
	}
	if n.Kind != yaml.SequenceNode {
		return fmt.Errorf("line %d: expected an IP list or string", n.Line)
	}
	entries := make(List, 0, len(n.Content))
	for _, item := range n.Content {
		if item.Kind != yaml.ScalarNode || item.Tag != "!!str" {
			return fmt.Errorf("line %d: expected an IP entry as a string", item.Line)
		}
		entries = append(entries, item.Value)
	}
	*l = entries
	return nil
}

type Kind int

const (
	Client Kind = iota
	MulticastAllow
	MulticastBlock
	Unicast
	Proxy
	Bind
)

var (
	multicast = netip.MustParsePrefix("224.0.0.0/4")
	reserved  = netip.MustParsePrefix("224.0.0.0/24")
	loopback  = netip.MustParsePrefix("127.0.0.0/8")
	broadcast = netip.MustParseAddr("255.255.255.255")
)

type entry struct {
	value  string
	prefix netip.Prefix
}

// Prefixes is immutable and can be shared across concurrent requests.
type Prefixes struct{ entries []entry }

func (p Prefixes) Values() List {
	values := make(List, 0, len(p.entries))
	for _, e := range p.entries {
		values = append(values, e.value)
	}
	return values
}

func (p Prefixes) Match(ip netip.Addr) (string, bool) {
	if ip.Zone() != "" {
		return "", false
	}
	ip = ip.Unmap()
	for _, e := range p.entries {
		if e.prefix.Contains(ip) {
			return e.value, true
		}
	}
	return "", false
}

func Compile(field string, values List, kind Kind, allowLoopback bool) (Prefixes, error) {
	var result Prefixes
	seen := make(map[netip.Prefix]int)
	for _, line := range values {
		for raw := range strings.FieldsFuncSeq(line, func(r rune) bool { return r == ',' || r == '\n' || r == '\r' }) {
			text := strings.TrimSpace(raw)
			if text == "" {
				continue
			}
			prefix, literal, err := parse(text, kind, allowLoopback)
			if err != nil {
				return Prefixes{}, fmt.Errorf("%s entry %q: %w", field, text, err)
			}
			if index, ok := seen[prefix]; ok {
				if literal {
					result.entries[index].value = prefix.Addr().String()
				}
				continue
			}
			value := prefix.String()
			if literal {
				value = prefix.Addr().String()
			}
			seen[prefix] = len(result.entries)
			result.entries = append(result.entries, entry{value, prefix})
		}
	}
	return result, nil
}

func parse(text string, kind Kind, allowLoopback bool) (netip.Prefix, bool, error) {
	literal := !strings.Contains(text, "/")
	var prefix netip.Prefix
	var err error
	if literal {
		var ip netip.Addr
		ip, err = netip.ParseAddr(text)
		if err == nil && ip.Zone() == "" {
			prefix = netip.PrefixFrom(ip, ip.BitLen())
		}
	} else {
		prefix, err = netip.ParsePrefix(text)
	}
	if err != nil || !prefix.IsValid() {
		return prefix, literal, errors.New("expected a literal IP address or a valid CIDR without an IPv6 zone")
	}
	ip := prefix.Addr()
	if kind == Client && ip.Is4In6() {
		if prefix.Bits() < 96 {
			return prefix, literal, errors.New("mapped IPv4 CIDR must be within ::ffff:0:0/96")
		}
		prefix = netip.PrefixFrom(ip.Unmap(), prefix.Bits()-96)
	}
	prefix = prefix.Masked()
	if kind != Client && !ip.Is4() {
		return prefix, literal, errors.New("expected a literal IPv4 address or IPv4 CIDR")
	}
	switch kind {
	case MulticastAllow, MulticastBlock:
		if prefix.Bits() < multicast.Bits() || !multicast.Contains(prefix.Addr()) {
			return prefix, literal, errors.New("network must be entirely within 224.0.0.0/4")
		}
		if kind == MulticastAllow && prefix.Bits() >= reserved.Bits() && reserved.Contains(prefix.Addr()) {
			return prefix, literal, errors.New("reserved local-control multicast groups 224.0.0.0/24 are always blocked")
		}
	case Unicast:
		if (!allowLoopback && prefix.Bits() >= loopback.Bits() && loopback.Contains(prefix.Addr())) ||
			(prefix.Bits() >= multicast.Bits() && multicast.Contains(prefix.Addr())) ||
			(prefix.Bits() == 32 && !unicastSender(ip, allowLoopback)) {
			return prefix, literal, errors.New("unicast sender network contains no permitted address; loopback requires explicit permission")
		}
	case Proxy:
		if !literal || !ip.IsLoopback() {
			return prefix, literal, errors.New("expected a literal IPv4 loopback proxy address")
		}
	case Bind:
		if !literal || ip.IsMulticast() || ip == broadcast {
			return prefix, literal, errors.New("expected a literal local IPv4 address or 0.0.0.0")
		}
	}
	return prefix, literal, nil
}

func unicastSender(ip netip.Addr, allowLoopback bool) bool {
	return ip.IsGlobalUnicast() || (ip.IsLoopback() && allowLoopback)
}

type Decision struct {
	Allowed bool   `json:"allowed"`
	Reason  string `json:"reason"`
	Matched string `json:"matched,omitempty"`
}

func (p Prefixes) Check(ip netip.Addr) Decision {
	if value, ok := p.Match(ip); ok {
		return Decision{true, "Address matches the allowlist", value}
	}
	return Decision{Reason: "No allowlist entry matched"}
}

type Sources struct {
	Multicast, Blocked, Unicast Prefixes
	AllowLoopback               bool
}

// AllowsAllMulticast detects a broad allow even when entered as several ranges.
// The hard and configured exclusions still apply when checking a source.
func (s Sources) AllowsAllMulticast() bool {
	type span struct{ first, size uint32 }
	ranges := make([]span, 0, len(s.Multicast.entries))
	for _, e := range s.Multicast.entries {
		if e.prefix == multicast {
			return true
		}
		octets := e.prefix.Addr().As4()
		first := uint32(octets[0])<<24 | uint32(octets[1])<<16 | uint32(octets[2])<<8 | uint32(octets[3])
		ranges = append(ranges, span{first, 1 << (32 - e.prefix.Bits())})
	}
	slices.SortFunc(ranges, func(a, b span) int { return int(a.first) - int(b.first) })
	next := uint32(0xe0000000)
	for _, r := range ranges {
		if r.first > next {
			return false
		}
		next = max(next, r.first+r.size)
	}
	return next == 0xf0000000
}

func CompileSources(multicast, blocked, unicast List, loopback bool) (Sources, error) {
	m, me := Compile("streaming.allowed_multicast_groups", multicast, MulticastAllow, false)
	b, be := Compile("streaming.blocked_multicast_networks", blocked, MulticastBlock, false)
	u, ue := Compile("streaming.allowed_unicast_sources", unicast, Unicast, loopback)
	return Sources{m, b, u, loopback}, errors.Join(me, be, ue)
}

func (s Sources) Check(kind string, ip netip.Addr) Decision {
	if !ip.Is4() {
		return Decision{Reason: "source_address must be a literal IPv4 address"}
	}
	switch kind {
	case "multicast":
		if !multicast.Contains(ip) {
			return Decision{Reason: "multicast source_address must be in 224.0.0.0/4"}
		}
		if reserved.Contains(ip) {
			return Decision{Reason: "reserved local-control multicast groups 224.0.0.0/24 are always blocked", Matched: reserved.String()}
		}
		if value, ok := s.Blocked.Match(ip); ok {
			return Decision{Reason: "multicast group is blocked by " + value, Matched: value}
		}
		if value, ok := s.Multicast.Match(ip); ok {
			return Decision{true, "Group matches the multicast allowlist", value}
		}
	case "unicast":
		if !unicastSender(ip, s.AllowLoopback) {
			return Decision{Reason: "unicast source must be a unicast IPv4 sender; loopback requires explicit permission"}
		}
		if value, ok := s.Unicast.Match(ip); ok {
			return Decision{true, "Sender matches the unicast allowlist", value}
		}
	default:
		return Decision{Reason: "source_type must be multicast or unicast"}
	}
	return Decision{Reason: "source address is not in the corresponding server allowlist"}
}
