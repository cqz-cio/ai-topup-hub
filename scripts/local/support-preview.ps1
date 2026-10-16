param([ValidateSet('start','status','stop')][string]$Action = 'start')
. (Join-Path $PSScriptRoot 'common.ps1')
$service = @{ Name='support-pet'; State='support-pet-preview.json'; Port=5175; Url='http://127.0.0.1:5175/'; Directory=(Join-Path $script:ProjectRoot 'frontend/user') }
New-Item -ItemType Directory -Path $script:LogRoot -Force | Out-Null
$state = Read-ServiceState $service
$owned = Get-OwnedProcess $service $state
if ($Action -eq 'status') {
    [ordered]@{ status=$(if ($owned) {'running'} else {'stopped'}); pid=$(if ($owned) {$owned.Id} else {$null}); url=$service.Url; source=$service.Directory; stdout=$state.stdout; stderr=$state.stderr } | ConvertTo-Json
    exit
}
if ($Action -eq 'stop') {
    if ($owned) { Stop-ProcessTree $owned; Write-Host 'Customer support preview stopped.' }
    else { Write-Host 'No owned customer support preview is running.' }
    exit
}
if ($owned) { Write-Host ('Customer support preview already running: ' + $service.Url); exit }
if (Test-LocalPort '127.0.0.1' $service.Port) { throw 'Port 5175 is occupied by another process; it was preserved.' }
if (-not (Test-LocalPort '127.0.0.1' 8080)) { throw 'Start the daily local API with D:\卡网\scripts\local\start.ps1 first. This preview reuses the local API without copying its config or database.' }
if (-not (Test-Path -LiteralPath (Join-Path $service.Directory 'node_modules/vite/bin/vite.js'))) { throw 'Install frontend/user dependencies with pnpm install --frozen-lockfile first.' }
Start-LocalService $service ''
