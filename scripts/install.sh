#!/usr/bin/env bash
# =============================================================================
# dotinschool — Linux installer / management script (x-ui / Sanaei-panel style)
#
# The dotinschool repo is private, so getting the source needs a GitHub
# token: a fine-grained personal access token, scoped to just this repository
# with "Contents: Read-only", is enough. Create one at
# https://github.com/settings/personal-access-tokens/new, then install with:
#
#   git clone https://YOUR_TOKEN@github.com/ariasam2017/dotinschool.git
#   sudo bash dotinschool/scripts/install.sh
#
# The token is only needed for that first "git clone": once cloned, this
# script reuses the same remote for /opt/dotinschool, so later updates (this
# script, the panel's "Update" button, systemd's rebuild-on-restart) keep
# working without it.
#
# The clone above can be deleted afterwards - /opt/dotinschool is its own
# independent copy. If you'd rather skip the local clone entirely, this also
# works in one line:
#
#   bash <(curl -Ls -H "Authorization: token YOUR_TOKEN" https://raw.githubusercontent.com/ariasam2017/dotinschool/main/scripts/install.sh)
#
# After install the "dotinschool" command is available:
#   dotinschool                 interactive management menu
#   dotinschool install         install / reinstall
#   dotinschool update          pull the latest version + rebuild + restart
#   dotinschool update-menu     update just this CLI script
#   dotinschool admin           show / change admin address, credentials, subdomain
#   dotinschool cert            issue/renew an SSL certificate over HTTP-01
#   dotinschool cert-cf         issue/renew an SSL certificate via Cloudflare DNS
#   dotinschool log             live log / clear logs
#   dotinschool reset           factory reset (wipe DB, new admin credentials)
#   dotinschool fail2ban        enable IP banning after repeated failed logins
#   dotinschool banlog          show fail2ban's dotinschool jail status
#   dotinschool warp            fix GitHub connectivity via Cloudflare WARP
#                               (some hosting networks block GitHub outright —
#                               if `git clone`/`update` just hangs, run this)
#   dotinschool start|stop|restart|status
#   dotinschool enable|disable  auto-start on boot
#   dotinschool uninstall
# =============================================================================
set -euo pipefail

BRANCH="${DOTINSCHOOL_BRANCH:-main}"

INSTALL_DIR="/opt/dotinschool"
CONFIG_DIR="/etc/dotinschool"
ENV_FILE="${CONFIG_DIR}/dotinschool.env"
SSL_DIR="${CONFIG_DIR}/ssl"
SERVICE_NAME="dotinschool"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
CLI_PATH="/usr/local/bin/dotinschool"
GO_VERSION="1.27.1"  # last-resort fallback only; see resolve_go_tarballs
GO_ROOT="/usr/local/go"
BIN_NAME="dotinschool"

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; CYAN='\033[0;36m'; BOLD='\033[1m'; NC='\033[0m'
info(){ echo -e "${CYAN}[i]${NC} $*"; }
ok(){ echo -e "${GREEN}[✓]${NC} $*"; }
warn(){ echo -e "${YELLOW}[!]${NC} $*"; }
err(){ echo -e "${RED}[x]${NC} $*" >&2; }
die(){ err "$*"; exit 1; }

