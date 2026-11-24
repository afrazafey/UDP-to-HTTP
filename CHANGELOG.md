# Changelog

## 0.4.1 — validation candidate, unreleased

- Reworked the operations shell, overview, channel forms, import and playlists, and responsive management screens.
- Added measured chart range selection, network topology, per-channel monitoring, exact log component filtering, and a redacted system diagnostic copy.
- Fixed PowerShell coverage argument parsing and added per-runner coverage reporting.
- Fixed Debian directory entries and added native AMD64/ARM64 install, upgrade, and lifecycle gates.
- Added browser viewport checks and candidate package inspection with SHA-256 validation.

## 0.4.0 — validation candidate, unreleased

- Configurable jittered receiver retry and silence rejoin, separate UDP/TS timestamps, operational health/recovery outcomes and structured lifecycle events.
- Channel metadata with transactional schema-4 migration; atomic bulk actions and M3U imports with active allowlist enforcement.
- Local M3U parsing, ordered/group-filtered primary/secondary playlist export, explicit advertising host and separately restricted opt-in remote download.
- Safe channel/listener diagnostics, Linux interface error/drop counters and Windows process CPU/RSS observations.
- Existing dashboard extended with health summaries/table/details, metadata, import/export and confirmed bulk workflows.
- Consistent SQLite snapshot/verification/new-destination restore CLI and helper scripts.
- Native Windows SCM implementation, LocalService installation/ACL guidance, bounded service logs and Windows packet-info multicast reception.
- Cross-platform candidate/Debian builders, linked source metadata/SHA-256, native Linux/Windows CI and manual approval-gated draft prereleases.
- Installation, networking, security, production, upgrade/restore and troubleshooting documentation rewritten against actual behavior.

Production acceptance, native SCM, Debian lifecycle and ARM64 runtime gates are tracked in docs/PHASE4_REPORT.md. No stable release is authorized by this changelog.

## 0.3.0 development baseline

Embedded operations dashboard, retained metrics/events, staged settings rollback and explicit same-host management proxy compatibility. See the retained PHASE3_REPORT.md for historical implementation/validation notes; it is not the Phase 4 report.
