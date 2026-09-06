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
#   dotinschool cert <domain>   set up HTTPS - pick a method interactively:
#                                 1) HTTPS-only :443, cert via Cloudflare DNS  }  the
#                                 2) HTTPS-only :443, cert via standalone HTTP  }  3x-ui
#                                    (app terminates TLS itself, no HTTP)       }  shape
#                                 3) Caddy in front + keep HTTP, auto Let's Encrypt
#                                 4) Caddy in front + keep HTTP, Cloudflare DNS
#   dotinschool cert <domain> 1  skip the prompt, use method 1 (or 2 / 3 / 4)
#   dotinschool cert off        remove HTTPS again (plain HTTP on APP_PORT)
#   dotinschool heal            recover a stuck / half-brought-up stack
#
# acme.sh (the same engine 3x-ui uses) is the certificate tool for methods
# 1/2/4; it auto-renews via its own cron. Methods 1/2 are HTTPS-only on :443
# (set HTTPS_PORT in .env to use another port). Methods 3/4 keep plain
# http://<ip>:<APP_PORT> alongside HTTPS.
#
# Optional one-shot HTTPS at install time:
#   DOTINSCHOOL_DOMAIN=panel.example.com  bash build/docker-install.sh
#   ( + CF_Token=<cloudflare-api-token>   to issue via Cloudflare DNS )
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
# CERT_MODE (set by 'dotinschool cert', don't edit by hand):
#   blank  = plain HTTP, or Caddy-auto HTTPS when DOMAIN is set
#   acme   = Caddy in front serving an acme.sh cert file
#   direct = HTTPS-only, app terminates TLS itself on HTTPS_PORT (3x-ui style)
CERT_MODE=
HTTPS_PORT=443
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
# Which compose file(s) every action uses depends on CERT_MODE in .env:
#   direct  -> docker-compose.tls.yml  ALONE.  HTTPS-only, the app terminates
#              TLS itself on :443 from an acme.sh cert - the 3x-ui model. No
#              plain-HTTP port at all.
#   acme    -> base + docker-compose.acme.yml.  Caddy in front, serving an
#              acme.sh cert file; plain HTTP on the IP kept.
#   (unset) -> base [+ docker-compose.prod.yml when DOMAIN is set].  Caddy with
#              its own automatic Let's Encrypt ACME; plain HTTP on the IP kept.
# Without the right file a plain "restart" would silently drop the HTTPS setup.
if grep -qE '^CERT_MODE=direct' .env 2>/dev/null; then
  COMPOSE_FILES=(-f build/docker-compose.tls.yml)
else
  COMPOSE_FILES=(-f build/docker-compose.yml)
  if grep -qE '^DOMAIN=[^[:space:]]' .env 2>/dev/null; then
    if grep -qE '^CERT_MODE=acme' .env 2>/dev/null; then
      COMPOSE_FILES+=(-f build/docker-compose.acme.yml)
    else
      COMPOSE_FILES+=(-f build/docker-compose.prod.yml)
    fi
  fi
fi
C=(docker compose --env-file .env "${COMPOSE_FILES[@]}")

CERT_DIR="/opt/dotinschool/certs"
ACME_HOME="/opt/dotinschool/.acme.sh"

has_domain(){ grep -qE '^DOMAIN=[^[:space:]]' .env 2>/dev/null; }
get_domain(){ grep -E '^DOMAIN=' .env 2>/dev/null | cut -d= -f2-; }
set_env_kv(){ # key value  -- upsert into .env
  if grep -qE "^$1=" .env 2>/dev/null; then sed -i "s|^$1=.*|$1=$2|" .env; else echo "$1=$2" >> .env; fi
}

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

