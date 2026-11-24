# Phase 4 production reliability report

This report records the Phase 4 baseline. The Phase 4.1 follow-up fixes the coverage argument and Debian archive construction and adds browser layout evidence. [Phase 4.1 report](PHASE4_1_REPORT.md)

Date: 2026-11-24 (Indian/Maldives). Target 0.4.0; local artifacts are **0.4.0-dev validation candidates**. Baseline commit: `35204787ef5f0efb726c72216a60fcbbb62c2846`; changes remain uncommitted and candidate metadata marks the dirty source. No push, live deployment, CI dispatch or release publication is performed.

**Phase 4 is not declared production-complete.** Functional implementation is provided, but required Linux/ARM64/SCM/race/packaging/soak acceptance is still blocked or pending. Do not promote a stable release from these local checks. The user-reported Phase 3 TRACE URBAN success is retained as a regression reference and has not been repeated on the live Ubuntu VM.

## 1. Implementation summary

Extended the existing receiver/fan-out and React dashboard rather than replacing the working engine. Added recovery configuration/observations, metadata, transactional batch/import, playlists, safe diagnostics, SQLite backup/verification/new-destination restore, native Windows SCM implementation, platform builds/packages and documentation. Preserved actual-peer restrictions, default loopback management, source/client allowlists and CSRF.

## 2. Locally demonstrated functionality

- Real Windows UDP multicast join → receive → TS fan-out → leave/rejoin passed in an earlier full run after the Windows packet-info/receive-loopback correction. This is locally generated traffic on a selected non-loopback adapter, not proof of external IPTV/IGMP routing or player decoding.
- Real unicast fan-out, slow-client/buffer isolation and lifecycle tests, actual socket-close recovery, simulated interface interruption/recovery, inactivity reopening with shared viewers, packet-info validation and bounded retry tests passed in that run. Simulated interface failure is explicitly distinguished from physically disabling a NIC.
- HTTP API tests exercised policy-rejected import preview, independent commit validation, atomic rejection, actual commit/bulk disabling, playlist MIME/external URLs/exposure restrictions, diagnostics destination restrictions and CSRF/security boundaries. Standalone M3U parsing/generation edge cases also pass.
- SQLite metadata migration/failure rollback, atomic SQL rollback, consistent backup verification/restore, no-overwrite, pre-upgrade backup and concurrent-writer busy-timeout/WAL tests pass in the final run.
- Final receiver tests also pass finite-budget/manual restart, concurrent create-only import intent, concurrent bulk/edit preservation and cancellation-before-joining a 20-receiver bulk delete.
- Frontend: 27 tests pass in 5 files, including new import selection/confirmation/policy-failure retention/upload bounds, bulk confirmation and safe diagnostic requests. TypeScript and production build pass.
- Measured actual Windows UDP→HTTP load at 1/10/25/50/100 viewers: one receiver at each size, zero framing-alignment integrity failures, clean client release, and 10 repeated start/stop cycles. See section 12.

## 3. Partially validated / incomplete acceptance

| Requirement | Implementation and remaining validation |
| --- | --- |
| P4.01 Recovery | Implemented; local recovery, finite-budget/manual-restart and bulk cleanup tests pass. Long physical-interface fault acceptance remains pending. |
| P4.02 Health | Current state/timestamps/counters and bounded histories implemented; long-duration retention/resource behavior remains a soak gate. |
| P4.03 M3U export | Implemented; HTTP generation/exposure and standalone playlist edge-case tests pass. Real client playlist/player acceptance remains pending. |
| P4.04 Import | Parser/preview/confirmed atomic commit implemented; HTTP allowlist/atomicity, standalone parser and concurrent create-only intent tests pass. Browser/file acceptance remains pending. |
| P4.05 Channel management | Metadata, search/group/sort, bulk actions, copy/M3U and duplicate draft implemented. Exact same-source aliases are intentionally still rejected; duplication requires a unique source/interface. |
| P4.06 Diagnostics | Fixed safe actions, socket/join/arrival/TS/payload separation and Linux counters implemented. OS route tables and other applications' memberships are explicitly unavailable. Linux counter runtime acceptance pending. |
| P4.07 Dashboard | Existing themes/components retained and feature tests pass. Browser visual/responsive/codec QA was not available. |
| P4.08 Logging | Structured/redacted lifecycle/import/export/config events, JSON/text console and bounded native-service files implemented. Native Windows file ACL/rotation acceptance under SCM pending; rotation/handler unit tests passed. |
| P4.09 SQLite | Schema 4, existing WAL/5s busy policy, atomic batches and consistent backups implemented. Migration/backup/restore/contention tests pass; native package upgrade remains pending. |
| P4.10 Security | Preserved loopback/proxy/CSRF/source/client controls; local-control multicast permanently blocked, no imported URL/logo fetch, bounded bodies/metadata/diagnostics. Final all-suite/security gate remains blocked. |
| P4.11 Linux install | Candidate builders, Debian hooks/unit and docs provided. Native dpkg/systemd inspection/install/upgrade/remove and Phase 4 dual-NIC regression pending. |
| P4.12 Windows install | Native SCM and packet-info path implemented, native compilation/local multicast and handler tests demonstrated. Actual SCM installation/start/restart/stop/uninstall not run locally; prepared disposable-CI integration test is required. No completed Windows service support claim. |
| P4.13 Packaging | Linux AMD64/ARM64 and Windows AMD64 builders, standard Debian format, linked metadata, source ZIP and SHA-256 manifest provided. Native ARM64, full release gates, licensing and stable approval remain blockers. |
| P4.14 Documentation | Root README/changelog and all requested docs provided against actual commands/routes. Platform commands not executed here are explicitly described as pending acceptance. |
| P4.15 Testing | Significant unit/integration/frontend/CI expansion implemented. Final Go run is blocked at multiple executable launches, race needs GCC, Linux/SCM acceptance unavailable. |
| P4.16 Performance | Short measured synthetic Windows fan-out completed. No long soak, network capacity, Linux load or real decoder benchmark is claimed. |

