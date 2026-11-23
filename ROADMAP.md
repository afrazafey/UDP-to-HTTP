# Roadmap

## Phases 1 and 2 — retained

CLI/config/discovery/SQLite/embedded UI/systemd/package infrastructure and the shared UDP/MPEG-TS playback engine remain. Historical reports record their actual validation limits; their source and original tests are retained.

## Phase 3 — historical management operations implementation

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

This is the historical Phase 3 checklist. Current behavior, including explicit same-host proxy compatibility and Phase 4 validation status, is documented in README.md and docs/PHASE4_REPORT.md. Native remote binding/account sessions and signed playback tokens remain unavailable.

## Phase 4 — implementation candidate, acceptance pending

Recovery/health, metadata/schema 4, transactional imports/bulk actions, M3U export, diagnostics, consistent backup/restore, native Windows SCM implementation, platform packaging, CI and documentation are implemented. Local evidence includes 27 frontend tests, receiver/recovery/concurrency tests and measured Windows fan-out up to 100 viewers. Full Go/race/native-platform/install/soak gates remain outstanding; the exact report distinguishes passes from blocked or unexecuted checks. No stable release, remote push or live deployment is performed.

## Phase 5 — after Phase 4 acceptance

First close current release blockers. Then consider hardware-backed soak automation, source aliases and improved telemetry, with any native remote authentication/TLS or signed playback design separately reviewed. Preserve source/client policies and the shared receiver architecture.
