#!/usr/bin/env bash
# =============================================================================
# dotinschool — Docker installer for a Linux host.
#
# The dotinschool repo is private, so getting the source needs a GitHub
# token: a fine-grained personal access token, scoped to just this repository
# with "Contents: Read-only", is enough. Create one at
# https://github.com/settings/personal-access-tokens/new, then run this —
# no token in the command itself, it asks for it interactively:
#
#   bash -c 'read -rsp "GitHub token: " T; echo; DOTINSCHOOL_GH_TOKEN="$T" bash <(curl -Ls -H "Authorization: token $T" https://raw.githubusercontent.com/ariasam2017/dotinschool/main/build/docker-install.sh)'
#
# The token is only needed for that first run: it gets embedded in the
# /opt/dotinschool clone's git remote, so later updates (this script, or
# `dotinschool update`) keep working without it.
#
# Installs Docker + Go if missing (auto — no prompts), clones the repo to
# /opt/dotinschool, generates a .env with a random admin password + secret
# admin path, builds the image the RELIABLE way (host-compile the binary,
# then package it — never an in-container `go build`), brings the stack up,
# and installs a wrapper command:
#
#   dotinschool                 interactive menu (free-port finder, HTTPS cert, ...)
#   dotinschool up | down | restart | logs | update | admin | ps | rebuild | port
#   dotinschool cert <domain>   get + auto-renew an HTTPS cert (Caddy/Let's Encrypt)
#   dotinschool cert off        turn HTTPS back off (plain HTTP on APP_PORT)
# =============================================================================
set -euo pipefail

BRANCH="${DOTINSCHOOL_BRANCH:-main}"
INSTALL_DIR="/opt/dotinschool"
CLI_PATH="/usr/local/bin/dotinschool"
GO_VERSION="1.27.1"  # last-resort fallback only; see resolve_go_tarballs
GO_ROOT="/usr/local/go"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; CYAN='\033[0;36m'; NC='\033[0m'
info(){ echo -e "${CYAN}[i]${NC} $*"; }
ok(){ echo -e "${GREEN}[✓]${NC} $*"; }
warn(){ echo -e "${YELLOW}[!]${NC} $*"; }
die(){ echo -e "${RED}[x]${NC} $*" >&2; exit 1; }

require_root(){ [[ ${EUID:-$(id -u)} -eq 0 ]] || die "Run as root (sudo)."; }

# Some hosts have a global/system git config with a "url.insteadOf" rewrite
# (a common convenience for people who normally use SSH keys with GitHub) that
# silently turns "https://TOKEN@github.com/..." into "git@github.com:...",
# which then fails with "Permission denied (publickey)" since this uses a
# token over HTTPS, not an SSH key. Bypass ANY such rewrite for git calls that
# touch GitHub, rather than trying to detect or explain the specific rule.
git_nc(){ GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null git "$@"; }

# The repo is private: prefer an explicitly-passed token, otherwise reuse the
# one already embedded in the existing clone's git remote (set on first
# install). Never echo the URL built from this — it carries the token.
gh_token(){
  if [[ -n "${DOTINSCHOOL_GH_TOKEN:-}" ]]; then echo "$DOTINSCHOOL_GH_TOKEN"; return; fi
  if [[ -d "$INSTALL_DIR/.git" ]]; then
    git_nc -C "$INSTALL_DIR" remote get-url origin 2>/dev/null | sed -n 's#https://\([^@]*\)@github\.com/.*#\1#p'
  fi
}

repo_url(){
  if [[ -n "${DOTINSCHOOL_REPO_URL:-}" ]]; then echo "$DOTINSCHOOL_REPO_URL"; return; fi
  local t; t=$(gh_token)
  if [[ -n "$t" ]]; then echo "https://${t}@github.com/ariasam2017/dotinschool.git"
  else echo "https://github.com/ariasam2017/dotinschool.git"; fi
}