## 4. Files added and modified

Added `internal/playlist`, `internal/service`, recovery/batch/platform receiver helpers, storage backup/batch/reliability tests, production HTTP endpoints/tests, CLI database commands/tests, Windows process observations, Linux interface counters, new production frontend page/tests and diagnostics component. Modified existing config/channel/manager/framer/monitor/guards/CLI/UI types/forms/pages while keeping existing playback routes.

Added portable build/release/load/backup/restore/Windows installation/SCM CI scripts, Windows config, reusable platform validation workflow and the requested docs/changelog. Updated CI/release workflow, samples, Makefile, root README/architecture, line-ending rules and historical Linux-testing links. `multicast_linux_test.go` becomes the explicitly tagged Linux-or-Windows integration test. Generated binaries, databases, dependencies and measurements stay ignored under `build/`/`data/`/tool caches. Review `git diff --stat` and `git status --short` for the exact working-tree manifest.

## 5. Database migrations

Forward-only schema 3→4 atomically adds missing JSON metadata defaults: logo_url, tvg_id, tvg_name, group, sort_order, description, preferred_playback_port. Existing channel identity/source/settings/revisions/events survive. Failed migration rolls back, newer schemas fail closed, and old binaries need a schema-compatible pre-upgrade snapshot. No database reset, destructive downgrade or reseeding occurs.

Backups use VACUUM INTO with a new reserved output, fsync and integrity/schema/SHA-256 verification. The backup CLI does not migrate its source, including a pre-upgrade schema-3 DB. Restore verifies both copies, requires offline confirmation and refuses existing destinations; administrators switch to the new restored path manually.

## 6. New API endpoints

| Method | Route |
| --- | --- |
| GET | `/api/v1/playlist/config` |
| GET | `/api/v1/playlist.m3u?port=preferred&group=...&id=...` |
| POST | `/api/v1/import/preview` |
| POST | `/api/v1/import/commit` |
| POST | `/api/v1/streams/bulk` |
| GET, POST | `/api/v1/diagnostics` |
| GET on playback only | `/playlist.m3u` (explicit opt-in + both CIDR checks) |

Existing `/streams/{id}/metrics` includes current health and bounded recent history. New management mutations inherit CSRF/content/size/rate/origin checks. Database/service operations are CLI-only; no arbitrary shell/file/network endpoint exists.

## 7. Dashboard changes

Overview health summary/table and restart totals; separate UDP/TS/recovery observations in details; metadata forms; group filters, selection and confirmed bulk actions; duplicate-as-draft; copied advertised playback URLs, individual M3U, full playlist controls; bounded file/paste preview and confirmed transactional import; fixed safe diagnostics. Current visual identity, themes, accessible dialogs, failed-draft retention and cancellable non-overlapping polling remain in use. Unsupported metrics remain unavailable. No automatic logo downloads or fake playback validation are added.

## 8. Linux installation and packaging status

