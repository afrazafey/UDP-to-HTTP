# UDPHTTP Proxy for Linux

Phase 3 development release, **0.3.0-dev**. A standalone Go/SQLite UDP-to-HTTP IPTV proxy with an embedded React/TypeScript operations dashboard. This extends the existing receiver and playback engine; it does not replace it.

**Linux runtime acceptance is pending. This is not a production-readiness claim.** Windows Application Control blocks some local tests; exact results are in [PHASE3_REPORT.md](PHASE3_REPORT.md). No Phase 4 work, transcoding, firewall changes, PHP, MySQL or Redis are included.

## Build and development

Requirements: Go 1.26+, Node 24/npm. Make is optional; Debian packaging requires a Linux dpkg-deb environment. Normal binaries use CGO-free SQLite; race tests need GCC/a supported race toolchain.

```sh
cd frontend
npm ci
npm run build
cd ..
go run ./cmd/udphttp-proxy config validate --config configs/config.example.yaml
go run ./cmd/udphttp-proxy interfaces
go run ./cmd/udphttp-proxy serve --config configs/config.example.yaml
```

Management: **http://127.0.0.1:8080**. Default playback: **127.0.0.1:7781/7782**. Empty default source/client allowlists deny playback and create no channel receivers. `version`, `interfaces`, and `config validate` remain supported. An explicit missing config is an error; no path uses safe development defaults. Stop with Ctrl+C/SIGTERM.

The frontend is embedded; the installed binary needs no Node/CDN. Without built assets, `/` returns an explanatory 503 while APIs remain available. For Vite development, run the daemon separately, then `cd frontend && npm run dev` and open `http://127.0.0.1:5173`. The loopback-only development API proxy targets management port 8080 and removes development Origin/Referer/fetch-site headers. Production is same-origin and preserves these security headers through Nginx.

From a downloaded Linux bundle: `./udphttp-proxy serve --config configs/config.example.yaml`. Use the source ZIP to build packages. All local bundles are development builds.

## Dashboard workflows

The application uses hash routes, for example `#/streams/news`; API/playback paths never pass through a frontend fallback.

- **Overview:** configured/running/stopped/unhealthy channels, waiting receivers, clients, payload throughput, uptime/version, measured chart history, real recent activity and health.
- **Streams:** search, status/type filters, sorting, 20-row pages and mobile card styling. Add/edit persistent channels; Start/Stop/Restart, confirmed deletion, URL copying, details and filtered logs all use real APIs. Channel edits replace the receiver and disconnect its viewers; failed validation/persistence preserves the prior running channel.
- **Stream details:** source/interface/URL, start time, uptime, clients, bytes/packets, discontinuities/CC errors, errors and measured per-channel chart history. Native MPEG-TS capability is inspected, but codec/browser playback is unverified; Copy VLC URL supplies a real external-player option without transcoding or automatic playback.
- **Network Interfaces:** actual names/IPv4/MAC (if available), index, MTU, operational and multicast flags. Configure default multicast selection and playback binds/ports with preview and save confirmation. The host interfaces/routes/firewall are never modified.
- **Connected Clients:** actual peer IP, stream, start/duration, delivered payload, lifetime-average output rate, state and confirmed disconnection. A closing writer may wait for its deadline; other viewers continue.
- **Monitoring:** actual UDP/HTTP rates, active receivers/clients, Go heap/runtime allocation, goroutines, Linux process CPU/RSS, and transport observations. Unsupported metrics say unavailable. Pause/resume and manual refresh affect display polling, not reception; the service sampler continues. Refresh interval (3..60s) is persisted by the backend.
- **Activity Logs:** retained structured operational events with severity, stream, search and time filters, pagination, copy and authorized sanitized NDJSON export (maximum 1,000 entries). This is not an unrestricted journald reader.
- **Settings:** real persisted General, Streaming, HTTP, Security and Logging sections. Read-only policies are explicitly labeled. Theme supports light/dark/system; application name, refresh interval and new-channel preferences are daemon-backed. Sidebar collapse is a browser presentation preference.
- **System Information:** actual version/runtime/platform/PID/uptime/goroutines, configuration path, DB health, listener status, interfaces/resources and measured dropped-event count.

Dialogs trap focus, support Escape/focus restoration and use explicit labels/descriptions. Navigation supports keyboard/mobile use. Loading, empty, API error, stale and paused states are distinct. Notifications appear only after backend or clipboard confirmation. No fabricated measurements/history are included.

