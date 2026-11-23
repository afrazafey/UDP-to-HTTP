# Production deployment and release gates

Phase 4 is a **validation candidate** until [PHASE4_REPORT.md](PHASE4_REPORT.md) gates pass. Keep the working Phase 3 binary/database backup available. The reported TRACE URBAN success predates these changes and is a regression reference, not a Phase 4 certification. Do not replace a live Ubuntu VM automatically.

## Service operation

Linux: use the packaged dedicated non-root user, hardened systemd unit and journald. The unit requires no capabilities/root for multicast, limits writable directories and gives a 30-second stop budget. Keep management shutdown timeout below that budget. Use `systemctl status`, `journalctl -u udphttp-proxy -o cat`, `systemctl restart/stop`.

Windows: use the native `UDPHTTPProxy` SCM service as LocalService with the install script's ACL layout. Configuration/binary should be writable only by administrators; only data/log directories are writable by the service. Control with `service start/stop/restart/status` or Windows Services. Service JSON files rotate at 20 MiB with three historical files. Native SCM integration must pass on the target OS before claiming support.

Use static IPs/DHCP reservations for both networks. Restrict input traffic to the intended source/VLAN and output clients to approved networks. Do not open management port 8080. Use SSH forwarding for remote administrators. Do not change firewall/route/IGMP policy through the application. Back up before upgrades and regularly thereafter; test restoring into a new isolated destination.

## Recovery and monitoring

The YAML-only recovery policy defaults to enabled, 250 ms minimum/30 s maximum delay, 20% jitter, rejoin after 30 s without accepted UDP, and unlimited retries with bounded delays. `max_attempts` limits consecutive reopen attempts (0 means unlimited); successful valid TS resets recovery and future backoff. A finite limit leaves the receiver offline for manual intervention. Setting enabled false leaves a healthy socket able to receive resumed traffic but disables automatic socket reopening after failure/silence.

`streaming.inactivity_timeout` separately controls waiting/degraded detection based on valid TS. Persistent garbage UDP can produce degraded health without a network-silence rejoin; inspect framing counters and upstream content. Interface disappearance/index/address changes trigger socket reopening. A missing source is not the same as a disconnected HTTP viewer. Last-viewer idle retirement and manual pinned starts retain existing semantics.

Monitor operational health, received UDP datagrams, TS packets/sync losses/CC errors, viewers, receiver restarts/recoveries, last valid data and recovery timestamps. Metrics histories are bounded RAM only. Recent errors/events are bounded; JSON logs redact recognized secrets and credential/query URLs. Dropped event counts indicate log persistence/queue pressure. Diagnostic exports are sanitized application events, not arbitrary host logs or packet payloads.

Tune requested UDP/socket/client queue budgets conservatively. Kernel buffers may be clamped; the proxy does not alter sysctls. Slow clients are isolated by bounded queues and write deadlines. No hardware-independent capacity or minimum RAM guarantee is established. Compare source rate × viewers with network/NIC capacity and measure CPU/RSS on the target. RSS includes runtime/SQLite allocations beyond the Go heap.

Run `python scripts/load-test.py --binary PATH --duration 6 --viewers 1,10,25,50,100` on a disposable host for reproducible synthetic loopback observations. Results include process CPU/RSS, heap, goroutines, one-receiver assertions, payload throughput, TS sync alignment and client cleanup. It is not a real IPTV decoder test or a long soak. Windows graceful SCM stop is tested separately; the standalone Windows load harness terminates its isolated process for cleanup and does not claim console-signal shutdown acceptance.

## Optional Nginx for playback

Direct Go HTTP playback avoids an extra buffering/identity boundary and is the default. A proxy can be useful for managed TLS/certificates or an existing access gateway. Before using it, validate streaming with buffering disabled, suitable long upstream read timeouts, no transcoding/caching/compression of MPEG-TS, and separate client authentication/authorization at the proxy. The Go listener sees Nginx's **actual peer IP**, not the viewer; spoofable X-Forwarded-For never grants playback privileges. Any explicit allowlisting of a local proxy therefore requires equivalent client restrictions at Nginx. Use TLS on public-facing endpoints.

No new Nginx playback server block is supplied as a validated configuration: Nginx is unavailable on this workstation. Test the intended configuration with `sudo nginx -t`, actual authenticated/unauthorized client requests, long playback and disconnect checks on the target before adopting it. The earlier management proxy example in `deploy/` remains compatibility documentation; use authenticated TLS and the explicit trusted-proxy design if retaining it, and never treat a successful origin check as authentication. This Phase 4 guide recommends SSH forwarding for management.

## Build/release process

`scripts/build-release.py` builds Linux AMD64/ARM64, Windows AMD64, gzip/ZIP bundles and standard Debian AMD64/ARM64 packages. The portable Debian builder emits standard ar/control/data members; native dpkg inspection/install/upgrade/removal is a separate Linux gate. `SOURCE_DATE_EPOCH` makes timestamps deterministic; version/commit/date are linked into binaries. Dirty checkouts are marked `+dirty`. Source builds without linked metadata explicitly say unlinked-source. No generated binaries are committed.

GitHub Actions CI invokes reusable Linux/Windows validation, including frontend tests/build, Go tests/race/coverage/vulnerability check, smoke, resource load, native Windows SCM and Debian lifecycle checks. Validation artifacts retain for 14 days. Release workflow is manual, builds artifacts with 30-day retention, and creates **only a draft prerelease** when the caller explicitly approves that action. Configure the `release-approval` GitHub Environment with required reviewers before using that path; the environment name alone is not an approval policy. Stable promotion is not automated. Linux ARM64 cross-builds are not native ARM64 runtime validation.

Before any stable release: all tests must pass on native supported targets, Windows SCM/network acceptance must pass, Debian packages must survive upgrade/remove tests, the working dual-NIC multicast deployment must pass regression checks, a longer recovery/slow-client soak is required, and licensing/distribution policy must be resolved. No workflow was dispatched or release published by this local implementation.
