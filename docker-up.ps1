# dotinschool - bring the whole stack up on Docker with ONE command.
#
#   powershell -ExecutionPolicy Bypass -File .\docker-up.ps1
#   (or just:  .\docker-up.ps1 )
#
# Installs Go via a direct download from go.dev/dl if it's missing (no
# winget - its GoLang.Go manifest has been known to point at a version
# whose installer 404s; a plain zip from the official source doesn't have
# that failure mode and doesn't need an installer/admin rights at all).
# Docker Desktop is NOT auto-installed - its installer is interactive and
# can require a reboot for the WSL2 kernel update, so if it's missing this
# script just prints where to get it and stops; re-run after installing it
# yourself. Once both are present, compiles the Linux binary on this
# Windows host (fast, reliable), packages it into a tiny image (a ~5s file
# copy - NO in-container `go build`, so the "rpc error ... EOF" build
# failure cannot happen), and starts the stack. Re-run any time to rebuild
# + recreate.
#
# ASCII-only on purpose: Windows PowerShell 5.1 mis-reads non-ASCII script files
# unless they carry a UTF-8 BOM. Access info (URL + credentials) is printed at
# the end regardless.

$ErrorActionPreference = 'Stop'
Set-Location -Path $PSScriptRoot

function Say($m) { Write-Host "==> $m" -ForegroundColor Cyan }
function Warn($m) { Write-Host "[!] $m" -ForegroundColor Yellow }
# `exit` would close the whole PowerShell window when this script is called
# with `&` from within another script/session (as windows-install.ps1 does,
# and as `.\docker-up.ps1` typed directly at a prompt also does) rather than
# launched as its own process - `throw` stops the script but leaves the
# window open so the error above stays readable.
#
# That said, if THIS script itself is what a shortcut/Run-dialog launched
# directly (no windows-install.ps1 wrapper catching the throw), the window
# still closes the instant the process ends either way - so pause here too.
# Guarded by an env var (not a script-scope flag) because this can be called
# from this script directly OR after it's been invoked with `&` from
# windows-install.ps1's own catch block - both share the same process, so this
# still only prompts once instead of twice for the same failure.
function Wait-BeforeClose {
  if ($env:DOTINSCHOOL_PAUSED_ONCE) { return }
  $env:DOTINSCHOOL_PAUSED_ONCE = '1'
  try { Write-Host ""; Read-Host "Press Enter to close this window" | Out-Null } catch {}
}
function Die($m) { Write-Host "[x] $m" -ForegroundColor Red; Wait-BeforeClose; throw $m }

# ---- 1. Go: locate it, or install it from the official zip ----
# No version is hardcoded: go.dev sometimes lists a release as stable in its
# metadata a little before every platform's archive has finished uploading
# (this is what actually happened - go1.27.1's own metadata was there, but
# its windows-amd64.zip 404'd). Ask go.dev for its real release list instead
# and try the few most recent stable ones in order, oldest fallback last.
$GoVersionFallback = '1.27.1'
$GoRoot = Join-Path $env:LOCALAPPDATA 'dotinschool\go'

function Find-Go {
  $cmd = Get-Command go -ErrorAction SilentlyContinue
  if ($cmd) { return $cmd.Source }
  foreach ($p in @(
    (Join-Path $GoRoot 'bin\go.exe'),
    (Join-Path $env:ProgramFiles 'Go\bin\go.exe'),
    (Join-Path $env:LOCALAPPDATA 'Programs\go\bin\go.exe'),
    'C:\Go\bin\go.exe'
  )) { if (Test-Path $p) { return $p } }
  return $null
}

function Expand-GoZip($zipPath) {
  $dest = Split-Path $GoRoot
  New-Item -ItemType Directory -Force -Path $dest | Out-Null
  if (Test-Path $GoRoot) { Remove-Item $GoRoot -Recurse -Force }
  Expand-Archive -Path $zipPath -DestinationPath $dest -Force
}

