# dotinschool - Windows installer.
#
#   irm https://raw.githubusercontent.com/ariasam2017/dotinschool/main/scripts/windows-install.ps1 | iex
#
# Fetches the latest dotinschool source from GitHub into %USERPROFILE%\dotinschool
# (or updates it if already there), then hands off to docker-up.ps1, which
# installs Go and Docker Desktop if missing, compiles the Linux binary on this
# host, builds the image, and starts the stack. Re-run the same command any
# time to pull the latest version and rebuild.
#
# ASCII-only on purpose: Windows PowerShell 5.1 mis-reads non-ASCII script files
# unless they carry a UTF-8 BOM, and `irm | iex` has no file of its own to tag
# with one.

$ErrorActionPreference = 'Stop'

function Say($m) { Write-Host "==> $m" -ForegroundColor Cyan }
function Die($m) { Write-Host "[x] $m" -ForegroundColor Red; exit 1 }

$RepoUrl    = 'https://github.com/ariasam2017/dotinschool.git'
$Branch     = 'main'
$InstallDir = Join-Path $env:USERPROFILE 'dotinschool'

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

# ---- 2. fetch or update the source from GitHub ----
if (Test-Path (Join-Path $InstallDir '.git')) {
  Say "updating existing checkout at $InstallDir"
  & $git -C $InstallDir fetch --depth 1 origin $Branch
  & $git -C $InstallDir reset --hard "origin/$Branch"
} else {
  Say "cloning dotinschool into $InstallDir"
  & $git clone --depth 1 --branch $Branch $RepoUrl $InstallDir
}
if ($LASTEXITCODE -ne 0) { Die "could not fetch the source from GitHub." }

# ---- 3. build + run via Docker ----
Say "handing off to docker-up.ps1"
& (Join-Path $InstallDir 'docker-up.ps1')