# terminal box menu, x-ui style (colour codes are invisible but count toward the
# string length, so padding is always computed from the plain text).
BOX_WIDTH=54
box_top(){ printf '╔'; printf '─%.0s' $(seq 1 $BOX_WIDTH); printf '╗\n'; }
box_bottom(){ printf '╚'; printf '─%.0s' $(seq 1 $BOX_WIDTH); printf '╝\n'; }
box_div(){ printf '│'; printf '─%.0s' $(seq 1 $BOX_WIDTH); printf '│\n'; }
box_line(){ local plain="$1" colored="$2" pad=$((BOX_WIDTH - 2 - ${#plain})); (( pad < 0 )) && pad=0; echo -e "│  ${colored}$(printf '%*s' "$pad" '')│"; }
box_title(){ box_line "$1" "${BOLD}${1}${NC}"; }
box_item(){ box_line "${1}.  ${2}" "${GREEN}${1}.${NC}  ${2}"; }

require_root(){ [[ ${EUID:-$(id -u)} -eq 0 ]] || die "This script must be run as root (sudo bash install.sh)."; }

# The repo is private: prefer an explicitly-passed token, otherwise reuse the
# one already embedded in the existing clone's git remote (set on first
# install). Never echo the URL built from this — it carries the token.
gh_token(){
  if [[ -n "${DOTINSCHOOL_GH_TOKEN:-}" ]]; then echo "$DOTINSCHOOL_GH_TOKEN"; return; fi
  if [[ -d "$INSTALL_DIR/.git" ]]; then
    git -C "$INSTALL_DIR" remote get-url origin 2>/dev/null | sed -n 's#https://\([^@]*\)@github\.com/.*#\1#p'
  fi
}

repo_url(){
  if [[ -n "${DOTINSCHOOL_REPO_URL:-}" ]]; then echo "$DOTINSCHOOL_REPO_URL"; return; fi
  local t; t=$(gh_token)
  if [[ -n "$t" ]]; then echo "https://${t}@github.com/ariasam2017/dotinschool.git"
  else echo "https://github.com/ariasam2017/dotinschool.git"; fi
}

# Prompts once, in the real shell (not a $(...) subshell, so the export
# actually sticks), if nothing else already supplies a token. Call this
# before any command that talks to GitHub.
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

detect_pkg_manager(){
  if command -v apt-get >/dev/null 2>&1; then echo apt
  elif command -v dnf >/dev/null 2>&1; then echo dnf
  elif command -v yum >/dev/null 2>&1; then echo yum
  else die "Unsupported Linux distribution (apt/dnf/yum only)."; fi
}

install_prereqs(){
  local pm; pm=$(detect_pkg_manager)
  info "Installing prerequisites (curl, git, tar, openssl)..."
  case "$pm" in
    apt) apt-get update -y >/dev/null; apt-get install -y curl git tar openssl ca-certificates >/dev/null ;;
    dnf) dnf install -y curl git tar openssl ca-certificates >/dev/null ;;
    yum) yum install -y curl git tar openssl ca-certificates >/dev/null ;;
  esac
  ok "Prerequisites ready."
}

arch_tag(){
  case "$(uname -m)" in
    x86_64|amd64) echo amd64 ;;
    aarch64|arm64) echo arm64 ;;
    *) die "Unsupported CPU architecture: $(uname -m)" ;;
  esac
}

go_ok(){ [[ -x "${GO_ROOT}/bin/go" ]] && "${GO_ROOT}/bin/go" version >/dev/null 2>&1; }

# No version is hardcoded as the primary source: go.dev sometimes lists a
# release as stable in its metadata a little before every platform's archive
# has finished uploading, so a pinned version can 404 (this happened for
# real, on Windows, with go1.27.1's windows-amd64.zip). Ask go.dev's JSON
# endpoint for the actual current release names instead; GO_VERSION below is
# only the last-resort fallback if that lookup itself fails.
resolve_go_tarballs(){
  local arch="$1" json
  json=$(curl -fsSL --connect-timeout 15 "https://go.dev/dl/?mode=json" 2>/dev/null) || true
  [[ -n "$json" ]] && echo "$json" | grep -o "\"go[0-9.]*\.linux-${arch}\.tar\.gz\"" | tr -d '"' | awk '!seen[$0]++'
}

