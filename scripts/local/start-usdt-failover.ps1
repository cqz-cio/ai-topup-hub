param(
    [Parameter(Mandatory=$true)][string]$Binary,
    [string]$LocalRoot,
    [string]$Profile,
    [switch]$SimulatePrimaryFailure
)
$ErrorActionPreference = 'Stop'
if (-not $LocalRoot) { $LocalRoot = Join-Path $PSScriptRoot '../..' }
if (-not $Profile) { $Profile = Join-Path $PSScriptRoot '../../ops/usdt-rpc-fallbacks.example.json' }
$LocalRoot = [IO.Path]::GetFullPath($LocalRoot)
. (Join-Path $LocalRoot 'scripts/local/common.ps1')
Assert-LocalConfiguration
$Binary = [IO.Path]::GetFullPath($Binary)
if (-not $Binary.StartsWith($script:RuntimeRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase) -or -not (Test-Path -LiteralPath $Binary -PathType Leaf)) {
    throw 'Binary must be a built executable inside the local work/codex-runtime directory.'
}
$Profile = [IO.Path]::GetFullPath($Profile)
$values = Get-Content -LiteralPath $Profile -Raw | ConvertFrom-Json
$allowed = @('BSC_RPC_URL','BSC_LOGS_RPC_URL','BSC_RECEIPT_RPC_URL','BSC_RPC_FALLBACKS','PLASMA_RPC_URL','PLASMA_RPC_FALLBACKS','TRON_RPC_URL','TRON_RPC_FALLBACKS')
foreach ($property in $values.PSObject.Properties) {
    if ($property.Name -notin $allowed -or $property.Value -isnot [string]) { throw 'Unexpected local RPC profile field.' }
}
foreach ($key in $allowed) {
    if (-not $values.PSObject.Properties[$key]) { throw ('Missing profile field: '+$key) }
}
$service = $script:Services | Where-Object { $_.Name -eq 'backend' }
$oldState = Read-ServiceState $service
$owned = Get-OwnedProcess $service $oldState
if ((Test-LocalPort '127.0.0.1' 8080) -and -not $owned) { throw 'An untracked process owns port 8080; leaving it untouched.' }
$snapshot = Join-Path $script:LogRoot ('before-usdt-failover-' + (Get-Date -Format 'yyyyMMdd-HHmmss-fff') + '.json')
if ($oldState) { $oldState | ConvertTo-Json | Set-Content -LiteralPath $snapshot -Encoding UTF8 }
$previous = @{}
$changed = $allowed + @('UPSTREAM_PURCHASE_ENABLED','PLASMA_LOGS_RPC_URL','PLASMA_RECEIPT_RPC_URL','TRON_API_KEY')
foreach ($key in $changed) { $previous[$key] = [Environment]::GetEnvironmentVariable($key,'Process') }
try {
    foreach ($property in $values.PSObject.Properties) { [Environment]::SetEnvironmentVariable($property.Name,$property.Value,'Process') }
    # Public-node profile must not inherit a credential meant for another host.
    foreach ($key in @('PLASMA_LOGS_RPC_URL','PLASMA_RECEIPT_RPC_URL','TRON_API_KEY')) { [Environment]::SetEnvironmentVariable($key,$null,'Process') }
    $env:UPSTREAM_PURCHASE_ENABLED = 'false'
    if ($SimulatePrimaryFailure) {
        foreach ($key in @('BSC_RPC_URL','PLASMA_RPC_URL','TRON_RPC_URL')) { [Environment]::SetEnvironmentVariable($key,'https://127.0.0.1:1','Process') }
    }
    if ($owned) { Stop-ProcessTree $owned }
    Start-LocalService $service $Binary
    $state = Read-ServiceState $service
    $state | Add-Member -NotePropertyName rpcFallbackProfile -NotePropertyValue $Profile -Force
    $state | Add-Member -NotePropertyName rpcPrimaryFailureSimulated -NotePropertyValue ([bool]$SimulatePrimaryFailure) -Force
    $state | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $script:LogRoot $service.State) -Encoding UTF8
} finally {
    foreach ($key in $changed) { [Environment]::SetEnvironmentVariable($key,$previous[$key],'Process') }
}