## Safe settings and network configuration

YAML/environment configuration bootstraps the daemon. Until the first settings save (revision 0), changes to bootstrap YAML/environment are honored. After a confirmed browser save, the persisted server-settings subset overrides those bootstrap fields on later starts. Management bind/port, database path and initial channel seeds stay outside the writable subset. Storage-path overrides and config selection still choose the bootstrap database/file. If you require YAML/environment authority again, use the recovery override below rather than deleting data.

1. Edit network/server settings. Existing stream configurations remain explicit and separate from a default interface selection.
2. Validate/preview. The backend checks typed bounds, actual interfaces/capabilities, duplicate listener ports/binds, existing channel compatibility and available new ports where practical. Owned ports are rechecked at restart, since the running daemon currently occupies them.
3. Confirm save with the current revision. SQLite atomically stores the new desired revision; stale edits/rollbacks receive 409. A failed save keeps the draft visible for correction. Refresh the revision after a transport-level uncertainty.
4. Dashboard/general preferences and logging controls apply live. Playback/network/streaming/timeouts are staged; a restart-required banner compares desired and active server settings.
5. Restart through the host service manager when ready: `sudo systemctl restart udphttp-proxy`. There is no generic process-control API. Start/Stop buttons control individual receivers only.
6. Startup revalidates saved data and actual playback binds. A failed staged bind or channel-count reduction restores the last accepted startup snapshot. If playback still cannot bind, local management stays available with playback disabled and visible failure details.

Management binding is immutable through this interface, so a playback address change does not move the current management endpoint. Port probing is point-in-time validation, not a reservation. The accepted revision is a successful **startup** snapshot: rollback can also revert live general/logging changes made since that startup. Discard Pending restores that snapshot with confirmation. The latest rollback reason is visible until a subsequent save/discard clears it.

Recovery without deleting SQLite:

```sh
# For a packaged installation, use an administrative account:
sudo systemctl stop udphttp-proxy
sudo -u udphttp-proxy env UDPHTTP_IGNORE_SAVED_SETTINGS=1 /usr/bin/udphttp-proxy serve --config /etc/udphttp-proxy/config.yaml
# After correcting/discarding settings in the UI, press Ctrl+C, then:
sudo systemctl start udphttp-proxy
```

The override uses bootstrap YAML/environment for that process and retains existing channels/events/settings. Use the local UI to correct or discard the bad saved proposal, stop the recovery process, then start the service normally. Keep the database/storage path consistent and back up state before upgrades. No YAML file is rewritten by the dashboard.

## Configuration and persistent data

Examples: [safe development defaults](configs/config.example.yaml), [isolated generator demo](configs/phase2-local.yaml), [Linux defaults](deploy/config.yaml), and [.env.example](.env.example). The application reads process environment, not .env files automatically. `UDPHTTP_CONFIG` selects YAML (unless --config wins), `UDPHTTP_STORAGE_PATH` selects the DB, and `UDPHTTP_LOG_LEVEL` is a bootstrap log override. Strict YAML rejects unknown/duplicate fields, multiple documents, files over 1 MiB and invalid policies/bounds.

Schema v3 adds `server_settings` (revision, desired, accepted, rollback reason) and `operational_events` without dropping v1 settings or v2 channels. Initial `channels` YAML imports once; API edits/deletions remain SQLite-authoritative and deleted seeds never reappear. Earlier binaries reject schema v3; rollback requires a matching state backup, not a destructive downgrade.

Writable settings include app name/theme/refresh/new-channel defaults; stream enablement/buffers/caps/timeouts; playback binds/ports and management HTTP timeouts; source/client allowlists and multicast exclusions; log level/event count/age retention. Management access, trust of proxies, session/token features, database/config paths, systemd startup and fixed recovery policy are read-only. Remote administration, trusted forwarding, signed playback tokens, browser journal access, hot reload and file logging are unavailable.

Channel fields: ID/name, source_type, source_address, source_port, interface, enabled, idle_timeout_seconds, max_clients, client_buffer_bytes, udp_receive_buffer and startup (`on_demand`/`always`). For unicast, source_address is the exact expected **sender** and source_port is the selected local interface's UDP destination port. Source hostnames/arbitrary URLs are prohibited. Loopback input requires both explicit permission and exact allowlisting.

