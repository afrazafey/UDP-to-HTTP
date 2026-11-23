param([Parameter(Mandatory)][string]$InputFile, [Parameter(Mandatory)][string]$Output, [switch]$ConfirmOffline, [string]$Binary = 'udphttp-proxy.exe')
$ErrorActionPreference = 'Stop'
if (-not $ConfirmOffline) { throw 'Stop the service and use -ConfirmOffline. Restore requires a new destination.' }
& $Binary database restore --input $InputFile --output $Output --confirm-offline
exit $LASTEXITCODE