function Install-Go {
  # Already downloaded by hand (e.g. via a browser, when this network blocks
  # dl.google.com for PowerShell but not for the browser)? Use that instead
  # of touching the network at all.
  $manual = @(Join-Path $PSScriptRoot 'go-windows-amd64.zip')
  if (Test-Path (Join-Path $env:USERPROFILE 'Downloads')) {
    $manual += Get-ChildItem (Join-Path $env:USERPROFILE 'Downloads') -Filter 'go*.windows-amd64.zip' -ErrorAction SilentlyContinue |
      Sort-Object LastWriteTime -Descending | ForEach-Object { $_.FullName }
  }
  $manual = $manual | Where-Object { Test-Path $_ } | Select-Object -First 1
  if ($manual) {
    Say "found a manually-downloaded Go zip: $manual"
    Expand-GoZip $manual
    return
  }

  # Windows PowerShell 5.1 sometimes defaults to an older TLS version than
  # go.dev/dl.google.com require, which fails silently as a generic
  # connection error - force TLS 1.2 (and 1.3 where supported) up front.
  try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls12
  } catch {}
  try {
    [Net.ServicePointManager]::SecurityProtocol = [Net.ServicePointManager]::SecurityProtocol -bor [Net.SecurityProtocolType]::Tls13
  } catch {}

  $candidates = @()
  try {
    $releases = Invoke-RestMethod -Uri 'https://go.dev/dl/?mode=json&include=all' -UseBasicParsing -TimeoutSec 30
    foreach ($rel in ($releases | Where-Object { $_.stable } | Select-Object -First 5)) {
      $file = $rel.files | Where-Object { $_.os -eq 'windows' -and $_.arch -eq 'amd64' -and $_.kind -eq 'archive' } | Select-Object -First 1
      if ($file) { $candidates += $file.filename }
    }
  } catch {
    Warn "could not look up Go releases online ($($_.Exception.Message))"
  }
  if ($candidates.Count -eq 0) { $candidates = @("go$GoVersionFallback.windows-amd64.zip") }
  Say "trying Go release(s): $($candidates -join ', ')"

  $ok = $false
  $tmp = $null
  foreach ($zipName in $candidates) {
    foreach ($mirrorBase in @('https://go.dev/dl', 'https://golang.google.cn/dl')) {
      $url = "$mirrorBase/$zipName"
      $tmp = Join-Path $env:TEMP $zipName
      try {
        Invoke-WebRequest -Uri $url -OutFile $tmp -UseBasicParsing -TimeoutSec 60
        if ((Test-Path $tmp) -and ((Get-Item $tmp).Length -gt 0)) { $ok = $true; break }
      } catch {
        Warn "download from $url failed: $($_.Exception.Message)"
      }
    }
    if ($ok) { break }
  }
  if (-not $ok) {
    Die @"
Could not download Go from go.dev or its China mirror after trying $($candidates.Count) release(s) (see the reason(s) above - repeated 404/403 on every version usually means this network blocks Google's download servers, the same way it may have blocked GitHub before). Two ways around it:
  1) Cloudflare WARP for Windows (free, same fix that unblocked GitHub on the Linux server): install from https://1.1.1.1/, turn it on, then re-run this script.
  2) Download the zip by hand (a browser sometimes gets through when PowerShell doesn't): https://go.dev/dl/ -> grab the "Windows" .zip (not the .msi) -> save it into your Downloads folder -> re-run this script, it will find and use it automatically.
"@
  }
  Expand-GoZip $tmp
  Remove-Item $tmp -Force
}

$go = Find-Go
if (-not $go) {
  Warn "Go not found"
  Install-Go
  $go = Find-Go
  if (-not $go) { Die "Go was installed but could not be located." }
}
Say "using Go: $go"

# ---- 2. Docker: must already be installed - this script does not install it ----
function Docker-On-Path { [bool](Get-Command docker -ErrorAction SilentlyContinue) }
function Docker-Ready {
  if (-not (Docker-On-Path)) { return $false }
  docker info *> $null
  return ($LASTEXITCODE -eq 0)
}

if (-not (Docker-On-Path)) {
  Die "Docker Desktop for Windows is not installed. Please install it from https://www.docker.com/products/docker-desktop/ and then continue with the steps (re-run this script)."
}

