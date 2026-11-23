# Phase 1 implementation report

Date: 2026-11-23. Scope: foundation only. Phase 2 has not been started.

## Implemented

All fourteen requested Phase 1 foundation items are implemented: Go project/CLI, typed strict configuration, OS network discovery, basic HTTP endpoints, signal-based graceful shutdown, JSON logs, SQLite infrastructure, React/TypeScript scaffolding, real-data dashboard, unit tests, development/build documentation, architecture/roadmap, and example configuration/environment files.

Additional foundation scaffolding includes an interface REST endpoint, restrictive local management policy, embedded production assets, a hardened systemd unit, conffile-preserving Debian package sources, and AMD64/ARM64 CI/release workflows. All files are newly created in the initially empty workspace. See README.md for the directory structure and commands.

The dashboard provides all eight planned navigation views. Overview, interfaces, service monitoring, effective settings, and version views use actual backend data. Streams, clients, traffic statistics, remote administration, browser log viewing, and configuration mutation are explicitly unavailable. Logs provides the real journald command. Theme, refresh, filter, and responsive menu controls are implemented.

## Validation results

The local host is Windows, with Node 24.21.0/npm 11.19.0 and a checksum-verified Go 1.27.4 toolchain downloaded into ignored workspace tooling. Dependencies are recorded in go.sum and frontend/package-lock.json.

| Check | Result |
| --- | --- |
| `gofmt -w cmd internal`, then format inspection | PASS |
| `go vet ./...` after the final frontend build | PASS |
| `go test ./...` | INCOMPLETE: configuration, interfaces, HTTP, and CLI pass; Windows Application Control blocks launching `storage.test.exe` |
| Final affected-package run: `go test ./internal/config ./internal/network ./internal/httpapi ./internal/cli` | PASS, all four packages |
| `go test -race ./...` | BLOCKED: CGO is disabled by default; explicitly enabling it is blocked while launching `cgo.exe` by Application Control. No C compiler is available on PATH either |
| `npm run typecheck` | PASS |
| `npm test` | PASS: 3 dashboard tests |
| `npm run build` | PASS: Vite 7.3.8 production output with embedded HTML, CSS, and JavaScript |
| `npm audit --audit-level=high` | PASS: 0 reported vulnerabilities after updating the test tooling |
| `CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build ...` | PASS |
| `CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ...` | PASS |
| Native daemon startup / HTTP smoke test | BLOCKED: Application Control blocks the built Windows executable |
| Linux systemd / Debian package lifecycle | NOT RUN locally: no Linux runtime, systemd, or dpkg-deb on this Windows host |
| Browser visual QA | NOT RUN: no supported browser preview/control tool is available; DOM behavior is covered by the frontend tests |

An initial storage test run exposed Windows URI path handling and a Windows-invalid test filename. The path construction and fixture were corrected. The final SQLite integration assertions remain unverified because subsequent launches are blocked by host Application Control; this is an outstanding validation requirement for Linux CI. The host policy was not disabled. A normal-host-access test attempt was permitted by automatic approval review but was still blocked by Windows Application Control.

Go checks must run after frontend builds finish because Vite replaces embedded files. An overlapping exploratory run encountered changing embed files; the final sequential vet and affected-package run passed. Native Linux race tests, package installation/upgrade/removal smoke tests, and conffile preservation checks are defined in CI but have not been executed on GitHub here.

Downloadable output is in `build/`: Linux AMD64 and ARM64 `.tar.gz` bundles, a Phase 1 source `.zip`, and SHA256SUMS. Bundles include the binary, configuration examples, systemd unit, and documentation. Local Debian packages are not claimed; their builder and release workflow are supplied.

## Known limitations

- No UDP receiver, MPEG-TS forwarding, HTTP playback, shared stream registry, client tracking, or traffic metrics. Reserved playback routes return HTTP 501 without contacting a source.
- Remote administration is rejected at configuration validation. No password, session, or configuration write endpoints exist; all local users able to connect to loopback can read management data.
- Streaming settings and future allowlists are validated but inactive. Only the management listener opens.
- Startup YAML/environment are authoritative. SQLite persists a startup snapshot and supplies a transactional settings repository; browser edits and hot reload are unavailable.
- Logs go to stderr/journald. Browser logs, file logging, and rotation are unavailable.
- Only IPv4 network selection is supported. Discovery includes interfaces without IPv4 addresses, but they cannot be selected for operation.
- Legacy Windows URL syntax and byte compatibility have not been verified or claimed.
- Linux runtime, systemd lifecycle, Debian upgrade/remove behavior, ARM64 execution, and browser visual QA cannot be proven by Windows cross-compilation. CI definitions supply Linux checks, but have not been run on GitHub in this workspace.
- The module namespace and Debian maintainer address are placeholders for the owner to select before public release. No GitHub repository, external deployment, tag, or release was created.
- Database parents must be trusted. Data and the system account remain after package purge to avoid deleting user state.

## Handoff

Development: build the frontend, then run `go run ./cmd/udphttp-proxy serve --config configs/config.example.yaml` and open http://127.0.0.1:8080. Linux builds and packaging commands are in README.md. Source, configuration examples, package/service scaffolding, and all documentation remain in this workspace.

Phase 2 requires explicit user approval.
