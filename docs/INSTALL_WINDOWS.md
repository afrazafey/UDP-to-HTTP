# Windows installation and native service

The Phase 4.1 browser layout check runs independently of Windows SCM acceptance. Match the `0.4.1-dev` archive and SHA-256 manifest from the same build. [Validation report](PHASE4_1_REPORT.md)

Targets: Windows 10/11 x64 and Windows Server 2019+. Windows ARM64 is not released. Native service execution/control and packet-info reception are implemented, but **service installation/runtime support is an acceptance gate**, not a claim from compilation. See [PHASE4_REPORT.md](PHASE4_REPORT.md) for the actual host test results. Do not disable Application Control or Defender to run untrusted builds.

## Interactive start

Obtain `udphttp-proxy_0.4.1-dev_windows_amd64.zip` and `SHA256SUMS` from an approved artifact set. No release is published by this task. Compare the ZIP's `Get-FileHash -Algorithm SHA256` value with its exact manifest entry before extracting:

```powershell
Get-FileHash -Algorithm SHA256 -LiteralPath .\udphttp-proxy_0.4.1-dev_windows_amd64.zip
Expand-Archive -LiteralPath .\udphttp-proxy_0.4.1-dev_windows_amd64.zip -DestinationPath .\udphttp-proxy
Set-Location .\udphttp-proxy
.\udphttp-proxy.exe version
.\udphttp-proxy.exe interfaces
.\udphttp-proxy.exe config validate --config .\configs\config.example.yaml
.\udphttp-proxy.exe serve --config .\configs\config.example.yaml
```

Open `http://127.0.0.1:8080`. Stop an interactive instance with Ctrl+C. Use explicit assigned IPv4 addresses for interface selection when possible; friendly names, including spaces, are also resolved against actual adapters. Source/client allowlists still start empty. Never use the Linux `/var/lib` storage path on Windows.

## Native SCM service

In elevated PowerShell, after reviewing the extracted scripts:

```powershell
.\scripts\install-windows.ps1 -Binary (Resolve-Path .\udphttp-proxy.exe).Path
$exe = Join-Path $env:ProgramFiles 'UDPHTTPProxy\udphttp-proxy.exe'
$config = Join-Path $env:ProgramData 'UDPHTTPProxy\config.yaml'
& $exe config validate --config $config
& $exe service start
& $exe service status
Get-Service UDPHTTPProxy
& $exe service stop
& $exe service restart
```

The installer uses `%ProgramFiles%\UDPHTTPProxy\udphttp-proxy.exe`, `%ProgramData%\UDPHTTPProxy\config.yaml`, `data\udphttp-proxy.db`, and `logs\service.jsonl`. It retains an existing YAML, configures LocalService (not LocalSystem), grants that identity read access to configuration and modify access only to data/log directories, and registers automatic startup. It refuses to overwrite an existing service installation. The binary location should retain Program Files administrator-only write permissions; do not install into a directory writable by ordinary users.

Direct service registration, if paths/ACLs are already configured, is `& $exe service install --config $config --log-dir (Join-Path $env:ProgramData 'UDPHTTPProxy\logs')`. This uses Windows SCM APIs, without NSSM or another wrapper. `service run` is SCM-only. `service start/stop/restart/status/uninstall` operate on the fixed `UDPHTTPProxy` service. Status includes native state and exit codes; `Get-Service` supplies friendly names.

Default service logging is JSON with a current file plus three rotated copies of at most 20 MiB each. `log_format: text` is available for interactive troubleshooting; retain JSON for service installations. Tail with:

```powershell
Get-Content -Wait -Tail 50 -LiteralPath (Join-Path $env:ProgramData 'UDPHTTPProxy\logs\service.jsonl')
```

SCM receives stop/shutdown controls and cancels the same receiver/server contexts as the interactive process. Configure HTTP shutdown timeout below SCM's 35-second stop budget. Automatic failure recovery uses delayed restarts, then stops retrying after repeated service failures; individual receiver recovery does not restart the service.

## Firewall and playback

Review addresses/profile before running an elevated rule. Example for the documented test network (replace it on another network):

```powershell
$exe = Join-Path $env:ProgramFiles 'UDPHTTPProxy\udphttp-proxy.exe'
New-NetFirewallRule -DisplayName 'UDPHTTP Playback' -Direction Inbound -Action Allow -Protocol TCP -LocalAddress 192.168.10.103 -LocalPort 7781,7782 -RemoteAddress 192.168.10.0/24 -Program $exe -Profile Private
```

Do **not** open management port 8080. A firewall opening does not override playback CIDRs. If UDP reception is blocked, review a narrowly scoped inbound rule for the selected adapter, UDP destination port, multicast destination and source network. Network profiles and IGMP/switch policies also matter. Test with VLC using the configured client-facing URL, not the loopback management address. See [NETWORK_CONFIGURATION.md](NETWORK_CONFIGURATION.md).

## Build from source

Install Go 1.26+ and Node 24/npm 11+, Git and Python 3.10+; race tests require a supported GCC/CGO toolchain. In PowerShell:

```powershell
git clone https://github.com/afrazafey/UDP-to-HTTP.git
Set-Location UDP-to-HTTP
npm.cmd --prefix frontend ci
npm.cmd --prefix frontend run typecheck
npm.cmd --prefix frontend test
npm.cmd --prefix frontend run build
go fmt ./...
go vet ./...
go test ./...
go test -race ./...
python scripts/build-release.py --version 0.4.1-dev
& .\build\udphttp-proxy-windows-amd64.exe serve --config .\configs\config.example.yaml
```

`scripts/test-windows-service.ps1` installs/starts/stops/uninstalls the native service only in a disposable GitHub Actions Windows runner. It refuses ordinary host execution or an existing service. Handler tests and this SCM integration test are separate. The real multicast test requires an up multicast-capable adapter; a skip/failure is reported separately from ordinary UDP tests. Synthetic local multicast does not prove an external IPTV source works.

For upgrade/uninstall, follow [UPGRADE.md](UPGRADE.md); always preserve data and take a verified backup first.