if (-not (Docker-Ready)) {
  Say "starting Docker Desktop"
  $dockerExe = Join-Path $env:ProgramFiles 'Docker\Docker\Docker Desktop.exe'
  if (Test-Path $dockerExe) { Start-Process $dockerExe }
  $tries = 0
  while (-not (Docker-Ready) -and $tries -lt 60) { Start-Sleep -Seconds 3; $tries++ }
  if (-not (Docker-Ready)) {
    Die "Docker Desktop did not become ready in time. Open it manually, wait for the whale icon to say it's running, then re-run this script."
  }
}
Say "Docker is ready"

# ---- 2b. install the global "dotinschool" command (Windows counterpart of the
# Linux Docker install's CLI wrapper) - a thin .cmd shim in
# %LOCALAPPDATA%\Microsoft\WindowsApps (already on every user's PATH, no admin
# rights, works in already-open shells too since only the PATH *variable* is
# cached, not directory listings) that dispatches into this checkout's own
# scripts\dotinschool-cli.ps1 - so it always runs the current version, no
# separate re-install step after "dotinschool update".
function Install-DotinschoolCli {
  try {
    $aliasDir = Join-Path $env:LOCALAPPDATA 'Microsoft\WindowsApps'
    if (-not (Test-Path $aliasDir)) { return }
    $cliScript = Join-Path $InstallDir 'scripts\dotinschool-cli.ps1'
    if (-not (Test-Path $cliScript)) { return }
    $shimPath = Join-Path $aliasDir 'dotinschool.cmd'
    $shimContent = "@echo off`r`npowershell -NoProfile -ExecutionPolicy Bypass -File `"$cliScript`" %*`r`n"
    $existing = if (Test-Path $shimPath) { Get-Content $shimPath -Raw } else { '' }
    if ($existing -ne $shimContent) {
      Set-Content -Path $shimPath -Value $shimContent -Encoding ascii -NoNewline
      Say "installed the 'dotinschool' command (try: dotinschool)"
    }
  } catch {
    # Never fail the whole run over this - it's a convenience, not a requirement.
    Warn "could not install the 'dotinschool' command ($($_.Exception.Message)) - use .\docker-up.ps1 / .\update.ps1 directly instead"
  }
}
$InstallDir = $PSScriptRoot
Install-DotinschoolCli

# ---- 3. .env (create on first run with a random admin password + secret path) ----
if (-not (Test-Path .env)) {
  Say "creating .env (random admin password + secret admin path)"
  $chars = '0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ'
  function Rand($n) { -join ((1..$n) | ForEach-Object { $chars[(Get-Random -Maximum $chars.Length)] }) }
  @(
    "APP_PORT=8080",
    "ADMIN_USERNAME=administrator",
    ("ADMIN_PASSWORD=" + (Rand 20)),
    ("ADMIN_PATH=panel-" + (Rand 8)),
    "ADMIN_SUBDOMAIN=",
    "DOMAIN=",
    "# IPPANEL_API_KEY=",
    "# IPPANEL_PATTERN_CODE=",
    "# IPPANEL_LOGIN_PATTERN_CODE=",
    "# IPPANEL_ADMIN_NOTIFY_PATTERN_CODE=",
    "# IPPANEL_ADMIN_PHONE=",
    "# AI_API_KEY=",
    "# AI_BASE_URL=https://api.openai.com/v1",
    "# AI_MODEL=gpt-4o-mini"
  ) | Set-Content -Encoding ascii .env
}

# ---- 4. pick a free host port (passed to compose as an env var, .env is not touched) ----
function PortFree($p) {
  try {
    $l = New-Object System.Net.Sockets.TcpListener([System.Net.IPAddress]::Loopback, $p)
    $l.Start(); $l.Stop(); return $true
  } catch { return $false }
}
$port = $null
foreach ($p in 8080, 8090, 8091, 8092, 8093, 3000, 9000) { if (PortFree $p) { $port = $p; break } }
if (-not $port) { $port = 8080 }
$env:APP_PORT = "$port"
Say "host port: $port"

# ---- 5. compile the Linux binary on the host ----
Say "compiling the linux binary on the host (first run may take ~1 min)"
New-Item -ItemType Directory -Force -Path dist | Out-Null
$env:GOOS = 'linux'; $env:GOARCH = 'amd64'; $env:CGO_ENABLED = '0'
try {
  & $go build -trimpath -ldflags='-s -w' -o dist/dotinschool_linux_amd64 ./cmd/server
} finally {
  Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
}
if (-not (Test-Path dist/dotinschool_linux_amd64)) { Die "go build failed." }

# ---- 6. build image + start stack ----
# This Dockerfile only copies the already-compiled binary in (no `go build`
# here), so it should never fail on memory - but Docker Desktop's BuildKit
# backend can still drop its own internal grpc connection mid-build on some
# machines ("rpc error: code = Unavailable ... EOF"), even on a trivial RUN
# step. That's a transient BuildKit hiccup, not a problem with this
# Dockerfile - retry a couple of times, then fall back to the classic
# (non-BuildKit) builder, which doesn't use that grpc protocol at all and so
# can't hit this specific failure.
Say "building the Docker image (packaging - a few seconds)"
function Build-Image([bool]$legacyBuilder) {
  if ($legacyBuilder) { $env:DOCKER_BUILDKIT = '0' } else { Remove-Item Env:DOCKER_BUILDKIT -ErrorAction SilentlyContinue }
  # Piped through Out-Host (not left bare): a PowerShell function's return
  # value is EVERY unsuppressed pipeline object written during it, not just
  # the explicit `return` - so `docker build`'s own console output would
  # otherwise get bundled into what this function "returns", and a non-empty
  # array is always truthy regardless of $ok, silently defeating the retry
  # check below on the very first (failing) attempt.
  docker build -f build/Dockerfile -t dotinschool:local . | Out-Host
  $ok = ($LASTEXITCODE -eq 0)
  Remove-Item Env:DOCKER_BUILDKIT -ErrorAction SilentlyContinue
  return $ok
}
$maxAttempts = 3
$built = $false
for ($attempt = 1; $attempt -le $maxAttempts; $attempt++) {
  if ($attempt -gt 1) {
    Warn "docker build failed (attempt $($attempt - 1) of $maxAttempts) - retrying$(if ($attempt -eq $maxAttempts) { ' with the classic (non-BuildKit) builder' })..."
    Start-Sleep -Seconds 3
  }
  if (Build-Image ($attempt -eq $maxAttempts)) { $built = $true; break }
}
if (-not $built) {
  Die "docker build failed after $maxAttempts attempts (BuildKit and the classic builder). This looks like a genuine Docker Desktop problem, not this project's Dockerfile - restart Docker Desktop (or reboot), then re-run this script. If it keeps happening, raise Docker Desktop's memory limit in Settings -> Resources."
}
Say "starting the stack"
docker compose -f build/docker-compose.yml up -d
if ($LASTEXITCODE -ne 0) { Die "docker compose up failed." }

# ---- 7. access info ----
$cfg = @{}
Get-Content .env | Where-Object { $_ -match '^\s*[^#].*=' } | ForEach-Object {
  $kv = $_ -split '=', 2; $cfg[$kv[0].Trim()] = $kv[1].Trim()
}
Start-Sleep -Seconds 3
Write-Host ""
Write-Host "----------------------------------------------" -ForegroundColor Green
Write-Host (" user panel :  http://localhost:{0}/" -f $port) -ForegroundColor Green
Write-Host (" admin panel:  http://localhost:{0}/{1}/" -f $port, $cfg['ADMIN_PATH']) -ForegroundColor Green
Write-Host (" username   :  {0}" -f $cfg['ADMIN_USERNAME']) -ForegroundColor Green
Write-Host (" password   :  {0}" -f $cfg['ADMIN_PASSWORD']) -ForegroundColor Green
Write-Host "----------------------------------------------" -ForegroundColor Green
Write-Host " logs : docker compose -f build/docker-compose.yml logs -f app"
Write-Host " stop : docker compose -f build/docker-compose.yml down"
if (Get-Command dotinschool -ErrorAction SilentlyContinue) {
  Write-Host " menu : dotinschool          (also: up / down / restart / logs / ps / rebuild / update / admin)"
}
