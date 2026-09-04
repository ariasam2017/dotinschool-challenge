# dotinschool - Windows installer.
#
# The dotinschool repo is private, so fetching this file and the git clone it
# runs both need a GitHub token: a fine-grained personal access token, scoped
# to just this repository with "Contents: Read-only", is enough. Create one at
# https://github.com/settings/personal-access-tokens/new, then install with:
#
#   $env:DOTINSCHOOL_GH_TOKEN = '<your token>'
#   $h = @{ Authorization = "token $env:DOTINSCHOOL_GH_TOKEN" }
#   irm -Headers $h https://raw.githubusercontent.com/ariasam2017/dotinschool/main/scripts/windows-install.ps1 | iex
#
# The token is only needed for this first run: it gets embedded in the local
# clone's git remote, so a later `.\docker-up.ps1` or re-run of this same
# command line keeps working without it.
#
# Fetches the latest dotinschool source from GitHub into %USERPROFILE%\dotinschool
# (or updates it if already there), then hands off to docker-up.ps1, which
# installs Go and Docker Desktop if missing, compiles the Linux binary on this
# host, builds the image, and starts the stack.
#
# ASCII-only on purpose: Windows PowerShell 5.1 mis-reads non-ASCII script files
# unless they carry a UTF-8 BOM, and `irm | iex` has no file of its own to tag
# with one.

$ErrorActionPreference = 'Stop'

function Say($m) { Write-Host "==> $m" -ForegroundColor Cyan }
function Die($m) { Write-Host "[x] $m" -ForegroundColor Red; exit 1 }

$Branch     = 'main'
$InstallDir = Join-Path $env:USERPROFILE 'dotinschool'

# Prefer an explicitly-passed token, otherwise reuse the one already embedded
# in the existing clone's git remote (set on first install). Never print the
# URL built from this - it carries the token.
function Get-RepoUrl($git) {
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

# ---- 2. fetch or update the source from GitHub ----
if (Test-Path (Join-Path $InstallDir '.git')) {
  Say "updating existing checkout at $InstallDir"
  & $git -C $InstallDir fetch --depth 1 origin $Branch
  & $git -C $InstallDir reset --hard "origin/$Branch"
} else {
  $repoUrl = Get-RepoUrl $git
  if (-not $env:DOTINSCHOOL_GH_TOKEN -and $repoUrl -notlike '*@github.com*') {
    Die "No GitHub token found (`$env:DOTINSCHOOL_GH_TOKEN) - this will fail, the repo is private. See the header of this script."
  }
  Say "cloning dotinschool into $InstallDir"
  & $git clone --depth 1 --branch $Branch $repoUrl $InstallDir
}
if ($LASTEXITCODE -ne 0) { Die "could not fetch the source from GitHub." }

# ---- 3. build + run via Docker ----
Say "handing off to docker-up.ps1"
& (Join-Path $InstallDir 'docker-up.ps1')
