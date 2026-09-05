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
function Warn($m) { Write-Host "[!] $m" -ForegroundColor Yellow }
# Guarded by an env var so a failure inside docker-up.ps1 (which pauses in
# its own Die() before this script's catch block runs) doesn't prompt twice
# for the same error.
function Wait-BeforeClose {
  if ($env:DOTINSCHOOL_PAUSED_ONCE) { return }
  $env:DOTINSCHOOL_PAUSED_ONCE = '1'
  try { Write-Host ""; Read-Host "Press Enter to close this window" | Out-Null } catch {}
}
function Die($m) {
  Write-Host "[x] $m" -ForegroundColor Red
  Wait-BeforeClose
  exit 1
}

if (-not (Get-Command git -ErrorAction SilentlyContinue)) { Die "git not found - install Git for Windows and try again." }
if (-not (Test-Path .git)) { Die "this folder is not a git checkout - clone the repo with git to use update.ps1." }

# On some machines, plain `git ...` at the console does NOT block the way it
# normally does - it spins up in its own separate window and this script
# races ahead before the fetch/reset has actually finished, which can also
# leave a stale .git/*.lock file behind for the NEXT run to trip over.
# Start-Process -NoNewWindow -Wait is the actual fix: it forces git to share
# this console (no window of its own) and guarantees this script does not
# continue until it has genuinely exited.
function Invoke-Git {
  param([Parameter(Mandatory)][string[]]$GitArgs)
  $p = Start-Process -FilePath (Get-Command git).Source -ArgumentList $GitArgs -NoNewWindow -Wait -PassThru
  return $p.ExitCode
}
foreach ($name in @('index.lock', 'shallow.lock', 'HEAD.lock', 'config.lock')) {
  $p = Join-Path '.git' $name
  if (Test-Path $p) {
    Warn "removing a leftover git lock file from an earlier attempt: $name"
    Remove-Item $p -Force -ErrorAction SilentlyContinue
  }
}

$branch = (git rev-parse --abbrev-ref HEAD 2>$null)
if (-not $branch -or $branch -eq 'HEAD') { $branch = 'main' }

Say "fetching the latest source (branch: $branch)"
if ((Invoke-Git -GitArgs @('fetch', '--depth', '1', 'origin', $branch)) -ne 0) {
  Die "could not reach GitHub - check this machine's internet connection."
}

$before = (git rev-parse HEAD)
if ((Invoke-Git -GitArgs @('reset', '--hard', "origin/$branch")) -ne 0) {
  Die "git reset failed - the checkout may be in a bad state. Delete this folder and re-run the install command to get a clean copy."
}
$after = (git rev-parse HEAD)

if ($before -eq $after) {
  Say "already up to date - no new version on GitHub."
  Wait-BeforeClose
  exit 0
}

Say ("new version pulled ({0} -> {1}) - rebuilding and restarting" -f $before, $after)
try {
  & (Join-Path $PSScriptRoot 'docker-up.ps1')
  if ($LASTEXITCODE -ne 0) { throw "docker-up.ps1 exited with code $LASTEXITCODE" }
} catch {
  Write-Host "[x] $($_.Exception.Message)" -ForegroundColor Red
  Wait-BeforeClose
  exit 1
}
Wait-BeforeClose