# Prompts once, in the real shell (not a $(...) subshell, so the export
# actually sticks), if nothing else already supplies a token.
ensure_gh_token(){
  [[ -n "${DOTINSCHOOL_GH_TOKEN:-}" ]] && return
  [[ -n "${DOTINSCHOOL_REPO_URL:-}" ]] && return
  [[ -d "$INSTALL_DIR/.git" ]] && [[ -n "$(gh_token)" ]] && return
  [[ -t 0 ]] || return
  echo "This repo is private. Create a fine-grained token (Contents: Read-only) at:"
  echo "  https://github.com/settings/personal-access-tokens/new"
  read -rsp "GitHub token: " DOTINSCHOOL_GH_TOKEN; echo
  export DOTINSCHOOL_GH_TOKEN
}

install_prereqs(){
  if command -v apt-get >/dev/null 2>&1; then apt-get update -y >/dev/null; apt-get install -y curl git tar openssl ca-certificates >/dev/null
  elif command -v dnf >/dev/null 2>&1; then dnf install -y curl git tar openssl ca-certificates >/dev/null
  elif command -v yum >/dev/null 2>&1; then yum install -y curl git tar openssl ca-certificates >/dev/null
  else die "Only apt/dnf/yum are supported."; fi
  ok "Prerequisites ready."
}

install_docker(){
  if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then ok "Docker is already installed."; return; fi
  info "Installing Docker..."
  # get.docker.com first; if it (or download.docker.com behind it) is
  # unreachable - common on filtered networks - fall back to the distro's own
  # docker.io + compose plugin, which come from the already-reachable OS repo.
  if ! curl -fsSL --connect-timeout 20 https://get.docker.com 2>/dev/null | sh >/dev/null 2>&1 \
     || ! command -v docker >/dev/null 2>&1; then
    warn "get.docker.com did not work - installing docker from the OS package repo instead."
    if command -v apt-get >/dev/null 2>&1; then
      apt-get update -y >/dev/null 2>&1 || true
      apt-get install -y docker.io docker-compose-v2 >/dev/null 2>&1 \
        || apt-get install -y docker.io docker-compose-plugin >/dev/null 2>&1 || true
    elif command -v dnf >/dev/null 2>&1; then
      dnf install -y docker docker-compose-plugin >/dev/null 2>&1 || true
    elif command -v yum >/dev/null 2>&1; then
      yum install -y docker docker-compose-plugin >/dev/null 2>&1 || true
    fi
  fi
  systemctl enable --now docker >/dev/null 2>&1 || true
  command -v docker >/dev/null 2>&1 || die "Could not install Docker (neither get.docker.com nor the OS package worked)."
  docker compose version >/dev/null 2>&1 || die "Docker is installed but the 'docker compose' plugin is missing."
  ok "Docker installed."
  configure_docker_registry_mirror
}

# On a network that can't reach Docker Hub (registry-1.docker.io), every
# image pull - including the alpine base this project builds on - fails. If a
# reachability probe confirms that, point Docker at public mirrors so pulls
# work without any per-command proxy. Only touches daemon.json if it does not
# already exist (so an existing custom config is never clobbered).
configure_docker_registry_mirror(){
  local code
  code=$(curl -s -o /dev/null -m 10 -w '%{http_code}' https://registry-1.docker.io/v2/ 2>/dev/null || echo 000)
  if [[ "$code" != "000" ]]; then
    return  # Docker Hub is reachable - nothing to do
  fi
  if [[ -s /etc/docker/daemon.json ]]; then
    warn "Docker Hub looks unreachable and /etc/docker/daemon.json already exists - add a"
    warn "\"registry-mirrors\" entry to it yourself if image pulls fail (e.g. https://docker.arvancloud.ir)."
    return
  fi
  warn "Docker Hub (registry-1.docker.io) is unreachable - configuring registry mirrors."
  mkdir -p /etc/docker
  cat > /etc/docker/daemon.json <<'JSON'
{
  "registry-mirrors": [
    "https://docker.arvancloud.ir",
    "https://docker.iranserver.com",
    "https://registry.docker.ir",
    "https://docker.mobinhost.com"
  ]
}
JSON
  systemctl restart docker >/dev/null 2>&1 || true
  ok "Registry mirrors configured in /etc/docker/daemon.json."
}

arch_tag(){ case "$(uname -m)" in x86_64|amd64) echo amd64 ;; aarch64|arm64) echo arm64 ;; *) die "Unsupported architecture: $(uname -m)";; esac; }

