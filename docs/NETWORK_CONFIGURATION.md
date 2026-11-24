# Network configuration

## Network list formats

IP-based lists accept scalar strings, YAML arrays, and block scalars. Entries
can share one line with commas or appear on separate lines, with LF or CRLF.
The parser trims whitespace, collapses duplicate hosts or ranges, and masks
CIDRs to the network boundary. Validation errors identify the field and entry.
The management API returns canonical arrays.

```yaml
streaming:
  allowed_client_networks:
    - 10.14.0.5/32
    - 192.168.1.0/24, 192.168.10.0/24
  allowed_multicast_groups: "239.1.1.0/24,239.255.20.0/24"
  allowed_unicast_sources: |-
    10.14.0.1,10.14.0.2
    10.20.0.0/24
  blocked_multicast_networks: [224.0.0.0/24]
```

Client lists accept IPv4 and IPv6 policy entries. Playback listeners currently
bind to IPv4. IPv4-mapped IPv6 peers and entries are matched as IPv4. Scoped
IPv6 addresses are rejected. Empty client and source allowlists deny all.
Remote playlist clients must match both the playback and playlist lists.

Multicast source and block entries must fit inside IPv4 `224.0.0.0/4`.
The local-control range `224.0.0.0/24` always stays blocked. Configured
multicast blocks override every allow, including `224.0.0.0/4`. Use the
smallest ranges that serve your channels. Adding the allow-all range in the
dashboard requires confirmation. Broad unicast ranges still enforce existing
sender checks and explicit loopback permission.

Playback client and source changes apply live. Excluded streams and clients
disconnect and their channel definitions stay in SQLite. Listener address
changes still require a restart. Settings can test an address against the
active or unsaved policy.

Bind lists require exact local IPv4 addresses. The explicit wildcard
`0.0.0.0` must stand alone. Trusted proxy lists require exact IPv4 loopback
addresses. Trusted origins use separate HTTP(S) origin validation.

## Existing deployment reference

The following is one deployment the user verified before Phase 4. Replace every address/interface for other networks. It is not an application default or an independent Phase 4 acceptance result.

```text
UDP source network: 10.14.0.0/24
UDP interface: ens18
UDP receiver: 10.14.0.37
HTTP client network: 192.168.10.0/24
HTTP interface: ens19
HTTP server: 192.168.10.103
Multicast group: 239.1.1.61
UDP port: 1234
Playback: http://192.168.10.103:7781/live/trace-urban.ts
```

Merge the network/policy subset with an existing validated config:

```yaml
management:
  host: 127.0.0.1
  port: 8080
network:
  multicast_interface: ens18
  http_interface: ens19
  http_port: 7781
  secondary_http_port: 7782
streaming:
  allowed_client_networks: [192.168.10.0/24]
  allowed_multicast_groups: [239.1.1.61]
  allowed_unicast_sources: []
  blocked_multicast_networks: [224.0.0.0/24]
playlist:
  advertised_host: 192.168.10.103
  remote_enabled: false
  allowed_client_networks: []
```

Add TRACE URBAN in the dashboard: ID `trace-urban`, name `TRACE URBAN`, source type `multicast`, address `239.1.1.61`, port `1234`, interface `ens18`, enabled, on demand. Optional group `MUSIC`, TVG ID `67`, TVG name `TRACE URBAN`, logo URL `http://m3u.afraaz.xyz/ChannelLogo/urban.png`, primary playback preference. Logos are metadata only; the daemon does not fetch them. Source validation and live interface checks must pass before saving. YAML channel seeds apply once to a fresh database; they do not replace existing channels.

For unicast, `source_address` is the exact **sender** IPv4 and `source_port` is the local UDP destination port. The interface is an assigned local IPv4/name. A hostname, HTTP URL, unspecified address, broadcast sender or unauthorized IP cannot become a receiver. Multicast input is not an HTTP pull.

Bind addresses select where sockets listen. The advertised playlist host is separate and contains no scheme/port/path. A wildcard playback bind is explicit opt-in, and still needs an explicit reachable advertised host for export. Secondary port 0 disables that listener. Per-channel preference can choose a configured secondary port; exports fail if that preference cannot be served.

Source and client policies are independent. Membership/group eligibility, a bound socket, observed UDP datagrams, valid 188-byte TS, HTTP payload delivery and client decoding are separate conditions. Diagnostics expose these separately. Driver errors/drops are reported from Linux sysfs when available; unsupported values are null/unavailable. Application membership observations do not enumerate memberships owned by other processes.

IGMP joins follow the selected interface. Ensure the source VLAN reaches that adapter, the switch/router has the appropriate multicast routing/IGMP snooping and querier configuration, and firewall policy permits the source/UDP destination. The proxy does not configure routes, switches, NICs, sysctls or firewall rules. Use static addresses or DHCP reservations so stored interface selections and advertised URLs remain valid.

On Linux: `ip -br addr`, `ip route`, `ip maddr show dev ens18`, `ss -lunp`, `ss -ltnp`, and, with permission, `sudo tcpdump -ni ens18 'udp and dst host 239.1.1.61 and dst port 1234'`. Packet capture can contain private payloads; retain it separately from diagnostic exports.

On Windows: `Get-NetAdapter`, `Get-NetIPAddress -AddressFamily IPv4`, `Get-NetRoute -AddressFamily IPv4`, `netsh interface ipv4 show joins`, `Get-NetUDPEndpoint`, `Get-NetTCPConnection -State Listen`. Use the actual adapter IPv4 in channel configuration when friendly names change. Do not interpret OS joins alone as proof that the application receives valid MPEG-TS.