install_go(){
  if go_ok; then ok "Go is already installed ($("${GO_ROOT}/bin/go" version | awk '{print $3}'))."; return; fi
  local arch; arch=$(arch_tag)
  info "Looking up the latest Go release..."
  local candidates=() c
  while IFS= read -r c; do [[ -n "$c" ]] && candidates+=("$c"); done < <(resolve_go_tarballs "$arch" | head -5)
  if [[ ${#candidates[@]} -eq 0 ]]; then
    warn "Could not look up Go releases online — falling back to a pinned version (${GO_VERSION})."
    candidates=("go${GO_VERSION}.linux-${arch}.tar.gz")
  fi
  info "Trying Go release(s): ${candidates[*]}"
  local tmp="" tgz base found=0
  for tgz in "${candidates[@]}"; do
    for base in "https://go.dev/dl" "https://mirrors.aliyun.com/golang" "https://golang.google.cn/dl"; do
      tmp=$(mktemp)
      if curl -fsSL --connect-timeout 15 "${base}/${tgz}" -o "$tmp" && [[ -s "$tmp" ]]; then found=1; break 2; fi
      rm -f "$tmp"; tmp=""
    done
  done
  [[ $found -eq 1 ]] || die "Could not download Go from any mirror/release. Install it manually from https://go.dev/dl/ and re-run this script."
  rm -rf "$GO_ROOT"
  tar -C /usr/local -xzf "$tmp"
  rm -f "$tmp"
  go_ok || die "Go installation failed."
  ok "Go $("${GO_ROOT}/bin/go" version | awk '{print $3}') installed."
}

random_password(){
  local p; p=$(openssl rand -base64 18 2>/dev/null | tr -dc 'A-Za-z0-9' | head -c 20) || true
  [[ -n "$p" ]] || die "Could not generate a random password (is openssl working?)."
  echo "$p"
}
random_admin_path(){ echo "panel-$(openssl rand -hex 6)"; }

port_in_use(){ (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null && { exec 3<&- 2>/dev/null; exec 3>&- 2>/dev/null; return 0; }; return 1; }
find_free_port(){ local port="$1" tries=0; while port_in_use "$port"; do port=$((port+1)); tries=$((tries+1)); [[ $tries -lt 50 ]] || { echo "$1"; return; }; done; echo "$port"; }

github_unreachable_hint(){
  local self=""
  [[ -n "${BASH_SOURCE[0]:-}" ]] && self=$(cd "$(dirname "${BASH_SOURCE[0]}")" 2>/dev/null && pwd)/$(basename "${BASH_SOURCE[0]}")
  err "Could not reach GitHub. Some hosting networks block it outright — if this hangs for a long time before failing, that's almost certainly it."
  err "Fix: sudo bash ${self:-scripts/install.sh} warp   (or option 11 in the menu) — installs Cloudflare WARP, then try again."
}

fetch_app(){
  if [[ -d "$INSTALL_DIR/.git" ]]; then
    info "Updating source from GitHub (${BRANCH})..."
    git -C "$INSTALL_DIR" fetch --depth 1 origin "$BRANCH" >/dev/null || { github_unreachable_hint; exit 1; }
    git -C "$INSTALL_DIR" reset --hard "origin/${BRANCH}" >/dev/null
    ok "Source ready at ${INSTALL_DIR}."
    return
  fi

  # Running from inside an already-cloned checkout of this same repo? (e.g.
  # `git clone https://TOKEN@github.com/.../dotinschool.git && sudo bash
  # dotinschool/scripts/install.sh`) Reuse it instead of asking for the
  # token again: a fast local clone, then repoint the remote at the
  # original (token-bearing) URL so later updates go straight to GitHub.
  local self_dir=""
  if [[ -n "${BASH_SOURCE[0]:-}" ]]; then
    self_dir=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." 2>/dev/null && pwd) || self_dir=""
  fi
  if [[ -n "$self_dir" && -d "$self_dir/.git" ]]; then
    local origin; origin=$(git -C "$self_dir" remote get-url origin 2>/dev/null)
    if [[ "$origin" == *"github.com/ariasam2017/dotinschool"* ]]; then
      info "Reusing the local checkout at ${self_dir}..."
      git clone --depth 1 --branch "$BRANCH" "$self_dir" "$INSTALL_DIR" >/dev/null
      git -C "$INSTALL_DIR" remote set-url origin "$origin"
      ok "Source ready at ${INSTALL_DIR}."
      return
    fi
  fi

  info "Fetching source from GitHub (${BRANCH})..."
  local url; url=$(repo_url)
  [[ -n "${DOTINSCHOOL_REPO_URL:-}" || "$url" == *"@github.com"* ]] || warn "No GitHub token found (DOTINSCHOOL_GH_TOKEN) — this will fail, the repo is private."
  rm -rf "$INSTALL_DIR"
  git clone --depth 1 --branch "$BRANCH" "$url" "$INSTALL_DIR" >/dev/null || { github_unreachable_hint; exit 1; }
  ok "Source ready at ${INSTALL_DIR}."
}

build_binary(){
  info "Compiling binary (CGO off, frontend already built and committed)..."
  ( cd "$INSTALL_DIR" && CGO_ENABLED=0 "${GO_ROOT}/bin/go" build -trimpath -ldflags="-s -w" -o "${BIN_NAME}.new" ./cmd/server )
  mv "${INSTALL_DIR}/${BIN_NAME}.new" "${INSTALL_DIR}/${BIN_NAME}"
  chmod +x "${INSTALL_DIR}/${BIN_NAME}"
  ok "Binary built: ${INSTALL_DIR}/${BIN_NAME}"
}

write_env_file(){
  mkdir -p "$CONFIG_DIR"
  if [[ ! -f "$ENV_FILE" ]]; then
    local pass adminpath port
    pass=$(random_password); adminpath=$(random_admin_path); port=$(find_free_port 8080)
    [[ "$port" == "8080" ]] || warn "Port 8080 is in use — using ${port} instead."
    cat > "$ENV_FILE" <<EOF
# dotinschool service configuration — edit via "dotinschool admin" / "dotinschool cert"
PORT=${port}
DATA_DIR=${INSTALL_DIR}/data
ADMIN_USERNAME=administrator
ADMIN_PASSWORD=${pass}
ADMIN_PATH=${adminpath}
# SSL_CERT_PATH=
# SSL_KEY_PATH=
# ---- SMS (ippanel) ----
# IPPANEL_API_KEY=
# IPPANEL_PATTERN_CODE=
# IPPANEL_LOGIN_PATTERN_CODE=
# IPPANEL_ADMIN_NOTIFY_PATTERN_CODE=
# IPPANEL_ADMIN_PHONE=
# ---- AI code-review assistant (also configurable from the panel) ----
# AI_API_KEY=
# AI_BASE_URL=https://api.openai.com/v1
# AI_MODEL=gpt-4o-mini
EOF
    chmod 600 "$ENV_FILE"
    ok "Config file created — random admin password and secret path generated."
  fi
}

write_systemd_unit(){
  cat > "$SERVICE_FILE" <<EOF
[Unit]
Description=dotinschool — programming challenge platform
After=network.target

[Service]
Type=simple
Environment=PATH=${GO_ROOT}/bin:/usr/local/bin:/usr/bin:/bin
EnvironmentFile=${ENV_FILE}
WorkingDirectory=${INSTALL_DIR}
# rebuild on start so the in-panel "update" button (git reset + restart) takes
# effect; a failed build keeps the previous working binary.
ExecStartPre=-/bin/sh -c 'cd ${INSTALL_DIR} && CGO_ENABLED=0 ${GO_ROOT}/bin/go build -trimpath -ldflags="-s -w" -o ${BIN_NAME}.new ./cmd/server && mv ${BIN_NAME}.new ${BIN_NAME}'
ExecStart=${INSTALL_DIR}/${BIN_NAME}
Restart=always
RestartSec=3
AmbientCapabilities=CAP_NET_BIND_SERVICE
NoNewPrivileges=true

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  ok "systemd unit written."
}

open_firewall_port(){
  local port="$1"
  if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "Status: active"; then ufw allow "${port}/tcp" >/dev/null 2>&1 || true; fi
  if command -v firewall-cmd >/dev/null 2>&1 && systemctl is-active --quiet firewalld 2>/dev/null; then
    firewall-cmd --permanent --add-port="${port}/tcp" >/dev/null 2>&1 || true; firewall-cmd --reload >/dev/null 2>&1 || true
  fi
}

install_cli(){ cp "${INSTALL_DIR}/scripts/install.sh" "$CLI_PATH"; chmod +x "$CLI_PATH"; ok "\"dotinschool\" command installed."; }

get_env_val(){ grep -E "^${1}=" "$ENV_FILE" 2>/dev/null | tail -n1 | cut -d= -f2- || true; }
set_env_val(){
  local key="$1" val="$2"
  if grep -qE "^${key}=" "$ENV_FILE" 2>/dev/null; then sed -i "s|^${key}=.*|${key}=${val}|" "$ENV_FILE"; else echo "${key}=${val}" >> "$ENV_FILE"; fi
}

server_ip(){
  local ip; ip=$(curl -fsSL -4 --connect-timeout 8 https://api.ipify.org 2>/dev/null) || true
  [[ -n "$ip" ]] || ip=$(hostname -I 2>/dev/null | awk '{print $1}') || true
  echo "${ip:-<this-server-IP>}"
}

print_access_info(){
  local port scheme ip adminpath subdomain
  port=$(get_env_val PORT); scheme="http"
  [[ -n "$(get_env_val SSL_CERT_PATH)" ]] && scheme="https"
  ip=$(server_ip)
  adminpath=$(get_env_val ADMIN_PATH); adminpath=${adminpath:-admin}
  subdomain=$(get_env_val ADMIN_SUBDOMAIN)
  echo
  ok "User panel (public address):          ${scheme}://${ip}:${port}/"
  ok "Admin panel (secret path — you only): ${scheme}://${ip}:${port}/${adminpath}/"
  [[ -n "$subdomain" ]] && ok "Admin panel (dedicated subdomain):    ${scheme}://${subdomain}/"
  ok "Admin username:                       $(get_env_val ADMIN_USERNAME)"
  ok "Admin password:                       $(get_env_val ADMIN_PASSWORD)"
  echo
  warn "Save this information somewhere safe — it can only be viewed again with \"dotinschool admin\"."
}

cmd_install(){
  require_root
  ensure_gh_token
  install_prereqs
  install_go
  fetch_app
  build_binary
  write_env_file
  write_systemd_unit
  install_cli
  systemctl enable "$SERVICE_NAME" >/dev/null 2>&1
  systemctl restart "$SERVICE_NAME"
  sleep 2
  local tries=0
  while ! systemctl is-active --quiet "$SERVICE_NAME" && [[ $tries -lt 5 ]]; do
    if journalctl -u "$SERVICE_NAME" --no-pager -n 20 2>/dev/null | grep -q "address already in use"; then
      local oldport newport; oldport=$(get_env_val PORT); newport=$(find_free_port $((oldport+1)))
      warn "Port ${oldport} is in use — switching to ${newport}."
      set_env_val PORT "$newport"; systemctl restart "$SERVICE_NAME"; sleep 2; tries=$((tries+1))
    else break; fi
  done
  open_firewall_port "$(get_env_val PORT)"
  if systemctl is-active --quiet "$SERVICE_NAME"; then ok "Install complete and the service is running."
  else err "Service failed to start — log:"; journalctl -u "$SERVICE_NAME" --no-pager -n 30 || true; exit 1; fi
  print_access_info
  prompt_domain_setup
}

cmd_update(){
  require_root
  [[ -d "$INSTALL_DIR" ]] || die "Not installed yet — run \"dotinschool install\" first."
  ensure_gh_token
  local before after
  before=$(git -C "$INSTALL_DIR" rev-parse HEAD 2>/dev/null || echo "")
  fetch_app
  after=$(git -C "$INSTALL_DIR" rev-parse HEAD 2>/dev/null || echo "")
  install_cli
  if [[ -n "$before" && "$before" == "$after" ]]; then ok "Already up to date — no new version."; return; fi
  build_binary
  systemctl restart "$SERVICE_NAME"
  ok "Update complete and the service restarted."
}

cmd_update_menu(){
  require_root
  ensure_gh_token
  info "Fetching the latest management script..."
  local tmp t; tmp=$(mktemp); t=$(gh_token)
  if [[ -n "${DOTINSCHOOL_CLI_SOURCE_URL:-}" ]]; then
    curl -fsSL "$DOTINSCHOOL_CLI_SOURCE_URL" -o "$tmp" || { rm -f "$tmp"; die "Download failed."; }
  elif [[ -n "$t" ]]; then
    curl -fsSL -H "Authorization: token ${t}" \
      "https://raw.githubusercontent.com/ariasam2017/dotinschool/${BRANCH}/scripts/install.sh" -o "$tmp" \
      || { rm -f "$tmp"; die "Download failed."; }
  else
    curl -fsSL "https://raw.githubusercontent.com/ariasam2017/dotinschool/${BRANCH}/scripts/install.sh" -o "$tmp" \
      || { rm -f "$tmp"; die "Download failed (the repo is private — set DOTINSCHOOL_GH_TOKEN)."; }
  fi
  bash -n "$tmp" 2>/dev/null || { rm -f "$tmp"; die "Downloaded script failed the syntax check — nothing changed."; }
  install -m 755 "$tmp" "$CLI_PATH"; rm -f "$tmp"
  ok "Management script updated."
}

cmd_uninstall(){
  require_root
  read -r -p "$(echo -e "${YELLOW}The service, files, and config will be removed. Are you sure? [y/N] ${NC}")" ans
  [[ "$ans" == "y" || "$ans" == "Y" ]] || { info "Cancelled."; return; }
  systemctl stop "$SERVICE_NAME" 2>/dev/null || true
  systemctl disable "$SERVICE_NAME" 2>/dev/null || true
  rm -f "$SERVICE_FILE"; systemctl daemon-reload
  rm -rf "$INSTALL_DIR" "$CONFIG_DIR" "$CLI_PATH"
  ok "dotinschool fully removed."
}

cmd_admin(){
  require_root
  [[ -f "$ENV_FILE" ]] || die "Not installed yet."
  box_top; box_title "dotinschool — Admin Panel Settings"
  box_item 0 "Back"; box_div
  box_item 1 "Show current access info"
  box_item 2 "Change username"
  box_item 3 "Generate a new random password"
  box_item 4 "Change port"
  box_item 5 "Change secret path"
  box_item 6 "Set admin subdomain"
  box_bottom; echo
  read -r -p "Choice [0-6]: " choice
  case "$choice" in
    0) return ;;
    1) print_access_info ;;
    2) read -r -p "New username: " u; set_env_val ADMIN_USERNAME "$u"; systemctl restart "$SERVICE_NAME"; ok "Changed."; print_access_info ;;
    3) local p; p=$(random_password); set_env_val ADMIN_PASSWORD "$p"; systemctl restart "$SERVICE_NAME"; ok "New password generated."; print_access_info ;;
    4) read -r -p "New port: " p; set_env_val PORT "$p"; open_firewall_port "$p"; systemctl restart "$SERVICE_NAME"; ok "Changed."; print_access_info ;;
    5) local np; np=$(random_admin_path); set_env_val ADMIN_PATH "$np"; systemctl restart "$SERVICE_NAME"; ok "Secret path changed — the old address no longer works."; print_access_info ;;
    6) read -r -p "Full subdomain (empty = disable): " sd; set_env_val ADMIN_SUBDOMAIN "$sd"; systemctl restart "$SERVICE_NAME";
       [[ -n "$sd" ]] && warn "Add a DNS A record for \"${sd}\" pointing to this server's IP. Run \"dotinschool cert\" again for SSL."
       print_access_info ;;
    *) warn "Invalid option." ;;
  esac
}

