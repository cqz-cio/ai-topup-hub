param([ValidateSet('start','stop','status')][string]$Action='start',[switch]$Build)
$ErrorActionPreference='Stop'
$project=Split-Path -Parent (Split-Path -Parent $PSScriptRoot)
$runtime=Join-Path $project 'work/codex-runtime/support-agent'
$logs=Join-Path $project 'work/codex-logs'
$binary=Join-Path $runtime 'support-agent.exe'
$private=Join-Path $runtime 'private/config.json'
$stateFile=Join-Path $logs 'support-agent.json'
New-Item -ItemType Directory -Path $runtime,$logs -Force | Out-Null
function Get-SupportProcess {
 if (-not(Test-Path -LiteralPath $stateFile)){return $null}
 $state=Get-Content -LiteralPath $stateFile -Raw | ConvertFrom-Json
 $process=Get-Process -Id $state.pid -ErrorAction SilentlyContinue
 if (-not $process){return $null}
 if ($process.StartTime.ToUniversalTime().Ticks -ne [long]$state.startedUtcTicks -or $process.Path -ne $binary){throw 'Saved support process identity mismatch; no process stopped.'}
 return $process
}
$owned=Get-SupportProcess
if($Action -eq 'status'){
 [ordered]@{running=($null -ne $owned);pid=if($owned){$owned.Id}else{$null};url='http://127.0.0.1:8085/api/v1/support/health';privateConfigured=(Test-Path -LiteralPath $private)} | ConvertTo-Json
 exit
}
if($Action -eq 'stop'){
 if($owned){$owned.Kill($true);if(-not $owned.WaitForExit(5000)){throw 'Support process did not stop'}}
 if(Test-Path -LiteralPath $stateFile){Remove-Item -LiteralPath $stateFile}
 Write-Output 'Local support service stopped.';exit
}
if($owned){if($Build){throw 'Stop the local support service before rebuilding'};Write-Output 'Local support service is already running.';exit}
if(-not(Test-Path -LiteralPath $private)){throw 'Missing private support configuration; see docs/customer-support-deepseek.md'}
if(Get-NetTCPConnection -LocalPort 8085 -State Listen -ErrorAction SilentlyContinue){throw 'Port 8085 belongs to another process; leaving it unchanged.'}
if($Build -or -not(Test-Path -LiteralPath $binary)){
 $stamp=Get-Date -Format 'yyyyMMdd-HHmmss'
 $out=Join-Path $logs ('support-build-'+$stamp+'.stdout.log')
 $err=Join-Path $logs ('support-build-'+$stamp+'.stderr.log')
 $go=(Get-Command go.exe -ErrorAction Stop).Source
 $builder=Start-Process -FilePath $go -ArgumentList @('build','-o',('"'+$binary+'"'),'./cmd/support-agent') -WorkingDirectory $project -WindowStyle Hidden -RedirectStandardOutput $out -RedirectStandardError $err -PassThru
 $watch=[Diagnostics.Stopwatch]::StartNew()
 while(-not $builder.HasExited -and $watch.Elapsed.TotalSeconds -lt 120){Start-Sleep -Milliseconds 500;$builder.Refresh()}
 if(-not $builder.HasExited){$builder.Kill($true);$null=$builder.WaitForExit(5000);throw ('Build timed out; process tree stopped. Logs: '+$out+' / '+$err)}
 $builder.WaitForExit()
 [ordered]@{command='go build ./cmd/support-agent';directory=$project;seconds=$watch.Elapsed.TotalSeconds;exitCode=$builder.ExitCode} | ConvertTo-Json | Set-Content -LiteralPath ($out+'.json')
 if($builder.ExitCode -ne 0){throw ('Build failed; inspect '+$err)}
}
$stamp=Get-Date -Format 'yyyyMMdd-HHmmss'
$out=Join-Path $logs ('support-agent-'+$stamp+'.stdout.log')
$err=Join-Path $logs ('support-agent-'+$stamp+'.stderr.log')
$process=Start-Process -FilePath $binary -ArgumentList @('-config',('"'+$private+'"')) -WorkingDirectory $project -WindowStyle Hidden -RedirectStandardOutput $out -RedirectStandardError $err -PassThru
[ordered]@{pid=$process.Id;startedUtcTicks=$process.StartTime.ToUniversalTime().Ticks;binary=$binary;stdout=$out;stderr=$err} | ConvertTo-Json | Set-Content -LiteralPath $stateFile
$ready=$false
for($attempt=0;$attempt -lt 10;$attempt++){
 if($process.HasExited){break}
 try{$health=Invoke-RestMethod -Uri 'http://127.0.0.1:8085/api/v1/support/health' -TimeoutSec 1;if($health.ready){$ready=$true;break}}catch{}
 Start-Sleep -Milliseconds 500
}
if(-not $ready){if(-not $process.HasExited){$process.Kill($true);$null=$process.WaitForExit(5000)};throw ('Support service did not become ready; inspect '+$err)}
Write-Output ('Local support service ready on 127.0.0.1:8085; PID '+$process.Id)
