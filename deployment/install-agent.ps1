param(
 [Parameter(Mandatory)][string]$PanelURL,
 [Parameter(Mandatory)][string]$ServerID,
 [Parameter(Mandatory)][string]$TokenFile,
 [string]$AgentExe="$PSScriptRoot\pulse-agent.exe",
 [string]$StateDirectory='C:\ProgramData\PulseAgent'
)
$ErrorActionPreference='Stop'
$agentPath=(Resolve-Path -LiteralPath $AgentExe).Path
$tokenPath=(Resolve-Path -LiteralPath $TokenFile).Path
foreach($argument in @($agentPath,$tokenPath,$StateDirectory,$PanelURL,$ServerID)){if($argument.Contains('"')){throw 'Arguments must not contain quotes'}}
if(Get-Service -Name PulseAgent -ErrorAction SilentlyContinue){throw 'PulseAgent already exists. Stop it and replace the binary to update.'}
New-Item -ItemType Directory -Force $StateDirectory | Out-Null
$binary='"'+$agentPath+'" -url "'+$PanelURL+'" -server "'+$ServerID+'" -token-file "'+$tokenPath+'" -state "'+$StateDirectory+'"'
New-Service -Name PulseAgent -DisplayName 'Pulse telemetry agent' -BinaryPathName $binary -StartupType Automatic | Out-Null
Start-Service -Name PulseAgent
Get-Service -Name PulseAgent
