param([Parameter(Mandatory)][string]$Binary)
$ErrorActionPreference = 'Stop'
$identity = [Security.Principal.WindowsIdentity]::GetCurrent()
if (-not ([Security.Principal.WindowsPrincipal]$identity).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) { throw 'Run this installation script in elevated PowerShell.' }
$installRoot = Join-Path ([Environment]::GetFolderPath('ProgramFiles')) 'UDPHTTPProxy'
$stateRoot = Join-Path ([Environment]::GetFolderPath('CommonApplicationData')) 'UDPHTTPProxy'
$dataRoot = Join-Path $stateRoot 'data'
$logRoot = Join-Path $stateRoot 'logs'
$config = Join-Path $stateRoot 'config.yaml'
$destination = Join-Path $installRoot 'udphttp-proxy.exe'
if (Get-Service -Name UDPHTTPProxy -ErrorAction SilentlyContinue) { throw 'Service already exists. Follow docs/UPGRADE.md; this script does not stop or overwrite a live installation.' }
foreach ($root in @($installRoot, $stateRoot)) {
    if (Get-Item -LiteralPath $root -Force -ErrorAction SilentlyContinue) {
        throw "Installation path already exists: $root. Follow the upgrade or recovery instructions for existing data."
    }
}
$source = Get-Item -LiteralPath $Binary -Force
if ($source.PSIsContainer -or ($source.Attributes -band [IO.FileAttributes]::ReparsePoint)) {
    throw 'Select a regular executable file from a trusted directory.'
}
function New-ProtectedDirectory([string]$Path, [Security.AccessControl.FileSystemRights]$ServiceRights) {
    $security = [Security.AccessControl.DirectorySecurity]::new()
    $security.SetAccessRuleProtection($true, $false)
    $inherit = [Security.AccessControl.InheritanceFlags]::ContainerInherit -bor [Security.AccessControl.InheritanceFlags]::ObjectInherit
    $permissions = @{
        'S-1-5-18' = [Security.AccessControl.FileSystemRights]::FullControl
        'S-1-5-32-544' = [Security.AccessControl.FileSystemRights]::FullControl
        'S-1-5-19' = $ServiceRights
    }
    $expected = @{}
    foreach ($account in $permissions.Keys) {
        $sid = [Security.Principal.SecurityIdentifier]::new($account)
        $rule = [Security.AccessControl.FileSystemAccessRule]::new($sid, $permissions[$account], $inherit, 'None', 'Allow')
        $security.AddAccessRule($rule)
        $expected[$account] = $rule.FileSystemRights
    }
    $directory = [IO.DirectoryInfo]::new($Path)
    if ($PSVersionTable.PSEdition -eq 'Core') {
        [IO.FileSystemAclExtensions]::Create($directory, $security)
    } else {
        [IO.Directory]::CreateDirectory($Path, $security) | Out-Null
    }
    $directory.Refresh()
    $actual = Get-Acl -LiteralPath $Path
    $rules = @($actual.GetAccessRules($true, $true, [Security.Principal.SecurityIdentifier]))
    if (($directory.Attributes -band [IO.FileAttributes]::ReparsePoint) -or -not $actual.AreAccessRulesProtected -or
        $rules.Count -ne $expected.Count) {
        throw "Could not establish the required directory permissions: $Path"
    }
    foreach ($rule in $rules) {
        $account = $rule.IdentityReference.Value
        if (-not $expected.ContainsKey($account) -or $rule.FileSystemRights -ne $expected[$account] -or
            $rule.IsInherited -or $rule.InheritanceFlags -ne $inherit -or $rule.PropagationFlags -ne 'None' -or
            $rule.AccessControlType -ne 'Allow') {
            throw "Unexpected directory permissions: $Path"
        }
        $expected.Remove($account)
    }
}
New-ProtectedDirectory $installRoot ReadAndExecute
New-ProtectedDirectory $stateRoot ReadAndExecute
New-ProtectedDirectory $dataRoot Modify
New-ProtectedDirectory $logRoot Modify
Copy-Item -LiteralPath $Binary -Destination $destination
$template = Get-Content -Raw -LiteralPath (Join-Path $PSScriptRoot '../deploy/config.windows.yaml')
$template.Replace('@DATA_DIR@', $dataRoot.Replace("'", "''")) | Set-Content -Encoding utf8 -LiteralPath $config
& $destination config validate --config $config
if ($LASTEXITCODE -ne 0) { throw 'Configuration validation failed; no service installed' }
& $destination service install --config $config --log-dir $logRoot
if ($LASTEXITCODE -ne 0) { throw 'Native service installation failed' }
Write-Output "Installed with automatic startup. Review $config, then run: & '$destination' service start"