# No version is hardcoded as the primary source: go.dev sometimes lists a
# release as stable in its metadata a little before every platform's archive
# has finished uploading, so a pinned version can 404. Ask go.dev's JSON
# endpoint for the actual current release names instead; GO_VERSION is only
# the last-resort fallback if that lookup itself fails.
resolve_go_tarballs(){
  local arch="$1" json u
  for u in "https://go.dev/dl/?mode=json" "https://golang.google.cn/dl/?mode=json"; do
    json=$(curl -fsSL --connect-timeout 15 "$u" 2>/dev/null) || true
    [[ -n "$json" ]] && break
  done
  [[ -n "$json" ]] && echo "$json" | grep -o "\"go[0-9.]*\.linux-${arch}\.tar\.gz\"" | tr -d '"' | awk '!seen[$0]++'
}

install_go(){
  if [[ -x "${GO_ROOT}/bin/go" ]] && "${GO_ROOT}/bin/go" version >/dev/null 2>&1; then ok "Go is already installed."; return; fi
  local arch; arch=$(arch_tag)
  info "Looking up the latest Go release..."
  local candidates=() c
  while IFS= read -r c; do [[ -n "$c" ]] && candidates+=("$c"); done < <(resolve_go_tarballs "$arch" | head -5)
  if [[ ${#candidates[@]} -eq 0 ]]; then
    warn "Could not look up Go releases online — falling back to a pinned version (${GO_VERSION})."
    candidates=("go${GO_VERSION}.linux-${arch}.tar.gz")
  fi
  info "Trying Go release(s): ${candidates[*]}"
  # aliyun / ustc / nju mirrors are the ones that tend to still work on
  # filtered networks where go.dev and dl.google.com don't.
  local tmp="" tgz base found=0
  for tgz in "${candidates[@]}"; do
    for base in \
      "https://go.dev/dl" \
      "https://mirrors.aliyun.com/golang" \
      "https://mirrors.ustc.edu.cn/golang" \
      "https://mirror.nju.edu.cn/golang" \
      "https://golang.google.cn/dl"; do
      tmp=$(mktemp)
      if curl -fsSL --connect-timeout 15 "${base}/${tgz}" -o "$tmp" && [[ -s "$tmp" ]]; then found=1; break 2; fi
      rm -f "$tmp"; tmp=""
    done
  done
  [[ $found -eq 1 ]] || die "Could not download Go from any mirror. On a filtered server, turn on a tunnel/DNS bypass and re-run, or install Go manually to ${GO_ROOT}."
  rm -rf "$GO_ROOT"; tar -C /usr/local -xzf "$tmp"; rm -f "$tmp"
  export PATH="${GO_ROOT}/bin:$PATH"
  ok "Go $("${GO_ROOT}/bin/go" version | awk '{print $3}') installed."
}

random_password(){ openssl rand -base64 18 | tr -dc 'A-Za-z0-9' | head -c 20; }
random_admin_path(){ echo "panel-$(openssl rand -hex 6)"; }

# A server already running something else (a VPN/proxy panel, another app...)
# may well have 8080 taken - this used to hardcode it anyway, which is exactly
# what caused "failed to bind host port 0.0.0.0:8080/tcp: address already in
# use" on a server with 3x-ui or similar already installed.
port_in_use(){ (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null && { exec 3<&- 2>/dev/null; exec 3>&- 2>/dev/null; return 0; }; return 1; }
find_free_port(){
  local port="${1:-8080}" tries=0
  while port_in_use "$port"; do port=$((port+1)); tries=$((tries+1)); [[ $tries -lt 100 ]] || { echo "$1"; return; }; done
  echo "$port"
}

fetch_repo(){
  if [[ -d "$INSTALL_DIR/.git" ]]; then
    git_nc -C "$INSTALL_DIR" fetch --depth 1 origin "$BRANCH" >/dev/null
    git_nc -C "$INSTALL_DIR" reset --hard "origin/${BRANCH}" >/dev/null
    ok "Source ready at ${INSTALL_DIR}."
    return
  fi

  # Running from inside an already-cloned checkout of this same repo? Reuse
  # it instead of asking for the token again (see the header comment above).
  local self_dir=""
  if [[ -n "${BASH_SOURCE[0]:-}" ]]; then
    self_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." 2>/dev/null && pwd) || self_dir=""
  fi
  if [[ -n "$self_dir" && -d "$self_dir/.git" ]]; then
    local origin; origin=$(git_nc -C "$self_dir" remote get-url origin 2>/dev/null)
    if [[ "$origin" == *"github.com/ariasam2017/dotinschool"* ]]; then
      git_nc clone --depth 1 --branch "$BRANCH" "$self_dir" "$INSTALL_DIR" >/dev/null
      git_nc -C "$INSTALL_DIR" remote set-url origin "$origin"
      ok "Source ready at ${INSTALL_DIR}."
      return
    fi
  fi

  local url; url=$(repo_url)
  [[ -n "${DOTINSCHOOL_REPO_URL:-}" || "$url" == *"@github.com"* ]] || warn "No GitHub token found (DOTINSCHOOL_GH_TOKEN) — this will fail, the repo is private."

  # `timeout` + inline env so a filtered network fails fast instead of hanging;
  # git_nc is a shell function so `timeout` can't call it directly.
  clone_try(){ rm -rf "$INSTALL_DIR"; timeout "${1}" env GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null \
      git clone --depth 1 --branch "$BRANCH" "${2}" "$INSTALL_DIR" >/dev/null 2>&1; }

  if clone_try 90 "$url"; then ok "Source ready at ${INSTALL_DIR}."; return; fi

  warn "Direct clone from GitHub failed/timed out - trying public git mirrors..."
  local m
  for m in "https://gh-proxy.com" "https://ghproxy.net" "https://ghfast.top" "https://gh.ddlc.top"; do
    info "  via ${m}"
    if clone_try 120 "${m}/${url}"; then
      git_nc -C "$INSTALL_DIR" remote set-url origin "$url"
      ok "Source ready at ${INSTALL_DIR} (via ${m})."
      return
    fi
  done

  die "Could not fetch the source from GitHub or any mirror. On a filtered server: turn on a tunnel / DNS bypass, 'git clone' the repo yourself into a folder, then run 'bash build/docker-install.sh' from INSIDE that folder - it reuses a local checkout without touching the network for the source."
}

write_env(){
  local env="${INSTALL_DIR}/.env"
  [[ -f "$env" ]] && { ok ".env already exists — left untouched."; return; }
  local port; port=$(find_free_port 8080)
  [[ "$port" == "8080" ]] || warn "Port 8080 is already in use on this server — using ${port} instead."
  cat > "$env" <<EOF
APP_PORT=${port}
DOMAIN=
ADMIN_USERNAME=administrator
ADMIN_PASSWORD=$(random_password)
ADMIN_PATH=$(random_admin_path)
ADMIN_SUBDOMAIN=
# IPPANEL_API_KEY=
# IPPANEL_PATTERN_CODE=
# IPPANEL_LOGIN_PATTERN_CODE=
# IPPANEL_ADMIN_NOTIFY_PATTERN_CODE=
# IPPANEL_ADMIN_PHONE=
# AI_API_KEY=
# AI_BASE_URL=https://api.openai.com/v1
# AI_MODEL=gpt-4o-mini
EOF
  chmod 600 "$env"
  ok ".env created — random admin password and secret path generated."
}

build_image(){
  info "Compiling the binary on the host + building the image (no go build inside the container)..."
  # goproxy.cn mirrors the module proxy AND the Go toolchain, and stays
  # reachable where proxy.golang.org / sum.golang.org don't - so the compile
  # step resolves dependencies (and any auto toolchain) on a filtered network
  # too. Harmless on an open one.
  ( cd "$INSTALL_DIR" \
    && PATH="${GO_ROOT}/bin:$PATH" \
       GOPROXY="https://goproxy.cn,https://goproxy.io,direct" \
       GOSUMDB="sum.golang.google.cn" \
       GOTOOLCHAIN="auto" \
       bash scripts/docker-build.sh "$(arch_tag)" )
  ok "Image dotinschool:local built."
}

install_cli(){
  # Written to a temp file and moved into place (atomic rename), NEVER
  # truncated in place - install_cli() can run from INSIDE an already-running
  # /usr/local/bin/dotinschool process (its own "update" action calls this
  # script with --reinstall-cli), and `cat > "$CLI_PATH"` truncating that
  # exact file WHILE it's still executing corrupts the running interpreter's
  # read of it ("unexpected EOF" mid-run, an in-place-self-overwrite hazard,
  # not something specific to this script). A rename swaps the directory
  # entry to a new inode - the still-open old one stays intact and fully
  # readable for whatever's still running from it.
  # Same directory as $CLI_PATH (not the default /tmp, which can be a
  # different filesystem) - mv is only atomic within one filesystem.
  local tmp; tmp=$(mktemp "$(dirname "$CLI_PATH")/.dotinschool.XXXXXX")
  cat > "$tmp" <<'WRAP'
#!/usr/bin/env bash
set -euo pipefail
cd /opt/dotinschool
# The app runs as a non-root user (uid 10001) inside the container, but
# ../data:/app/data is a bind mount to a HOST directory - if Docker had to
# auto-create that host directory the first time (e.g. the very first "up"
# never got far enough to succeed, such as the port-8080-in-use failure this
# was chasing), it creates it as root:root, and the container then crash-
# loops forever with "unable to open database file" (SQLITE_CANTOPEN) since
# uid 10001 can't write there. Idempotent, so safe to run on every
# invocation, not just the first.
mkdir -p data
chown -R 10001:10001 data 2>/dev/null || true
# --env-file .env (not relying on compose's own auto-discovery): compose
# resolves ${APP_PORT:-8080} in build/docker-compose.yml relative to the
# COMPOSE FILE's directory (build/), not this cd'd-to directory - so without
# this flag, APP_PORT from ./.env is silently ignored and it always falls
# back to the literal default 8080, no matter what .env actually says. This
# is exactly what made the "port" menu option (below) look like it worked
# (it wrote the new port to .env and said so) while docker still bound 8080.
# When DOMAIN is set in .env, every action needs the prod override too (it
# adds the Caddy container that terminates HTTPS) - otherwise a plain
# "restart" would quietly drop back to HTTP-only and stop Caddy.
COMPOSE_FILES=(-f build/docker-compose.yml)
if grep -qE '^DOMAIN=[^[:space:]]' .env 2>/dev/null; then
  COMPOSE_FILES+=(-f build/docker-compose.prod.yml)
fi
C=(docker compose --env-file .env "${COMPOSE_FILES[@]}")

has_domain(){ grep -qE '^DOMAIN=[^[:space:]]' .env 2>/dev/null; }
get_domain(){ grep -E '^DOMAIN=' .env 2>/dev/null | cut -d= -f2-; }

port_in_use(){ (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null && { exec 3<&- 2>/dev/null; exec 3>&- 2>/dev/null; return 0; }; return 1; }
find_free_port(){
  local port="${1:-8080}" tries=0
  while port_in_use "$port"; do port=$((port+1)); tries=$((tries+1)); [[ $tries -lt 100 ]] || { echo ""; return 1; }; done
  echo "$port"
}
show_ports(){
  echo "Ports currently listening on this server:"
  (ss -tulpn 2>/dev/null || netstat -tulpn 2>/dev/null) | awk 'NR==1 || /LISTEN|udp/'
  echo
  local cur; cur=$(grep -E '^APP_PORT=' .env 2>/dev/null | cut -d= -f2)
  echo "dotinschool is currently using port: ${cur:-unknown}"
  local suggestion; suggestion=$(find_free_port 8080)
  if [[ -n "$suggestion" ]]; then
    echo "First free port found (from 8080 up): $suggestion"
  else
    echo "Could not find a free port in the 8080-8179 range."
    return
  fi
  if [[ -t 0 && "$suggestion" != "$cur" ]]; then
    read -rp "Set APP_PORT to $suggestion and restart now? [y/N] " ans
    if [[ "$ans" == "y" || "$ans" == "Y" ]]; then
      sed -i "s|^APP_PORT=.*|APP_PORT=${suggestion}|" .env
      "${C[@]}" up -d
      echo "APP_PORT set to $suggestion - restarted."
    fi
  fi
}

server_ip(){
  local ip; ip=$(curl -fsSL -4 --connect-timeout 8 https://api.ipify.org 2>/dev/null) || true
  [[ -n "$ip" ]] || ip=$(hostname -I 2>/dev/null | awk '{print $1}') || true
  echo "${ip:-<this-server-IP>}"
}

# Point a domain's DNS A record at this server first, then: dotinschool cert
# <domain>. Writes DOMAIN into .env and brings the stack up with the Caddy
# override - Caddy then fetches and auto-renews a Let's Encrypt certificate
# on its own, no certbot/acme.sh. "dotinschool cert off" reverts to plain
# HTTP on APP_PORT.
do_cert(){
  local domain="${1:-}"
  local BASE="docker compose --env-file .env -f build/docker-compose.yml"
  local PROD="$BASE -f build/docker-compose.prod.yml"
  if [[ "$domain" == "off" ]]; then
    if grep -qE '^DOMAIN=' .env; then sed -i 's|^DOMAIN=.*|DOMAIN=|' .env; fi
    echo "HTTPS disabled - back to plain HTTP on APP_PORT."
    $PROD down --remove-orphans 2>/dev/null || true
    $BASE up -d --remove-orphans
    return
  fi
  if [[ -z "$domain" && -t 0 ]]; then
    read -rp "Domain (its DNS A record must already point at this server), e.g. panel.example.com: " domain
  fi
  [[ -n "$domain" ]] || { echo "No domain given."; return 1; }
  local blocked=0 p
  for p in 80 443; do
    if port_in_use "$p"; then echo "Port $p is already in use (another web server / panel?). Free it first."; blocked=1; fi
  done
  [[ $blocked -eq 0 ]] || return 1
  if grep -qE '^DOMAIN=' .env; then sed -i "s|^DOMAIN=.*|DOMAIN=${domain}|" .env; else echo "DOMAIN=${domain}" >> .env; fi
  echo "Bringing up with Caddy (automatic HTTPS via Let's Encrypt)..."
  # Stop the running plain (HTTP-only) stack first - it holds APP_PORT on
  # 0.0.0.0, which the prod stack would then fail to rebind (the app moves to
  # 127.0.0.1 behind Caddy) with "address already in use".
  $BASE down --remove-orphans 2>/dev/null || true
  $PROD up -d
  local apath; apath=$(grep -E '^ADMIN_PATH=' .env | cut -d= -f2-)
  echo
  echo "Done. Caddy fetches the certificate on the first HTTPS request (~30s)."
  echo "  panel : https://${domain}/${apath}/"
  echo "  logs  : dotinschool logs   (Caddy prints the cert issuance there)"
}

run_action(){
  case "$1" in
    up)      "${C[@]}" up -d ;;
    down)    "${C[@]}" down ;;
    restart) "${C[@]}" restart ;;
    logs)    "${C[@]}" logs -f ;;
    ps)      "${C[@]}" ps ;;
    port)    show_ports ;;
    cert)    do_cert "${2:-}" ;;
    rebuild) PATH="/usr/local/go/bin:$PATH" bash scripts/docker-build.sh "$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')" && "${C[@]}" up -d ;;
    update)  GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null git fetch --depth 1 origin main \
               && GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null git reset --hard origin/main \
               && bash build/docker-install.sh --reinstall-cli \
               && PATH="/usr/local/go/bin:$PATH" bash scripts/docker-build.sh "$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')" \
               && "${C[@]}" up -d ;;
    admin)
      e=.env
      apath=$(grep -E '^ADMIN_PATH=' $e | cut -d= -f2-)
      if has_domain; then
        base="https://$(get_domain)"
      else
        base="http://$(server_ip):$(grep -E '^APP_PORT=' $e | cut -d= -f2-)"
      fi
      echo "User panel:   ${base}/"
      echo "Admin panel:  ${base}/${apath}/"
      echo "Username:     $(grep -E '^ADMIN_USERNAME=' $e | cut -d= -f2-)"
      echo "Password:     $(grep -E '^ADMIN_PASSWORD=' $e | cut -d= -f2-)"
      ;;
    *) echo "usage: dotinschool {up|down|restart|logs|ps|port|cert|rebuild|update|admin}"; exit 1 ;;
  esac
}

