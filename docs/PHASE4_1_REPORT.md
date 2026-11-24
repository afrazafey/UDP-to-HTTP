# Phase 4.1 completion report

Local candidate **0.4.1-dev**. Build source is based on `65439eea3989b497406334766e8add60d4cf6c51` with working-tree changes. The artifact metadata records the full commit, dirty state, and UTC build time.

1. **UI redesign** The supplied reference guides the Overview layout. Five metric cards sit above two columns for measured throughput, channel status, service health, and retained activity. The collapsible rail shows live process metrics, and the header provides quick access to activity and theme controls.
2. **Pages** Overview, Channels, Channel Details, Playlists, Import / Export, Network, Connected Clients, Monitoring, Activity Logs, Settings, and System Information share the redesigned shell and controls. Existing hash URLs still resolve. The Playlists page links to the separate import workflow.
3. **Reusable components** Channel identity and contextual menu, icon metric cards, accessible drawer and form variants, selectable throughput history, and shared form, status, and table styles. Neutral channel icons avoid fetching imported logos in the browser. [Design system](UI_DESIGN_SYSTEM.md)
4. **Desktop screenshot** Edge captures at 1360px and 1440px using the Windows daemon, a temporary SQLite database, a synthetic UDP sender, and one real HTTP viewer. [Reference viewport](screenshots/overview-1360.png) · [Full desktop](screenshots/overview-1440.png)
5. **Tablet screenshot** Edge capture at 768px. [Tablet overview](screenshots/overview-768.png)
6. **Mobile screenshots** Edge captures at 390px. [Mobile overview](screenshots/overview-390.png) · [Channel form](screenshots/channel-form-390.png)
7. **Dark mode screenshot** Edge capture at 1440px after changing the theme through the daemon's revisioned settings API. [Dark overview](screenshots/overview-dark-1440.png)
8. **Coverage failure cause** The default Windows shell split the unquoted `-coverprofile=build/coverage.out` argument. Reproducing the workflow command locally produced the same `.out` package error and a truncated output path.
9. **Coverage fix** The matrix explicitly selects Bash, creates `build/coverage`, passes quoted atomic coverage flags, and writes `coverage.out` and `summary.txt`. Matrix artifact names include the runner OS. Failed checks propagate. [Exact Bash and PowerShell commands](CI_CD_VALIDATION.md)
10. **Debian installation failure cause** The previous artifact contained zero parent directory entries. Its conffile extraction failed when dpkg tried to open `/etc/udphttp-proxy/config.yaml.dpkg-new`. The broken pipe followed the unpack failure.
11. **Packaging fix** Linux stages the package tree and invokes native `dpkg-deb --root-owner-group`. The Windows fallback emits each parent directory before its files. Static verification rejects missing or misordered directory records and checks ownership, modes, conffile registration, hooks, architecture, and hashes. [Package layout and inspection commands](DEBIAN_PACKAGING.md)
12. **Fresh installation** Native dpkg and systemd acceptance is pending. The Windows host has no WSL distribution, Docker, or Linux runtime. The clean-runner CI gate checks package status, files, permissions, user, systemd, and health without precreating the configuration directory.
13. **Upgrade** The native Debian gate builds the Phase 3 baseline at `35204787ef5f0efb726c72216a60fcbbb62c2846`, seeds real settings and a test channel, verifies playback, and migrates using the candidate. It checks conffile and data preservation, rollback of an injected migration failure on a copy, reinstall, restart, remove, and purge. Local migration regression tests passed. Native package upgrade execution is pending.
14. **Windows** The packaged AMD64 binary printed its linked 0.4.1-dev version, source commit, and timestamp. A native run served the dashboard, accepted synthetic UDP, and delivered MPEG-TS over HTTP. The three-second load harness passed at one and ten viewers with one shared receiver per case, zero framing failures, clean reader disconnects, and ten start/stop cycles. Windows Go tests include socket, multicast, service handler, and persistence tests. Actual SCM installation remains a CI gate.
15. **Backend tests** The earlier Phase 4.1 run of `go test -json ./...` reported 91 top-level passes and 244 including subtests. For the reference update, `go vet ./...` passed and the daemon built and served the dashboard in the Edge smoke test. Windows Application Control blocked `httpapi.test.exe` during the Go suite recheck. The other packages with tests passed.
16. **Frontend tests** `npm ci` installed 156 packages using the checked-in lockfile. The reference update passed type checking, all 30 frontend tests, and the production build. Edge verified 12 routes at 10 viewport widths from 320px to 1920px with the live daemon and fixture API. The browser check exercised theme saving, drawer focus restoration, and the mobile channel dialog. The fixture run also checked the 30-second chart window, keyboard sample inspection, and omission of the 15-minute option when retention was insufficient.
17. **Coverage results** The corrected command wrote a profile and `go tool cover -func=build/coverage/coverage.out` produced a partial 69.6% report. Application Control blocked the instrumented CLI test at launch. Other observed package values include Operations 28.1%, HTTP API 69.4%, storage 74.1%, and stream 80.2%. The complete release coverage result remains pending.
18. **Race detector** `CGO_ENABLED=1 go test -race ./...` could not build because GCC is unavailable on this host.
19. **Linux smoke** Native smoke and load are pending on AMD64 and ARM64. The workflow runs the matching binary on each architecture. The prior verified Ubuntu TRACE URBAN deployment remains a separate real-network checkpoint.
20. **Artifacts** Generated the Linux AMD64 and ARM64 tarballs and Debian packages, Windows AMD64 ZIP, source ZIP, and `SHA256SUMS`. Static verification passed on all nine manifest entries, both package structures, architecture headers, and embedded build metadata.
21. **Outstanding gates** Native AMD64 and ARM64 Debian lifecycle and Linux smoke, Windows SCM acceptance, complete coverage and race instrumentation, and the target dual-NIC IPTV regression. GitHub workflow definitions are ready for a reviewed branch. Production readiness requires those results.
22. **Ubuntu upgrade** Verify the matching artifact and back up both the YAML and SQLite database. The exact commands validate and verify the snapshot before stopping the daemon, preserve the locally modified conffile during installation, and check service status and health. [Ubuntu upgrade commands](UPGRADE.md#phase-41-upgrade-on-ubuntu-amd64)

## Candidate filenames

The verified files are generated under `build/`.

- `udphttp-proxy_0.4.1-dev_linux_amd64.tar.gz`
- `udphttp-proxy_0.4.1-dev_linux_arm64.tar.gz`
- `udphttp-proxy_0.4.1-dev_windows_amd64.zip`
- `udphttp-proxy_0.4.1~dev_amd64.deb`
- `udphttp-proxy_0.4.1~dev_arm64.deb`
- `udphttp-proxy_0.4.1-dev_source.zip`
- `SHA256SUMS`