# --- acme.sh (the same engine the 3x-ui / sanaei panel uses) --------------
# Installs acme.sh under /opt/dotinschool/.acme.sh. Tries the official
# get.acme.sh first, then a git clone via the same GitHub mirrors the rest of
# this installer uses (so it also works on a filtered network).
acme_install(){
  [[ -x "$ACME_HOME/acme.sh" ]] && return 0
  echo "Installing acme.sh ..."
  command -v socat >/dev/null 2>&1 || {
    apt-get install -y socat >/dev/null 2>&1 || dnf install -y socat >/dev/null 2>&1 || yum install -y socat >/dev/null 2>&1 || true
  }
  local tmp; tmp=$(mktemp -d)
  if curl -fsSL --connect-timeout 15 https://get.acme.sh -o "$tmp/get.sh" 2>/dev/null && [[ -s "$tmp/get.sh" ]]; then
    ( cd "$tmp" && sh get.sh --home "$ACME_HOME" >/dev/null 2>&1 ) || true
  fi
  if [[ ! -x "$ACME_HOME/acme.sh" ]]; then
    local m
    for m in "https://github.com" "https://gh-proxy.com/https://github.com" \
             "https://ghproxy.net/https://github.com" "https://ghfast.top/https://github.com"; do
      if GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null \
           git clone --depth 1 "${m}/acmesh-official/acme.sh" "$tmp/acme.sh" >/dev/null 2>&1; then
        ( cd "$tmp/acme.sh" && ./acme.sh --install --home "$ACME_HOME" >/dev/null 2>&1 ) || true
        break
      fi
    done
  fi
  rm -rf "$tmp"
  [[ -x "$ACME_HOME/acme.sh" ]] || { echo "[!] Could not install acme.sh (network?). Install it to $ACME_HOME by hand, then retry."; return 1; }
  "$ACME_HOME/acme.sh" --set-default-ca --server letsencrypt >/dev/null 2>&1 || true
}

# Issue + install a cert into $CERT_DIR/<domain>/{fullchain,privkey}.pem, with
# acme.sh's cron handling renewal and reloading Caddy on its own thereafter.
# method: cf (Cloudflare DNS API) | standalone (temporary :80 bind)
acme_issue(){
  local domain="$1" method="$2"
  acme_install || return 1
  mkdir -p "$CERT_DIR/$domain"
  local ok=0
  if [[ "$method" == "cf" ]]; then
    if [[ -z "${CF_Token:-}" && -t 0 ]]; then
      echo "Create a Cloudflare API token (Permissions: Zone > DNS > Edit,"
      echo "and Zone > Zone > Read), scoped to this domain's zone:"
      echo "  https://dash.cloudflare.com/profile/api-tokens"
      read -rsp "Cloudflare API token: " CF_Token; echo
    fi
    [[ -n "${CF_Token:-}" ]] || { echo "No Cloudflare token given."; return 1; }
    export CF_Token
    echo "Issuing via Cloudflare DNS (works even if the record is proxied/orange-cloud) ..."
    "$ACME_HOME/acme.sh" --issue --dns dns_cf -d "$domain" --home "$ACME_HOME" --keylength ec-256 && ok=1
  else
    echo "Issuing via standalone HTTP - port 80 is needed for ~20s ..."
    local had_caddy=0
    if docker ps --format '{{.Names}}' 2>/dev/null | grep -qx dotinschool-caddy; then
      had_caddy=1
      echo "  (pausing Caddy so acme.sh can bind :80)"
      docker stop dotinschool-caddy >/dev/null 2>&1 || true
    fi
    "$ACME_HOME/acme.sh" --issue --standalone -d "$domain" --home "$ACME_HOME" --keylength ec-256 --httpport 80 && ok=1
    [[ $had_caddy -eq 1 ]] && docker start dotinschool-caddy >/dev/null 2>&1 || true
  fi
  [[ $ok -eq 1 ]] || { echo "[!] acme.sh could not issue the certificate (see its output above)."; return 1; }
  # Reload whatever is terminating TLS after a renewal: Caddy picks up a new
  # cert with a live reload; the app (HTTPS-only mode) reads its cert once at
  # startup, so it needs a quick restart. Run both, ignore the one absent.
  "$ACME_HOME/acme.sh" --install-cert -d "$domain" --home "$ACME_HOME" --ecc \
    --fullchain-file "$CERT_DIR/$domain/fullchain.pem" \
    --key-file "$CERT_DIR/$domain/privkey.pem" \
    --reloadcmd "docker exec dotinschool-caddy caddy reload --config /etc/caddy/Caddyfile --force >/dev/null 2>&1; docker restart dotinschool >/dev/null 2>&1; true" \
    >/dev/null 2>&1
  [[ -s "$CERT_DIR/$domain/fullchain.pem" && -s "$CERT_DIR/$domain/privkey.pem" ]] || { echo "[!] Certificate files were not written."; return 1; }
  chmod 644 "$CERT_DIR/$domain/fullchain.pem"; chmod 640 "$CERT_DIR/$domain/privkey.pem"
  echo "[ok] Certificate saved to $CERT_DIR/$domain/ (acme.sh will auto-renew it)."
}

