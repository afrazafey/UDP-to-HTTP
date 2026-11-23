param([Parameter(Mandatory)][string]$Config, [Parameter(Mandatory)][string]$Output, [string]$Binary = 'udphttp-proxy.exe')
$ErrorActionPreference = 'Stop'
& $Binary database backup --config $Config --output $Output
exit $LASTEXITCODE
