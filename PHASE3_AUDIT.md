# Phase 3 repository audit

Audited 2026-11-23 before implementation. Existing version: 0.2.0-dev. This is the existing standalone workspace, not a connected Git repository. README, architecture, roadmap, both phase reports, Linux instructions, module/dependencies, config/network/stream/HTTP/storage/CLI source, frontend components/tests, configuration examples, deployment, packaging hooks and CI/release workflows were inspected.

## Actual backend

- CLI: serve, version, config validate and interfaces. Standard-library HTTP, slog JSON, context/signal shutdown; embedded Vite assets.
- Management: GET /health, /api/v1/status, /api/v1/interfaces, /api/v1/csrf; GET/POST /api/v1/streams; GET/PUT/DELETE /api/v1/streams/{id}; POST /{id}/start, /stop, /restart; GET /{id}/status and /errors; GET /api/v1/clients.
- Separate playback listeners: GET /live/{id}.ts and /udp/{group}:{port}. The latter selects only preconfigured sources. Unknown APIs are JSON 404; the frontend currently has no route fallback. Playback is not served by management.
- Security: literal IPv4 loopback management binding and direct-peer/Host/Origin/cross-site checks; random per-process CSRF token and strict bounded JSON for mutations. Local users are trusted administrators. No accounts, sessions, remote authentication, trusted proxy support, playback tokens or TLS administration. Forwarded IP headers are ignored. Playback uses a separate actual-peer IPv4 CIDR allowlist.
- Channels: ID/name, multicast/unicast source address and destination port, explicit interface name/IP, enabled, idle timeout, max clients, UDP/client buffers, on_demand/always startup.
- Engine: one shared receiver per normalized identity, runtime socket reservations, bounded fan-out and queue budgets, idle handoff, cancellation/joined restart, retries, group/ingress and sender filtering, TS synchronization and continuity tracking. Preserve this engine.
- Snapshots: state, clients, worker uptime, last valid packet, sampled input/output bitrate, accepted input/output bytes, TS/corruption/CC/discontinuity/duplicate counters, rejected datagrams, retries, slow-client disconnects and up to 20 recent errors. Client snapshots currently expose ID/channel/IP/connected time/queued bytes/closing, but lack delivered bytes and bitrate. Stream start timestamp is not exposed yet.
- Network discovery: index/name/MTU/up/loopback/multicast and IPv4 addresses. No MAC field yet. OS API, no host network mutations.
- Strict YAML/environment server configuration. Server settings are startup-only and browser read-only. Channel seeds are imported once into SQLite; later channel CRUD is DB-authoritative. Management port 8080, playback 7781/7782 are safe defaults; example LAN addresses are not assumed.
- SQLite schema 1 settings table; additive schema 2 channels table, JSON validity constraints, atomic updates, one-time seed marker. No operational event/revision tables or metric history.

## Actual frontend

Existing React/TypeScript/Vite, Tailwind 4 and Lucide. App.tsx owns navigation, dark/light theme, status/interface polling and mostly read-only pages. Streaming.tsx provides real overview counters, channel CRUD forms, client lists, transport tables and action/copy controls. Nine frontend tests cover Phase 1 behavior and channel actions/forms/errors/clipboard. No routing/deep links, system theme, collapsible desktop sidebar, reusable accessible modal system, throughput charts, full details page, settings saves, bounded journal viewer or CPU/RSS API.

## Validation state and implementation decisions

Existing reports explicitly distinguish successful Windows component runs, intermittently blocked executable launches, missing GCC for race testing, cross-builds and unexecuted Linux runtime acceptance. No connected GitHub repository/Linux host has been provided. Preserve these distinctions and do not bypass Application Control.

Reuse all listed routes/models. Add operations aggregates/history, settings/revisions, logs and system APIs only where missing. Extend snapshots additively for start/client metrics and safe disconnection. Preserve local authorization and CSRF; remote binding stays unsupported rather than exposing a new unauthenticated API. Use durable, revision-checked settings staged for restart, point-in-time network preflight, last-accepted rollback and a YAML recovery override; management binding/storage paths remain read-only. Persist bounded operational events, keep runtime metric history bounded in memory, and retain the existing UDP/MPEG-TS engine. Do not start Phase 4.
