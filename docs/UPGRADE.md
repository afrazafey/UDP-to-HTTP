# Safe upgrade, backup, restore and uninstall

Keep the last known-working binary, configuration and a verified matching database backup. Phase 4 schema 4 is forward-only; old Phase 3 binaries reject it. Downgrade requires the old binary **and its pre-upgrade backup**, not a database edit, reset, or schema-version decrement.

## Consistent backups

Run the new CLI against the same database/configuration and with the appropriate filesystem permissions. The backup command opens the source without migrating it, so it can capture a schema-3 database before an upgrade:

```sh
sudo -u udphttp-proxy /path/to/new/udphttp-proxy database backup --config /etc/udphttp-proxy/config.yaml --output /var/lib/udphttp-proxy/pre-phase4.db
sudo -u udphttp-proxy /path/to/new/udphttp-proxy database verify --input /var/lib/udphttp-proxy/pre-phase4.db
```

Both outputs report schema, channel count, bytes and SHA-256. Choose a **new** destination each time. The command uses SQLite's [consistent VACUUM INTO snapshot](https://www.sqlite.org/lang_vacuum.html), including committed WAL contents, then flushes/verifies it. The backup is not complete until verification succeeds. Protect/copy the verified snapshot to the backup system separately and retain its SHA-256. Neither a checksum alone nor copying a running main `.db` file establishes consistency. Verify accepts immutable snapshot files, not a live database with a separate WAL.

Windows equivalent (elevated account with access to private data):

```powershell
$exe = Join-Path $env:ProgramFiles 'UDPHTTPProxy\udphttp-proxy.exe'
$config = Join-Path $env:ProgramData 'UDPHTTPProxy\config.yaml'
& $exe database backup --config $config --output 'C:\PrivateBackups\udphttp-pre-phase4.db'
& $exe database verify --input 'C:\PrivateBackups\udphttp-pre-phase4.db'
```

The helper scripts in `scripts/backup.*` and `restore.*` wrap these same commands. They never automatically delete production state.

## Linux upgrade

1. Verify the new artifact checksums; read the Phase 4 report and test migrations on a restored copy first.
2. Take/verify the pre-upgrade snapshot above and retain the old config/binary.
3. `sudo systemctl stop udphttp-proxy`; confirm stopped and no active database writers.
4. `sudo apt install './udphttp-proxy_0.4.0~dev_amd64.deb'` (or arm64). Debian preserves modified YAML as a conffile. Review any conffile prompt; never accept replacing a production policy blindly.
5. Validate the retained config, start/restart the service, inspect journald, health and expected channel count/settings. Package configuration can start/restart the service; perform the upgrade in a planned maintenance window.
6. Re-test the known working multicast source and playback ports with allowed clients. Monitor recovery outcomes, errors and resources. Keep the old backup until acceptance finishes.

The migration adds missing metadata keys to channel JSON inside one transaction and advances user_version to 4. It does not delete channels/settings/events or reseed deleted channels. A failed migration rolls back and leaves the prior version/data intact; fix the reported problem and retry. Do not delete the database to make startup succeed.

## Windows upgrade

1. Verify/checksum and back up with a compatible new CLI before replacing the installed binary.
2. `& $exe service stop`; confirm `Get-Service UDPHTTPProxy` is Stopped.
3. Copy the reviewed new executable to the existing Program Files path; retain the old one separately. Preserve Program Files ACLs and the ProgramData config/data/log directories. Do not rerun installation over a live existing service.
4. Validate configuration, `& $exe service start`, inspect `service status`, JSON logs and health, and re-test reception/playback. The existing registration/account/startup mode remains in place.

## Restore/rollback without overwriting live state

Stop the service first. Keep the original database and any WAL/SHM together for diagnosis. Restore the verified snapshot into a **new private directory/path**, avoiding all stale sidecars:

```sh
sudo systemctl stop udphttp-proxy
sudo -u udphttp-proxy /usr/bin/udphttp-proxy database restore --input /var/lib/udphttp-proxy/pre-phase4.db --output /var/lib/udphttp-proxy/restored/state.db --confirm-offline
```

Windows: stop SCM, then `& $exe database restore --input 'C:\PrivateBackups\udphttp-pre-phase4.db' --output (Join-Path $env:ProgramData 'UDPHTTPProxy\restored\state.db') --confirm-offline`. Ensure LocalService has modify access to the new data directory using the same ACL pattern as installation.

Restore refuses existing destinations, verifies integrity before/after copying, flushes output and compares SHA-256. Point `storage.path` to that restored file in the retained configuration. For a rollback, install the matching old binary and start with its schema-compatible snapshot. For a normal recovery, the new binary can migrate an older restored snapshot forward. Validate before starting. Keep the original state until acceptance; no automatic destructive swap is performed.

## Saved settings recovery

Management binding/port, storage path, recovery, playlist and proxy trust stay YAML-only. The writable dashboard subset is persisted in SQLite and can override bootstrap YAML after the first save. Startup rejects a failed staged playback bind and restores accepted settings while keeping local management available. To recover without deleting state, stop the service, run interactively against the same database with `UDPHTTP_IGNORE_SAVED_SETTINGS=1`, correct/discard the bad proposal through the local UI, stop that recovery process and start the service normally. On PowerShell set `$env:UDPHTTP_IGNORE_SAVED_SETTINGS='1'` for that process and remove it afterward with `Remove-Item Env:UDPHTTP_IGNORE_SAVED_SETTINGS`.

## Uninstall

Linux: `sudo apt remove udphttp-proxy` stops the service while retaining config/data. `sudo apt purge udphttp-proxy` removes packaged configuration; the package deliberately retains the data/account. Copy verified backups before any intentional data removal. After confirming the service is stopped, inspect the named `/var/lib/udphttp-proxy` directory and remove it manually only if its retained data is no longer required.

Windows: `& $exe service stop`, then `& $exe service uninstall` removes only registration. Retain ProgramData for recovery. Remove the named Program Files application folder and its reviewed firewall rule (`Remove-NetFirewallRule -DisplayName 'UDPHTTP Playback'`) when no longer needed. To intentionally remove private state, back it up, confirm the service is absent/stopped, inspect `%ProgramData%\UDPHTTPProxy`, and delete that named folder manually. No wildcard/automatic data deletion is included.
