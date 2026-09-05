# dotinschool - Windows management command (Docker install).
#
# This is the file the global "dotinschool" command actually runs - installed
# by docker-up.ps1 as a thin shim (dotinschool.cmd) in
# %LOCALAPPDATA%\Microsoft\WindowsApps (already on every user's PATH, no
# admin rights needed) that just calls this script out of the checkout. That
# means "dotinschool update" always runs the LATEST copy of this file after
# pulling - no separate re-install step, unlike a copy baked in at install
# time would need.
#
#   dotinschool                interactive menu
#   dotinschool up              start the stack (no rebuild)
#   dotinschool down            stop the stack
#   dotinschool restart         restart the containers
#   dotinschool logs            follow the app container's logs
#   dotinschool ps               show container status
#   dotinschool rebuild          recompile the binary + repackage + recreate
#   dotinschool update           git pull the latest version, then rebuild
#   dotinschool admin            show the panel URL + admin credentials
#
# Mirrors the Linux Docker install's wrapper (build/docker-install.sh) so the
# same command set works the same way on both platforms.
#
# ASCII-only on purpose: Windows PowerShell 5.1 mis-reads non-ASCII script
# files unless they carry a UTF-8 BOM.

$ErrorActionPreference = 'Stop'

# This file lives at <checkout>\scripts\dotinschool-cli.ps1.
$InstallDir = Split-Path -Parent $PSScriptRoot
Set-Location $InstallDir

# --env-file .env (not relying on compose's own auto-discovery): compose
# resolves ${APP_PORT:-8080} in build/docker-compose.yml relative to the
# COMPOSE FILE's directory (build/), not $InstallDir - so without this flag,
# APP_PORT from .env here is silently ignored and it always falls back to
# the literal default 8080, no matter what .env actually says (docker-up.ps1
# sidesteps this by exporting $env:APP_PORT directly before its own compose
# call, which this wrapper does not do).
$ComposeArgs = @('--env-file', '.env', '-f', 'build/docker-compose.yml')

function Say($m) { Write-Host "==> $m" -ForegroundColor Cyan }
function Die($m) { Write-Host "[x] $m" -ForegroundColor Red; throw $m }

function Show-Admin {
  $envFile = Join-Path $InstallDir '.env'
  if (-not (Test-Path $envFile)) {
    Write-Host "No .env found yet - this hasn't been started. Run: dotinschool up"
    return
  }
  $cfg = @{}
  Get-Content $envFile | Where-Object { $_ -match '^\s*[^#].*=' } | ForEach-Object {
    $kv = $_ -split '=', 2
    $cfg[$kv[0].Trim()] = $kv[1].Trim()
  }
  $port = if ($cfg.ContainsKey('APP_PORT') -and $cfg['APP_PORT']) { $cfg['APP_PORT'] } else { '8080' }
  Write-Host ("user panel  : http://localhost:{0}/" -f $port)
  Write-Host ("admin panel : http://localhost:{0}/{1}/" -f $port, $cfg['ADMIN_PATH'])
  Write-Host ("username    : {0}" -f $cfg['ADMIN_USERNAME'])
  Write-Host ("password    : {0}" -f $cfg['ADMIN_PASSWORD'])
}

function Invoke-Action([string]$action) {
  switch ($action) {
    'up' {
      Say "starting the stack"
      docker compose @ComposeArgs up -d
    }
    'down' {
      Say "stopping the stack"
      docker compose @ComposeArgs down
    }
    'restart' {
      Say "restarting"
      docker compose @ComposeArgs restart
    }
    'logs' {
      docker compose @ComposeArgs logs -f app
    }
    'ps' {
      docker compose @ComposeArgs ps
    }
    'rebuild' {
      & (Join-Path $InstallDir 'docker-up.ps1')
    }
    'update' {
      & (Join-Path $InstallDir 'update.ps1')
    }
    'admin' {
      Show-Admin
    }
    default {
      Die "unknown command: $action`nusage: dotinschool {up|down|restart|logs|ps|rebuild|update|admin}"
    }
  }
}

function Show-Menu {
  Write-Host ""
  Write-Host "+----------------------------------------------+"
  Write-Host "|            dotinschool - management            |"
  Write-Host "+----------------------------------------------+"
  Write-Host "|  0. Exit                                        |"
  Write-Host "|  1. Start (up)                                  |"
  Write-Host "|  2. Stop (down)                                 |"
  Write-Host "|  3. Restart                                     |"
  Write-Host "|  4. Rebuild (recompile + recreate)               |"
  Write-Host "|  5. Update (pull latest from GitHub + rebuild)   |"
  Write-Host "|  6. Show logs                                   |"
  Write-Host "|  7. Show container status                        |"
  Write-Host "|  8. Show admin URL + credentials                 |"
  Write-Host "+----------------------------------------------+"
  $choice = Read-Host "Choice [0-8]"
  switch ($choice) {
    '0' { return }
    '1' { Invoke-Action 'up' }
    '2' { Invoke-Action 'down' }
    '3' { Invoke-Action 'restart' }
    '4' { Invoke-Action 'rebuild' }
    '5' { Invoke-Action 'update' }
    '6' { Invoke-Action 'logs' }
    '7' { Invoke-Action 'ps' }
    '8' { Invoke-Action 'admin' }
    default { Write-Host "Enter a number from 0 to 8." }
  }
}

if ($args.Count -eq 0) {
  Show-Menu
} else {
  Invoke-Action $args[0]
}