install_acme(){ [[ -x "$HOME/.acme.sh/acme.sh" ]] || { info "Installing acme.sh..."; curl -fsSL https://get.acme.sh | sh -s email="admin@$(hostname -f 2>/dev/null || echo localhost)" >/dev/null 2>&1; }; }

# Common tail shared by every issuance method: hand the fresh cert to the
# app and point it at HTTPS. Assumes acme.sh already issued a cert for $1.
activate_cert(){
  local domain="$1"
  "$HOME/.acme.sh/acme.sh" --install-cert -d "$domain" \
    --fullchain-file "${SSL_DIR}/${domain}/fullchain.pem" \
    --key-file "${SSL_DIR}/${domain}/privkey.pem" \
    --reloadcmd "systemctl restart ${SERVICE_NAME}" >/dev/null
  set_env_val SSL_CERT_PATH "${SSL_DIR}/${domain}/fullchain.pem"
  set_env_val SSL_KEY_PATH "${SSL_DIR}/${domain}/privkey.pem"
  set_env_val PORT 443
  open_firewall_port 443
}

prompt_cert_domain(){
  read -r -p "Domain (empty = cancel): " domain
  [[ -n "$domain" ]] || return 1
  domain_args=(-d "$domain")
  local subdomain; subdomain=$(get_env_val ADMIN_SUBDOMAIN)
  if [[ -n "$subdomain" && "$subdomain" != "$domain" ]]; then
    read -r -p "Include the admin subdomain (${subdomain}) under the same certificate? [Y/n] " inc
    [[ "$inc" != "n" && "$inc" != "N" ]] && domain_args+=(-d "$subdomain")
  fi
  return 0
}

cmd_cert(){
  require_root
  echo "Issuing an SSL certificate requires a domain whose A record points to this server's IP."
  warn "If you're only using a raw IP with no domain, skip this step — the panel works fine over plain HTTP with no certificate errors."
  local domain="" domain_args=()
  prompt_cert_domain || { info "Cancelled."; return; }
  install_acme
  mkdir -p "${SSL_DIR}/${domain}"
  info "Stopping the service temporarily to free port 80..."
  systemctl stop "$SERVICE_NAME" 2>/dev/null || true
  if ! "$HOME/.acme.sh/acme.sh" --issue "${domain_args[@]}" --standalone --httpport 80; then
    err "Certificate issuance failed — check the domain(s) and that port 80 is free."
    systemctl start "$SERVICE_NAME" 2>/dev/null || true; return 1
  fi
  activate_cert "$domain"
  systemctl start "$SERVICE_NAME"; sleep 1
  if systemctl is-active --quiet "$SERVICE_NAME"; then ok "Certificate issued and active — the panel is now on HTTPS (auto-renewed via acme.sh)."; print_access_info
  else err "Service failed to start with HTTPS — journalctl -u ${SERVICE_NAME} -n 50"; fi
}

cmd_cert_cf(){
  require_root
  echo "Issues a certificate via the Cloudflare DNS API — no need for port 80 to be reachable, and works even before the domain finishes pointing at this server."
  local domain="" domain_args=()
  prompt_cert_domain || { info "Cancelled."; return; }
  echo "Needs a Cloudflare API token with Zone:DNS:Edit on this domain's zone — create one at https://dash.cloudflare.com/profile/api-tokens"
  read -r -p "Cloudflare API token: " cf_token
  [[ -n "$cf_token" ]] || { info "Cancelled."; return; }
  read -r -p "Cloudflare Account ID (dashboard right sidebar): " cf_account
  [[ -n "$cf_account" ]] || { info "Cancelled."; return; }
  install_acme
  mkdir -p "${SSL_DIR}/${domain}"
  if ! CF_Token="$cf_token" CF_Account_ID="$cf_account" "$HOME/.acme.sh/acme.sh" --issue --dns dns_cf "${domain_args[@]}"; then
    err "Certificate issuance failed — check the token's permissions and that this domain's zone is on that Cloudflare account."
    return 1
  fi
  activate_cert "$domain"
  systemctl restart "$SERVICE_NAME"; sleep 1
  if systemctl is-active --quiet "$SERVICE_NAME"; then ok "Certificate issued and active — the panel is now on HTTPS (auto-renewed via acme.sh)."; print_access_info
  else err "Service failed to start with HTTPS — journalctl -u ${SERVICE_NAME} -n 50"; fi
}

prompt_domain_setup(){
  echo
  read -r -p "Do you have a domain pointing at this server's IP? Set up SSL now? [y/N] " ans
  [[ "$ans" == "y" || "$ans" == "Y" ]] || return
  echo "1) Let's Encrypt over HTTP-01 (needs port 80 reachable from the internet)"
  echo "2) Let's Encrypt via Cloudflare DNS (needs a Cloudflare API token, works without port 80)"
  read -r -p "Choice [1/2, empty = skip]: " m
  case "$m" in
    1) cmd_cert ;;
    2) cmd_cert_cf ;;
    *) info "Skipped — run \"dotinschool cert\" or \"dotinschool cert-cf\" later." ;;
  esac
}