Built Linux AMD64/ARM64 and Debian AMD64/ARM64 candidates, keeping systemd, dedicated non-root account, private persistent directory, conffiles, journald and retained state on uninstall. Static inspection passed for 9 SHA-256 entries, 2 Linux bundles, 1 Windows bundle, 1 source archive and 2 standard ar-format Debian packages, including expected ELF/PE architecture, linked metadata and packaged docs/hooks/unit. Portable Debian output still needs native dpkg inspection/install/upgrade/remove verification. This workstation has no Linux/WSL/Docker/dpkg/systemd runtime; neither ELF generation nor archive inspection is native acceptance. The reusable CI workflow prepares these checks but was not dispatched.

## 9. Windows installation and service status

Native AMD64 compilation succeeds. Real local multicast/unicast/load measurements and Windows packet-info/SCM-handler/log-retention unit tests have passed. LocalService SCM registration/execution/control, bounded JSON files and ACL/install/upgrade/uninstall paths are implemented. Actual service integration is unexecuted here to avoid installing a host service; the guarded GitHub Actions script runs only on a disposable runner. Windows ARM64 is excluded. Do not claim native SCM validation from compilation/handler tests.

## 10. README/documentation status

README has the requested 20 topics, actual artifact names and command/route reference. Added docs/ARCHITECTURE, INSTALL_LINUX, INSTALL_WINDOWS, NETWORK_CONFIGURATION, M3U_IMPORT_EXPORT, PRODUCTION_DEPLOYMENT, TROUBLESHOOTING, UPGRADE, SECURITY, PHASE4_PLAN and this report; added CHANGELOG. Addresses from the user-verified dual-NIC TRACE URBAN deployment are examples only, never defaults. No new Nginx playback configuration is represented as validated without Nginx runtime testing. SSH forwarding is the management recommendation. No repository license was found or invented.

## 11. Exact local validation commands and outcomes

Go commands used the local toolchain `.tools/go/bin/go.exe` with workspace GOPATH/GOCACHE/GOTMPDIR. Standard command spelling below denotes that executable, not a system Go installation.

| Command | Observed result |
| --- | --- |
| `go fmt ./...` | Pass, no remaining format changes |
| `go vet ./...` | Pass |
| `go test -json ./...` earlier full snapshot | 69 top-level tests passed; 211 including subtests; no assertion failures; playlist/storage launches blocked (see build/phase4-go-tests.jsonl) |
| `go test -json ./...` final pre-package snapshot | 63 top-level tests passed (150 including subtests); cli and httpapi test executables blocked by Application Control. No executed test assertion failures. Full suite **not passed** (build/phase4-go-tests-final.jsonl). Earlier final-source snapshots also exercised those packages successfully before the last metadata-preview refinement; they do not replace its pending final execution. |
| `CGO_ENABLED=1 go test -race ./...` | Cannot build: GCC is absent; no race result claimed (build/phase4-race.txt) |
| `go run golang.org/x/vuln/cmd/govulncheck@v1.10.0 ./...` | Pass: No vulnerabilities found for the analyzed host build |
| `npm.cmd ci --cache .npm-cache` | Pass, 156 packages installed; audit 0 vulnerabilities. Earlier attempt hit a locked native CSS module; verified workspace Vite was stopped, dependencies reinstalled and preview restored |
| `npm.cmd run typecheck` | Pass |
| `npm.cmd test` | Pass: 27 tests / 5 files |
| `npm.cmd run build` | Pass: production React/Vite assets embedded; latest output 338.44 kB JS and 41.29 kB CSS before compression |
| `python -m py_compile scripts/build-release.py scripts/build-deb.py scripts/load-test.py scripts/linux-smoke.py` | Pass |
| PowerShell AST parser on install/service-test/backup/restore scripts | Pass syntax checks; actual service install intentionally unexecuted |
| `python scripts/load-test.py --binary build/udphttp-proxy-windows-amd64.exe` | Pass, actual standalone Windows UDP→HTTP measurement; not an SCM stop test |
| `python scripts/build-release.py --version 0.4.0-dev --go .tools/go/bin/go.exe` | Pass: Linux AMD64/ARM64, Windows AMD64, both Debian packages, bundles/source ZIP and manifest built |
| `python scripts/verify-artifacts.py` | Pass: 9 checksums, container/architecture, metadata and packaged-file inspection; native execution not implied |
| Final packaged Windows executable `version` | Launch blocked by Application Control; linked version/SHA/UTC timestamp are verified by static binary inspection, not a claimed runtime output |
| Linux smoke, native dpkg/systemd, native ARM64 | Not executable in this workstation; prepared in CI/docs, not passed |