# `dotinschool cert <domain>` — set up HTTPS. acme.sh is the certificate
# engine (the same one the 3x-ui / sanaei panel uses). Two shapes:
#
#   direct (default) — the 3x-ui model. The app terminates TLS itself on :443,
#     HTTPS ONLY, no reverse proxy, no plain-HTTP port. Reached as
#     https://<domain>/<admin-path>/.
#   caddy — Caddy sits in front and plain http://<ip>:<APP_PORT> is kept too
#     (handy while DNS is still propagating).
#
# `dotinschool cert off` tears HTTPS down and goes back to plain HTTP.
do_cert(){
  local domain="${1:-}"
  local method="${2:-}"
  local BASE="docker compose --env-file .env -f build/docker-compose.yml"
  local PROD="$BASE -f build/docker-compose.prod.yml"
  local ACME="$BASE -f build/docker-compose.acme.yml"
  local TLS="docker compose --env-file .env -f build/docker-compose.tls.yml"
  local apath; apath=$(grep -E '^ADMIN_PATH=' .env | cut -d= -f2-)
  local ip; ip=$(server_ip)
  local appport; appport=$(grep -E '^APP_PORT=' .env | cut -d= -f2-)

  if [[ "$domain" == "off" ]]; then
    echo "Disabling HTTPS - back to plain HTTP on APP_PORT ..."
    docker rm -f dotinschool-caddy >/dev/null 2>&1 || true
    $PROD down --remove-orphans 2>/dev/null || true
    $ACME down --remove-orphans 2>/dev/null || true
    $TLS  down --remove-orphans 2>/dev/null || true
    set_env_kv DOMAIN ""
    set_env_kv CERT_MODE ""
    $BASE up -d --remove-orphans
    echo "Done. Panel: http://${ip}:${appport}/${apath}/"
    return
  fi

  if [[ -z "$domain" && -t 0 ]]; then
    read -rp "Domain (a (sub)domain you control, its DNS pointing at this server), e.g. panel.example.com: " domain
  fi
  [[ -n "$domain" ]] || { echo "No domain given."; return 1; }
  case "$domain" in
    *.cloudflare.com|cloudflare.com|*.example.com|localhost)
      echo "[!] '$domain' is not a domain you can get a certificate for - use YOUR own (sub)domain."; return 1 ;;
  esac

  if [[ -z "$method" && -t 0 ]]; then
    echo
    echo "How do you want HTTPS set up for ${domain}?"
    echo "  1) HTTPS-only on :443, cert via Cloudflare DNS   (like 3x-ui; needs a CF API token;"
    echo "                                                    works even behind the Cloudflare proxy)"
    echo "  2) HTTPS-only on :443, cert via standalone HTTP  (like 3x-ui; needs :80 free for ~20s)"
    echo "  3) Caddy in front, keep plain HTTP too, automatic Let's Encrypt"
    echo "  4) Caddy in front, keep plain HTTP too, cert via Cloudflare DNS"
    read -rp "Choice [1-4]: " method
  fi
  [[ -n "$method" ]] || method=1

  docker rm -f dotinschool-caddy >/dev/null 2>&1 || true
  set_env_kv DOMAIN "$domain"

  case "$method" in
    1|cf|cloudflare|direct)          _cert_direct "$domain" cf         "$apath" "$ip" "$appport" ;;
    2|standalone|http)               _cert_direct "$domain" standalone "$apath" "$ip" "$appport" ;;
    4|caddy-cf)                      _cert_caddy  "$domain" cf         "$apath" "$ip" "$appport" ;;
    *)                               _cert_caddy  "$domain" auto       "$apath" "$ip" "$appport" ;;
  esac
}

