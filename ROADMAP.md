# Roadmap

## Phases 1 and 2 — retained

CLI/config/discovery/SQLite/embedded UI/systemd/package infrastructure and the shared UDP/MPEG-TS playback engine remain. Historical reports record their actual validation limits; their source and original tests are retained.

## Phase 3 — management operations implementation

- [x] Mandatory repository audit and actual route/model inventory.
- [x] Modular professional dashboard, light/dark/system themes, collapsible/mobile/keyboard navigation.
- [x] Reusable accessible dialogs, forms, loading/empty/error/stale states and confirmed notifications.
- [x] Overview summaries, actual activity/health and bounded historical throughput charts.
- [x] Search/sort/type/state filters, pagination/mobile stream rows, CRUD/actions/details/log links.
- [x] Stream details, measured transport counters/history and honest external-player preview fallback.
- [x] Real interface/MAC/IPv4/capability discovery and staged network settings with preview/confirmation.
- [x] Revisioned settings persistence, active/desired state, startup rollback and recovery override.
- [x] Client duration/bytes/rate/state and safe confirmed disconnection.
- [x] Runtime/CPU/RSS monitoring where supported, pause/manual refresh and persisted interval.
- [x] Bounded sanitized event store, filters/pagination/copy and diagnostic export.
- [x] General/Streaming/HTTP/Security/Logging settings with explicit read-only capabilities.
- [x] Real system/listener/config path/process metadata.
- [x] Additive schema v3, regression/new tests, development builds, packaging/CI/documentation.
- [ ] Complete all local Go/race executions currently blocked by Application Control/missing GCC.
- [ ] Execute Ubuntu CI/native Linux acceptance, including multicast, occupied-port rollback, systemd and packages.
- [ ] Perform actual browser visual/accessibility review and real IPTV/codec validation.

No production readiness is claimed until acceptance gates pass. Remote administration stays prohibited, with trusted local users and CSRF as the management boundary. Unavailable sessions/accounts/trusted forwarding/playback tokens are labeled read-only rather than exposed without implementation.

## Phase 4 — not started

Await explicit review/approval before additional work. Potential future scope includes authenticated/TLS remote administration, account/session policy, signed playback authorization/trusted proxy configuration, immutable audit guarantees, file/journal integration, backup/restore/signing and sustained hardware acceptance. No such feature is silently enabled by Phase 3.
