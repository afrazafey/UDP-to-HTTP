# UDPHTTP Proxy for Linux

An independently implemented IPv4 UDP multicast/unicast to HTTP MPEG-TS proxy for Debian and Ubuntu. Phase 2 adds shared reception, playback, persistent channel management, real traffic counters, and dashboard controls to the existing Go/SQLite/React foundation.

**Linux runtime validation is pending. This build is not claimed production-ready.** The local Windows host blocks some executables; see [PHASE2_REPORT.md](PHASE2_REPORT.md). Phase 3 has not started. No transcoding, proprietary proxy code, PHP, MySQL, Redis, or firewall changes are included.

## Build and run

Requirements: Go 1.26+, Node 24/npm, and optionally Make. Normal builds use CGO-free SQLite; race tests require a supported race toolchain and C compiler.

```sh
cd frontend
npm ci
npm run build
cd ..
go run ./cmd/udphttp-proxy config validate --config configs/config.example.yaml
go run ./cmd/udphttp-proxy interfaces
go run ./cmd/udphttp-proxy serve --config configs/config.example.yaml
```

Management: **http://127.0.0.1:8080**. Playback listeners default to **127.0.0.1:7781** and **127.0.0.1:7782**. The default playback CIDR and source allowlists are empty, so playback is denied and no channel receiver starts. The frontend is embedded; Node is not required to run the built binary. API-only builds without frontend assets return an explanatory 503 at `/`.

For a downloaded Linux binary bundle, extract it and run `./udphttp-proxy serve --config configs/config.example.yaml` from the extracted directory. The source ZIP is needed to build Debian packages yourself.

Other commands: `version`, `interfaces`, and `config validate [--config PATH]`. An explicitly missing config file fails. Without a config path, safe development defaults apply. Ctrl+C/SIGTERM trigger graceful shutdown.

For frontend development, run the daemon separately and `cd frontend && npm run dev`; visit `http://127.0.0.1:5173`. The loopback Vite proxy targets management port 8080. Update the proxy target if that port changes. The proxy rewrites Origin only during local development.

## Local synthetic stream

No commercial provider or video source is needed:

```sh
go run ./cmd/udphttp-proxy serve --config configs/phase2-local.yaml
# Another terminal:
python3 scripts/udp-generator.py --seconds 60
# Another terminal, while the generator runs:
curl --max-time 3 http://127.0.0.1:7781/live/local-demo.ts -o sample.ts
```

The generator emits correctly framed synthetic transport packets, not a decodable television program. Curl intentionally times out because live playback has no EOF. A real raw MPEG-TS UDP feed can be configured for actual players. RTP encapsulation is not supported.

## Configuration and channel persistence

Examples: [default development](configs/config.example.yaml), [isolated local demo](configs/phase2-local.yaml), and [Linux package defaults](deploy/config.yaml).

Precedence: defaults → selected strict YAML → documented environment overrides. `UDPHTTP_CONFIG` chooses a YAML file unless overridden by `--config`; `UDPHTTP_STORAGE_PATH` and `UDPHTTP_LOG_LEVEL` override those fields. `.env.example` documents process environment variables; `.env` is not loaded automatically. Unknown/duplicate YAML keys, multiple documents, files over 1 MiB, invalid literals, bounds, durations, or port collisions fail validation.

Server policies and limits are loaded from YAML. `channels` is an **initial import**, performed once into SQLite during the first Phase 2 startup. Later additions/edits/deletions use the management API/dashboard; changing the initial YAML channel array does not overwrite or resurrect database records. Use a fresh database for the isolated demo. SQLite schema v2 adds a channels table without dropping the Phase 1 settings table or startup snapshot. Back up state before upgrading; a Phase 1 binary cannot read the newer schema.

| Channel field | Meaning |
| --- | --- |
| `id`, `name` | Stable URL-safe ID (1..64 characters) and display name |
| `source_type` | `multicast` or `unicast` |
| `source_address` | Multicast destination group, or expected unicast **sender** IPv4 address |
| `source_port` | Local UDP destination port, 1..65535 |
| `interface` | Explicit local interface name or assigned IPv4 address |
| `enabled` | Persistent permission to receive/play the channel |
| `startup` | `on_demand` or `always`; default `on_demand` |
| `idle_timeout_seconds` | On-demand idle retention; zero releases after the final viewer disconnects |
| `max_clients` | Per-channel cap, bounded by the global cap; zero inherits it |
| `client_buffer_bytes` | Bounded client backlog; zero inherits the global default (256 KiB) |
| `udp_receive_buffer` | Requested OS receive buffer; zero inherits the default (4 MiB) |

