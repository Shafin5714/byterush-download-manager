Add-Type -AssemblyName System.Drawing

$desktop = Join-Path $PSScriptRoot "..\resources"
$extension = Join-Path $PSScriptRoot "..\..\extension\icons"
$masterPath = Join-Path $desktop "icon-v2-transparent.png"

if (-not (Test-Path -LiteralPath $masterPath)) {
  throw "Icon master not found: $masterPath"
}

New-Item -ItemType Directory -Path $desktop -Force | Out-Null
New-Item -ItemType Directory -Path $extension -Force | Out-Null

function Export-IconSize($source, $size, $outPath) {
  $bitmap = New-Object System.Drawing.Bitmap(
    $size,
    $size,
    [System.Drawing.Imaging.PixelFormat]::Format32bppArgb
  )
  $graphics = [System.Drawing.Graphics]::FromImage($bitmap)
  try {
    $graphics.Clear([System.Drawing.Color]::Transparent)
    $graphics.CompositingMode = [System.Drawing.Drawing2D.CompositingMode]::SourceCopy
    $graphics.CompositingQuality = [System.Drawing.Drawing2D.CompositingQuality]::HighQuality
    $graphics.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $graphics.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
    $graphics.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::HighQuality
    $graphics.DrawImage($source, 0, 0, $size, $size)
    $bitmap.Save($outPath, [System.Drawing.Imaging.ImageFormat]::Png)
  } finally {
    $graphics.Dispose()
    $bitmap.Dispose()
  }
}

$master = [System.Drawing.Image]::FromFile($masterPath)
try {
  foreach ($size in @(16, 32, 48, 128, 256)) {
    Export-IconSize $master $size (Join-Path $desktop "icon-$size.png")
    if ($size -ne 256) {
      Export-IconSize $master $size (Join-Path $extension "icon-$size.png")
    }
  }
} finally {
  $master.Dispose()
}

Write-Output "ByteRush icons generated from $masterPath"
