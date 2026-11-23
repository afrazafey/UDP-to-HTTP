# Phase 2 implementation report

Date: 2026-11-23. Phase 2 continues the existing application; Phase 3 has not started.

## Implemented functionality

- IPv4 multicast/unicast receivers with explicit interface selection, receive-buffer requests, 500ms read deadlines, sender/group/interface filtering, explicit membership leave and cancellation cleanup.
- 188-byte MPEG-TS synchronization/recovery across datagram boundaries; immutable forwarding without transcoding; structural corruption, TEI, CC, duplicate and discontinuity counters.
- Shared receiver per configured identity, duplicate-source rejection, bounded client/stream/global queues, global/channel caps, slow-client termination, idle release and joined lifecycle shutdown/restart.
- Actual states and inactivity thresholds, actionable bounded recent errors, interface disappearance checks and bounded exponential retry.
- Separate playback listeners and client CIDRs, startup-payload deadline, per-write deadlines/flushes, real traffic and client measurements.
- Additive SQLite schema v2, one-time YAML seed and persistent channel CRUD preserving Phase 1 settings/data.
- Loopback management with CSRF-protected JSON mutations; no unauthenticated remote management exposure.
- Existing React layout retained with real stream/client/transport data, CRUD forms, Start/Stop/Restart, errors and URL copying.
- Automated transport, lifecycle, endpoint/security, migration and frontend tests; Linux synthetic runtime script and CI/release gates.

## Created and modified files

New: `internal/stream/{channel,receiver,mpegts,manager}.go`, associated unit/integration/Linux multicast tests; `internal/httpapi/streams.go` and tests; `frontend/src/Streaming.tsx` and tests; `configs/phase2-local.yaml`; `scripts/udp-generator.py`; `scripts/linux-smoke.py`; `LINUX_TESTING.md`; this report.

Modified: configuration/defaults/tests, storage/migration/repository/tests, HTTP server/CSRF/status, CLI listener coordination, version metadata, frontend App/API/styles, configuration examples/Linux defaults, systemd description, Makefile, Debian builder, CI/release workflows, README, ARCHITECTURE, ROADMAP. Phase 1 source was extended; its historical report and original downloadable bundles are retained.

## Endpoints

Management (loopback, existing port 8080 by default): health/status/interfaces plus GET csrf, GET/POST streams, GET/PUT/DELETE stream by ID, POST start/stop/restart, GET status/errors per stream, and GET clients. JSON mutations need the current token and application/json. Playback ports 7781/7782 by default expose GET `/live/{id}.ts` and `/udp/{group}:{port}` only, with a separate actual-peer CIDR allowlist. See README for complete status codes and response envelopes.

## Validation results

Local toolchain: checksum-verified Go 1.27.4, Node 24.21.0 and npm 11.19.0 on Windows. Dependencies are pinned by go.sum and frontend/package-lock.json.

| Check | Observed result |
| --- | --- |
| gofmt and `go vet ./...` | PASS after the final frontend build |
| `go test ./...` | INCOMPLETE: intermittent Windows Application Control launch failures; no executed test assertion failure was observed |
| CLI, configuration, interface packages | PASS on successful launches across the recorded runs; later CLI/config launches can be blocked by host policy |
| Stream package | PASS on an earlier revision in 6.361s, including actual loopback UDP shared delivery, framing, lifecycle, inactivity, caps, bounded slow-client queues, retry, cancellation and concurrent churn. The final socket-reservation and idle-handoff hardening compiles/vets, but its test executable launch is blocked |
| HTTP API/playback package | PASS on an earlier revision in 5.863s, including simultaneous real local HTTP viewers, startup timeout, CRUD, CSRF, separate access policy and existing Phase 1 tests; final rerun is blocked |
| SQLite package | PASS in 2.103s, including the actual schema v1→v2 migration, retained settings, atomic updates, channel persistence and non-resurrecting seeds |
| Repeat `go test -v ./internal/stream` | BLOCKED before execution by Application Control, illustrating the inconsistent host launch limitation; not recorded as another pass |
| `CGO_ENABLED=1 go test -race ./...` | BLOCKED: GCC is not installed/on PATH; runtime/cgo build fails. Earlier host-policy blocks can also affect generated executables |
| Frontend TypeScript check | PASS |
| Frontend tests | PASS: 9 tests across 2 files, including API mutations, errors, CRUD, clipboard, themes and Phase 1 behavior |
| Frontend production build | PASS: Vite 7.3.8; embedded HTML/CSS/JS generated |
| npm audit | PASS: 0 reported vulnerabilities |
| Python script syntax checks | PASS for generator and Linux smoke test |
| Local Python UDP generator | PASS: sent 35 synthetic TS packets / 6580 payload bytes in a short local fixture run |
| Linux AMD64 application build | PASS, cross-compiled with CGO disabled |
| Linux AMD64 stream test executable | PASS compilation, including the Linux-only multicast test; NOT executed |
| Linux ARM64 application build | PASS cross-compilation; NOT executed |
| Linux runtime / systemd / multicast / resource measurements | PENDING; no Linux runtime is present and CI has not run |
| Browser visual QA | NOT RUN: no supported browser preview/control tool is available; DOM behavior is covered by automated tests |

