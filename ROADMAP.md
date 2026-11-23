# Roadmap

## Phase 1 — foundation retained

- [x] CLI, strict typed YAML/environment config, interface discovery.
- [x] Loopback management, health/status, structured logs, graceful shutdown.
- [x] SQLite/settings infrastructure, embedded React dashboard, themes.
- [x] systemd/Debian scaffolding, CI/release workflows, documentation.

The historical PHASE1_REPORT.md records that phase's verification limits.

## Phase 2 — streaming implementation

- [x] Allowlisted IPv4 multicast/unicast, explicit interface selection and source filtering.
- [x] Multicast destination/interface checks, join/leave and receive-buffer requests.
- [x] MPEG-TS synchronization across datagrams, 188-byte forwarding, corruption and CC monitoring.
- [x] One receiver per normalized source, shared viewers, duplicate-source rejection.
- [x] Client caps, bounded queues, slow-viewer isolation and idle cleanup.
- [x] Lifecycle states, inactivity detection, bounded retries, interface disappearance recovery.
- [x] Separate playback listeners/ports and CIDR policy, `/live/` and configured-source `/udp/` routes.
- [x] Additive SQLite migration, persistent channel CRUD and one-time YAML seeds.
- [x] Secure local management writes with CSRF tokens and strict JSON.
- [x] Dashboard channel/client/status/traffic/error views and real lifecycle actions.
- [x] Synthetic transport tests, local generator, Linux acceptance script and CI gates.
- [x] README, architecture, Linux testing instructions and Phase 2 report.
- [ ] Complete final Go/race/SQLite/HTTP suite execution where Windows Application Control blocks launches.
- [ ] Run native Linux socket, multicast, HTTP, systemd, graceful-shutdown and upgrade validation in CI.
- [ ] Record measured Linux resource use and hardware load behavior.

Implementation checkboxes do not claim runtime acceptance or production readiness. Linux validation remains pending by the user's selected validation plan. Legacy Windows compatibility remains unverified.

## Phase 3 — not started

- Authenticated remote administration, password hashing, session/CSRF lifecycle, throttling and transport security.
- Account bootstrap, audit history and server policy editing/reload.
- Bounded journal viewing, optional rotated file logging.
- Backup/restore tooling, package signing, dependency/license inventory.
- Hardware/ARM64 native runtime tests and sustained production acceptance.

Await explicit approval before beginning Phase 3. Close the Phase 2 validation gaps before claiming production readiness.