# HTTPS-only, app-terminated TLS on :443 (the 3x-ui shape).
_cert_direct(){ # domain issuemethod apath ip appport
  local domain="$1" im="$2" apath="$3" ip="$4" appport="$5"
  local TLS="docker compose --env-file .env -f build/docker-compose.tls.yml"
  grep -qE '^HTTPS_PORT=' .env || set_env_kv HTTPS_PORT 443
  local hp; hp=$(grep -E '^HTTPS_PORT=' .env | cut -d= -f2-); hp=${hp:-443}

  if port_in_use "$hp" && ! docker ps --format '{{.Names}}' 2>/dev/null | grep -qx dotinschool; then
    echo "[!] Port ${hp} is already in use by something else. Free it (or set HTTPS_PORT in .env) and retry."
    return 1
  fi
  # Stop any Caddy stack (frees :80 for the standalone challenge, and clears
  # the old shape before we switch to HTTPS-only).
  docker rm -f dotinschool-caddy >/dev/null 2>&1 || true
  docker compose --env-file .env -f build/docker-compose.yml -f build/docker-compose.prod.yml down --remove-orphans 2>/dev/null || true
  docker compose --env-file .env -f build/docker-compose.yml -f build/docker-compose.acme.yml down --remove-orphans 2>/dev/null || true

  if ! acme_issue "$domain" "$im"; then _cert_rollback "$domain" "$ip" "$appport" "$apath"; return 1; fi

  set_env_kv CERT_MODE direct
  echo "Starting the panel in HTTPS-only mode on :${hp} ..."
  if $TLS up -d --remove-orphans; then
    local shown=":${hp}"; [[ "$hp" == "443" ]] && shown=""
    echo
    echo "Done - HTTPS only, no plain HTTP (exactly like the 3x-ui panel)."
    echo "  panel : https://${domain}${shown}/${apath}/"
    echo "  user  : https://${domain}${shown}/"
    echo "  logs  : dotinschool logs"
    echo "  cert auto-renews (acme.sh cron); turn it off with:  dotinschool cert off"
  else
    _cert_rollback "$domain" "$ip" "$appport" "$apath"; return 1
  fi
}

# Caddy in front, plain HTTP on the IP kept alongside HTTPS.
_cert_caddy(){ # domain issuemethod apath ip appport   (issuemethod: auto|cf)
  local domain="$1" im="$2" apath="$3" ip="$4" appport="$5"
  local BASE="docker compose --env-file .env -f build/docker-compose.yml"
  local PROD="$BASE -f build/docker-compose.prod.yml"
  local ACME="$BASE -f build/docker-compose.acme.yml"
  local p
  for p in 80 443; do
    if port_in_use "$p" && ! docker ps --format '{{.Names}}' 2>/dev/null | grep -qx dotinschool-caddy; then
      echo "  note: something already listens on :$p - if it isn't ours, Caddy can't bind it."
    fi
  done
  # Leave HTTPS-only mode if we were in it.
  docker compose --env-file .env -f build/docker-compose.tls.yml down --remove-orphans 2>/dev/null || true

  if [[ "$im" == "cf" ]]; then
    set_env_kv CERT_MODE acme
    if acme_issue "$domain" cf && $ACME up -d --remove-orphans; then
      _cert_ok "$domain" "$apath" "$ip" "$appport"
    else _cert_rollback "$domain" "$ip" "$appport" "$apath"; return 1; fi
  else
    set_env_kv CERT_MODE ""
    echo "Bringing up Caddy with automatic Let's Encrypt ..."
    if $PROD up -d --remove-orphans; then
      _cert_ok "$domain" "$apath" "$ip" "$appport"
      echo "  (HTTPS goes green once Caddy finishes the ACME challenge, ~30s - watch: dotinschool logs)"
    else _cert_rollback "$domain" "$ip" "$appport" "$apath"; return 1; fi
  fi
}