cmd_log(){
  require_root
  box_top; box_title "dotinschool — Logs"
  box_item 0 "Back"; box_div
  box_item 1 "Live log (Ctrl+C to stop)"
  box_item 2 "Clear all logs"
  box_bottom; echo
  read -r -p "Choice [0-2]: " choice
  case "$choice" in
    0) return ;;
    1) journalctl -u "$SERVICE_NAME" -e --no-pager -f ;;
    2) journalctl --rotate >/dev/null 2>&1 || true; journalctl --vacuum-time=1s >/dev/null 2>&1 || true; ok "Logs cleared." ;;
    *) warn "Invalid option." ;;
  esac
}

cmd_reset(){
  require_root
  [[ -f "$ENV_FILE" ]] || die "Not installed yet."
  warn "This wipes the database (all participants, the challenge, and event settings) and generates a new admin password and secret path — same as a fresh install, with the app itself left in place. The port, subdomain, and SSL certificate are not touched."
  read -r -p "$(echo -e "${YELLOW}Are you sure? [y/N] ${NC}")" ans
  [[ "$ans" == "y" || "$ans" == "Y" ]] || { info "Cancelled."; return; }
  systemctl stop "$SERVICE_NAME" 2>/dev/null || true
  rm -rf "${INSTALL_DIR}/data"
  local pass adminpath; pass=$(random_password); adminpath=$(random_admin_path)
  set_env_val ADMIN_PASSWORD "$pass"
  set_env_val ADMIN_PATH "$adminpath"
  systemctl start "$SERVICE_NAME"; sleep 2
  if systemctl is-active --quiet "$SERVICE_NAME"; then ok "Reset complete — fresh database, new admin credentials."; print_access_info
  else err "Service failed to start — log:"; journalctl -u "$SERVICE_NAME" --no-pager -n 30 || true; return 1; fi
}