Application Control is an independent host restriction; it was not disabled, bypassed or treated as successful testing. More successful earlier runs do not replace the required clean final platform/race gate. JSON outcomes are retained as evidence; no “all tests passed” claim is made.

## 12. Actual performance observations

From `build/phase4-load-results.json`: synthetic valid framing, local unicast sender and real HTTP readers on the Windows development host, approximately six measured seconds per case, whole-daemon resources (including monitor/SQLite/logging). CPU is the latest 3-second process sample; 100% represents one CPU core. RSS/goroutines are sampled maxima within that short case, not a long-term peak.

| Viewers | Output Mbps | Latest CPU % | Max observed RSS MiB | Max goroutines | Active receivers | Integrity errors |
| --- | ---: | ---: | ---: | ---: | ---: | ---: |
| 1 | 0.665 | 0.00 | 64.13 | 15 | 1 | 0 |
| 10 | 6.943 | 5.73 | 65.29 | 33 | 1 | 0 |
| 25 | 16.120 | 9.38 | 66.72 | 63 | 1 | 0 |
| 50 | 32.237 | 11.98 | 68.90 | 113 | 1 | 0 |
| 100 | 65.757 | 22.39 | 71.14 | 213 | 1 | 0 |

All readers disconnected and 10 start/stop cycles completed. Zero CPU in the first latest sample is the observed counter result, not an assumed unavailable value. Integrity checks verify 188-byte alignment/sync, not PID timing/decoder correctness or exact datagram loss. Windows standalone cleanup is process termination; graceful shutdown belongs to the separate SCM integration gate. Linux load/shutdown and longer slow-client/recovery soaks remain pending. Measurements preceded the final bulk/import concurrency and metadata-preview refinements; those changes do not alter the measured UDP/HTTP fan-out path. These are not throughput capacity guarantees.

## 13. Security review

Loopback management, explicit source/client policies, security headers, strict CSRF/JSON/bounds/rate guards and existing trusted-proxy checks remain. Reserved local-control groups are always blocked. Imports never authorize or fetch remote sources/logos; only supported URL patterns convert to literal UDP configuration. Metadata is revalidated for export. Diagnostics probe only configured daemon-owned local listeners without redirects/proxies; no shell or arbitrary target is accepted. Remote playlists are explicit opt-in with two actual-peer CIDR policies. Backups are private and restore never overwrites live state. Logging redacts recognized sensitive values. Nginx authentication/TLS and local OS account protections remain separate administrative responsibilities.

## 14. Known issues and limits

- The final Go run is blocked at cli/httpapi executable launch; race instrumentation cannot build without GCC. Other final tests, including config/playlist/storage/recovery/concurrency/service-handler tests, pass. The legacy reserved-range override expectation now passes with the required permanent local-control multicast block. The latest metadata-preview preservation test still requires an unblocked httpapi execution.
- Explicit channel replacements/import updates use a complete preview snapshot, without per-channel optimistic revisions. Avoid simultaneous edits to the same channel; server-wide settings retain their existing revision checks.
- Linux Phase 4 runtime, Debian lifecycle, ARM64 runtime and native Windows SCM acceptance are pending.
- No browser visual/real codec/player QA or long-duration physical-network fault/slow-client soak was available.
- Duplicate draft requires a unique source identity; source aliases are not a second receiver path.
- Histories/counters reset with process/channel replacement; they are bounded operational observations, not durable billing/audit metrics.
- Remote playlists use client CIDRs, not signed URLs. Existing proxy support relies on external authenticated TLS policy, not native accounts.
- No repository license is declared; distribution/stable publication needs owner review. Generated bundles carry available dependency notices without inventing a project license.

## 15. Remaining production release blockers

Run the prepared native Linux/Windows CI with a working C toolchain, resolve every failed/blocked assertion, inspect/install/upgrade/remove the Debian packages, validate native SCM/ACL/logging/shutdown, validate ARM64 on actual ARM hardware/runner, repeat the working dual-NIC TRACE URBAN test, complete longer recovery/buffer/slow-client/resource soaks and browser/player acceptance, resolve license/distribution policy, then obtain explicit stable-release approval. No live host mutation or remote push is authorized by completion of local source work.

## 16. Recommended Phase 5 scope

First close the Phase 4 acceptance gates. Then consider hardware-backed long-soak automation, improved per-platform telemetry, optional source aliases with carefully designed shared ownership, durable aggregate health reporting and separately designed authenticated TLS administration if explicitly requested. Do not add transcoding, broad remote access or a new streaming architecture before the current reliability candidate is accepted.
