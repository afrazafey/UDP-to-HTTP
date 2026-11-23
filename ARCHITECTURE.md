# Architecture

## Components and boundaries

```mermaid
flowchart LR
  CLI[CLI / signal context] --> Config[Strict YAML server policy]
  CLI --> DB[(SQLite v2)]
  CLI --> Manager[Shared stream manager]
  CLI --> Admin[Loopback management listener]
  CLI --> Playback[Separate playback listeners / CIDR policy]
  Admin --> DB
  Admin --> Manager
  Admin --> UI[Embedded React dashboard]
  Playback --> Manager
  Manager --> UDP[One UDP receiver per configured identity]
  UDP --> TS[Bounded MPEG-TS framer]
  TS --> Queues[Bounded client queues]
```

Phase 1 CLI, network discovery, storage/settings, JSON logging and embedded UI remain. `internal/stream` has no HTTP or UI dependency. `internal/config` and `internal/storage` use the engine's typed Channel model; the engine accepts a narrow repository interface and never imports either package. `internal/cli` composes everything and owns all listener lifetimes.

## Configuration and storage

Server-wide YAML/environment policy remains authoritative. Channel definitions become SQLite-authoritative after a one-time YAML seed. Additive schema v2 creates `channels(id, configuration, updated_at)` with JSON validity checks while preserving schema v1 settings and the startup snapshot. Seed import and its marker commit atomically. Deletes are not resurrected on restart. Parameterized INSERT/UPDATE/DELETE operations persist a definition before runtime replacement, so a failed write preserves the running channel. Unsupported future schemas are rejected.

Channel configuration includes ID/name, multicast/unicast type, literal address, UDP destination port, explicit interface selection, enabled state, idle retention, caps, buffer requests and startup policy. Bounds and allowlists are enforced in the engine as well as YAML validation. Management creates/updates also resolve actual host interfaces. A stored interface that later disappears produces retrying state; policy-invalid stored channels remain visible in error state without opening sockets. Duplicate normalized sources, or unicast bind collisions, are rejected.

Database files use 0600; newly created state directories use 0700. Direct database symlinks are rejected. Parent directories must be trusted. The service uses a dedicated system account and writable state directory. Backup before schema migration; older binaries reject newer schemas. There is no YAML rewrite or hot reload.

## Concurrency, lifecycle and bounds

The manager has an operations mutex serializing configuration and start/stop/restart/subscribe operations, plus a state mutex protecting entries, clients, counters and budgets. HTTP/network I/O never occurs while holding the state mutex. Interface discovery and identity checks during configuration are short local OS calls; normal packet delivery does not perform discovery under that mutex.

Each accepted channel owns at most one receiver worker. All HTTP subscriptions share it, including across playback listeners. Duplicate channel aliases for the same upstream are deliberately rejected, rather than creating duplicate memberships. A resolved socket reservation is acquired before opening, so interface/address changes cannot create duplicate memberships or unicast binds. Reservations are released after closure. An idle worker marks itself retiring under the state mutex; a new viewer/start waits for that worker to release its socket before replacement. The worker context derives from the service context. Stop cancels and joins the worker before restart/replacement can rebind. Manager Close cancels all workers and joins them. No receiver is created per HTTP viewer.

State transitions:

```mermaid
stateDiagram-v2
  stopped --> starting: Start or first viewer
  starting --> waiting: Socket open / membership joined
  waiting --> running: Valid synchronized TS payload
  running --> waiting: Inactivity threshold
  running --> retrying: Socket or interface failure
  waiting --> retrying: Socket or interface failure
  starting --> retrying: Bind / join failure
  retrying --> waiting: Reopen succeeds
  retrying --> stopped: Stop / idle / cancellation
  running --> stopped: Stop / idle / cancellation
  waiting --> stopped: Stop / idle / cancellation
```

Disabled definitions cannot start. Invalid persisted policy produces `error` without reception. `waiting` is an observed lack of valid TS packets, not an inferred UDP connection failure. Inactivity leaves the socket open. Socket failures close membership/resources before bounded exponential retry (250ms..30s); stable reception resets backoff. Interface identity/assigned IPv4/up/multicast support is rechecked about once a second. On-demand idle retention applies even while retries are pending. Always-start and manually pinned receivers keep running with no viewers. Runtime Stop blocks on-demand restart until Start/Restart; persistent `enabled`/`startup` govern the next process start.

Each subscription owns a nonblocking queue capped at 64 immutable chunks and its configured byte backlog. Per-stream and global queued-byte caps provide additional bounds. A publisher never waits on a viewer: overflow terminates that subscription and records an event/counter. Closed queues are drained to release references. Closing HTTP handlers keep their client-cap slot until handler cleanup, preventing repeated slow connections from bypassing the cap. A handler can hold one in-flight chunk (at most about one UDP payload) outside queue accounting; Go/net/http and kernel socket buffers are additional bounded overhead, not included in the queue budget.

