# Phase 3 completion report — development build

Date: 2026-11-23. Version: 0.3.0-dev. The existing application was extended; the streaming engine was retained. Phase 4 has not begun. This report does not claim production readiness or Linux runtime acceptance.

## Implemented features and completed pages

Overview, Streams, Stream Details, Network Interfaces, Connected Clients, Monitoring, Activity Logs, Settings and System Information are implemented with real APIs. The UI includes hash navigation, collapsible/mobile sidebar, keyboard/focus-aware dialogs, explicit validation/confirmation, light/dark/system theme, loading/empty/error/stale/paused states and post-confirmation notifications. Charts use bounded actual backend samples; no invented history/resources or decorative action buttons.

Stream search/sort/status/type filtering, 20-row pages/mobile styling, persistent create/edit/delete, lifecycle actions, URL/VLC copying, details/log links and client disconnect work through real backend calls. Network configuration uses detected interfaces, capability/port checks, preview, revision checks, confirmation, restart-required state and rollback/recovery. General/Streaming/HTTP/Security/Logging expose supported persisted settings; management binding, database path, accounts/sessions/proxies/token playback/process startup/fixed recovery are explicitly read-only or unavailable. Host network/firewall rules are unchanged.

## Changed files

- New audit: PHASE3_AUDIT.md, written before code changes.
- New backend: internal/operations settings/preflight/monitor/Linux resources and tests; internal/events redacted bounded slog capture and tests; internal/storage/operations.go and tests; internal/httpapi/operations.go and tests; internal/cli/serve.go.
- Extended: config/defaults/JSON durations, network MAC discovery, stream snapshots/client metrics/receiver count/disconnection, storage migration, HTTP routing/security/activity, CLI composition, version metadata.
- Frontend: new modular OperationsApp, pages/components/hooks/services/types/utils/operations styles and Phase 3 tests. Existing App moved to LegacyApp; old Streaming and original tests remain as the Phase 1/2 fallback/regression surface. No large UI/chart framework was added.
- Updated configuration/environment examples, generator/runtime smoke instructions, Makefile, systemd/package documentation inputs, CI/vulnerability/development release workflows, README, ARCHITECTURE, ROADMAP and LINUX_TESTING. Historical reports/bundles are retained.

## API additions and extensions

Added GET dashboard, metrics, system, logs and capped logs/export; GET/PUT settings, POST settings/validate and settings/rollback; DELETE clients/{id}; GET streams/{id}/metrics. Existing status/health report phase 3 where operations is configured; interface discovery adds MAC; client snapshots add duration/delivered payload/lifetime rate; stream snapshots add worker start time. Existing channel/playback routes remain unchanged. There is no duplicate discovery API, frontend catch-all interception or generic process command endpoint.

All management routes remain local-only. Writes require CSRF, strict bounded JSON and content type; confirmed revisioned settings writes and rollback reject stale editors. Management mutations are rate limited. Network policy changes are staged rather than silently rebinding the live session. Full request/status/response contracts are documented in README.

## Database migrations and settings behavior

Schema v3 adds server_settings and operational_events, preserving all v1/v2 tables/data. Settings writes atomically update desired JSON/revision with optimistic checks. Accepted means the last successful startup snapshot; general/logging apply live but can also be reverted with the startup snapshot during rollback. New server/network/timeouts wait for restart. Point-in-time port probes cannot reserve a future bind; startup performs real binding and falls back to accepted settings. If playback remains unavailable, management remains usable with explicit disabled/failure status. Recovery override ignores saved server settings without deleting data.

Operational events use a 256-entry queue and bounded DB count/age retention; drops/failures are counted and shutdown drains best-effort for two seconds. Metrics stay in bounded RAM (240 aggregate points, 120/current channel); no samples are fabricated or written to SQLite.

## Test and build results

Local tools: Go 1.27.4, Node 24.21.0/npm 11.19.0, Windows development host. All blocked checks are recorded as blocked/incomplete rather than success.

