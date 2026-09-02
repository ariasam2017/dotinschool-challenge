# dotinschool - bring the whole stack up on Docker with ONE command.
#
#   powershell -ExecutionPolicy Bypass -File .\docker-up.ps1
#   (or just:  .\docker-up.ps1 )
#
# Compiles the Linux binary on this Windows host (fast, reliable), packages it
# into a tiny image (a ~5s file copy - NO in-container `go build`, so the
# "rpc error ... EOF" build failure cannot happen), and starts the stack.
# Re-run any time to rebuild + recreate.
#
# ASCII-only on purpose: Windows PowerShell 5.1 mis-reads non-ASCII script files
# unless they carry a UTF-8 BOM. Access info (URL + credentials) is printed at
# the end regardless.

$ErrorActionPreference = 'Stop'
Set-Location -Path $PSScriptRoot

function Say($m) { Write-Host "==> $m" -ForegroundColor Cyan }
function Die($m) { Write-Host "[x] $m" -ForegroundColor Red; exit 1 }

# ---- 1. tools ----
$go = (Get-Command go -ErrorAction SilentlyContinue).Source
if (-not $go) {
  $cand = Join-Path $env:LOCALAPPDATA 'Programs\go\bin\go.exe'
  if (Test-Path $cand) { $go = $cand } else { Die "Go not found. Install it or add it to PATH." }
}
if (-not (Get-Command docker -ErrorAction SilentlyContinue)) { Die "docker not found. Install Docker Desktop." }
docker info 2>&1 | Out-Null
if ($LASTEXITCODE -ne 0) { Die "Docker is not running. Open Docker Desktop and wait for it to be ready." }

# ---- 2. .env (create on first run with a random admin password + secret path) ----
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

# ---- 3. pick a free host port (passed to compose as an env var, .env is not touched) ----
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

# ---- 4. compile the Linux binary on the host ----
Say "compiling the linux binary on the host (first run may take ~1 min)"
New-Item -ItemType Directory -Force -Path dist | Out-Null
$env:GOOS = 'linux'; $env:GOARCH = 'amd64'; $env:CGO_ENABLED = '0'
try {
  & $go build -trimpath -ldflags='-s -w' -o dist/dotinschool_linux_amd64 ./cmd/server
} finally {
  Remove-Item Env:GOOS, Env:GOARCH, Env:CGO_ENABLED -ErrorAction SilentlyContinue
}
if (-not (Test-Path dist/dotinschool_linux_amd64)) { Die "go build failed." }

# ---- 5. build image + start stack ----
Say "building the Docker image (packaging - a few seconds)"
docker build -f build/Dockerfile -t dotinschool:local .
if ($LASTEXITCODE -ne 0) { Die "docker build failed." }
Say "starting the stack"
docker compose -f build/docker-compose.yml up -d
if ($LASTEXITCODE -ne 0) { Die "docker compose up failed." }

# ---- 6. access info ----
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