Defaults: 100 clients, 128 channels, 256 KiB client backlog, 8 MiB per-stream queued bytes, 64 MiB global queued bytes, 4 MiB requested UDP socket buffer, 10s startup/inactivity/write deadlines. Kernel buffers can be clamped by the OS; no sysctls/root tuning is performed. Multicast 224.0.0.0/24 is blocked by default; overrides remain explicit. Duplicate source aliases/unicast binds are rejected; viewers across all playback listeners share one receiver.

## Playback and transport

```text
GET http://PLAYBACK_HOST:7781/live/CHANNEL_ID.ts
GET http://PLAYBACK_HOST:7781/udp/239.1.1.1:1234
```

The group route selects an enabled preconfigured multicast source and returns 409 if multiple interfaces make it ambiguous. Use a channel ID for disambiguation. No exact Windows UDP-to-HTTP Proxy 3.0 compatibility is claimed.

MPEG-TS forwarding uses 188-byte framing across datagrams, three-sync acquisition/recovery, bounded immutable fan-out and immediate HTTP flushes/video/mp2t. Invalid structure/garbage is discarded and counted; TEI-marked packets are counted but preserved. No timestamps/codecs are rewritten. CC errors/discontinuities are observations, not exact network packet loss. RTP, transcoding, FEC, PSI CRC/PES/codec validation and real browser/codec acceptance are outside this build.

States: disabled/stopped/starting/waiting/running/retrying/policy error. Running requires recent valid TS, not a UDP handshake. Inactivity keeps sockets open; socket/interface errors retry with fixed bounded exponential delays (250ms..30s). Interface changes are checked, socket reservations prevent duplicate receivers, idle retirement is coordinated with new viewers, and stops/restarts join cleanup. Slow viewers are isolated by bounded queues and write deadlines. Counters reset on process/channel replacement; normal Start/Stop/Restart preserve them. Manual Start/Stop overrides last for the process; enabled/startup persist.

A wildcard playback bind uses a local URL for copying; choose a reachable explicit host for sharing or forward playback ports along with management over SSH. CIDR policy is always enforced on actual peers.

## Management API and security

Management is literal IPv4 loopback-only, with direct-peer, Host/port, Origin/Referer and cross-site checks; forwarded IPs never grant access. This is a **trusted local-user boundary**, not an account/session authentication system. No remote management bind is permitted. Optional same-host reverse-proxy support is disabled by default and requires explicit trusted TCP peers and exact external origins. The proxy must enforce authentication and client authorization on every route; see [Ubuntu/Nginx deployment instructions](deploy/NGINX.md) and the [Nginx example](deploy/nginx-management.conf). Playback has independent actual-peer CIDRs and no administration routes; empty CIDRs deny all, even localhost.

Writes require a random per-process token from GET /api/v1/csrf, `X-CSRF-Token`, exact application/json, and strict bounded bodies. No permissive CORS; successful mutations are limited to 60/minute per daemon. Secrets in recognized sensitive keys, credentialed/query URLs and bearer/assignment patterns are redacted before JSON output/event retention. The application never logs playback request credentials or complete configuration bodies.

| Method | Management path | Purpose |
| --- | --- | --- |
| GET | /health, /api/v1/status, /api/v1/interfaces, /api/v1/csrf | Existing readiness/status/discovery/token |
| GET / POST | /api/v1/streams | Existing channel snapshots/create |
| GET / PUT / DELETE | /api/v1/streams/{id} | Existing read/edit/delete |
| POST | /api/v1/streams/{id}/start, /stop, /restart | Existing lifecycle controls |
| GET | /api/v1/streams/{id}/status, /errors | Existing snapshots/errors |
| GET | /api/v1/streams/{id}/metrics | Snapshot and bounded channel history |
| GET | /api/v1/clients | Extended duration/bytes/average bitrate |
| DELETE | /api/v1/clients/{id} | Safe subscription disconnect |
| GET | /api/v1/dashboard, /api/v1/metrics | Summary/resources/history/activity |
| GET | /api/v1/system | Real system/listener/security state |
| GET / PUT | /api/v1/settings | Desired/active revisions and confirmed save |
| POST | /api/v1/settings/validate, /rollback | Preview or confirmed restore |
| GET | /api/v1/logs, /api/v1/logs/export | Sanitized filtered events / capped NDJSON |

