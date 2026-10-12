param([string]$GoExe='D:\Go\bin\go.exe',[switch]$SkipFrontend)
$ErrorActionPreference='Stop'
$projectRoot=Split-Path $PSScriptRoot -Parent
Push-Location $projectRoot
$previousOS=$env:GOOS; $previousArch=$env:GOARCH; $previousCGO=$env:CGO_ENABLED
try {
 if(-not $SkipFrontend){npm run build; if($LASTEXITCODE -ne 0){throw 'Frontend build failed'}}
 $releaseRoot=Join-Path $projectRoot 'artifacts/releases'
 New-Item -ItemType Directory -Force $releaseRoot | Out-Null
 Copy-Item -LiteralPath (Join-Path $projectRoot 'out') -Destination $releaseRoot -Recurse -Force
 $env:CGO_ENABLED='0'
 foreach($target in @(@{os='windows';arch='amd64';suffix='.exe'},@{os='linux';arch='amd64';suffix=''},@{os='linux';arch='arm64';suffix=''})) {
  $env:GOOS=$target.os; $env:GOARCH=$target.arch
  $targetDir=Join-Path $releaseRoot "$($target.os)-$($target.arch)"
  New-Item -ItemType Directory -Force $targetDir | Out-Null
  foreach($module in @('backend','agent')) {
   Push-Location (Join-Path $projectRoot $module)
   try {$name=if($module -eq 'backend'){'pulse-monitor'}else{'pulse-agent'}; & $GoExe build -trimpath -o (Join-Path $targetDir ($name+$target.suffix)) .; if($LASTEXITCODE -ne 0){throw "$module build failed"}}
   finally {Pop-Location}
  }
 }
 Write-Output "Release files: $releaseRoot"
} finally {$env:GOOS=$previousOS;$env:GOARCH=$previousArch;$env:CGO_ENABLED=$previousCGO;Pop-Location}
