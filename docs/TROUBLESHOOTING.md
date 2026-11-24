# Troubleshooting

If the dashboard goes blank after Nginx sign-in and the browser reports `application/json` for CSS or JS, inspect the failed request's response body. A 403 with `Cross-origin administration denied` points to the management origin or forwarding configuration. [Blank dashboard after signing in](../deploy/NGINX.md#blank-dashboard-after-signing-in)

If Windows Application Control blocks an unsigned candidate, use an approved runner or a signed executable. The installer requires unused application directories. Follow the upgrade instructions for installations with existing state. [Windows installation](INSTALL_WINDOWS.md)

An older development package that fails with `config.yaml.dpkg-new` and a missing parent directory needs a rebuilt package with the corrected archive layout. Inspect the replacement package and follow the upgrade steps if a prior installation exists. A Go coverage failure referring to package `.out` requires quoting the complete coverage flag or using the Bash CI command. [Debian packaging](DEBIAN_PACKAGING.md) · [Coverage commands](CI_CD_VALIDATION.md)

Begin with the actual service log and `/health`; do not reset SQLite or broaden allowlists to suppress an error. A successful syntax check, UDP bind, multicast membership, received datagram, valid TS packet and decoded player session prove different things.

| Symptom | Check and action |
| --- | --- |
| No multicast packets | Confirm selected interface IPv4/up/multicast capability, source VLAN, group/port, route/IGMP snooping and querier, and firewall. Check traffic on the UDP adapter, not the HTTP adapter. |
| Wrong interface / changed IP | Run `interfaces`; choose its actual assigned IPv4/name. An index/IP change triggers receiver reinitialization. Correct the persisted channel or network defaults; defaults do not rewrite existing explicit channel interfaces. |
| Invalid source allowlist | The multicast group or exact unicast sender must be in the corresponding active policy. Imports cannot authorize it; explicitly review policy and restart staged settings before previewing again. |
| Playback 403 | Actual client peer is not in `streaming.allowed_client_networks`; empty denies all. Behind a playback proxy the peer is the proxy, so enforce client policy there before explicitly allowing that proxy. X-Forwarded-For does not grant access. |
| Playlist 403/404 | Remote download is disabled by default (404); enabling requires its additional client CIDRs and the playback policy. Management export requires local/authenticated trusted management access. |
| Playback 404 | Check enabled channel ID and exact `/live/ID.ts`; `/udp/IP:port` requires an existing enabled unambiguous multicast channel. Management and playback ports expose different routes. |
| No MPEG-TS / 503 | Inspect accepted UDP datagrams versus TS packet/sync counters. Arbitrary UDP, RTP and codecs are not equivalent to TS. Startup without valid data returns 503 after connect timeout. Check source/interface before retrying clients. |
| Port already used | Inspect listeners on management/primary/secondary ports; do not kill unrelated services. Correct staged ports; startup can roll back a failed playback bind while retaining management. |
| Windows Firewall blocks playback | Inspect profile/program/remote subnet rules for the executable and playback ports; no rule should open management. Review narrowly scoped UDP inbound policy on the source adapter. |
| Linux firewall blocks traffic | Inspect existing nftables/ufw policy and source/client networks; application installation does not change firewall rules. |
| Stream stalls | Check last datagram and valid TS times separately. Look for waiting/degraded/recovering health, retries, restart/recovery outcomes, interface changes and upstream traffic. Finite retry exhaustion needs manual start/restart; do not restart the entire service for one stalled channel. |
| Slow client / buffers exhausted | Slow-client disconnect counters show bounded queue protection. Reduce client/network pressure or tune limits after measurements. Larger queues do not fix missing upstream packets or slow downstream links. |
| Database lock / busy | Ensure only intended processes write the same local SQLite file, filesystem locking works, and storage is writable. busy_timeout is 5s; stop duplicate daemons or conflicting maintenance tools. Avoid unsupported network filesystems. Back up before repair; do not delete WAL/SHM from a live DB. |
| Service fails after upgrade | Check retained config, actual binary version, file ACL/ownership, free ports, schema compatibility and migration error. Use the verified pre-upgrade backup plus matching old binary for rollback. |
| Management proxy assets 403 | Verify exact trusted origin, actual loopback proxy peer, complete overwritten single-hop headers, preserved browser origin/referrer, authentication/TLS/client ACLs. Never remove security checks or whitelist arbitrary forwarding headers. |
| Vite ECONNREFUSED :8080 | Vite starts only the frontend. Start `go run ./cmd/udphttp-proxy serve --config configs/config.example.yaml` separately. A backend bound to another port needs an explicitly adjusted development proxy. |
| Windows test executable blocked | Host Application Control is an independent policy. Do not bypass it; use approved tooling/CI and record the blocked gate. A successful cross-build is not a test pass. |

Linux diagnostic commands (substitute the actual adapter/group):

```sh
udphttp-proxy version
udphttp-proxy interfaces
sudo systemctl status udphttp-proxy --no-pager
journalctl -u udphttp-proxy -n 100 -o cat --no-pager
ip -br addr
ip route
ip maddr show dev ens18
ss -lunp
ss -ltnp
sudo nft list ruleset
curl --fail http://127.0.0.1:8080/api/v1/status
sudo tcpdump -ni ens18 'udp and dst host 239.1.1.61 and dst port 1234'
```

Windows PowerShell:

```powershell
Get-Service UDPHTTPProxy
Get-NetAdapter
Get-NetIPAddress -AddressFamily IPv4
Get-NetRoute -AddressFamily IPv4
netsh interface ipv4 show joins
Get-NetUDPEndpoint -LocalPort 1234
Get-NetTCPConnection -State Listen
Get-NetFirewallRule -DisplayName 'UDPHTTP Playback' | Get-NetFirewallPortFilter
Test-NetConnection -ComputerName 192.168.10.103 -Port 7781
Invoke-RestMethod http://127.0.0.1:8080/api/v1/status
Get-Content -Tail 100 -LiteralPath (Join-Path $env:ProgramData 'UDPHTTPProxy\logs\service.jsonl')
```

`Test-NetConnection`/a diagnostic HTTP response confirms connectivity, not a decoded stream. Use VLC and compare TS/payload counters for playback acceptance. Do not attach raw config, credentials, database files or packet captures to public issues. Export sanitized application events through Activity Logs for initial diagnosis.
