$ErrorActionPreference='Stop'
if(-not ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole([Security.Principal.WindowsBuiltInRole]::Administrator)) {
 Write-Output 'SKIP: permission installation test requires an elevated administrator, like the installer'
 return
}
. (Join-Path $PSScriptRoot '../deployment/windows-permissions.ps1')
$testDirectory=Join-Path $env:TEMP ('pulse-acl-test-'+[Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $testDirectory | Out-Null
$originalAcl=Get-Acl -LiteralPath $testDirectory
try {
 & (Join-Path $env:SystemRoot 'System32/icacls.exe') $testDirectory /grant '*S-1-1-0:(OI)(CI)F' | Out-Null
 if($LASTEXITCODE -ne 0){throw 'Fixture ACL setup failed'}
 Set-PulseDirectoryPermissions -Path $testDirectory -ServiceRights ReadAndExecute
 $acl=Get-Acl -LiteralPath $testDirectory
 if(-not $acl.AreAccessRulesProtected){throw 'Private directory still inherits public permissions'}
 $allowed=@('S-1-5-18','S-1-5-32-544','S-1-5-19')
 foreach($rule in $acl.GetAccessRules($true,$true,[Security.Principal.SecurityIdentifier])) {
  if($rule.IdentityReference.Value -notin $allowed){throw 'An existing broad explicit ACL survived protection'}
 }
 if($acl.GetOwner([Security.Principal.SecurityIdentifier]).Value -ne [Security.Principal.WindowsIdentity]::GetCurrent().User.Value){throw 'Untrusted owner retained DACL control'}
 Write-Output 'PASS: private directory replaces inherited and explicit broad ACLs'
 $binary=Join-Path $testDirectory 'pulse-monitor.exe'
 [IO.File]::WriteAllText($binary,'fixture')
 $fileAcl=Get-Acl -LiteralPath $binary
 $fileAcl.SetAccessRuleProtection($true,$true)
 $fileAcl.AddAccessRule([Security.AccessControl.FileSystemAccessRule]::new([Security.Principal.SecurityIdentifier]::new('S-1-1-0'),[Security.AccessControl.FileSystemRights]::Modify,[Security.AccessControl.AccessControlType]::Allow))
 Set-Acl -LiteralPath $binary -AclObject $fileAcl
 $refused=$false
 try {Assert-PulsePrivateTree -Path $testDirectory -AllowLocalService} catch {$refused=$true}
 if(-not $refused){throw 'An unsafe existing executable was accepted for privileged upgrade'}
 Write-Output 'PASS: unsafe existing executable is refused before an upgrade'
 $serviceFile=Join-Path $testDirectory 'pulse.db-wal'
 [IO.File]::WriteAllText($serviceFile,'fixture')
 & (Join-Path $env:SystemRoot 'System32/icacls.exe') $serviceFile /setowner '*S-1-5-19' | Out-Null
 if($LASTEXITCODE -ne 0){throw 'Service owner fixture setup failed'}
 Assert-PulsePrivateTree -Path $serviceFile -AllowLocalService
 Write-Output 'PASS: private files created by Local Service remain upgradeable'
} finally {
 Set-Acl -LiteralPath $testDirectory -AclObject $originalAcl
 $resolved=[IO.Path]::GetFullPath($testDirectory)
 if($resolved.StartsWith([IO.Path]::GetFullPath($env:TEMP).TrimEnd('\')+'\') -and (Split-Path $resolved -Leaf).StartsWith('pulse-acl-test-')){Remove-Item -LiteralPath $resolved -Recurse -Force}
}
