# UDPHTTP Proxy 0.4.3-rc.1 validation

## Baseline

The prior release parsed IPv4 client CIDRs in the playback handler, compared
source addresses as strings, and parsed multicast blocks in channel validation.
YAML and persisted settings shared configuration validation but required
arrays with narrower syntax. Dashboard playback policy changes waited for a
restart. The supply-chain workflow used the module root as the executable
entry point, so CycloneDX returned `no Go files` after the vulnerability scan.

Before the changes, all 31 frontend tests passed. Windows Application Control
blocked several native Go test executables during the baseline run.

## Implementation

- `internal/netpolicy` parses literal IPs and CIDRs with `net/netip`, masks
  CIDRs, preserves readable hosts, and deduplicates equivalent entries. It
  compiles immutable prefixes for authorization without expanding ranges.
- `internal/config`, `internal/operations`, and `internal/storage` share
  normalization for YAML, persisted revisions, and validated API writes.
  Settings writes return their revision from the same SQLite transaction.
- `internal/stream` and `internal/httpapi` use compiled policy snapshots for
  channel validation, M3U preview and commit, playback across both listeners
  and URL formats, startup, recovery, and live revalidation.
- `frontend/src/components/network` and the Network and Settings pages add
  multiline list editors, chips, draft reset, validation, allow-all approval,
  and backend decision previews.
- `.github/workflows` and the release, browser, and Debian scripts update
  candidate versioning, the SBOM entry point, and acceptance checks.

## Policy behavior

The source policy applies protocol and hard address rules first. The multicast
local-control range is always blocked. Custom multicast blocks take priority
over all allows. Broad unicast CIDRs still require a permitted sender and
explicit loopback permission. Playback checks the actual TCP peer, including
normalized IPv4-mapped addresses. Empty policies deny traffic. Client and
source allowlists apply live and exclude only affected clients or streams.
Saved channel definitions survive policy changes.

Playback listeners remain IPv4. Client lists accept IPv6 entries for
compatibility with the policy representation. Management stays IPv4 loopback
with exact trusted proxy addresses and origins. YAML remains the edit surface
for trusted proxies, origins, and remote playlist configuration.

## API and SQLite compatibility

Existing arrays still work. The settings API also accepts scalar strings and
mixed arrays for the supported IP lists. Responses return normalized arrays.
`POST /api/v1/settings/policy-test` accepts an IP, a policy kind, and optionally
a settings draft. It does not open a socket. Mutations retain revision, CSRF,
and JSON checks. A newly broad multicast allow requires separate approval.
There is no new schema migration. SQLite stays on schema 4 and channel IDs,
metadata, and playback URLs retain their existing formats.

## Verification

| Check | Result |
| --- | --- |
| Module integrity and Go vet | Passed. Go vet ran for Windows and Linux AMD64. |
| Go parser and configuration tests | Passed on Windows and the Go WebAssembly target. |
| Native Go suite | CLI, configuration, HTTP API, network policy, networking, operations, playlist, and stream packages passed across recorded runs. Application Control prevented a complete run. |
| Race detector and coverage | Windows race requires GCC, which is unavailable. The coverage run was partial because of Application Control. |
| Frontend clean install, typecheck, tests, and build | Passed. 35 tests. |
| Edge dashboard | Passed with fixture data at 11 widths. Exercised the list editor on desktop and mobile. |
| Go and npm dependency audits | No vulnerabilities reported by the executed scanners. |
| CycloneDX Go SBOM | The pinned tool's application entry point is corrected. Local Application Control blocked the scanner executable. Hosted CI execution is pending. |
| Go source and Git secret scans | Application Control blocked the native gosec and Gitleaks executables. Hosted CI execution is pending. |

The policy review checked mapped IPv4 boundaries, reserved ranges, null and
invalid list entries, confirmation for equivalent broad multicast ranges,
request size limits, and policy application on new and active subscriptions.
React renders policy values and errors as text.

## Performance

The native Windows matcher benchmark used worst-case misses and measured about
26 ns, 275 ns, and 2.83 µs for 8, 128, and 1,024 prefixes respectively, with
zero allocations per operation. The Go WebAssembly run also recorded zero
allocations. These results measure lookup. The packet forwarding loop does not
parse CIDRs. Windows Application Control blocked the candidate process in the
load test. Comparative throughput and memory validation remain release gates.

## Release and deployment

Local candidate outputs belong in `build/phase43/`. The existing packaging
path builds Linux AMD64 and ARM64 binaries, Debian packages and tar archives,
a Windows AMD64 binary and zip, a source zip, and `SHA256SUMS`. Application
Control blocked the PE resource tool, so the local candidate uses the existing
build option for Windows packages without version resources. CI retains the
full resource and supply-chain gates.

The [production network example](../configs/production-network.example.yaml)
documents the Ubuntu and Jellyfin settings. Follow the
[upgrade and rollback guide](UPGRADE.md) to preserve existing state. Install
only after native runtime and package acceptance checks pass. No deployment
or publication occurred.

## Outstanding validation

Native Linux AMD64 and ARM64 streaming, the prior-version Debian upgrade,
Windows SCM, the complete race and coverage runs, and the hosted release
workflow require their CI runners. Physical multicast and Jellyfin acceptance
require the deployment network.