PID history is bounded by the 8192 possible PIDs, using lazy records; pending resynchronization data is capped at two packets, with at most a datagram plus that prefix during parsing. Error history is capped at 20 entries of 1024 bytes. Channel/client counts and retry delays are bounded. No unbounded playback history is retained.

## UDP and MPEG-TS

Unicast binds to the selected local interface address and destination port. It accepts only the configured allowlisted sender IPv4; other sources increment rejected-datagram counts and never enter the framer. Loopback reception is explicitly opt-in for fixtures/local operation.

Multicast uses Go's UDP multicast listener (reuse/membership setup) and `golang.org/x/net/ipv4` for ancillary destination/interface metadata and explicit membership leave. Both ingress interface index and destination group are checked, preventing cross-delivery between groups sharing a port. Joins require an up, multicast-capable IPv4 interface. Closing also releases kernel membership. The OS may clamp the requested receive buffer; no sysctls are changed.

Read deadlines are 500ms, allowing cancellation without a permanent blocking read or an extra goroutine per datagram. Receivers check interface state and inactivity between reads. Socket/interface errors are structured events with actionable source/interface context. The engine does not call UDP connected/disconnected based on packet arrival.

The framer searches for three sync bytes 188 bytes apart, buffers split datagrams, emits whole packets unchanged, and drops invalid adaptation headers/garbage. TEI-marked packets are counted but retained. Payload continuity counters, explicit adaptation discontinuities and identical duplicates are monitored per PID; null PID continuity is ignored. No timestamp/codec rewriting occurs. Packet-loss counts are not invented from ambiguous CC gaps; continuity errors and discarded bytes are reported separately. PSI CRC/PES/codec decoding, RTP and FEC are out of scope.

Counters measure accepted UDP payload bytes/datagrams, forwarded TS packets, corrupt observations, discarded bytes, CC errors, explicit discontinuities, duplicates, retries, rejected sources, slow viewers, and payload bytes accepted by HTTP writes. Rates average counter deltas between samples; they are not NIC-level or player-acknowledged throughput. Counters reset on process/channel configuration replacement, not normal Start/Stop/Restart. Uptime resets for each receiver worker lifetime.

## HTTP and local administration

Management stays loopback-only, validating direct peer, literal Host/port, Origin and browser cross-site metadata. Forwarded headers do not grant access. GET/HEAD remain available for status/discovery; channel mutations additionally require strict bounded JSON plus a cryptographically random per-process CSRF header token. There is no remote auth/password/session system yet. Local processes able to reach loopback are trusted administrators. Origin/token failures are denied before reaching storage or the engine.

Playback is served only on independent listeners with their own actual-peer CIDR allowlist; empty means deny all. Only configured channels can be selected. `/udp/{group}:{port}` selects a unique enabled registered multicast channel, not an arbitrary network target. Unicast is selected by `/live/{id}.ts`. Management routes never appear on playback listeners. Legacy Windows URL compatibility is unverified.

Playback waits for a valid TS chunk before sending 200/video/mp2t. Startup timeout returns 503 without a misleading successful empty stream. Subsequent writes have per-write deadlines and immediate flushes; the server has no absolute lifetime WriteTimeout. Request cancellation closes the subscription. Service cancellation interrupts reads/handlers, shuts down every listener with bounded grace, forces connection closure on expiry, then closes the manager and SQLite. Structured server errors use slog JSON.

## Frontend and packaging

The existing React navigation/layout/theme/interface views remain. Streaming.tsx adds measured snapshots, clients, counters/errors, channel forms, lifecycle actions, and actual URL copying. Fetches have deadlines, polling does not overlap, unmount aborts reads, and failures visibly mark stale data. Channel mutations fetch a current token and report backend validation errors. Server settings stay read-only. No fabricated stream/resource data is used.

Vite output is embedded by Go; frontend builds must finish before Go checks/builds read those assets. Production needs no Node/CDN. systemd keeps least privilege and separate local management/playback policies. dpkg preserves config; schema migrations preserve state. CI includes native Linux tests, race tests, architecture builds, multicast tests where interfaces permit, synthetic four-viewer/two-listener acceptance testing, persistence/restart/SIGTERM checks and package lifecycle checks. These definitions are not evidence they have run: see PHASE2_REPORT.md and LINUX_TESTING.md.

Primary references: [Go net](https://pkg.go.dev/net), [ipv4 multicast operations](https://pkg.go.dev/golang.org/x/net/ipv4), [Go HTTP response controller](https://pkg.go.dev/net/http#ResponseController), and [SQLite driver](https://pkg.go.dev/modernc.org/sqlite). Source remains independently implemented.
