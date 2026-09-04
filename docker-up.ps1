# dotinschool - bring the whole stack up on Docker with ONE command.
#
#   powershell -ExecutionPolicy Bypass -File .\docker-up.ps1
#   (or just:  .\docker-up.ps1 )
#
# Installs Go via winget if it's missing (no prompts). Docker Desktop is
# NOT auto-installed - its installer is interactive and can require a
# reboot for the WSL2 kernel update, so if it's missing this script just
# prints where to get it and stops; re-run after installing it yourself.
# Once both are present, compiles the Linux binary on this Windows host
# (fast, reliable), packages it into a tiny image (a ~5s file copy - NO
# in-container `go build`, so the "rpc error ... EOF" build failure cannot
# happen), and starts the stack. Re-run any time to rebuild + recreate.
#
# ASCII-only on purpose: Windows PowerShell 5.1 mis-reads non-ASCII script files
# unless they carry a UTF-8 BOM. Access info (URL + credentials) is printed at
# the end regardless.

$ErrorActionPreference = 'Stop'
Set-Location -Path $PSScriptRoot

function Say($m) { Write-Host "==> $m" -ForegroundColor Cyan }
function Warn($m) { Write-Host "[!] $m" -ForegroundColor Yellow }
function Die($m) { Write-Host "[x] $m" -ForegroundColor Red; exit 1 }

function Have-Winget { [bool](Get-Command winget -ErrorAction SilentlyContinue) }

function Winget-Install($id, $label) {
  if (-not (Have-Winget)) {
    Die "$label is missing and winget is not available on this system. Install $label manually and re-run this script."
  }
  Say "installing $label via winget (id: $id)"
  winget install -e --id $id --accept-package-agreements --accept-source-agreements --silent
  if ($LASTEXITCODE -ne 0) {
    Die "winget could not install $label. Install it manually and re-run this script."
  }
}

# ---- 1. Go: locate it, or install it ----
function Find-Go {
  $cmd = Get-Command go -ErrorAction SilentlyContinue
  if ($cmd) { return $cmd.Source }
  foreach ($p in @(
    (Join-Path $env:ProgramFiles 'Go\bin\go.exe'),
    (Join-Path $env:LOCALAPPDATA 'Programs\go\bin\go.exe'),
    'C:\Go\bin\go.exe'
  )) { if (Test-Path $p) { return $p } }
  return $null
}

$go = Find-Go
if (-not $go) {
  Warn "Go not found"
  Winget-Install 'GoLang.Go' 'Go'
  $go = Find-Go
  if (-not $go) { Die "Go was installed but could not be located. Open a new terminal (so PATH refreshes) and re-run this script." }
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
Say "building the Docker image (packaging - a few seconds)"
docker build -f build/Dockerfile -t dotinschool:local .
if ($LASTEXITCODE -ne 0) { Die "docker build failed." }
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