_cert_ok(){ # domain apath ip appport
  echo
  echo "Done."
  echo "  https : https://${1}/${2}/"
  echo "  http  : http://${3}:${4}/${2}/        (always available)"
  echo "  watch : dotinschool logs"
}
_cert_rollback(){ # domain ip appport apath
  echo
  echo "[!] HTTPS setup failed - reverting to plain HTTP so the panel stays up."
  set_env_kv DOMAIN ""
  set_env_kv CERT_MODE ""
  docker rm -f dotinschool-caddy >/dev/null 2>&1 || true
  docker compose --env-file .env -f build/docker-compose.yml up -d --remove-orphans || true
  echo "    Panel still on: http://${2}:${3}/${4}/"
  echo "    Fix the cause and retry:  dotinschool cert ${1}"
}

running_count(){ "${C[@]}" ps -q 2>/dev/null | grep -c . || true; }

run_action(){
  case "$1" in
    up)      "${C[@]}" up -d --remove-orphans ;;
    down)    "${C[@]}" down --remove-orphans ;;
    restart) "${C[@]}" up -d --remove-orphans ;;
    logs)
      if [[ "$(running_count)" == "0" ]]; then
        echo "Nothing is running yet - start it with 'dotinschool up' (menu option 1), then check the logs."
        exit 0
      fi
      "${C[@]}" logs -f ;;
    ps|status)
      "${C[@]}" ps
      [[ "$(running_count)" == "0" ]] && echo && echo "(no containers running - 'dotinschool up' / menu option 1 brings the stack up)" ;;
    port)    show_ports ;;
    cert)    do_cert "${2:-}" "${3:-}" ;;
    heal|fix|reset)
      echo "Recovering the stack..."
      docker rm -f dotinschool dotinschool-caddy >/dev/null 2>&1 || true
      docker compose --env-file .env -f build/docker-compose.yml -f build/docker-compose.prod.yml down --remove-orphans 2>/dev/null || true
      docker compose --env-file .env -f build/docker-compose.yml -f build/docker-compose.acme.yml down --remove-orphans 2>/dev/null || true
      docker compose --env-file .env -f build/docker-compose.tls.yml down --remove-orphans 2>/dev/null || true
      # If .env says an HTTPS mode but the cert file is gone (a half-finished
      # issue), don't crash-loop - drop back to plain HTTP.
      if grep -qE '^CERT_MODE=(acme|direct)' .env 2>/dev/null && [[ ! -s "${CERT_DIR}/$(get_domain)/fullchain.pem" ]]; then
        echo "  (cert file missing - falling back to plain HTTP; re-run 'dotinschool cert <domain>' to retry)"
        set_env_kv DOMAIN ""; set_env_kv CERT_MODE ""
        docker compose --env-file .env -f build/docker-compose.yml up -d --remove-orphans
      else
        "${C[@]}" up -d --remove-orphans
      fi
      echo "Done."
      "${C[@]}" ps ;;
    rebuild) PATH="/usr/local/go/bin:$PATH" bash scripts/docker-build.sh "$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')" && "${C[@]}" up -d ;;
    update)  GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null git fetch --depth 1 origin main \
               && GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null git reset --hard origin/main \
               && bash build/docker-install.sh --reinstall-cli \
               && PATH="/usr/local/go/bin:$PATH" bash scripts/docker-build.sh "$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')" \
               && "${C[@]}" up -d ;;
    admin)
      e=.env
      apath=$(grep -E '^ADMIN_PATH=' $e | cut -d= -f2-)
      ipbase="http://$(server_ip):$(grep -E '^APP_PORT=' $e | cut -d= -f2-)"
      if grep -qE '^CERT_MODE=direct' $e 2>/dev/null; then
        hp=$(grep -E '^HTTPS_PORT=' $e | cut -d= -f2-); hp=${hp:-443}
        sfx=":${hp}"; [[ "$hp" == "443" ]] && sfx=""
        dbase="https://$(get_domain)${sfx}"
        echo "User panel:   ${dbase}/"
        echo "Admin panel:  ${dbase}/${apath}/"
        echo "              (HTTPS only - no plain HTTP)"
      elif has_domain; then
        dbase="https://$(get_domain)"
        echo "User panel:   ${dbase}/            (HTTPS - once the cert is issued)"
        echo "              ${ipbase}/            (plain HTTP - always available)"
        echo "Admin panel:  ${dbase}/${apath}/"
        echo "              ${ipbase}/${apath}/"
      else
        echo "User panel:   ${ipbase}/"
        echo "Admin panel:  ${ipbase}/${apath}/"
      fi
      echo "Username:     $(grep -E '^ADMIN_USERNAME=' $e | cut -d= -f2-)"
      echo "Password:     $(grep -E '^ADMIN_PASSWORD=' $e | cut -d= -f2-)"
      ;;
    *) echo "usage: dotinschool {up|down|restart|logs|ps|port|cert|rebuild|update|admin|heal}"; exit 1 ;;
  esac
}

