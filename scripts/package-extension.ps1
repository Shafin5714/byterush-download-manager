# Package ByteRush Browser Extension for GitHub Releases
param (
    [string]$OutputDir = ""
)

$ErrorActionPreference = "Stop"

$RootDir = Resolve-Path (Join-Path $PSScriptRoot "..")
$ExtensionDir = Join-Path $RootDir "apps\extension"
$ManifestPath = Join-Path $ExtensionDir "manifest.json"

if (-not (Test-Path $ManifestPath)) {
    Write-Error "manifest.json not found at $ManifestPath"
    exit 1
}

$manifestJson = Get-Content -Raw -Path $ManifestPath | ConvertFrom-Json
$version = $manifestJson.version
$name = "byterush-extension"

if ([string]::IsNullOrWhiteSpace($OutputDir)) {
    $OutputDir = Join-Path $RootDir "release"
}

if (-not (Test-Path $OutputDir)) {
    New-Item -ItemType Directory -Path $OutputDir -Force | Out-Null
}

$zipFileName = "${name}-v${version}.zip"
$zipFilePath = Join-Path $OutputDir $zipFileName
$latestZipPath = Join-Path $OutputDir "${name}-latest.zip"

Write-Host "==> Packaging ByteRush Extension v$version..." -ForegroundColor Cyan

# Create a clean temporary staging directory
$tempStageDir = Join-Path ([System.IO.Path]::GetTempPath()) "byterush-ext-pkg-$(Get-Random)"
if (Test-Path $tempStageDir) {
    Remove-Item -Recurse -Force $tempStageDir
}
New-Item -ItemType Directory -Path $tempStageDir -Force | Out-Null

try {
    # Files and folders to include
    $itemsToCopy = @(
        "manifest.json",
        "background.js",
        "popup.html",
        "popup.css",
        "popup.js",
        "youtube.js",
        "youtube.css",
        "icons"
    )

    foreach ($item in $itemsToCopy) {
        $sourcePath = Join-Path $ExtensionDir $item
        if (Test-Path $sourcePath) {
            Copy-Item -Path $sourcePath -Destination (Join-Path $tempStageDir $item) -Recurse -Force
        } else {
            Write-Warning "Item not found: $sourcePath"
        }
    }

    # Remove existing zip if present
    if (Test-Path $zipFilePath) {
        Remove-Item -Force $zipFilePath
    }
    if (Test-Path $latestZipPath) {
        Remove-Item -Force $latestZipPath
    }

    # Compress staging folder contents directly to root of zip
    Compress-Archive -Path "$tempStageDir\*" -DestinationPath $zipFilePath -CompressionLevel Optimal
    Copy-Item -Path $zipFilePath -Destination $latestZipPath -Force

    $zipItem = Get-Item $zipFilePath
    $sizeKb = [math]::Round($zipItem.Length / 1024, 2)

    Write-Host ""
    Write-Host "[SUCCESS] Successfully generated extension package:" -ForegroundColor Green
    Write-Host "  - Version Package : $zipFilePath ($sizeKb KB)" -ForegroundColor White
    Write-Host "  - Latest Package  : $latestZipPath" -ForegroundColor White

    # Verify Zip contents
    Write-Host ""
    Write-Host "Verifying archive contents:" -ForegroundColor Cyan
    Add-Type -AssemblyName System.IO.Compression.FileSystem
    $zip = [System.IO.Compression.ZipFile]::OpenRead($zipFilePath)
    foreach ($entry in $zip.Entries) {
        $entryKb = [math]::Round($entry.Length / 1024, 1)
        Write-Host "  [$entryKb KB] $($entry.FullName)" -ForegroundColor Gray
    }
    $zip.Dispose()

} finally {
    if (Test-Path $tempStageDir) {
        Remove-Item -Recurse -Force $tempStageDir -ErrorAction SilentlyContinue
    }
}
