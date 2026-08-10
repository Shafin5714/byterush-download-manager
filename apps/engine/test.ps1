param(
  [string]$EngineDir = "$env:TEMP\byterush-test\engine"
)

$ErrorActionPreference = "Stop"
Remove-Item -Recurse -Force $EngineDir -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Path $EngineDir -Force | Out-Null

$proc = Start-Process -FilePath "bin\engine.exe" -ArgumentList "--port","29641","--dir","$EngineDir" -PassThru -RedirectStandardOutput "$EngineDir\out.log" -RedirectStandardError "$EngineDir\err.log" -WindowStyle Hidden

function Ping-Engine {
  for ($i = 0; $i -lt 20; $i++) {
    try {
      $r = Invoke-RestMethod -Uri "http://127.0.0.1:29641/api/ping" -TimeoutSec 2
      return $r
    } catch { Start-Sleep -Milliseconds 300 }
  }
  throw "engine did not start: $(Get-Content "$EngineDir\err.log" -Raw)"
}

try {
  $ping = Ping-Engine
  Write-Output "PING OK: $($ping.app) v$($ping.version) port $($ping.port)"

  $body = @{ url = "https://proof.ovh.net/files/10Mb.dat"; folder = "$EngineDir\dl" } | ConvertTo-Json
  $add = Invoke-RestMethod -Uri "http://127.0.0.1:29641/api/downloads" -Method Post -Body $body -ContentType "application/json"
  Write-Output "ADDED: id=$($add.id) status=$($add.status) kind=$($add.kind)"

  $prev = 0
  $idle = 0
  for ($i = 0; $i -lt 200; $i++) {
    Start-Sleep -Milliseconds 500
    $d = Invoke-RestMethod -Uri "http://127.0.0.1:29641/api/downloads"
    $dl = $d | Where-Object { $_.id -eq $add.id } | Select-Object -First 1
    if (-not $dl) { throw "download disappeared" }
    if ($i % 10 -eq 0 -or $dl.status -ne "active") {
      Write-Output "  [$i] status=$($dl.status) downloaded=$($dl.downloaded) total=$($dl.totalSize) speed=$($dl.speed) segs=$($dl.segments.Count)"
    }
    if ($dl.status -eq "completed") { Write-Output "COMPLETED OK"; break }
    if ($dl.status -eq "error") { throw "download error: $($dl.error)" }
    if ($dl.downloaded -eq $prev) { $idle++; if ($idle -gt 30) { throw "stalled" } } else { $idle = 0; $prev = $dl.downloaded }
  }

  $files = Get-ChildItem "$EngineDir\dl"
  Write-Output "FILES: $($files.Name -join ', ') size=$($files[0].Length)"

  # pause / resume test (unique filename + big file so pause lands mid-download)
  $body2 = @{ url = "https://proof.ovh.net/files/100Mb.dat"; filename = "100Mb-pause.dat"; folder = "$EngineDir\dl" } | ConvertTo-Json
  $add2 = Invoke-RestMethod -Uri "http://127.0.0.1:29641/api/downloads" -Method Post -Body $body2 -ContentType "application/json"
  Start-Sleep -Seconds 5
  Invoke-RestMethod -Uri "http://127.0.0.1:29641/api/downloads/$($add2.id)/pause" -Method Post | Out-Null
  Start-Sleep -Milliseconds 800
  $list = Invoke-RestMethod -Uri "http://127.0.0.1:29641/api/downloads"
  $paused = $list | Where-Object { $_.id -eq $add2.id } | Select-Object -First 1
  Write-Output "pause target: id=$($paused.id) filename=$($paused.filename) status=$($paused.status) downloaded=$($paused.downloaded)"
  $partial = [int64]$paused.downloaded
  Write-Output "PAUSED at $partial bytes (status=$($paused.status))"
  if ($paused.status -ne "paused" -or $partial -eq 0) { throw "pause failed" }

  Invoke-RestMethod -Uri "http://127.0.0.1:29641/api/downloads/$($add2.id)/resume" -Method Post | Out-Null
  for ($i = 0; $i -lt 200; $i++) {
    Start-Sleep -Milliseconds 500
    $list2 = Invoke-RestMethod -Uri "http://127.0.0.1:29641/api/downloads"
    $d2 = $list2 | Where-Object { $_.id -eq $add2.id } | Select-Object -First 1
    if ($d2.status -eq "completed") { Write-Output "RESUME OK (final $($d2.downloaded) bytes)"; break }
    if ($d2.status -eq "error") { throw "resume error: $($d2.error)" }
  }
  if ($d2.status -ne "completed") { throw "resume did not complete" }
  $finalFiles = Get-ChildItem "$EngineDir\dl" -Filter "100Mb-pause.dat*"
  $finalCheck = $finalFiles | Where-Object { $_.Name -eq "100Mb-pause.dat" } | Select-Object -First 1
  if (-not $finalCheck) { throw "final file not renamed: $($finalFiles.Name -join ',')" }
  Write-Output "RENAME OK: $($finalCheck.Name) size=$($finalCheck.Length)"

  # speed limiter test: set 512 KB/s, expect average throughput < 900 KB/s
  $settingsBody = @{ speedLimitKBs = 512 } | ConvertTo-Json
  Invoke-RestMethod -Uri "http://127.0.0.1:29641/api/settings" -Method Post -Body $settingsBody -ContentType "application/json" | Out-Null
  $body3 = @{ url = "https://proof.ovh.net/files/10Mb.dat"; filename = "10Mb-limit.dat"; folder = "$EngineDir\dl" } | ConvertTo-Json
  $add3 = Invoke-RestMethod -Uri "http://127.0.0.1:29641/api/downloads" -Method Post -Body $body3 -ContentType "application/json"
  $t0 = Get-Date
  for ($i = 0; $i -lt 200; $i++) {
    Start-Sleep -Milliseconds 500
    $list3 = Invoke-RestMethod -Uri "http://127.0.0.1:29641/api/downloads"
    $d3 = $list3 | Where-Object { $_.id -eq $add3.id } | Select-Object -First 1
    if ($d3.status -eq "completed" -or $d3.status -eq "error") { break }
  }
  $secs = ((Get-Date) - $t0).TotalSeconds
  $avgKBs = [math]::Round(10485760 / $secs / 1024)
  Write-Output "LIMIT TEST: avg=$avgKBs KB/s (limit=512 KB/s)"
  if ($avgKBs -gt 900) { throw "speed limit not enforced" }
  Invoke-RestMethod -Uri "http://127.0.0.1:29641/api/settings" -Method Post -Body (@{ speedLimitKBs = 0 } | ConvertTo-Json) -ContentType "application/json" | Out-Null

  # persistence test: add paused download, kill engine, restart, verify restored
  $body4 = @{ url = "https://proof.ovh.net/files/100Mb.dat"; filename = "100Mb-persist.dat"; folder = "$EngineDir\dl" } | ConvertTo-Json
  $add4 = Invoke-RestMethod -Uri "http://127.0.0.1:29641/api/downloads" -Method Post -Body $body4 -ContentType "application/json"
  Start-Sleep -Seconds 4
  Invoke-RestMethod -Uri "http://127.0.0.1:29641/api/downloads/$($add4.id)/pause" -Method Post | Out-Null
  Start-Sleep -Milliseconds 600
  $persistId = $add4.id
  try { Invoke-RestMethod -Uri "http://127.0.0.1:29641/api/shutdown" -Method Post -TimeoutSec 2 | Out-Null } catch {}
  Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
  Start-Sleep -Seconds 1
  $proc2 = Start-Process -FilePath "bin\engine.exe" -ArgumentList "--port","29641","--dir","$EngineDir" -PassThru -RedirectStandardOutput "$EngineDir\out2.log" -RedirectStandardError "$EngineDir\err2.log" -WindowStyle Hidden
  $ping2 = Ping-Engine
  $list4 = Invoke-RestMethod -Uri "http://127.0.0.1:29641/api/downloads"
  $d4 = $list4 | Where-Object { $_.id -eq $persistId } | Select-Object -First 1
  Write-Output "PERSIST TEST: found=$(-not [string]::IsNullOrEmpty($d4)) status=$($d4.status) downloaded=$($d4.downloaded)"
  if ([string]::IsNullOrEmpty($d4) -or $d4.status -ne "paused") { throw "persistence failed" }
  $proc = $proc2

  Write-Output "ALL TESTS PASSED"
} finally {
  try { Invoke-RestMethod -Uri "http://127.0.0.1:29641/api/shutdown" -Method Post -TimeoutSec 2 | Out-Null } catch {}
  Start-Sleep -Milliseconds 500
  Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
}