# Some hosting networks (notably in Iran) block GitHub's IPs outright, so
# every git/curl call to github.com just hangs until it times out. Cloudflare
# WARP tunnels outbound traffic through Cloudflare's network instead, which
# is normally not blocked, and unblocks GitHub access as a side effect.
install_warp(){
  require_root
  if command -v warp-cli >/dev/null 2>&1; then
    ok "Cloudflare WARP is already installed."
  else
    info "Installing Cloudflare WARP..."
    local pm; pm=$(detect_pkg_manager)
    case "$pm" in
      apt)
        local codename=""
        [[ -f /etc/os-release ]] && codename=$(. /etc/os-release && echo "${VERSION_CODENAME:-}")
        [[ -n "$codename" ]] || codename=$(lsb_release -cs 2>/dev/null) || true
        [[ -n "$codename" ]] || codename="focal"
        curl -fsSL https://pkg.cloudflareclient.com/pubkey.gpg | gpg --yes --dearmor --output /usr/share/keyrings/cloudflare-warp-archive-keyring.gpg
        echo "deb [signed-by=/usr/share/keyrings/cloudflare-warp-archive-keyring.gpg] https://pkg.cloudflareclient.com/ ${codename} main" > /etc/apt/sources.list.d/cloudflare-client.list
        apt-get update -y >/dev/null && apt-get install -y cloudflare-warp >/dev/null
        ;;
      dnf|yum)
        curl -fsSL https://pkg.cloudflareclient.com/cloudflare-warp-ascii.repo -o /etc/yum.repos.d/cloudflare-warp.repo
        "$pm" install -y cloudflare-warp >/dev/null
        ;;
      *) die "Cloudflare WARP packaging is only set up for apt/dnf/yum here — install it manually from https://pkg.cloudflareclient.com/" ;;
    esac
  fi
  command -v warp-cli >/dev/null 2>&1 || die "Cloudflare WARP installation failed."
  warp-cli --accept-tos registration new >/dev/null 2>&1 || true
  warp-cli --accept-tos connect >/dev/null 2>&1 || true
  sleep 2
  if curl -s --max-time 8 https://www.cloudflare.com/cdn-cgi/trace/ 2>/dev/null | grep -q "warp=on"; then
    ok "Cloudflare WARP is connected — outbound traffic (including GitHub) now routes through Cloudflare."
  else
    warn "WARP installed but doesn't look connected yet. Check manually: warp-cli status"
  fi
}

