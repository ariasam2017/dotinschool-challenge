# dotinschool - Windows installer.
#
# The dotinschool repo is private, so fetching this file and the git clone it
# runs both need a GitHub token: a fine-grained personal access token, scoped
# to just this repository with "Contents: Read-only", is enough. Create one at
# https://github.com/settings/personal-access-tokens/new, then run this - no
# token in the command itself, it asks for it interactively (input hidden):
#
#   $s = Read-Host "GitHub token" -AsSecureString
#   $T = [Runtime.InteropServices.Marshal]::PtrToStringAuto([Runtime.InteropServices.Marshal]::SecureStringToBSTR($s))
#   $env:DOTINSCHOOL_GH_TOKEN = $T
#   irm -Headers @{Authorization="token $T"} https://raw.githubusercontent.com/ariasam2017/dotinschool/main/scripts/windows-install.ps1 | iex
#
# The token is only needed for this first run: it gets embedded in the local
# clone's git remote, so a later `.\docker-up.ps1` or re-run of this same
# command line keeps working without it.
#
# Fetches the latest dotinschool source from GitHub into %USERPROFILE%\dotinschool
# (or updates it if already there), then hands off to docker-up.ps1, which
# installs Go if missing and builds/starts the stack (Docker Desktop itself
# is not auto-installed - docker-up.ps1 will tell you if it's missing).
#
# ASCII-only on purpose: Windows PowerShell 5.1 mis-reads non-ASCII script files
# unless they carry a UTF-8 BOM, and `irm | iex` has no file of its own to tag
# with one.

$ErrorActionPreference = 'Stop'

# This whole file runs via `irm | iex`, which evaluates it as text and isn't
# subject to the script-file execution policy - but the moment it later
# invokes docker-up.ps1 AS A FILE (below), that specific file load IS checked,
# and fails on the still-common default (Restricted) with "cannot be loaded
# because running scripts is disabled on this system". Loosen it for just
# this process (no admin rights needed, does not touch the machine/user-wide
# policy, reverts automatically when this PowerShell process exits).
try { Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass -Force } catch {}

function Say($m) { Write-Host "==> $m" -ForegroundColor Cyan }
function Warn($m) { Write-Host "[!] $m" -ForegroundColor Yellow }

# Some launch paths (a desktop shortcut, "Win+R" -> powershell -Command "...",
# a batch file) run this in a PowerShell window that closes itself the instant
# the command ends - success OR failure - which was hiding every error behind
# a window that vanished before it could be read. `throw` alone (as opposed to
# `exit`) keeps the window open ONLY when this is `iex`'d into an already-
# interactive session; it does nothing for a window with no prompt loop under
# it. Pausing explicitly is the only thing that works in both cases.
# Guarded by an env var so a failure inside docker-up.ps1 (which pauses in
# its own Die() before rethrowing) doesn't prompt twice for the same error.
function Wait-BeforeClose {
  if ($env:DOTINSCHOOL_PAUSED_ONCE) { return }
  $env:DOTINSCHOOL_PAUSED_ONCE = '1'
  try { Write-Host ""; Read-Host "Press Enter to close this window" | Out-Null } catch {}
}
function Die($m) {
  Write-Host "[x] $m" -ForegroundColor Red
  Wait-BeforeClose
  throw $m
}

$Branch     = 'main'
$InstallDir = Join-Path $env:USERPROFILE 'dotinschool'

# On some machines, plain `& git.exe ...` does NOT block the way it normally
# does - git spins up in its own separate console window and this script
# races ahead to the next step (checking the checkout, or even starting a
# SECOND git call) before that window's clone/fetch has actually finished.
# That produces two symptoms seen together in practice: this script gives up
# ("could not get a complete copy") while the OTHER window is still visibly
# transferring, and a second concurrent git call against the same folder
# fails with "Unable to create '.git/shallow.lock': File exists." (a second
# process colliding with the first one still running).
#
# Start-Process -NoNewWindow -Wait is the actual fix, not a workaround: it
# forces the child process to share this console (no window of its own) and
# guarantees this script does not continue until it has genuinely exited.
function Invoke-Git {
  param([Parameter(Mandatory)][string[]]$GitArgs)
  $p = Start-Process -FilePath $git -ArgumentList $GitArgs -NoNewWindow -Wait -PassThru
  return $p.ExitCode
}

# Clears a lock file left behind by an earlier git process that either got
# interrupted mid-operation or - see above - was still running when this
# script wrongly moved on without it. Safe to remove now that Invoke-Git
# guarantees this script's OWN git calls never overlap with each other.
function Clear-StaleGitLocks {
  param([string]$Dir)
  foreach ($name in @('index.lock', 'shallow.lock', 'HEAD.lock', 'config.lock')) {
    $p = Join-Path $Dir (Join-Path '.git' $name)
    if (Test-Path $p) {
      Warn "removing a leftover git lock file from an earlier attempt: $name"
      Remove-Item $p -Force -ErrorAction SilentlyContinue
    }
  }
}

# Prompts once (hidden input), if nothing else already supplies a token.
function Ensure-GhToken {
  if ($env:DOTINSCHOOL_GH_TOKEN) { return }
  if (Test-Path (Join-Path $InstallDir '.git')) { return }
  $sec = Read-Host "GitHub token (repo is private - https://github.com/settings/personal-access-tokens/new)" -AsSecureString
  $bstr = [Runtime.InteropServices.Marshal]::SecureStringToBSTR($sec)
  try { $env:DOTINSCHOOL_GH_TOKEN = [Runtime.InteropServices.Marshal]::PtrToStringAuto($bstr) }
  finally { [Runtime.InteropServices.Marshal]::ZeroFreeBSTR($bstr) }
}

