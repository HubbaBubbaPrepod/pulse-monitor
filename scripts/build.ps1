#requires -Version 7.4
param(
 [string]$Version='v0.4.0',
 [string]$GoExe='go',
 [string]$GoToolchain='go1.27.2',
 [switch]$SkipFrontend
)
$ErrorActionPreference='Stop'
if($Version -notmatch '^v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$'){throw 'Version must be a semantic version, for example v0.4.0'}
$projectRoot=Split-Path $PSScriptRoot -Parent
$isMonitor=Test-Path (Join-Path $projectRoot 'backend/go.mod')
$name=if($isMonitor){'pulse-monitor'}else{'pulse-agent'}
$moduleRoot=if($isMonitor){Join-Path $projectRoot 'backend'}else{$projectRoot}
$goCommand=(Get-Command $GoExe -ErrorAction Stop).Source
$previousOS=$env:GOOS; $previousArch=$env:GOARCH; $previousCGO=$env:CGO_ENABLED; $previousToolchain=$env:GOTOOLCHAIN
Push-Location $projectRoot
try {
 if($isMonitor -and -not $SkipFrontend){npm run build; if($LASTEXITCODE -ne 0){throw 'Frontend build failed'}}
 if($isMonitor -and -not (Test-Path 'out/index.html')){throw 'Static export missing; run npm run build first'}
 foreach($required in @('README.md','LICENSE')){if(-not (Test-Path $required)){throw "Release requires $required"}}
 $releaseRoot=Join-Path $projectRoot 'artifacts/releases'
 New-Item -ItemType Directory -Force $releaseRoot | Out-Null
 $stageRoot=Join-Path $releaseRoot ('.stage-'+[guid]::NewGuid().ToString('N'))
 New-Item -ItemType Directory $stageRoot | Out-Null
 $archives=@()
 $env:CGO_ENABLED='0'
 $env:GOTOOLCHAIN=$GoToolchain
 foreach($target in @(@{os='windows';arch='amd64';suffix='.exe'},@{os='linux';arch='amd64';suffix=''},@{os='linux';arch='arm64';suffix=''})) {
  $env:GOOS=$target.os; $env:GOARCH=$target.arch
  $targetDir=Join-Path $stageRoot "$($target.os)-$($target.arch)"
  New-Item -ItemType Directory $targetDir | Out-Null
  Push-Location $moduleRoot
  try {& $goCommand build -trimpath -ldflags '-s -w' -o (Join-Path $targetDir ($name+$target.suffix)) .; if($LASTEXITCODE -ne 0){throw "$name build failed for $($target.os)/$($target.arch)"}}
  finally {Pop-Location}
  foreach($item in @('LICENSE','deployment')){if(Test-Path (Join-Path $projectRoot $item)){Copy-Item -LiteralPath (Join-Path $projectRoot $item) -Destination $targetDir -Recurse -Force}}
  Get-ChildItem -LiteralPath $projectRoot -File -Filter '*.md' | Where-Object { $_.Name -match '^(README|CHANGELOG|CONTRIBUTING|SECURITY|SUPPORT|CODE_OF_CONDUCT)(\.(en|ru))?\.md$' } | ForEach-Object { Copy-Item -LiteralPath $_.FullName -Destination $targetDir }
  $docs=Join-Path $projectRoot 'docs'
  if(Test-Path $docs){
   $docTarget=Join-Path $targetDir 'docs'
   New-Item -ItemType Directory $docTarget | Out-Null
   Get-ChildItem -LiteralPath $docs -File -Filter '*.md' | ForEach-Object {Copy-Item -LiteralPath $_.FullName -Destination $docTarget}
   if(Test-Path (Join-Path $docs 'assets')){Copy-Item -LiteralPath (Join-Path $docs 'assets') -Destination $docTarget -Recurse}
  }
  if($isMonitor){Copy-Item -LiteralPath (Join-Path $projectRoot 'out') -Destination $targetDir -Recurse}
  $extension=if($target.os -eq 'windows'){'zip'}else{'tar.gz'}
  $archive=Join-Path $releaseRoot ("${name}_${Version}_$($target.os)_$($target.arch).$extension")
  if($target.os -eq 'windows'){Compress-Archive -Path (Join-Path $targetDir '*') -DestinationPath $archive -Force}
    else {
   $archiveStream=[IO.File]::Create($archive)
   $gzip=[IO.Compression.GZipStream]::new($archiveStream,[IO.Compression.CompressionLevel]::Optimal)
   $writer=[System.Formats.Tar.TarWriter]::new($gzip,$true)
   try {
    foreach($file in Get-ChildItem -LiteralPath $targetDir -Recurse -File){
     $relative=[IO.Path]::GetRelativePath($targetDir,$file.FullName).Replace('\','/')
     $entry=[System.Formats.Tar.PaxTarEntry]::new([System.Formats.Tar.TarEntryType]::RegularFile,$relative)
     $mode=if($relative -eq $name -or $relative.EndsWith('.sh')){493}else{420}
     $entry.Mode=[IO.UnixFileMode]$mode
     $entry.ModificationTime=[DateTimeOffset]::FromUnixTimeSeconds(0)
     $entry.DataStream=[IO.File]::OpenRead($file.FullName)
     try {$writer.WriteEntry($entry)} finally {$entry.DataStream.Dispose()}
    }
   } finally {$writer.Dispose(); $gzip.Dispose(); $archiveStream.Dispose()}
  }
  $archives+=$archive
 }
 $checksums=@($archives | ForEach-Object {('{0}  {1}' -f (Get-FileHash -LiteralPath $_ -Algorithm SHA256).Hash.ToLowerInvariant(),(Split-Path $_ -Leaf))})
 [IO.File]::WriteAllLines((Join-Path $releaseRoot 'checksums.txt'),$checksums,[Text.UTF8Encoding]::new($false))
 Write-Output "Release archives and checksums: $releaseRoot"
 Write-Output "Staging files retained at: $stageRoot"
} finally {$env:GOOS=$previousOS; $env:GOARCH=$previousArch; $env:CGO_ENABLED=$previousCGO; $env:GOTOOLCHAIN=$previousToolchain; Pop-Location}
