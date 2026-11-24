# UDPHTTP Proxy Phase 4.2 completion report

**0.4.2-rc.1 · Developed by Afraaz.** The dashboard’s Production indicator reports deployment environment. The app’s About panel and System Information report release maturity and native acceptance independently. [afraaz.xyz](https://afraaz.xyz)

The release candidate preserves the existing management and playback listeners, settings schema, and stored channels. The deployment’s ens18 and ens19 addresses and TRACE URBAN stream remain deployment-specific.

## Dashboard

The sidebar and mobile drawer show service status, version, environment and attribution. Overview keeps compact metric cards, stable refresh control sizing, and locally bundled fonts and icons. Throughput and activity panels reuse rendering work when their inputs are unchanged. System Information separates About and release maturity from runtime status.

[Desktop](screenshots/phase42-overview-1360.png) · [Tablet](screenshots/phase42-overview-768.png) · [Mobile](screenshots/phase42-overview-390.png) · [Dark mode](screenshots/phase42-overview-dark-1440.png)

## Security

The audit inspected the browser client, management API, receive and fan-out pipeline, SQLite schema and backups, platform installers, logging, dependencies, and release workflows. Ten findings received code changes: one high, six medium, and three low. The review also recorded deployment trust assumptions. [Finding register](SECURITY_AUDIT.md) · [Threat model](THREAT_MODEL.md)

Changes address source archive scope, same-site management requests, sensitive headers and diagnostic context, Windows destination ACLs, backup path resolution, CI artifact identity, strict JSON, management browser policies, legacy logo metadata, and accepted TCP connection limits. Streaming retains authorized source selection, shared receivers, and bounded client queues. [Hardening](SECURITY_HARDENING.md) · [Security tests](SECURITY_TEST_RESULTS.md)

## Local validation

- Edge verified 12 routes at 11 widths, from 320 to 2560 px, with the fixture API and the live candidate serving synthetic UDP and HTTP. It exercised mobile drawer focus, channel actions, filter disclosure, chart inspection, theme changes, and settings tabs. The screenshots show the live candidate.
- All 31 frontend tests, type checking, and the production build passed. A clean `npm ci` passed in an isolated directory after the original checkout's native CSS binary was locked. Full and production npm audits each returned zero advisories.
- Go formatting, vet and module verification passed. Govulncheck reported no known vulnerabilities. The final Go run recorded 31 top-level passes, including the HTTP API suite, while Windows Application Control blocked seven package test executables. GCC is unavailable for the race detector.
- The local candidate credential-pattern scan reported zero matches across release files and nested members. Gitleaks and the current Gosec executable were blocked locally. The GitHub workflows pin both tools and configure CodeQL and Dependabot. A hosted run remains pending.
- The Windows PE resource generation step was blocked by Application Control on this host. The local Windows archive contains linked Go build identity and a build metadata JSON file. The release pipeline requires the PE resource and the Windows native service validation checks its author information.
- There is no Linux runtime on this host. Native Debian installation, 0.4.1 upgrade preservation, systemd sandbox scores before and after, and playback inside the hardened unit remain pending. The CI fixture covers both the older schema 3 migration and the direct schema 4 upgrade.

### Synthetic Windows comparison

The previous local Phase 4.1 daemon and the exact Phase 4.2 candidate ran isolated local UDP and HTTP at all five viewer counts. Each run retained one active receiver. Packet integrity errors and slow-client disconnects were zero under the harness's healthy consumption pattern. Idle CPU was 0% with 12 goroutines in both builds. The idle Go runtime reserved about 11.3 MiB in both builds.

Each pair reads Phase 4.1 → Phase 4.2. Measurements are synthetic and local.

| Viewers | Measured seconds | UDP Mbps | HTTP Mbps | Latest CPU | Max RSS MiB | Max goroutines | Median connection ms |
| ---: | --- | --- | --- | --- | --- | --- | --- |
| 1 | 8.18 → 8.22 | 0.655 → 0.676 | 0.66 → 0.68 | 1.56% → 0.52% | 20.3 → 58.7 | 15 → 15 | 28 → 29 |
| 10 | 8.15 → 8.14 | 0.661 → 0.670 | 6.62 → 6.70 | 6.25% → 15.10% | 22.2 → 59.8 | 33 → 33 | 14 → 14 |
| 25 | 8.19 → 8.23 | 0.677 → 0.675 | 16.93 → 16.89 | 27.61% → 34.37% | 22.8 → 60.8 | 63 → 63 | 11 → 11 |
| 50 | 8.16 → 8.21 | 0.673 → 0.675 | 33.61 → 33.68 | 11.48% → 8.86% | 24.4 → 62.8 | 113 → 113 | 10 → 9 |
| 100 | 8.14 → 8.19 | 0.679 → 0.675 | 67.86 → 67.36 | 13.52% → 101.57% | 27.3 → 66.1 | 213 → 214 | 12 → 10 |

The first comparison showed idle working-set memory rising from 16.3 to 55.4 MiB and an elevated final CPU sample at 100 viewers. The 12-second repeat at 1 and 100 viewers measured 16.3 and 16.4 MiB idle and about 67.3 Mbps at 100 viewers for both builds. The earlier working-set spike did not recur.

| 100-viewer sustained comparison | Phase 4.1 | Phase 4.2 |
| --- | ---: | ---: |
| Measured window | 30.22 s | 30.22 s |
| UDP input | 0.672 Mbps | 0.672 Mbps |
| HTTP output | 67.19 Mbps | 67.22 Mbps |
| Idle working set | 16.14 MiB | 16.38 MiB |
| Max working set | 27.38 MiB | 28.69 MiB |
| Max observed Go heap | 4.29 MiB | 4.45 MiB |
| Latest CPU sample, where 100% is one core | 114.05% | 92.19% |
| Max goroutines | 213 | 213 |
| Median connection latency | 26.6 ms | 18.1 ms |
| Active receivers | 1 | 1 |
| Packet integrity errors | 0 | 0 |

The sustained comparison showed similar throughput and working-set use for the isolated Windows workload. CPU readings varied across samples. Native Linux comparison and dedicated slow-client and recovery acceptance remain open. Raw JSON measurements are in `build/phase42-evidence/`.

## Release and upgrade

The local package set in `build/phase42/` includes Linux AMD64 and ARM64 tarballs, a Windows AMD64 ZIP, an AMD64 and ARM64 Debian package, a source ZIP, CycloneDX inventories, and a SHA-256 manifest. Static validation verified 15 manifest entries and checked formats, architecture, linked build metadata, and Debian directory, ownership, and conffile records. The credential-pattern scan checked 556 files and nested archive members with zero matches. The hosted workflow builds once and validates those bytes before the optional draft prerelease.

| Artifact | File |
| --- | --- |
| Linux AMD64 | `udphttp-proxy_0.4.2-rc.1_linux_amd64.tar.gz` |
| Linux ARM64 | `udphttp-proxy_0.4.2-rc.1_linux_arm64.tar.gz` |
| Windows AMD64 | `udphttp-proxy_0.4.2-rc.1_windows_amd64.zip` |
| Debian AMD64 | `udphttp-proxy_0.4.2~rc.1_amd64.deb` |
| Debian ARM64 | `udphttp-proxy_0.4.2~rc.1_arm64.deb` |
| Integrity and software inventories | `SHA256SUMS`, `go-sbom.json`, `frontend-sbom.json`, and scanner results |

Run the Ubuntu upgrade commands only after approving the release candidate for the intended environment. The procedure takes and verifies a SQLite snapshot and saves the existing YAML before stopping the service and installing the package. [Exact Ubuntu commands and rollback](UPGRADE.md#phase-42-upgrade-on-ubuntu-amd64) · [Release gates](RELEASE_CHECKLIST.md)

```bash
set -euo pipefail
[[ "$(dpkg --print-architecture)" == amd64 ]]
PACKAGE=udphttp-proxy_0.4.2~rc.1_amd64.deb
awk -v package="$PACKAGE" '$2 == package { print }' SHA256SUMS | sha256sum --check --strict -
dpkg-deb --info "$PACKAGE"
dpkg-deb --contents "$PACKAGE"
stage=$(mktemp -d)
chmod 0755 "$stage"
dpkg-deb -x "$PACKAGE" "$stage"
candidate="$stage/usr/bin/udphttp-proxy"
config=/etc/udphttp-proxy/config.yaml
backup="/var/lib/udphttp-proxy/pre-phase42-$(date -u +%Y%m%dT%H%M%SZ).db"
sudo install -d -m 0700 /var/backups/udphttp-proxy
sudo cp -p "$config" "/var/backups/udphttp-proxy/config.pre-phase42.$(date -u +%Y%m%dT%H%M%SZ).yaml"
sudo -u udphttp-proxy "$candidate" config validate --config "$config"
sudo -u udphttp-proxy "$candidate" database backup --config "$config" --output "$backup"
sudo -u udphttp-proxy "$candidate" database verify --input "$backup"
sudo systemctl stop udphttp-proxy
sudo env DEBIAN_FRONTEND=noninteractive apt-get \
  -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold \
  install "./$PACKAGE"
sudo systemctl enable --now udphttp-proxy
systemctl is-active udphttp-proxy
curl --fail --retry 8 --retry-connrefused http://127.0.0.1:8080/health
sudo journalctl -u udphttp-proxy -n 50 --no-pager
```

Compare the retained channel and settings counts with the pre-upgrade records. Check ports 7781 and 7782 and play the known TRACE URBAN URL from an authorized client. Keep the prior package, private YAML copy, and verified snapshot until acceptance is complete.
