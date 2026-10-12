$ErrorActionPreference = 'Stop'
$script:ProjectRoot = [IO.Path]::GetFullPath((Join-Path $PSScriptRoot '../..'))
$script:LogRoot = Join-Path $script:ProjectRoot 'work/codex-logs'
$script:RuntimeRoot = Join-Path $script:ProjectRoot 'work/codex-runtime'
$script:Services = @(
    @{ Name='backend'; State='local-backend.json'; Port=8080; Url='http://127.0.0.1:8080/health'; Directory=$script:ProjectRoot },
    @{ Name='storefront'; State='storefront-preview.json'; Port=5173; Url='http://127.0.0.1:5173/'; Directory=(Join-Path $script:ProjectRoot 'frontend/user') },
    @{ Name='admin'; State='admin-preview.json'; Port=5174; Url='http://127.0.0.1:5174/login'; Directory=(Join-Path $script:ProjectRoot 'frontend/admin') }
)

function Read-ServiceState($Service) {
    $path = Join-Path $script:LogRoot $Service.State
    if (Test-Path -LiteralPath $path) { return Get-Content -LiteralPath $path -Raw | ConvertFrom-Json }
    return $null
}

function Get-OwnedProcess($Service, $State) {
    if (-not $State) { return $null }
    $process = Get-Process -Id ([int]$State.pid) -ErrorAction SilentlyContinue
    if (-not $process) { return $null }
    if ($State.processStartedAt -and $process.StartTime.ToUniversalTime().Ticks -ne [long]$State.processStartedAt) { return $null }
    $detail = Get-CimInstance Win32_Process -Filter ('ProcessId = ' + [int]$State.pid) -OperationTimeoutSec 5
    if ($Service.Name -eq 'backend') {
        if (-not $State.binary) { return $null }
        $binary = [IO.Path]::GetFullPath($State.binary)
        if (-not $binary.StartsWith($script:RuntimeRoot + [IO.Path]::DirectorySeparatorChar, [StringComparison]::OrdinalIgnoreCase)) { return $null }
        if ($detail.ExecutablePath -ne $binary) { return $null }
    } else {
        $marker = Join-Path $Service.Directory 'node_modules/vite/bin/vite.js'
        if (-not $detail.CommandLine -or -not $detail.CommandLine.Replace('\','/').Contains($marker.Replace('\','/'))) { return $null }
        if ([IO.Path]::GetFileName($detail.ExecutablePath) -ne 'node.exe') { return $null }
    }
    return $process
}

function Test-LocalPort([string]$HostName, [int]$Port) {
    $client = New-Object Net.Sockets.TcpClient
    try {
        $pending = $client.BeginConnect($HostName, $Port, $null, $null)
        if (-not $pending.AsyncWaitHandle.WaitOne(1000)) { return $false }
        $client.EndConnect($pending)
        return $true
    } catch { return $false } finally { $client.Dispose() }
}

function Stop-ProcessTree($Process) {
    $Process.Refresh()
    if ($Process.HasExited) { return }
    # Capture descendants before taskkill, then verify the captured identities.
    $snapshot = @(Get-CimInstance Win32_Process -OperationTimeoutSec 5)
    $ids = @($Process.Id)
    do {
        $count = $ids.Count
        $ids += @($snapshot | Where-Object { $_.ParentProcessId -in $ids -and $_.ProcessId -notin $ids } | ForEach-Object { [int]$_.ProcessId })
    } while ($ids.Count -gt $count)
    $tracked = @($ids | ForEach-Object { Get-Process -Id $_ -ErrorAction SilentlyContinue })
    $stamp = Get-Date -Format 'yyyyMMdd-HHmmss-fff'
    $killer = Start-Process taskkill.exe -ArgumentList @('/PID', $Process.Id, '/T', '/F') -WindowStyle Hidden -PassThru -RedirectStandardOutput (Join-Path $script:LogRoot ($stamp + '.stop.log')) -RedirectStandardError (Join-Path $script:LogRoot ($stamp + '.stop.stderr.log'))
    if (-not $killer.WaitForExit(5000)) { $killer.Kill() }
    foreach ($child in $tracked) {
        $child.Refresh()
        if (-not $child.HasExited) { Stop-Process -InputObject $child -Force -ErrorAction SilentlyContinue }
    }
    $survivors = @($tracked | Where-Object { $_.Refresh(); -not $_.HasExited })
    if ($survivors.Count) { throw ('Unable to stop process tree: ' + ($survivors.Id -join ', ')) }
}

function Invoke-LocalBuild {
    $binary = Join-Path $script:RuntimeRoot 'local-server.exe'
    $stamp = Get-Date -Format 'yyyyMMdd-HHmmss'
    $prefix = Join-Path $script:LogRoot ('backend-build-' + $stamp)
    $go = (Get-Command go.exe -ErrorAction Stop).Source
    $started = Get-Date
    $child = Start-Process $go -ArgumentList @('build','-mod=readonly','-o',('"' + $binary + '"'),'./cmd/server') -WorkingDirectory $script:ProjectRoot -WindowStyle Hidden -RedirectStandardOutput ($prefix + '.stdout.log') -RedirectStandardError ($prefix + '.stderr.log') -PassThru
    $handle = $child.Handle
    $lastSize = -1L
    $lastProgress = Get-Date
    try {
        while (-not $child.WaitForExit(500)) {
            $size = ((Get-Item -LiteralPath ($prefix + '.stdout.log')).Length + (Get-Item -LiteralPath ($prefix + '.stderr.log')).Length)
            if ($size -ne $lastSize) { $lastSize=$size; $lastProgress=Get-Date }
            if (((Get-Date)-$started).TotalSeconds -ge 120 -or ((Get-Date)-$lastProgress).TotalSeconds -ge 60) {
                Stop-ProcessTree $child
                throw ('Build timed out or made no log progress for 60 seconds. See ' + $prefix + '.stderr.log')
            }
        }
        if ($child.ExitCode -ne 0) { throw ('Build failed. See ' + $prefix + '.stderr.log') }
    } finally {
        @{ command='go build -mod=readonly -o work/codex-runtime/local-server.exe ./cmd/server'; cwd=$script:ProjectRoot; elapsedSeconds=[math]::Round(((Get-Date)-$started).TotalSeconds,2); exitCode=$child.ExitCode } | ConvertTo-Json | Set-Content -LiteralPath ($prefix + '.json') -Encoding UTF8
    }
    return $binary
}

function Assert-LocalConfiguration {
    $path = Join-Path $script:ProjectRoot 'config.yml'
    if (-not (Test-Path -LiteralPath $path)) { throw 'Create a private config.yml first; see docs/local-development.md.' }
    $config = Get-Content -LiteralPath $path -Raw
    # Only inspect the small scalar blocks needed for this local launcher.
    # The application remains responsible for complete YAML validation.
    $server = [regex]::Match($config, '(?ms)^server:\s*\r?\n(.*?)(?=^\S|\z)').Groups[1].Value
    if ($server -notmatch '(?m)^\s+host:\s*["'']?127\.0\.0\.1["'']?\s*(?:#.*)?$' -or $server -notmatch '(?m)^\s+port:\s*8080\s*(?:#.*)?$') {
        throw 'The local launcher requires server.host=127.0.0.1 and server.port=8080 in config.yml.'
    }
    $redis = [regex]::Match($config, '(?ms)^redis:\s*\r?\n(.*?)(?=^\S|\z)').Groups[1].Value
    if ($redis -match '(?m)^\s+enabled:\s*true\s*(?:#.*)?$') {
        $redisHost = [regex]::Match($redis, '(?m)^\s+host:\s*["'']?([^\s"''#]+)').Groups[1].Value
        $redisPort = [int][regex]::Match($redis, '(?m)^\s+port:\s*(\d+)').Groups[1].Value
        if ($redisHost -notin @('127.0.0.1','localhost','::1')) { throw 'Use a separate local Redis in config.yml.' }
        if (-not (Test-LocalPort $redisHost $redisPort)) { throw ('Local Redis is unavailable on port ' + $redisPort + '; start the existing Redis service first.') }
    }
}

function Start-LocalService($Service, [string]$Binary) {
    $stamp = Get-Date -Format 'yyyyMMdd-HHmmss-fff'
    $prefix = Join-Path $script:LogRoot ($Service.Name + '-' + $stamp)
    if ($Service.Name -eq 'backend') {
        $executable = $Binary
        $arguments = @('-mode','storefront')
    } else {
        $executable = (Get-Command node.exe -ErrorAction Stop).Source
        $vite = Join-Path $Service.Directory 'node_modules/vite/bin/vite.js'
        $arguments = @(('"' + $vite + '"'),'--host','127.0.0.1','--port',$Service.Port,'--strictPort')
    }
    $child = Start-Process $executable -ArgumentList $arguments -WorkingDirectory $Service.Directory -WindowStyle Hidden -RedirectStandardOutput ($prefix + '.stdout.log') -RedirectStandardError ($prefix + '.stderr.log') -PassThru
    $handle = $child.Handle
    $state = @{ pid=$child.Id; processStartedAt=$child.StartTime.ToUniversalTime().Ticks.ToString(); url=$Service.Url; stdout=($prefix + '.stdout.log'); stderr=($prefix + '.stderr.log'); startedAt=(Get-Date).ToString('o') }
    if ($Service.Name -eq 'backend') { $state.binary=$Binary; $state.mode='storefront'; $state.upstreamPurchaseEnabled=$false }
    $state | ConvertTo-Json | Set-Content -LiteralPath (Join-Path $script:LogRoot $Service.State) -Encoding UTF8
    $deadline = (Get-Date).AddSeconds(25)
    $ready = $false
    while ((Get-Date) -lt $deadline) {
        $child.Refresh()
        if ($child.HasExited) { break }
        try {
            if ((Invoke-WebRequest -UseBasicParsing -Uri $Service.Url -TimeoutSec 2).StatusCode -eq 200) { $ready=$true; break }
        } catch { }
        Start-Sleep -Milliseconds 250
    }
    if (-not $ready) {
        Stop-ProcessTree $child
        throw ($Service.Name + ' did not become ready within 25 seconds. See ' + $prefix + '.stderr.log')
    }
    Write-Host ($Service.Name + ' ready: ' + $Service.Url + ' (PID ' + $child.Id + ')')
}
