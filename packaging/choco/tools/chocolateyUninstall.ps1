$ErrorActionPreference = 'Stop'

# Remove the Start Menu shortcut created on install. The shimmed binaries are
# cleaned up automatically by Chocolatey when the package is removed.
$startMenu = [Environment]::GetFolderPath('CommonPrograms')
$shortcut = Join-Path $startMenu 'Praetor.lnk'
if (Test-Path $shortcut) {
  Remove-Item $shortcut -Force -ErrorAction SilentlyContinue
}