Names and assigned addresses selecting the same socket identity are normalized for duplicate detection. Duplicate channel sources are rejected with 409, so an upstream is never intentionally opened once per viewer or channel alias. Unicast channels cannot share a local interface/port with different expected senders; use different destination ports. The selected unicast interface/IP is the bind address, not the sender. Source packets from another sender are counted as rejected and never forwarded.

Source IPs must occur in the corresponding exact server allowlist. Multicast groups must be in 224.0.0.0/4 and outside `blocked_multicast_networks` (default: 224.0.0.0/24). Operators can explicitly change the blocked CIDRs. Unicast loopback is denied unless `allow_loopback_unicast: true` and the specific loopback source is also allowlisted. No upstream hostname resolution, arbitrary URL fetch, or request-driven source creation is supported.

Global defaults: 100 clients, 128 streams, 256 KiB/client, 8 MiB queued per stream, 64 MiB queued globally, 10s inactivity threshold, 10s startup timeout, and 10s HTTP write deadline. Queues also have a 64-chunk cap. Shared immutable payloads reduce copying; queue byte accounting is deliberately conservative. Kernel receive buffers may be clamped by the OS; the application does not change sysctls or require root for tuning.

`network.http_interface` selects a local playback bind interface/IP. `http_bind_addresses` can explicitly select up to eight local IPv4 addresses instead; `0.0.0.0` is opt-in and cannot be combined with specific binds. Each address opens the primary and optional secondary port (`0` disables the secondary). All listeners share one engine. `streaming.enabled: false` disables playback listeners and receiver starts while retaining channel management.

## Playback and transport behavior

```text
GET http://PLAYBACK_HOST:7781/live/CHANNEL_ID.ts
GET http://PLAYBACK_HOST:7781/udp/239.1.1.1:1234
```

`/udp/` selects only an enabled, preconfigured multicast source; it never opens arbitrary requested targets. A group/port configured on multiple interfaces is ambiguous and returns 409: use `/live/` to select the channel. Exact Windows UDP-to-HTTP Proxy 3.0 URL compatibility has not been verified or claimed.

Playback uses `video/mp2t`, no-store/no-cache headers, disabled reverse-proxy buffering, per-write deadlines, and flushes after each available aligned payload. It waits for a valid TS payload before returning 200; startup timeout or a stopped/disabled stream returns 503. A slow viewer is disconnected without blocking other viewers. Proxies in front of playback must preserve streaming and apply their own appropriate timeout policy.

Framing preserves 188-byte packets across datagrams and requires three spaced sync bytes to establish/recover synchronization. Invalid headers/adaptation lengths and unsynchronized garbage are discarded and counted. Transport-error-indicator packets are counted but forwarded unchanged. Continuity errors, explicit discontinuities, duplicates, valid framed packets, accepted UDP bytes, and HTTP payload bytes are measured. No timestamps/codecs are rewritten. PSI CRC, PES/codec validity, RTP, FEC, and exact lost-packet reconstruction are outside Phase 2.

State is `disabled`, `stopped`, `starting`, `waiting`, `running`, `retrying`, or policy `error`. UDP has no connection handshake: `running` means recent valid TS payload; `waiting` means none within the inactivity threshold. Inactivity keeps the membership/socket open rather than treating it as a disconnected TCP source. Socket/interface errors close resources and retry with bounded 250ms..30s exponential delays. A stable reception period resets backoff.

Manual Start pins reception until Stop/process exit. Stop disconnects viewers and blocks on-demand starts until Start/Restart; this override is runtime-only. Persistent `enabled` and `startup` govern the next process start. Updates stop the old worker before replacing it. Receiver cleanup is joined before a restart rebinds. On-demand idle receivers release sockets, including during retries.

## Management API and security

Management remains literal IPv4 loopback-only with peer IP, Host/port, Origin, and cross-site checks. Forwarded headers are not trusted. Local users who can reach loopback can administer channels; this is **not a multi-user authentication system**. Remote management, passwords, sessions, and TLS administration remain Phase 3 work. Do not publish the management listener through a public proxy.

Writes additionally require `Content-Type: application/json` and an unpredictable per-process `X-CSRF-Token`, retrieved from `GET /api/v1/csrf`. Browser requests must be same-origin; there is no permissive CORS. This protects the local write boundary without pretending authentication already exists.

| Method | Management path | Result |
| --- | --- | --- |
| GET | `/health` | SQLite readiness (200/503) |
| GET | `/api/v1/status` | Process/build, effective server config, component availability |
| GET | `/api/v1/interfaces` | Current interface flags, MTU and IPv4 addresses |
| GET | `/api/v1/csrf` | Token for local JSON mutations |
| GET / POST | `/api/v1/streams` | List snapshots / create channel (201) |
| GET / PUT / DELETE | `/api/v1/streams/{id}` | Read / update / delete (204) |
| POST | `/api/v1/streams/{id}/start`, `/stop`, `/restart` | Runtime action (202); inspect state/errors afterward |
| GET | `/api/v1/streams/{id}/status`, `/errors` | Snapshot / bounded recent error history |
| GET | `/api/v1/clients` | Active and closing playback subscriptions |

