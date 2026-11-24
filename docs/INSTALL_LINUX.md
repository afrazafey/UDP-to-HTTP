# Linux installation

The installer restricts `/etc/udphttp-proxy/config.yaml` to `root:udphttp-proxy` mode 0640 and keeps state in the service's 0700 directory. Native installation and multicast acceptance are separate release gates. [Service hardening](SECURITY_HARDENING.md)

The Phase 4.2 builder includes the configuration directory in the Debian payload. A clean install can unpack the conffile without manual directory creation. Follow the upgrade procedure for installations with existing YAML and SQLite data. [Package layout](DEBIAN_PACKAGING.md) · [Ubuntu upgrade](UPGRADE.md#phase-42-upgrade-on-ubuntu-amd64)

Targets: Ubuntu 22.04/24.04 and Debian 12+, AMD64 and ARM64. The user verified Phase 3 multicast playback on Ubuntu with ens18/ens19; Phase 4 native Linux/ARM acceptance must pass CI and deployment testing. This workspace does not certify the new build from cross-compilation.

## Obtain and verify a candidate

This work builds local artifacts; it does not publish a GitHub release. Once an approved tag has matching artifacts, download `SHA256SUMS`, `udphttp-proxy_0.4.2~rc.1_amd64.deb` and/or `udphttp-proxy_0.4.2-rc.1_linux_amd64.tar.gz` from that release. Use `arm64` for an ARM64 OS; check `uname -m` (`x86_64` → amd64, `aarch64` → arm64). Never assume an unverified tag exists.

```sh
sudo apt update
sudo apt install -y curl ca-certificates
# In the downloaded artifact directory:
sha256sum --check --ignore-missing SHA256SUMS
dpkg-deb --info './udphttp-proxy_0.4.2~rc.1_amd64.deb'
sudo apt install './udphttp-proxy_0.4.2~rc.1_amd64.deb'
sudo systemctl enable --now udphttp-proxy
sudo systemctl status udphttp-proxy --no-pager
curl --fail http://127.0.0.1:8080/health
```

The same commands work on the stated Ubuntu and Debian targets. The package installs `/usr/bin/udphttp-proxy`, `/etc/udphttp-proxy/config.yaml`, `/usr/lib/systemd/system/udphttp-proxy.service`, and documentation under `/usr/share/doc/udphttp-proxy`. It creates a non-login `udphttp-proxy` account and private `/var/lib/udphttp-proxy` (0700), enables service startup where systemd is running, and retains configuration/data on removal. Debian conffiles preserve modified YAML on upgrade.

Open the dashboard on the server at `http://127.0.0.1:8080`. From an administrative workstation, use SSH forwarding:

```sh
ssh -N -L 8080:127.0.0.1:8080 ADMIN@SERVER
```

Then browse `http://127.0.0.1:8080` locally. Avoid a workstation port conflict; the forwarded destination and browser Host must match the management port. Remote binding is rejected.

## First channel

1. Run `/usr/bin/udphttp-proxy interfaces` and find the UDP IPv4/multicast-capable interface and HTTP client-facing interface.
2. Edit the YAML for the deployment: `network.multicast_interface`, `network.http_interface`, source/client allowlists, and `playlist.advertised_host`. Empty allowlists deny all playback. See the explicit example in [NETWORK_CONFIGURATION.md](NETWORK_CONFIGURATION.md).
3. Validate with `sudo -u udphttp-proxy /usr/bin/udphttp-proxy config validate --config /etc/udphttp-proxy/config.yaml`, then restart the service. Syntax/policy validation does not prove interface presence, multicast delivery or decoding.
4. In Streams → Add stream, enter the channel ID/name, multicast IP, UDP port and explicit interface. Leave startup on demand unless reception should run continuously. Save; validate errors are shown without changing the old configuration.
5. Open the configured `/live/CHANNEL_ID.ts` URL in VLC on an allowed client. Inspect stream details for UDP datagrams, valid TS packets and delivered HTTP bytes as separate observations.

YAML seeds import only once. Existing channels remain SQLite-authoritative. Saved dashboard network/security settings may override their bootstrap subset after the first confirmed save; see [UPGRADE.md](UPGRADE.md) for recovery mode. New recovery, playlist and management trust settings are YAML-only.

## Build without a package

With Go 1.26+ (CI 1.27), Node 24/npm 11+, Python 3.10+, Git, Make and GCC for race tests:

```sh
git clone https://github.com/afrazafey/UDP-to-HTTP.git
cd UDP-to-HTTP
npm --prefix frontend ci
npm --prefix frontend run build
go fmt ./...
go vet ./...
go test ./...
go test -race ./...
python3 scripts/build-release.py --version 0.4.2-rc.1
dpkg-deb --contents './build/udphttp-proxy_0.4.2~rc.1_amd64.deb'
python3 scripts/linux-smoke.py --binary build/udphttp-proxy-linux-amd64
python3 scripts/load-test.py --binary build/udphttp-proxy-linux-amd64
```

For a tar bundle, extract to a controlled directory and run `./udphttp-proxy serve --config configs/config.example.yaml`. Persistent paths are relative to the current directory unless configured absolutely. Install a reviewed service unit before using a tar build as a service. Source builds and candidates remain development builds until the release gates pass.

Logs: `journalctl -u udphttp-proxy -f -o cat`. Stop/restart: `sudo systemctl stop udphttp-proxy` / `sudo systemctl restart udphttp-proxy`. See [UPGRADE.md](UPGRADE.md) before upgrades and [TROUBLESHOOTING.md](TROUBLESHOOTING.md) for failures.
