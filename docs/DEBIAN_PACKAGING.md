# Debian package construction and acceptance

The Phase 4 portable builder emitted the configuration file and its peers with zero explicit payload directory entries. During a fresh install, dpkg needed `/etc/udphttp-proxy` before it could open `config.yaml.dpkg-new`. Its extraction error caused the downstream broken pipe.

On Linux, the builder now stages real parent directories and runs `dpkg-deb --root-owner-group -Zgzip --build`. The portable fallback emits root-owned directory entries before files, using POSIX archive paths. Both paths retain a registered conffile, 0755 directory and executable modes, a 0644 root-owned configuration file, and the existing package hooks and systemd unit. The package creates the private 0700 service data directory at configuration time.

```bash
PACKAGE=build/udphttp-proxy_0.4.1~dev_amd64.deb
dpkg-deb --info "$PACKAGE"
dpkg-deb --contents "$PACKAGE"
dpkg-deb --fsys-tarfile "$PACKAGE" | tar -tvf -
python3 scripts/verify-artifacts.py
```

On a disposable systemd-capable GitHub runner, `scripts/test-debian-package.sh` asserts clean starting paths, installs the candidate, and checks permissions, service registration and health. It then builds and installs the pinned Phase 3 source, creates a test stream, persists custom settings, verifies playback, exercises an injected migration failure on a separate database, and upgrades to Phase 4.1. It verifies the user-edited conffile byte for byte, schema 4, channel and settings preservation, and MPEG-TS payload before and after upgrade. Reinstallation, restart, removal, and purge complete the suite. This job runs separately on native AMD64 and ARM64.

Ordinary remove retains YAML and the database. Purge removes the registered YAML and preserves the data directory and service account. Use the backed-up, schema-compatible rollback procedure if a deployment fails. [Upgrade guide](UPGRADE.md)
