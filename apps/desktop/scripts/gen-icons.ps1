Add-Type -AssemblyName System.Drawing

function New-Icon($size, $outPath) {
  $bmp = New-Object System.Drawing.Bitmap($size, $size)
  $g = [System.Drawing.Graphics]::FromImage($bmp)
  $g.SmoothingMode = 'AntiAlias'
  $rect = New-Object System.Drawing.Rectangle(0, 0, $size, $size)
  $c1 = [System.Drawing.Color]::FromArgb(255, 79, 140, 255)
  $c2 = [System.Drawing.Color]::FromArgb(255, 58, 111, 216)
  $brush = New-Object System.Drawing.Drawing2D.LinearGradientBrush($rect, $c1, $c2, 45)
  $round = [System.Drawing.Drawing2D.GraphicsPath]::new()
  $r = [int]($size * 0.22)
  $round.AddArc(0, 0, $r, $r, 180, 90)
  $round.AddArc($size - $r, 0, $r, $r, 270, 90)
  $round.AddArc($size - $r, $size - $r, $r, $r, 0, 90)
  $round.AddArc(0, $size - $r, $r, $r, 90, 90)
  $round.CloseFigure()
  $g.FillPath($brush, $round)

  $white = New-Object System.Drawing.SolidBrush([System.Drawing.Color]::White)
  $stem = New-Object System.Drawing.Rectangle(
    [int]($size * 0.47), [int]($size * 0.20), [int]($size * 0.06), [int]($size * 0.36))
  $g.FillRectangle($white, $stem)
  $tip = @(
    (New-Object System.Drawing.PointF([float]($size * 0.5), [float]($size * 0.70))),
    (New-Object System.Drawing.PointF([float]($size * 0.30), [float]($size * 0.52))),
    (New-Object System.Drawing.PointF([float]($size * 0.70), [float]($size * 0.52)))
  )
  $g.FillPolygon($white, $tip)
  $g.Dispose()
  $bmp.Save($outPath, [System.Drawing.Imaging.ImageFormat]::Png)
  $bmp.Dispose()
}

$desktop = Join-Path $PSScriptRoot "..\resources"
$ext = Join-Path $PSScriptRoot "..\..\extension\icons"
New-Item -ItemType Directory -Path $desktop -Force | Out-Null
New-Item -ItemType Directory -Path $ext -Force | Out-Null

foreach ($s in @(16, 32, 48, 128, 256)) {
  New-Icon $s (Join-Path $desktop "icon-$s.png")
  if ($s -ne 256) { New-Icon $s (Join-Path $ext "icon-$s.png") }
}
Write-Output "icons generated"
