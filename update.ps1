# dotinschool - update a Docker install to the latest version and restart:
#
#   powershell -ExecutionPolicy Bypass -File .\update.ps1
#   (or just:  .\update.ps1 )
#
# Works for ANY Docker install of this project (the one-line
# `docker compose up -d --build`, the host-compile `.\docker-up.ps1` flow) -
# run it from the project's root folder.
#
# Pulls the latest source from GitHub (git fetch + hard reset, so a dirty
# working tree never causes a merge conflict), then hands off to
# .\docker-up.ps1 to rebuild the host-compiled binary, repackage the image,
# and restart the stack - no in-container `go build`, so it can't fail on
# memory / a blocked proxy / a BuildKit RPC drop.
#
# ASCII-only on purpose: Windows PowerShell 5.1 mis-reads non-ASCII script
# files unless they carry a UTF-8 BOM.

$ErrorActionPreference = 'Stop'
Set-Location -Path $PSScriptRoot

function Say($m) { Write-Host "==> $m" -ForegroundColor Cyan }
function Die($m) { Write-Host "[x] $m" -ForegroundColor Red; exit 1 }

if (-not (Get-Command git -ErrorAction SilentlyContinue)) { Die "git not found - install Git for Windows and try again." }
if (-not (Test-Path .git)) { Die "this folder is not a git checkout - clone the repo with git to use update.ps1." }

$branch = (git rev-parse --abbrev-ref HEAD 2>$null)
if (-not $branch -or $branch -eq 'HEAD') { $branch = 'main' }

Say "fetching the latest source (branch: $branch)"
git fetch --depth 1 origin $branch
if ($LASTEXITCODE -ne 0) { Die "could not reach GitHub - check this machine's internet connection." }

$before = (git rev-parse HEAD)
git reset --hard "origin/$branch" | Out-Null
$after = (git rev-parse HEAD)

if ($before -eq $after) {
  Say "already up to date - no new version on GitHub."
  exit 0
}

Say ("new version pulled ({0} -> {1}) - rebuilding and restarting" -f $before, $after)
& (Join-Path $PSScriptRoot 'docker-up.ps1')
exit $LASTEXITCODE
