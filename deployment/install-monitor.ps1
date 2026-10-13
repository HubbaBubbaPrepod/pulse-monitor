param(
 [string]$PackageDirectory=(Split-Path $PSScriptRoot -Parent),
 [string]$InstallDirectory='C:\ProgramData\PulseMonitor',
 [string]$ListenAddress='127.0.0.1:8080',
 [string]$PublicURL='',
 [string]$TLSCert='',
 [string]$TLSKey=''
)
$ErrorActionPreference='Stop'
if(-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)){throw 'Run PowerShell as Administrator'}
$PackageDirectory=(Resolve-Path -LiteralPath $PackageDirectory).Path
$InstallDirectory=[IO.Path]::GetFullPath($InstallDirectory)
if($InstallDirectory.TrimEnd('\') -eq [IO.Path]::GetPathRoot($InstallDirectory).TrimEnd('\')){throw 'Install directory must not be a drive root'}
$ancestor=$InstallDirectory
while($ancestor){
 if((Test-Path -LiteralPath $ancestor) -and ((Get-Item -LiteralPath $ancestor).Attributes -band [IO.FileAttributes]::ReparsePoint)){throw 'Install directory ancestors must not be links'}
 $ancestor=Split-Path $ancestor -Parent
}
foreach($argument in @($InstallDirectory,$ListenAddress,$PublicURL,$TLSCert,$TLSKey)){if($argument.Contains('"') -or $argument.Contains("`n") -or $argument.Contains("`r")){throw 'Arguments must not contain quotes or newlines'}}
if(-not (Test-Path (Join-Path $PackageDirectory 'pulse-monitor.exe')) -or -not (Test-Path (Join-Path $PackageDirectory 'out/index.html'))){throw 'Package must contain pulse-monitor.exe and out/index.html'}
if(($TLSCert -eq '') -ne ($TLSKey -eq '')){throw 'Provide both TLSCert and TLSKey'}
New-Item -ItemType Directory -Force $InstallDirectory | Out-Null
& icacls.exe $InstallDirectory /inheritance:r /grant:r '*S-1-5-18:(OI)(CI)F' '*S-1-5-32-544:(OI)(CI)F' | Out-Null
if($LASTEXITCODE -ne 0){throw 'Failed to protect monitor directory'}
if(Get-ScheduledTask -TaskName PulseMonitor -ErrorAction SilentlyContinue){Stop-ScheduledTask -TaskName PulseMonitor; Start-Sleep -Seconds 2}
$installedExe=Join-Path $InstallDirectory 'pulse-monitor.exe'
if((Join-Path $PackageDirectory 'pulse-monitor.exe') -ne $installedExe){Copy-Item -LiteralPath (Join-Path $PackageDirectory 'pulse-monitor.exe') -Destination $installedExe -Force}
# Replace only static assets; persistent data is never copied or deleted.
$staticDirectory=Join-Path $InstallDirectory 'out'
if($PackageDirectory -ne $InstallDirectory){
 if(Test-Path $staticDirectory){
  if([IO.Path]::GetFullPath($staticDirectory) -ne [IO.Path]::Combine($InstallDirectory,'out')){throw 'Unexpected static path'}
  if((Get-Item -LiteralPath $staticDirectory).Attributes -band [IO.FileAttributes]::ReparsePoint){throw 'Static directory must not be a link'}
  Remove-Item -LiteralPath $staticDirectory -Recurse -Force
 }
 Copy-Item -LiteralPath (Join-Path $PackageDirectory 'out') -Destination $InstallDirectory -Recurse
}
$arguments='-addr "'+$ListenAddress+'" -data "'+(Join-Path $InstallDirectory 'data')+'" -static "'+$staticDirectory+'"'
if($TLSCert){$arguments+=' -tls-cert "'+(Resolve-Path -LiteralPath $TLSCert).Path+'" -tls-key "'+(Resolve-Path -LiteralPath $TLSKey).Path+'"'}
if($PublicURL){$arguments+=' -public-url "'+$PublicURL+'"'}
$action=New-ScheduledTaskAction -Execute $installedExe -Argument $arguments -WorkingDirectory $InstallDirectory
$trigger=New-ScheduledTaskTrigger -AtStartup
$principal=New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
$settings=New-ScheduledTaskSettingsSet -RestartCount 999 -RestartInterval (New-TimeSpan -Minutes 1) -ExecutionTimeLimit ([TimeSpan]::Zero) -MultipleInstances IgnoreNew -StartWhenAvailable
Register-ScheduledTask -TaskName PulseMonitor -Action $action -Trigger $trigger -Principal $principal -Settings $settings -Force | Out-Null
Start-ScheduledTask -TaskName PulseMonitor
Get-ScheduledTask -TaskName PulseMonitor