Responses use `streams`, `stream`, `clients`, `errors`, or `error` envelopes. Validation errors are 400, token/origin/IP denial 403, missing resources 404, conflicts 409, oversized body 413, media type 415, capacity limits 429, persistence failure 500, and stopped/unavailable playback 503. Unknown methods receive 405. JSON bodies are strict and limited to 1 MiB.

Playback listeners expose only playback routes and enforce `allowed_client_networks` on the actual peer IP, independently of management. Empty CIDRs deny all, including localhost. Management paths are not served there, and streaming URLs are not served by management. For SSH viewing, forward management port 8080; forward the playback port too if needed.

The dashboard shows actual stream states, active/offline totals, clients, measured input/output rates, uptime, health counters, errors, and persistent channel forms. Start/Stop/Restart/Create/Update/Delete call backend APIs; Copy URL uses the clipboard and reports failures. Server-wide settings remain read-only. Themes, interface discovery/filtering and all Phase 1 CLI/API functionality remain available. Logs remain structured JSON on stderr/journald; recent stream errors are available separately, not a full browser journal viewer.

Counters are process/configuration-entry lifetime measurements; channel edits replace the entry and reset them, while Start/Stop/Restart preserve accumulated counters. Bitrates average measured deltas between status samples (at least one second), not instantaneous NIC rates. Output counts payload accepted by HTTP writes, not bytes acknowledged by the player. No invented traffic or resource metrics are displayed.

## Linux builds, installation and checks

```sh
make build
make linux VERSION=0.2.0
sh scripts/build-deb.sh amd64 0.2.0 build/udphttp-proxy-linux-amd64
sh scripts/build-deb.sh arm64 0.2.0 build/udphttp-proxy-linux-arm64
sudo apt install ./build/udphttp-proxy_0.2.0_amd64.deb
sudo systemctl restart udphttp-proxy
journalctl -u udphttp-proxy -f -o cat
```

Paths: `/usr/bin/udphttp-proxy`, `/etc/udphttp-proxy/config.yaml`, `/var/lib/udphttp-proxy/`, and `/usr/lib/systemd/system/udphttp-proxy.service`. The service account has no login/capabilities; default high ports need no root. Package scripts preserve conffiles during upgrades, stop on removal, and never modify the firewall. `apt remove` preserves config/data; `apt purge` removes packaged config but deliberately retains data and the system account. File logging and `/var/log/udphttp-proxy/` are not enabled.

Build without Make: build the frontend, then `CGO_ENABLED=0 go build -trimpath -o build/udphttp-proxy ./cmd/udphttp-proxy`; add `GOOS=linux GOARCH=amd64` or `arm64` for cross-compilation. PowerShell uses `$env:CGO_ENABLED='0'` and analogous assignments.

```sh
gofmt -w cmd internal
go vet ./...
go test ./...
go test -race ./...
cd frontend
npm run typecheck
npm test
npm run build
```

Run Go commands **after** frontend builds complete because Vite replaces embedded files. [LINUX_TESTING.md](LINUX_TESTING.md) describes actual Linux socket/multicast, HTTP, persistence and systemd checks. CI runs race tests, Linux multicast where a suitable interface exists, both architecture builds, a synthetic runtime smoke test, and package lifecycle checks. CI/release workflows are prepared but have not been dispatched here. Do not equate cross-compilation with Linux runtime validation.

## Source layout

```text
cmd/udphttp-proxy/       CLI entrypoint and signal handling
internal/cli/           startup, listener coordination and shutdown
internal/config/        strict server policy and initial channel configuration
internal/network/       interface discovery and resolution
internal/storage/       schema v1/v2 migrations and channel/settings repositories
internal/stream/        source validation, UDP reception, MPEG-TS framing, shared manager
internal/httpapi/       loopback management, CSRF, separate playback handlers/listeners
internal/web/           embedded Vite output
frontend/src/           existing dashboard plus real channel/client/transport views
configs/                safe defaults and isolated synthetic unicast demo
deploy/                 hardened service and Linux defaults
packaging/debian/       conffile-preserving lifecycle hooks
scripts/                packaging, local generator and Linux runtime acceptance test
.github/workflows/      Linux CI and gated release builds
```

The module namespace and Debian maintainer contact remain project identity placeholders. No GitHub repository or release has been created. See [ARCHITECTURE.md](ARCHITECTURE.md), [ROADMAP.md](ROADMAP.md), and [PHASE2_REPORT.md](PHASE2_REPORT.md). The historical Phase 1 report remains unchanged.
# UDP-to-HTTP