Settings PUT body: `{ "revision": 0, "settings": { ...complete settings from GET... }, "confirm": true }`. Validation uses the same settings object without confirmation. Rollback uses revision and confirm. Error envelopes use `error`; normal envelopes preserve existing stream/client routes. Validation 400, denial 403, missing 404, method 405, conflict 409, oversized 413, content type 415, capacity/rate 429, storage 500, unavailable/startup deadline 503. There is no duplicate network-discovery alias or generic service-command endpoint.

Log filters: search, level (`debug/info/warn/error`), stream_id, RFC3339 since/until, limit (1..100) and offset (0..10000). Export caps at 1,000. Default persisted events: newest 2,000 and 7 days, configurable within 100..10000 events/1..30 days. Age/count pruning runs on insertion and retention updates; reads enforce the age policy. The queue is capped at 256; overflow/store failures are counted. Shutdown drains best-effort for two seconds. Event capture follows the configured logging level; this is operational history, not an immutable audit ledger.

Metrics sample every 3s into bounded RAM: aggregate history 240 points, current channel histories 120 each. Deleted channel histories are removed. No runtime metrics are stored in SQLite. Rates are payload averages between samples; client rates average their own connection lifetime. CPU uses actual Linux getrusage deltas (100% = one core), RSS uses /proc, Go heap/goroutines use runtime APIs. Unsupported CPU/RSS values are null/unavailable, not zero.

## Installation, builds and validation

```sh
make build
make linux VERSION=0.3.0-dev
sh scripts/build-deb.sh amd64 0.3.0-dev build/udphttp-proxy-linux-amd64
sh scripts/build-deb.sh arm64 0.3.0-dev build/udphttp-proxy-linux-arm64
sudo apt install ./build/udphttp-proxy_0.3.0-dev_amd64.deb
sudo systemctl restart udphttp-proxy
journalctl -u udphttp-proxy -f -o cat
```

Paths remain /usr/bin/udphttp-proxy, /etc/udphttp-proxy/config.yaml, /var/lib/udphttp-proxy and /usr/lib/systemd/system/udphttp-proxy.service. Dedicated non-login account, no capabilities, high default ports; systemd limits writable state. dpkg conffiles survive upgrades. Remove stops the service and retains config/data; purge removes packaged config but deliberately retains state/account. No firewall changes or automatic root-only host tuning. File logging is disabled; raw JSON goes to stderr/journald.

```sh
gofmt -w cmd internal
go vet ./...
go test ./...
go test -race ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.10.0 ./...
cd frontend
npm run typecheck
npm test
npm run build
npm audit
```

Frontend builds must finish before Go checks/builds read embedded assets. Linux cross-builds use CGO_ENABLED=0 GOOS=linux GOARCH=amd64/arm64; PowerShell sets these with `$env:NAME='value'`. CI includes actual native Linux unit/race tests, synthetic playback, persistence, settings validation and occupied-port rollback, then systemd/package lifecycle tests. Workflows are prepared; no repository was connected or CI dispatched here. See [LINUX_TESTING.md](LINUX_TESTING.md).

For a provider-free local fixture, serve configs/phase2-local.yaml and run `python3 scripts/udp-generator.py --seconds 60`. Request /live/local-demo.ts on playback port 7781. The generator emits synthetic transport packets, not a decodable video program. Curl timeout is normal for a live stream with no EOF.

## Source layout

```text
internal/stream/       preserved receiver/framer/manager; additive client/start metrics
internal/operations/   staged settings, preflight, bounded monitoring and OS resources
internal/events/       sanitized slog capture and bounded queue
internal/storage/      additive v1/v2/v3 migrations, channels/settings/events
internal/httpapi/      preserved management/playback plus operations APIs
internal/cli/          startup validation, rollback and listener coordination
frontend/src/pages/    overview, streams/details, network, clients, monitoring, logs, settings, system
frontend/src/components/  reusable UI, charts, forms and stream controls
frontend/src/hooks,services,types,utils/  shared polling, API, contracts and formatting
frontend/src/LegacyApp.tsx,Streaming.tsx  retained Phase 1/2 UI fallback/regression coverage
```

Project/module and Debian maintainer identity placeholders remain for the owner before public distribution. Historical reports/bundles are retained. Read [PHASE3_AUDIT.md](PHASE3_AUDIT.md), [PHASE3_REPORT.md](PHASE3_REPORT.md), [ARCHITECTURE.md](ARCHITECTURE.md), and [ROADMAP.md](ROADMAP.md). Stop for review before Phase 4.