show_menu(){
  echo
  echo "+-------------------------------------------------+"
  echo "|             dotinschool - management             |"
  echo "+-------------------------------------------------+"
  echo "|   0. Exit                                        |"
  echo "|   1. Start / bring everything up (IP + HTTPS)    |"
  echo "|   2. Stop (down)                                 |"
  echo "|   3. Restart                                     |"
  echo "|   4. Rebuild (recompile + recreate)              |"
  echo "|   5. Update (pull latest + rebuild)             |"
  echo "|   6. Show logs                                   |"
  echo "|   7. Show container status                       |"
  echo "|   8. Find a free port                            |"
  echo "|   9. Show admin URL + credentials                |"
  echo "|  10. Set up HTTPS  (3x-ui style :443, or Caddy)  |"
  echo "|  11. Disable HTTPS (back to plain HTTP)          |"
  echo "|  12. Recover / fix a stuck stack                 |"
  echo "+-------------------------------------------------+"
  read -rp "Choice [0-12]: " choice
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
    12) run_action heal ;;
    *) echo "Enter a number from 0 to 12." ;;
  esac
}

if [[ $# -eq 0 ]]; then show_menu; else run_action "$1"; fi
WRAP
  chmod +x "$tmp"
  mv "$tmp" "$CLI_PATH"
  ok "\"dotinschool\" command installed."
}

# Poll the app's own port until it answers (or give up) - so the installer
# only prints "done" once the panel is actually serving, and surfaces a
# crash-loop instead of leaving the user to discover it.
wait_healthy(){
  local port; port=$(grep -E '^APP_PORT=' "${INSTALL_DIR}/.env" | cut -d= -f2-)
  [[ -n "$port" ]] || port=8080
  local i
  for i in $(seq 1 30); do
    if curl -fsS -m 3 "http://127.0.0.1:${port}/api/config" >/dev/null 2>&1; then
      ok "Panel is up and answering on port ${port}."
      return 0
    fi
    sleep 2
  done
  warn "The panel didn't answer on port ${port} within 60s - check:  dotinschool logs"
  return 1
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
  wait_healthy || true

  # Optional one-shot HTTPS: DOTINSCHOOL_DOMAIN=panel.example.com bash docker-install.sh
  # (the A record must already point here). Safe to skip - HTTP on the IP works
  # regardless, and `dotinschool cert <domain>` can be run any time later.
  if [[ -n "${DOTINSCHOOL_DOMAIN:-}" ]]; then
    echo
    # HTTPS-only (3x-ui style). CF_Token in the env -> Cloudflare-DNS (method 1,
    # works behind the Cloudflare proxy); otherwise standalone HTTP (method 2).
    local m=2; [[ -n "${CF_Token:-}" ]] && m=1
    info "DOTINSCHOOL_DOMAIN set - setting up HTTPS-only for ${DOTINSCHOOL_DOMAIN} (method ${m})..."
    ( cd "$INSTALL_DIR" && CF_Token="${CF_Token:-}" "$CLI_PATH" cert "$DOTINSCHOOL_DOMAIN" "$m" ) || \
      warn "HTTPS setup didn't complete - the panel is still up on the IP. Retry later with: dotinschool cert ${DOTINSCHOOL_DOMAIN}"
  fi

  echo
  ( cd "$INSTALL_DIR" && "$CLI_PATH" admin )
  echo
  warn "For HTTPS later: point a domain's DNS A record at this server, then run:  dotinschool cert <domain>"
  warn "If anything ever looks stuck:  dotinschool heal"
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
