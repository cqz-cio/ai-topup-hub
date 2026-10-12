. (Join-Path $PSScriptRoot 'common.ps1')
New-Item -ItemType Directory -Path $script:LogRoot -Force | Out-Null
foreach ($service in @($script:Services[2], $script:Services[1], $script:Services[0])) {
    $state = Read-ServiceState $service
    $owned = Get-OwnedProcess $service $state
    if ($owned) { Stop-ProcessTree $owned; Write-Host ($service.Name + ' stopped (PID ' + $owned.Id + ')') }
    elseif (Test-LocalPort '127.0.0.1' $service.Port) { Write-Warning ('Untracked listener on port ' + $service.Port + ' was preserved.') }
    else { Write-Host ($service.Name + ' already stopped') }
}
Write-Host 'Local data and Redis were preserved.'
