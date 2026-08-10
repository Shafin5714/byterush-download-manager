$ErrorActionPreference = "Stop"
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$root = Split-Path -Parent $scriptDir
$engineDir = Join-Path (Split-Path -Parent $root) "engine"
$out = Join-Path $root "resources\engine.exe"

$go = "$env:ProgramFiles\Go\bin\go.exe"
if (-not (Test-Path $go)) { $go = "go" }

New-Item -ItemType Directory -Path (Split-Path -Parent $out) -Force | Out-Null
Push-Location $engineDir
try {
  & $go build -o $out "."
} finally {
  Pop-Location
}
if (-not $?) { exit 1 }
Write-Output "engine built -> $out"