install_fail2ban(){
  require_root
  if ! command -v fail2ban-client >/dev/null 2>&1; then
    info "Installing fail2ban..."
    local pm; pm=$(detect_pkg_manager)
    case "$pm" in
      apt) apt-get update -y >/dev/null; apt-get install -y fail2ban >/dev/null ;;
      dnf) dnf install -y fail2ban >/dev/null ;;
      yum) yum install -y epel-release >/dev/null 2>&1 || true; yum install -y fail2ban >/dev/null ;;
    esac
  fi
  mkdir -p /etc/fail2ban/filter.d /etc/fail2ban/jail.d
  cat > /etc/fail2ban/filter.d/dotinschool.conf <<'EOF'
[Definition]
failregex = ^.*msg="admin login failed" ip=<HOST>\s*$
ignoreregex =
EOF
  cat > /etc/fail2ban/jail.d/dotinschool.conf <<EOF
[dotinschool]
enabled = true
backend = systemd
journalmatch = _SYSTEMD_UNIT=${SERVICE_NAME}.service
filter = dotinschool
maxretry = 5
findtime = 10m
bantime = 1h
action = %(action_)s
EOF
  systemctl enable --now fail2ban >/dev/null 2>&1
  systemctl restart fail2ban
  ok "fail2ban is watching admin login attempts — 5 failures in 10 minutes bans the IP for 1 hour."
  warn "If you ever lock yourself out: fail2ban-client set dotinschool unbanip <your-ip>"
}

