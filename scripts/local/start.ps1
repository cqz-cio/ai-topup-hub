param([switch]$Build)
. (Join-Path $PSScriptRoot 'common.ps1')
New-Item -ItemType Directory -Path $script:LogRoot, $script:RuntimeRoot -Force | Out-Null
Assert-LocalConfiguration
$running = @{}
foreach ($service in $script:Services) {
    $state = Read-ServiceState $service
    $owned = Get-OwnedProcess $service $state
    $running[$service.Name] = $owned
    if ($owned) {
        if ($service.Name -eq 'backend' -and ($state.upstreamPurchaseEnabled -ne $false -or $state.mode -ne 'storefront')) { throw 'Stop the existing backend before switching to the local storefront profile.' }
        if ($Build -and $service.Name -eq 'backend') { throw 'Stop the local backend before using -Build.' }
        if (-not (Test-LocalPort '127.0.0.1' $service.Port)) { throw ($service.Name + ' process exists but its port is not ready; inspect its logs.') }
    } elseif (Test-LocalPort '127.0.0.1' $service.Port) {
        throw ('Port ' + $service.Port + ' belongs to an untracked process. It was preserved.')
    }
    if ($service.Name -ne 'backend' -and -not (Test-Path -LiteralPath (Join-Path $service.Directory 'node_modules/vite/bin/vite.js'))) { throw ('Install dependencies in ' + $service.Directory + ' first.') }
}
$backendState = Read-ServiceState $script:Services[0]
$binary = Join-Path $script:RuntimeRoot 'local-server.exe'
if ($backendState -and $backendState.binary) {
    $candidate = [IO.Path]::GetFullPath($backendState.binary)
    if ($candidate.StartsWith($script:RuntimeRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) { $binary=$candidate }
}
if ($Build) { $binary=Invoke-LocalBuild }
if (-not $running.backend -and -not (Test-Path -LiteralPath $binary)) { throw 'No local backend binary. Run start.ps1 -Build first.' }
$previousPurchase = $env:UPSTREAM_PURCHASE_ENABLED
try {
    $env:UPSTREAM_PURCHASE_ENABLED = 'false'
    foreach ($service in $script:Services) {
        if ($running[$service.Name]) { Write-Host ($service.Name + ' already running: ' + $service.Url + ' (PID ' + $running[$service.Name].Id + ')') }
        else { Start-LocalService $service $binary }
    }
} finally { $env:UPSTREAM_PURCHASE_ENABLED=$previousPurchase }
