# UDPHTTP Proxy threat model

## Assets and trust boundaries

| Boundary | Assets and trust | Controls and residual exposure |
| --- | --- | --- |
| Browser to management listener | Authorized local operators can edit channels and settings and read diagnostics. Browser origins and remote peers are untrusted. | IPv4 loopback, exact host and origin, Referer and Fetch Metadata checks, CSRF token, strict JSON and body caps. All local OS users able to access the listener have local administrative trust. |
| Trusted reverse proxy to daemon | The authenticated proxy mediates authorized LAN users. | Forwarded values are accepted from configured loopback peers for exact origins. The proxy replaces client-supplied headers and performs authentication and IP checks. A compromised trusted proxy shares operator authority. |
| UDP sender to receiver | Channel sources and interface identity determine which MPEG-TS datagrams can enter fan-out. | Literal IPv4 and exact allowlists, multicast IF selection, source address checks for unicast, TS framing, bounded receiver set and recovery delay. Source IP does not authenticate the sender. The surrounding network must prevent spoofing. |
| HTTP playback client to listener | Clients receive MPEG-TS and may request explicitly enabled playlists. | Actual-peer CIDR checks, separate listeners, finite clients and queues, deadlines and disconnect cleanup. TLS or a private network belongs at the deployment boundary where compatible with IPTV clients. |
| Operator and service to SQLite | Channels, saved settings, and retained operational logs. | Parameters, transactions and revisions, owner-only database files, dedicated state directories, consistent snapshots and exclusive restore destinations. OS permissions protect the directory and sidecars from untrusted writers. |
| Dependencies and CI to packages | Packages, checksums, SBOMs, embedded assets, private source. | Lockfiles, module integrity, scanned archives, pinned CI actions, read-only validation and explicit draft approval. Verify checksums obtained over an authenticated release channel. |

Risk treatment follows the [Phase 4.2 audit](SECURITY_AUDIT.md). Native performance and installation evidence is required to validate environment-specific controls.
