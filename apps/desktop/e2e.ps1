$ErrorActionPreference = "Stop"
$desktop = "C:\Users\Shafin_MTL\Documents\Local\byterush-download-manager\apps\desktop"
Push-Location $desktop

$proc = Start-Process -FilePath "npx.cmd" -ArgumentList "electron","." -PassThru -RedirectStandardOutput "$env:TEMP\byterush-e2e\out.log" -RedirectStandardError "$env:TEMP\byterush-e2e\err.log" -WindowStyle Hidden

function Wait-Ping {
  for ($i = 0; $i -lt 30; $i++) {
    try {
      $r = Invoke-RestMethod -Uri "http://127.0.0.1:29641/api/ping" -TimeoutSec 2
      return $r
    } catch { Start-Sleep -Milliseconds 500 }
  }
  throw "engine under electron did not start: $(Get-Content "$env:TEMP\byterush-e2e\err.log" -Raw -ErrorAction SilentlyContinue)"
}

try {
  $ping = Wait-Ping
  Write-Output "E2E PING OK: $($ping.app) port=$($ping.port)"

  $body = @{ url = "https://proof.ovh.net/files/10Mb.dat"; filename = "e2e.dat"; folder = "$env:TEMP\byterush-e2e\dl" } | ConvertTo-Json
  $add = Invoke-RestMethod -Uri "http://127.0.0.1:29641/api/downloads" -Method Post -Body $body -ContentType "application/json"
  Write-Output "ADDED: $($add.id) status=$($add.status)"

  for ($i = 0; $i -lt 120; $i++) {
    Start-Sleep -Milliseconds 500
    $list = Invoke-RestMethod -Uri "http://127.0.0.1:29641/api/downloads"
    $d = $list | Where-Object { $_.id -eq $add.id } | Select-Object -First 1
    if ($d.status -eq "completed" -or $d.status -eq "error") { break }
  }
  Write-Output "E2E DOWNLOAD: status=$($d.status) bytes=$($d.downloaded)"
  if ($d.status -ne "completed") { throw "e2e download failed: $($d.error)" }
  if (-not (Test-Path "$env:TEMP\byterush-e2e\dl\e2e.dat")) { throw "file missing" }
  Write-Output "E2E FILE OK: $((Get-Item "$env:TEMP\byterush-e2e\dl\e2e.dat").Length) bytes"
  Write-Output "E2E ALL PASSED"
} finally {
  try { Invoke-RestMethod -Uri "http://127.0.0.1:29641/api/shutdown" -Method Post -TimeoutSec 2 | Out-Null } catch {}
  Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
  Start-Sleep -Milliseconds 500
  Pop-Location
}