The final full-suite attempt was blocked launching CLI, configuration, HTTP, storage and stream test executables; interfaces passed from cache. Successful component executions on earlier revisions are listed above rather than presented as a final aggregate pass. Application Control can block the same source on a subsequent launch. Final entire-suite/race execution, including the resolved-receiver reservation test and idle-worker handoff, remains an acceptance gate. No host security policy was disabled and failing/blocked commands were not silently skipped.

The user explicitly selected: use local checks and prepare CI, report Linux validation as pending. No external repository/host was supplied or created, and no CI run or publication is claimed.

## Resource use and Linux validation

No Linux CPU/RSS/capacity measurements are available. Queue/counter inspection in synthetic tests is not a substitute for measured deployment resource use. `scripts/linux-smoke.py` records actual max RSS/CPU and traffic when run on Linux; it refuses non-Linux execution.

Linux AMD64 and ARM64 builds, plus compilation of the Linux stream test binary, succeeded. Native Linux UDP/multicast, HTTP playback, systemd startup, graceful service shutdown, SQLite runtime and package lifecycle remain **pending**. The CI definition executes these gates (multicast explicitly skips only when no suitable interface exists), but has not been dispatched here. Cross-compilation is not Linux runtime validation.

Downloadable Phase 2 source and Linux binary bundles are in `build/`, alongside SHA256SUMS. The historical Phase 1 bundles remain intact. Debian package build/lifecycle sources are supplied; no locally tested `.deb` is claimed.

## Known issues and limitations

- This is not claimed production-ready: final blocked Go/race tests and actual Linux acceptance must pass first.
- Administration is trusted-local-user only; no remote authentication/accounts/passwords/session/TLS system is implemented. Management remains loopback-bound; Phase 3 security work is not started.
- Playback is IPv4/raw UDP MPEG-TS. No RTP, transcoding, FEC, PSI CRC/PES/codec validation, or exact packet-loss reconstruction. Three-sync acquisition can delay the initial small payload until more packets arrive.
- Only preconfigured approved sources are playable. Duplicate aliases are rejected; unicast senders sharing a destination interface/port require different ports. The `/udp/` route cannot disambiguate multiple configured interfaces; use channel IDs.
- Channel Start/Stop pin/override state is runtime-only; enabled/startup persist. Editing a channel resets its runtime counters and disconnects its viewers. Server policies require YAML edit/restart.
- Initial YAML seeds are imported once, not a live configuration synchronization feature.
- Kernel socket buffers may be clamped. Reported bitrate averages sample deltas; output bytes are payload accepted by writes, not player acknowledgments. No fabricated metrics are used.
- Queue budgets exclude one in-flight HTTP chunk per viewer, framer/PID history, HTTP/kernel buffers and process overhead. Closed handlers retain client-cap slots until cleanup.
- Full journald browsing, file logging/rotation, authentication, hot reload and backup tooling remain unavailable.
- Legacy Windows URL/byte compatibility is unverified. Module/maintainer identity placeholders remain for the project owner.

## Phase 3 readiness

The Phase 2 implementation is available for review and Linux acceptance testing. Close the reported validation gaps before promoting it. Phase 3 has not begun and requires explicit approval.
