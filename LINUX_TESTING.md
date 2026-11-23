# Linux runtime validation

Status: **pending**. The development host is Windows without WSL/Docker/Linux. The user selected local checks plus prepared CI, with Linux validation reported as pending. Neither an ELF cross-build nor this document proves runtime behavior.

## Native Ubuntu/Debian checks

On a Linux AMD64 host with Go 1.26+, Node 24, npm, Python 3, Make, and GCC:

```sh
set -eu
cd frontend
npm ci
npm run typecheck
npm test
npm run build
cd ..
test -z "$(gofmt -l cmd internal)"
go vet ./...
go test -v ./...
go test -race ./...
make linux VERSION=0.3.0-dev-test
python3 scripts/linux-smoke.py --binary build/udphttp-proxy-linux-amd64
```

The smoke test starts the actual Linux binary using isolated temp config/state and local ephemeral ports. It checks SQLite readiness, a real loopback UDP bind/selected interface, synthetic MPEG-TS reception, four concurrent HTTP viewers across two playback listeners, payload counters, Start/Stop/Restart, persistent CRUD, process restart, and successful SIGTERM shutdown. A failure raises an assertion and exits nonzero. It writes `build/linux-smoke-results.json` only after success, including observed traffic and process CPU/max-RSS figures. These small smoke measurements are not a capacity benchmark.

## Multicast

`go test -v ./internal/stream -run TestMulticastJoinForwardLeave` selects an up multicast-capable IPv4 interface. If none exists, the test explicitly reports a skip with that reason. If one exists, bind/join/send/forward/leave/rejoin failures are test failures. The generator sets outgoing interface, TTL 1 and multicast loopback for local testing. This does not establish that an external IPTV source or switch IGMP configuration works.

Check actual interface availability with `build/udphttp-proxy-linux-amd64 interfaces`. Run the local generator for a chosen approved group if validating an actual LAN:

```sh
python3 scripts/udp-generator.py --host APPROVED_GROUP --bind ASSIGNED_INTERFACE_IPV4 --port 15000 --seconds 30
```

Add that exact group to `allowed_multicast_groups`, choose the actual local multicast interface in the channel, configure client CIDRs, and start playback on `/live/CHANNEL.ts` or its unique `/udp/GROUP:PORT` route. Do not use an unassigned example address. Reserved multicast CIDRs remain blocked unless explicitly changed. Capture the exact skip/failure and host interface capabilities in the acceptance record.

## systemd and packaging

On a disposable Ubuntu/Debian host or CI runner (administrative installation privileges required):

```sh
sh scripts/build-deb.sh amd64 0.3.0-dev~test build/udphttp-proxy-linux-amd64
sudo dpkg -i build/udphttp-proxy_0.3.0-dev~test_amd64.deb
sudo systemd-analyze verify deploy/udphttp-proxy.service
sudo systemctl restart udphttp-proxy
curl --fail http://127.0.0.1:8080/health
sudo systemctl stop udphttp-proxy
sudo systemctl status udphttp-proxy --no-pager
journalctl -u udphttp-proxy -o cat --no-pager
```

Stopped status exits nonzero intentionally; inspect the journal and assert the expected inactive state with `systemctl is-active` in a test harness. The CI workflow installs/restarts the real unit, checks readiness, appends a config preservation marker, reinstalls the package, verifies the marker, removes it while preserving conffiles, then purges packaged config. It never modifies firewall rules or deletes retained state/account.

Before a real upgrade, stop the old service and back up its entire state directory using SQLite-aware backup procedures. To validate Phase 1 data preservation, the automated storage test constructs the exact schema v1 with an existing settings value, migrates to v3, and checks settings/channel persistence and non-resurrecting seeds. Also repeat an installation upgrade with your real backed-up configuration. Older Phase 1 binaries reject schema v3; restore a matching backup for rollback.

## Cross-build and reporting

The ARM64 build validates compilation only. Native ARM64 execution must be recorded separately. Record OS/kernel/architecture, Go version, interface/multicast support, all command exit codes, any skipped test reasons, HTTP results, service logs, shutdown status, and resource measurements. A failed or unexecuted gate must remain visible. No production readiness claim is allowed while these runtime gates are pending.


## Phase 3 operations acceptance

The runtime smoke script now also checks phase >= 3 operations endpoints, real Linux process metadata, dashboard summary, persisted general settings, preview and confirmed saves, invalid listener rejection, settings persistence after restart and a deliberately occupied staged port. It must prove rollback to the accepted playback bind while management remains reachable, then verify playback again and query real operational events. These are test definitions, not executed results on this Windows host.

After unit/race tests and frontend build, run the smoke test with the current Linux AMD64 binary. Verify saved server/client policies under the dedicated service account and repeat occupied-port recovery on a disposable host. Open the actual dashboard in light/dark/system modes on desktop/mobile, exercise keyboard dialog focus, change interval/pause/manual refresh, and inspect actual chart/unknown/stale states. Record browser versions and screenshots only if actually captured. Synthetic packets do not verify provider video or codecs.

Before a v2→v3 upgrade, back up state and test migration while retaining existing channels/settings. Check /api/v1/settings active/desired/revision and /api/v1/system listener state. Follow README recovery mode with the same database and service account; never delete/recreate production state to recover a settings edit. Management is intentionally loopback-only. A tunnel must forward playback too if needed; remote bind remains unsupported.

Log exports must remain authorized and sanitized; test token/credential redaction and retention with private fixtures, not real secrets. Validate that unknown CPU/RSS stays null off Linux. Linux CPU percent is per-core, RSS is process resident memory and heap is Go allocation; do not compare them as equivalent metrics. Record resource/stream observations from the real host, never from mocked frontend fixtures.
