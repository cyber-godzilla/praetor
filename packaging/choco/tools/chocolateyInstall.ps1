$ErrorActionPreference = 'Stop'
$toolsDir = Split-Path -Parent $MyInvocation.MyCommand.Definition

# Both praetor.exe (GUI) and praetor-tui.exe (TUI) ship in this tools dir and
# are auto-shimmed onto PATH by Chocolatey. The praetor.exe.gui marker file
# tells Chocolatey to build a windowed (no-console) shim for the GUI.

# Start Menu shortcut for the GUI so it appears in the applications list
# (requirement 4).
$guiExe = Join-Path $toolsDir 'praetor.exe'
$startMenu = [Environment]::GetFolderPath('CommonPrograms')
$shortcut = Join-Path $startMenu 'Praetor.lnk'
if (Test-Path $guiExe) {
  Install-ChocolateyShortcut `
    -ShortcutFilePath $shortcut `
    -TargetPath $guiExe `
    -Description 'Praetor — desktop client for The Eternal City'
}
