param([Parameter(Mandatory)][string]$Binary)
$ErrorActionPreference = 'Stop'
$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
if (-not ([Security.Principal.WindowsPrincipal]$identity).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { throw 'Run this installation script in elevated PowerShell.' }
$installRoot = Join-Path $env:ProgramFiles 'UDPHTTPProxy'
$stateRoot = Join-Path $env:ProgramData 'UDPHTTPProxy'
$dataRoot = Join-Path $stateRoot 'data'
$logRoot = Join-Path $stateRoot 'logs'
$config = Join-Path $stateRoot 'config.yaml'
$destination = Join-Path $installRoot 'udphttp-proxy.exe'
if (Get-Service -Name UDPHTTPProxy -ErrorAction SilentlyContinue) { throw 'Service already exists. Follow docs/UPGRADE.md; this script does not stop or overwrite a live installation.' }
New-Item -ItemType Directory -Force -Path $installRoot,$stateRoot,$dataRoot,$logRoot | Out-Null
Copy-Item -LiteralPath $Binary -Destination $destination
if (-not (Test-Path -LiteralPath $config)) {
    $template = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot '../deploy/config.windows.yaml')
    $template.Replace('@DATA_DIR@', $dataRoot.Replace("'", "''")) | Set-Content -Encoding utf8 -LiteralPath $config
}
& icacls.exe $stateRoot /inheritance:r /grant:r '*S-1-5-18:(OI)(CI)F' '*S-1-5-32-544:(OI)(CI)F' '*S-1-5-19:(OI)(CI)RX' | Out-Null
if ($LASTEXITCODE -ne 0) { throw 'Cannot restrict configuration directory permissions' }
foreach ($directory in @($dataRoot,$logRoot)) {
    & icacls.exe $directory /grant:r '*S-1-5-19:(OI)(CI)M' | Out-Null
    if ($LASTEXITCODE -ne 0) { throw "Cannot grant LocalService write access to $directory" }
}
& $destination config validate --config $config
if ($LASTEXITCODE -ne 0) { throw 'Configuration validation failed; no service installed' }
& $destination service install --config $config --log-dir $logRoot
if ($LASTEXITCODE -ne 0) { throw 'Native service installation failed' }
Write-Output "Installed with automatic startup. Review $config, then run: & '$destination' service start"