cmd_banlog(){
  require_root
  command -v fail2ban-client >/dev/null 2>&1 || die "fail2ban is not installed — run \"dotinschool fail2ban\" first."
  fail2ban-client status dotinschool 2>/dev/null || warn "The dotinschool jail isn't active yet."
}

cmd_service(){ require_root; systemctl "$1" "$SERVICE_NAME"; [[ "$1" == "status" ]] || ok "Command \"$1\" applied to the service."; }

show_status(){
  if ! [[ -f "$ENV_FILE" ]]; then echo -e "Panel status: ${RED}not installed${NC}"; return; fi
  systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null && echo -e "Panel status: ${GREEN}running${NC}" || echo -e "Panel status: ${YELLOW}stopped${NC}"
  systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null && echo -e "Start on boot: ${GREEN}yes${NC}" || echo -e "Start on boot: ${RED}no${NC}"
}

show_menu(){
  box_top; box_title "dotinschool — Management Script"
  box_item 0 "Exit"; box_div
  box_item 1 "Install"
  box_item 2 "Update to the latest version"
  box_item 3 "Update this menu script"
  box_item 4 "Uninstall"; box_div
  box_item 5 "Admin panel settings"
  box_item 6 "SSL certificate (HTTP-01)"
  box_item 7 "SSL certificate (Cloudflare DNS)"
  box_item 8 "Logs"
  box_item 9 "Factory reset"
  box_item 10 "Brute-force protection (fail2ban)"
  box_item 11 "Fix GitHub connectivity (Cloudflare WARP)"; box_div
  box_item 12 "Start"; box_item 13 "Stop"; box_item 14 "Restart"; box_item 15 "Status"; box_div
  box_item 16 "Enable autostart"; box_item 17 "Disable autostart"
  box_bottom; echo; show_status; echo
  read -r -p "Choice [0-17]: " choice
  case "$choice" in
    1) cmd_install ;; 2) cmd_update ;; 3) cmd_update_menu ;; 4) cmd_uninstall ;;
    5) cmd_admin ;; 6) cmd_cert ;; 7) cmd_cert_cf ;; 8) cmd_log ;; 9) cmd_reset ;; 10) install_fail2ban ;; 11) install_warp ;;
    12) cmd_service start ;; 13) cmd_service stop ;; 14) cmd_service restart ;; 15) cmd_service status ;;
    16) require_root; systemctl enable "$SERVICE_NAME" >/dev/null; ok "Autostart enabled." ;;
    17) require_root; systemctl disable "$SERVICE_NAME" >/dev/null; ok "Autostart disabled." ;;
    0) exit 0 ;;
    *) warn "Enter a number between 0 and 17." ;;
  esac
}

main(){
  case "${1:-}" in
    install) cmd_install ;;
    update) cmd_update ;;
    update-menu) cmd_update_menu ;;
    uninstall) cmd_uninstall ;;
    admin) cmd_admin ;;
    cert) cmd_cert ;;
    cert-cf) cmd_cert_cf ;;
    log) cmd_log ;;
    reset) cmd_reset ;;
    fail2ban) install_fail2ban ;;
    banlog) cmd_banlog ;;
    warp) install_warp ;;
    start|stop|restart|status) cmd_service "$1" ;;
    enable) require_root; systemctl enable "$SERVICE_NAME" >/dev/null; ok "Autostart enabled." ;;
    disable) require_root; systemctl disable "$SERVICE_NAME" >/dev/null; ok "Autostart disabled." ;;
    "") show_menu ;;
    *) die "Invalid command: $1" ;;
  esac
}
main "$@"