show_menu(){
  echo
  echo "+-------------------------------------------------+"
  echo "|             dotinschool - management             |"
  echo "+-------------------------------------------------+"
  echo "|   0. Exit                                        |"
  echo "|   1. Start (up)                                  |"
  echo "|   2. Stop (down)                                 |"
  echo "|   3. Restart                                     |"
  echo "|   4. Rebuild (recompile + recreate)              |"
  echo "|   5. Update (pull latest + rebuild)             |"
  echo "|   6. Show logs                                   |"
  echo "|   7. Show container status                       |"
  echo "|   8. Find a free port                            |"
  echo "|   9. Show admin URL + credentials                |"
  echo "|  10. Get HTTPS certificate (needs a domain)      |"
  echo "|  11. Disable HTTPS (back to plain HTTP)          |"
  echo "+-------------------------------------------------+"
  read -rp "Choice [0-11]: " choice
  case "$choice" in
    0) return ;;
    1) run_action up ;;
    2) run_action down ;;
    3) run_action restart ;;
    4) run_action rebuild ;;
    5) run_action update ;;
    6) run_action logs ;;
    7) run_action ps ;;
    8) run_action port ;;
    9) run_action admin ;;
    10) do_cert "" ;;
    11) do_cert off ;;
    *) echo "Enter a number from 0 to 11." ;;
  esac
}

