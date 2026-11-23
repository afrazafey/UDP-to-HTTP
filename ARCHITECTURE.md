# UDPHTTP Proxy architecture

The current architecture and concurrency/security invariants are documented in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

Phase 4 extends the existing shared receiver, bounded fan-out and embedded dashboard. It preserves separate loopback management/playback listeners, explicit input interfaces, source/client allowlists and SQLite-authoritative channels. See [docs/PHASE4_PLAN.md](docs/PHASE4_PLAN.md) for the baseline audit and implementation sequence, and [docs/PHASE4_REPORT.md](docs/PHASE4_REPORT.md) for validation limits.
