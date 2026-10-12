. (Join-Path $PSScriptRoot 'common.ps1')
foreach ($service in $script:Services) {
    $state = Read-ServiceState $service
    $owned = Get-OwnedProcess $service $state
    $listening = Test-LocalPort '127.0.0.1' $service.Port
    $status = if ($owned -and $listening) { 'running' } elseif ($owned) { 'not ready' } elseif ($listening) { 'untracked listener' } else { 'stopped' }
    [pscustomobject]@{ Service=$service.Name; Status=$status; PID=$(if ($owned) { $owned.Id } else { $null }); URL=$service.Url }
}
