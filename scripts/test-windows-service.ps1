param([Parameter(Mandatory)][string]$Binary)
$ErrorActionPreference = 'Stop'
if ($env:GITHUB_ACTIONS -ne 'true') { throw 'This installs a native service and is restricted to a disposable GitHub Actions runner.' }
if (Get-Service UDPHTTPProxy -ErrorAction SilentlyContinue) { throw 'An existing service must never be replaced by this integration test.' }
$binaryPath = (Resolve-Path -LiteralPath $Binary).ProviderPath
$testRoot = Join-Path $env:ProgramData ('UDPHTTPProxy-ci-' + [guid]::NewGuid().ToString('N'))
$dataRoot = Join-Path $testRoot 'data'
$logRoot = Join-Path $testRoot 'logs'
New-Item -ItemType Directory -Force -Path $testRoot,$dataRoot,$logRoot | Out-Null
& icacls.exe $testRoot /inheritance:r /grant:r '*S-1-5-18:(OI)(CI)F' '*S-1-5-32-544:(OI)(CI)F' '*S-1-5-19:(OI)(CI)RX' | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'Cannot apply test directory ACL' }
foreach ($directory in @($dataRoot,$logRoot)) { & icacls.exe $directory /grant:r '*S-1-5-19:(OI)(CI)M' | Out-Null; if ($LASTEXITCODE -ne 0) { throw 'Cannot allow LocalService test data access' } }
# GitHub's checkout can be under a profile or a restricted runner directory.
Copy-Item -LiteralPath $binaryPath -Destination (Join-Path $testRoot 'udphttp-proxy.exe')
$binaryPath = Join-Path $testRoot 'udphttp-proxy.exe'
$ports = [Collections.Generic.HashSet[int]]::new()
while ($ports.Count -lt 3) { $listener = [Net.Sockets.TcpListener]::new([Net.IPAddress]::Loopback,0); $listener.Start(); [void]$ports.Add($listener.LocalEndpoint.Port); $listener.Stop() }
$portList = @($ports)
$config = Join-Path $testRoot 'config.yaml'
@{management=@{host='127.0.0.1';port=$portList[0];shutdown_timeout='5s'};network=@{http_interface='127.0.0.1';http_port=$portList[1];secondary_http_port=$portList[2]};storage=@{path=(Join-Path $dataRoot 'state.db')}} | ConvertTo-Json -Depth 6 | Set-Content -LiteralPath $config -Encoding utf8
$installed = $false
$report = @{status='failed';native_scm=$true;test_root=$testRoot}
try {
    & $binaryPath service install --config $config --log-dir $logRoot
    if ($LASTEXITCODE -ne 0) { throw 'SCM installation failed' }
    $installed = $true
    & $binaryPath service start
    if ($LASTEXITCODE -ne 0) { throw 'SCM start failed' }
    $ready = $false
    for ($attempt=0; $attempt -lt 40; $attempt++) { try { $health=Invoke-RestMethod -Uri "http://127.0.0.1:$($portList[0])/health" -TimeoutSec 1; if ($health.storage_ready) { $ready=$true; break } } catch { }; Start-Sleep -Milliseconds 250 }
    if (-not $ready) { throw 'Native service did not become healthy' }
    & $binaryPath service status
    if ($LASTEXITCODE -ne 0) { throw 'SCM status failed' }
    & $binaryPath service restart
    if ($LASTEXITCODE -ne 0) { throw 'SCM restart failed' }
    $health = Invoke-RestMethod -Uri "http://127.0.0.1:$($portList[0])/health" -TimeoutSec 5
    if (-not $health.storage_ready) { throw 'Service failed after restart' }
    & $binaryPath service stop
    if ($LASTEXITCODE -ne 0 -or (Get-Service UDPHTTPProxy).Status -ne 'Stopped') { throw 'Graceful SCM stop failed' }
    if (-not (Test-Path -LiteralPath (Join-Path $dataRoot 'state.db'))) { throw 'Service data did not persist' }
    if (-not (Test-Path -LiteralPath (Join-Path $logRoot 'service.jsonl'))) { throw 'Service log missing' }
    $report.status='passed'
} finally {
    if ($installed) { & $binaryPath service stop; & $binaryPath service uninstall }
    New-Item -ItemType Directory -Force -Path build | Out-Null
    $report | ConvertTo-Json | Set-Content -Encoding utf8 -LiteralPath build/windows-service-results.json
}