if [[ $# -eq 0 ]]; then show_menu; else run_action "$1"; fi
WRAP
  chmod +x "$tmp"
  mv "$tmp" "$CLI_PATH"
  ok "\"dotinschool\" command installed."
}

main(){
  require_root
  ensure_gh_token
  install_prereqs
  install_docker
  install_go
  fetch_repo
  write_env
  build_image
  install_cli
  # Via the installed CLI's own "up" action (not a separate compose call here)
  # so this always resolves APP_PORT from .env the same, already-correct way
  # - see the --env-file comment inside install_cli() above.
  "$CLI_PATH" up
  echo
  ( cd "$INSTALL_DIR" && "$CLI_PATH" admin )
  echo
  warn "For HTTPS: point a domain's DNS A record at this server, then run:  dotinschool cert <domain>"
}
# Lets the installed CLI's own "update" action refresh /usr/local/bin/
# dotinschool from whatever this file's install_cli() currently contains,
# right after pulling the latest source - without this, "dotinschool
# update" updated the app but never itself, so a new CLI feature (a menu
# item, a new subcommand) only ever showed up after re-running the whole
# one-line installer again from scratch.
if [[ "${1:-}" == "--reinstall-cli" ]]; then
  require_root
  install_cli
  exit 0
fi
main "$@"
