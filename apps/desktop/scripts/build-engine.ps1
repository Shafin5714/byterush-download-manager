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
  & $go build -buildvcs=false -o $out "."
  if ($LASTEXITCODE -ne 0) { exit $LASTEXITCODE }
} finally {
  Pop-Location
}
Write-Output "engine built -> $out"