| Check | Result |
| --- | --- |
| gofmt inspection and go vet ./... | PASS on final source after frontend build |
| Final go test ./... | INCOMPLETE: CLI (4.086s), config (0.524s), HTTP/API (3.021s), events and operations passed; network, storage and stream executable launches were blocked by Windows Application Control |
| CLI recovery integration | PASS: an actual occupied staged playback port restored the accepted bind while the management API remained reachable; shutdown completed |
| Earlier SQLite revision/event tests | PASS in an earlier run (2.307s); later added cases/final full package execution are blocked and not claimed passed |
| New operations summary/bounded-history tests | PASS; final run reused successful cached results |
| Event redaction/structured capture tests | PASS; final run reused successful cached results |
| CGO_ENABLED=1 go test -race ./... | BLOCKED: GCC is not installed/on PATH; runtime/cgo cannot build |
| Frontend strict TypeScript check | PASS |
| Frontend tests | PASS: 21 tests in 4 files, including all 9 original Phase 1/2 tests |
| Frontend production build | PASS: Vite 7.3.8; final embedded HTML/CSS/JS built |
| npm dependency audit | PASS: 0 reported vulnerabilities |
| Go govulncheck v1.10.0 | PASS: no vulnerabilities found in the host build configuration |
| Linux AMD64 application cross-build | PASS, CGO disabled |
| Linux ARM64 application cross-build | PASS, CGO disabled |
| Linux AMD64 operations and stream test binaries | PASS compilation, including Linux-only process/multicast code; NOT executed |
| Python generator/Linux smoke syntax | PASS |
| Native Linux, multicast, systemd, Debian lifecycle, real IPTV/codecs | NOT RUN/PENDING: no connected Linux runtime or repository/CI runner |
| Actual browser rendering/screenshots/visual QA | NOT RUN: no supported browser-control/preview tool available; DOM tests are not screenshots or device acceptance |

There were no silently excluded test packages. Intermittent policy blocks can affect a previously successful binary on another launch. The failed aggregate command is not reported as a pass. No Application Control policy was bypassed. Frontend tests initially identified accessible label/help association issues; those were corrected and the final full frontend suite passed.

The final source and architecture-specific ELF bundles are packaged as 0.3.0-dev development artifacts in build/, with SHA256SUMS. Existing Phase 1/2 bundles are preserved. These are cross-built Linux binaries, not verified Linux installations or locally tested Debian packages.

## Security review findings

- Existing peer/Host/Origin/cross-site/CIDR boundaries and streaming source validation are preserved.
- Remote management binding remains prohibited. There are no passwords/accounts/sessions here; local processes/users able to reach loopback are trusted administrators. Do not expose through a public proxy or claim multi-user authentication.
- Every new write uses CSRF/local authorization, size/type validation, revision/confirmation where appropriate and a rate limit. No generic host service-control or host network/firewall changes.
- Diagnostic access uses the same authorization as management. Recognized sensitive attrs, bearer/assignment patterns and credentialed/query URLs are redacted before capture/output. No raw playback credentials, complete configuration or private secrets are exported.
- SQLite queries are parameterized; migrations are additive. Settings cannot move management binding/storage paths; invalid network proposals do not replace running listeners. Count/age/history/queue/query caps bound storage and memory.
- Proxy headers, remote accounts, signed playback tokens and browser journal access remain unsupported and explicitly labeled. No security policy was bypassed to execute tests.

## Linux validation status, known issues and remaining work

No Linux runtime/connected GitHub repository is available. The user selected local validation plus prepared Ubuntu CI with Linux acceptance pending. Cross-build success, compiled Linux tests and Go vulnerability analysis are not Linux runtime acceptance. No multicast/provider IPTV/browser codec verification, systemd/package runtime verification, real Linux resource measurements or screenshots are claimed.

Pending acceptance: complete the blocked Go/race suite on an allowed host; run prepared Ubuntu unit/multicast/HTTP/settings-rollback/systemd/package checks; conduct actual browser visual/accessibility and real-stream/codec review. No screenshots were fabricated. Native ARM64 execution and sustained resource/capacity testing remain separate from cross-compilation. Read-only/unavailable security/logging/backup features are described in README; this development release must not be promoted as production-ready.

Stop for review. Phase 4 has not started.