# Prefer an explicitly-passed token, otherwise reuse the one already embedded
# in the existing clone's git remote (set on first install). Never print the
# URL built from this - it carries the token.
function Get-RepoUrl {
  if (Test-Path (Join-Path $InstallDir '.git')) {
    $existing = & $git -C $InstallDir remote get-url origin 2>$null
    if ($existing -match '^https://([^@]+)@github\.com/') { return $existing }
  }
  if ($env:DOTINSCHOOL_GH_TOKEN) {
    return "https://$($env:DOTINSCHOOL_GH_TOKEN)@github.com/ariasam2017/dotinschool.git"
  }
  return 'https://github.com/ariasam2017/dotinschool.git'
}

# ---- 1. git: locate it, or install it via winget ----
function Find-Git {
  $cmd = Get-Command git -ErrorAction SilentlyContinue
  if ($cmd) { return $cmd.Source }
  $cand = Join-Path $env:ProgramFiles 'Git\cmd\git.exe'
  if (Test-Path $cand) { return $cand }
  return $null
}

$git = Find-Git
if (-not $git) {
  Say "git not found - installing via winget"
  if (-not (Get-Command winget -ErrorAction SilentlyContinue)) {
    Die "winget is not available. Install Git from https://git-scm.com/download/win and re-run this command."
  }
  winget install -e --id Git.Git --accept-package-agreements --accept-source-agreements --silent
  if ($LASTEXITCODE -ne 0) { Die "winget could not install Git. Install it manually and re-run this command." }
  $git = Find-Git
  if (-not $git) { Die "Git was installed but could not be located. Open a new terminal and re-run this command." }
}

# A file that can only exist once the working tree is actually fully checked
# out - used below to tell a genuinely complete clone/update apart from one
# that got cut short and left a `.git` folder that LOOKS present but isn't
# usable.
$MarkerFile = 'docker-up.ps1'

function Checkout-Complete {
  Test-Path (Join-Path $InstallDir $MarkerFile)
}

# ---- 2. fetch or update the source from GitHub, verified, with retries ----
# A `git fetch`/`reset --hard` that fails partway through does not always
# exit non-zero in a way that stops the script here (e.g. against a `.git`
# left behind by an earlier interrupted attempt) - so completeness is
# verified explicitly afterwards rather than trusted from exit codes alone.
Ensure-GhToken
$maxAttempts = 3
$attempt = 0
$fetched = $false
while (-not $fetched -and $attempt -lt $maxAttempts) {
  $attempt++
  if (Test-Path (Join-Path $InstallDir '.git')) {
    Clear-StaleGitLocks $InstallDir
    Say "updating existing checkout at $InstallDir (attempt $attempt of $maxAttempts)"
    $fetchOk = ((Invoke-Git -GitArgs @('-C', $InstallDir, 'fetch', '--depth', '1', 'origin', $Branch)) -eq 0)
    if ($fetchOk) {
      $fetchOk = ((Invoke-Git -GitArgs @('-C', $InstallDir, 'reset', '--hard', "origin/$Branch")) -eq 0)
    }
  } else {
    $repoUrl = Get-RepoUrl
    if (-not $env:DOTINSCHOOL_GH_TOKEN -and $repoUrl -notlike '*@github.com*') {
      Die "No GitHub token found (`$env:DOTINSCHOOL_GH_TOKEN) - this will fail, the repo is private. See the header of this script."
    }
    Say "cloning dotinschool into $InstallDir (attempt $attempt of $maxAttempts)"
    $fetchOk = ((Invoke-Git -GitArgs @('clone', '--depth', '1', '--branch', $Branch, $repoUrl, $InstallDir)) -eq 0)
  }

  if ($fetchOk -and (Checkout-Complete)) {
    $fetched = $true
  } else {
    Warn "checkout looks incomplete or failed (this can happen on a slow or filtered connection)"
    if ($attempt -lt $maxAttempts) {
      Warn "removing the partial checkout and retrying..."
      if (Test-Path $InstallDir) { Remove-Item $InstallDir -Recurse -Force -ErrorAction SilentlyContinue }
      Start-Sleep -Seconds 3
    }
  }
}

if (-not $fetched) {
  Die @"
Could not get a complete copy of the source from GitHub after $maxAttempts attempts.
This usually means the connection to GitHub is slow or blocked on this network. Two things to try:
  1) Cloudflare WARP for Windows (free): install from https://1.1.1.1/, turn it on, then re-run this command.
  2) Just re-run this command again in a few minutes - transient network issues are the most common cause.
"@
}

# ---- 3. build + run via Docker ----
# docker-up.ps1's own Die() throws rather than exits, so a failure there
# propagates up as a terminating error instead of just setting $LASTEXITCODE -
# caught here so this window still pauses instead of vanishing with it.
Say "handing off to docker-up.ps1"
$dockerUp = Join-Path $InstallDir 'docker-up.ps1'
try {
  & $dockerUp
  if ($LASTEXITCODE -ne 0) { throw "docker-up.ps1 exited with code $LASTEXITCODE" }
} catch {
  Write-Host "[x] $($_.Exception.Message)" -ForegroundColor Red
  Wait-BeforeClose
  throw
}
# Success: docker-up.ps1 already printed the panel URL + admin credentials -
# pause here too so a window that would otherwise close itself doesn't take
# that information down with it before it's been read.
Wait-BeforeClose
