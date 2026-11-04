#!/usr/bin/env bash
# =============================================================================
# dotinschool-challenge — Linux installer / management script (x-ui / Sanaei-panel style)
#
# One command, no GitHub token, no compiler - it downloads the prebuilt
# release from the public dotinschool-install repo (the source repo stays private):
#
#   bash <(curl -Ls https://raw.githubusercontent.com/ariasam2017/dotinschool-install/main/challenge.sh)
#
# Run as root. On a fresh server it installs straight away; once installed,
# the same command opens the management menu.
#
# After install the "dotinschool-challenge" command is available:
#   dotinschool-challenge                 interactive management menu
#   dotinschool-challenge install         install / reinstall
#   dotinschool-challenge update          download the latest release + restart
#   dotinschool-challenge update-menu     update just this CLI script
#   dotinschool-challenge admin           show / change admin address, credentials, subdomain
#   dotinschool-challenge ssl             certificate & HTTPS-lock menu (all options below)
#   dotinschool-challenge cert            free certificate: Let's Encrypt over HTTP-01
#   dotinschool-challenge cert-cf         free certificate: Let's Encrypt via Cloudflare DNS
#   dotinschool-challenge cert-zerossl    free certificate: ZeroSSL over HTTP-01
#   dotinschool-challenge csr             own/paid certificate (Certum, company CA) step 1: key + CSR
#   dotinschool-challenge cert-import     own/paid certificate step 2: install the files the CA sent
#   dotinschool-challenge lock on|off|status
#                               HTTPS lock: "on" = production mode, the panel then
#                               ONLY works over HTTPS (docs/security/TLS-POLICY.md)
#   dotinschool-challenge log             live log / clear logs
#   dotinschool-challenge reset           factory reset (wipe DB, new admin credentials)
#   dotinschool-challenge fail2ban        enable IP banning after repeated failed logins
#   dotinschool-challenge banlog          show fail2ban's dotinschool-challenge jail status
#   dotinschool-challenge warp            fix GitHub connectivity via Cloudflare WARP
#                               (some hosting networks block GitHub outright —
#                               if `install`/`update` just hangs, run this)
#   dotinschool-challenge tunnel          access with a VPN on: Cloudflare Tunnel for
#                               visitors from foreign IPs (Iranian data centres
#                               often drop those) — token <TOKEN> | on | status | off
#                               (token = your domain on Cloudflare; works from Iran.
#                               "on" = free trycloudflare.com, filtered in Iran)
#   dotinschool-challenge start|stop|restart|status
#   dotinschool-challenge enable|disable  auto-start on boot
#   dotinschool-challenge uninstall
# =============================================================================
set -euo pipefail

PUBLIC_REPO="ariasam2017/dotinschool-install"
RELEASE_TAG="challenge-latest"

INSTALL_DIR="/opt/dotinschool-challenge"
CONFIG_DIR="/etc/dotinschool-challenge"
ENV_FILE="${CONFIG_DIR}/dotinschool-challenge.env"
SSL_DIR="${CONFIG_DIR}/ssl"
SERVICE_NAME="dotinschool-challenge"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
CLI_PATH="/usr/local/bin/dotinschool-challenge"
BIN_NAME="dotinschool-challenge"
SSL_TOOLS="${INSTALL_DIR}/build/ssl-tools.sh"

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
# NB: two statements on purpose — bash expands every word of a `local` line
# before assigning any of them, so `local plain="$1" pad=${#plain}` reads an
# unset $plain and aborts under `set -u`.
box_line(){ local plain="$1" colored="$2" pad; pad=$((BOX_WIDTH - 2 - ${#plain})); (( pad < 0 )) && pad=0; echo -e "│  ${colored}$(printf '%*s' "$pad" '')│"; }
box_title(){ box_line "$1" "${BOLD}${1}${NC}"; }
box_item(){ box_line "${1}.  ${2}" "${GREEN}${1}.${NC}  ${2}"; }

require_root(){ [[ ${EUID:-$(id -u)} -eq 0 ]] || die "This script must be run as root (sudo bash install.sh)."; }

# shared certificate helpers (shipped in the release bundle)
load_ssl_tools(){
  [[ -f "$SSL_TOOLS" ]] || die "Missing ${SSL_TOOLS} - run \"dotinschool-challenge update\" first."
  # shellcheck source=../build/ssl-tools.sh
  source "$SSL_TOOLS"
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

random_password(){
  local p; p=$(openssl rand -base64 18 2>/dev/null | tr -dc 'A-Za-z0-9' | head -c 20) || true
  [[ -n "$p" ]] || die "Could not generate a random password (is openssl working?)."
  echo "$p"
}
random_admin_path(){ echo "panel-$(openssl rand -hex 6)"; }

port_in_use(){ (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null && { exec 3<&- 2>/dev/null; exec 3>&- 2>/dev/null; return 0; }; return 1; }
find_free_port(){ local port="$1" tries=0; while port_in_use "$port"; do port=$((port+1)); tries=$((tries+1)); [[ $tries -lt 50 ]] || { echo "$1"; return; }; done; echo "$port"; }

github_unreachable_hint(){
  err "Could not download from GitHub (or any mirror). Some hosting networks block it outright."
  err "Fix: run \"dotinschool-challenge warp\" (or menu option 11) to install Cloudflare WARP, then try again."
}

# The source repo is private; installs come from the public release repo
# (prebuilt binary + Docker files, no source). GitHub itself first, then
# public mirrors for networks that block it.
gh_get(){ # <path on github.com> <dest file>
  local m
  for m in "" "https://gh-proxy.com/" "https://ghproxy.net/" "https://ghfast.top/" "https://gh.ddlc.top/"; do
    [[ -n "$m" ]] && info "  retrying via ${m}"
    # a progress bar on a terminal, so a slow link never looks like a hang
    local show=(-sS); [[ -t 2 ]] && show=(--progress-bar)
    if curl -fL "${show[@]}" --connect-timeout 15 --max-time 900 "${m}https://github.com/${1}" -o "$2" && [[ -s "$2" ]]; then
      return 0
    fi
  done
  return 1
}

# Downloads the latest release bundle and lays it over INSTALL_DIR. The bundle
# holds no data/ directory, so the database (and .env) are never touched.
fetch_bundle(){
  local arch tmp; arch=$(arch_tag); tmp=$(mktemp -d)
  info "Downloading the latest ${BIN_NAME} release (linux/${arch})..."
  if ! gh_get "${PUBLIC_REPO}/releases/download/${RELEASE_TAG}/${BIN_NAME}_linux_${arch}.tar.gz" "$tmp/bundle.tgz"; then
    rm -rf "$tmp"; github_unreachable_hint; exit 1
  fi
  tar -xzf "$tmp/bundle.tgz" -C "$tmp" 2>/dev/null && [[ -s "$tmp/${BIN_NAME}/dist/${BIN_NAME}_linux_${arch}" ]] \
    || { rm -rf "$tmp"; die "The downloaded release is incomplete or damaged - run this again."; }
  mkdir -p "$INSTALL_DIR"
  cp -a "$tmp/${BIN_NAME}/." "$INSTALL_DIR/"
  rm -rf "$tmp"
  ok "Release $(cat "${INSTALL_DIR}/VERSION" 2>/dev/null || echo '?') ready at ${INSTALL_DIR}."
}

install_binary(){
  local arch; arch=$(arch_tag)
  # copy-then-rename: overwriting a running binary in place fails ("text file busy")
  cp "${INSTALL_DIR}/dist/${BIN_NAME}_linux_${arch}" "${INSTALL_DIR}/${BIN_NAME}.new"
  chmod +x "${INSTALL_DIR}/${BIN_NAME}.new"
  mv -f "${INSTALL_DIR}/${BIN_NAME}.new" "${INSTALL_DIR}/${BIN_NAME}"
  ok "Binary installed: ${INSTALL_DIR}/${BIN_NAME}"
}

write_env_file(){
  mkdir -p "$CONFIG_DIR"
  if [[ ! -f "$ENV_FILE" ]]; then
    local pass adminpath port
    pass=$(random_password); adminpath=$(random_admin_path); port=$(find_free_port 8081)
    [[ "$port" == "8081" ]] || warn "Port 8081 is in use — using ${port} instead."
    cat > "$ENV_FILE" <<EOF
# dotinschool-challenge service configuration — edit via "dotinschool-challenge admin" / "dotinschool-challenge cert"
PORT=${port}
DATA_DIR=${INSTALL_DIR}/data
ADMIN_USERNAME=administrator
ADMIN_PASSWORD=${pass}
ADMIN_PATH=${adminpath}
# SSL_CERT_PATH=
# SSL_KEY_PATH=
# ---- stage (docs/security/TLS-POLICY.md) ----
# setup = first stage: plain HTTP works, admin login over HTTP allowed so the
# install can be checked. "dotinschool-challenge lock on" switches to
# APP_ENV=production once HTTPS works: then ONLY HTTPS, admin panel only from
# ADMIN_ALLOWED_CIDRS, and the service refuses to start without a certificate.
APP_ENV=setup
ALLOW_INSECURE_ADMIN=1
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
Description=dotinschool-challenge — programming challenge platform (coding contest)
After=network.target

[Service]
Type=simple
EnvironmentFile=${ENV_FILE}
WorkingDirectory=${INSTALL_DIR}
ExecStart=${INSTALL_DIR}/${BIN_NAME}
Restart=always
RestartSec=3
AmbientCapabilities=CAP_NET_BIND_SERVICE
NoNewPrivileges=true
# Sandboxing that costs the app nothing: it needs no shared /tmp, never loads
# kernel modules or tunes the kernel, and never runs setuid helpers.
PrivateTmp=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictSUIDSGID=true
LockPersonality=true
UMask=0077

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

install_cli(){ cp "${INSTALL_DIR}/scripts/install.sh" "$CLI_PATH"; chmod +x "$CLI_PATH"; ok "\"dotinschool-challenge\" command installed."; }

get_env_val(){ grep -E "^${1}=" "$ENV_FILE" 2>/dev/null | tail -n1 | cut -d= -f2- || true; }
set_env_val(){
  local key="$1" val="$2"
  if grep -qE "^${key}=" "$ENV_FILE" 2>/dev/null; then sed -i "s|^${key}=.*|${key}=${val}|" "$ENV_FILE"; else echo "${key}=${val}" >> "$ENV_FILE"; fi
}

# Inside WSL (Linux on Windows) the internet-facing IP belongs to the router,
# not to this machine, so that address never opens; WSL's own IP is the one
# reachable from the Windows host.
is_wsl(){ grep -qi microsoft /proc/sys/kernel/osrelease 2>/dev/null; }

server_ip(){
  local ip=""
  if ! is_wsl; then
    ip=$(curl -fsSL -4 --connect-timeout 8 https://api.ipify.org 2>/dev/null) || true
  fi
  [[ -n "$ip" ]] || ip=$(hostname -I 2>/dev/null | awk '{print $1}') || true
  echo "${ip:-<this-server-IP>}"
}

print_access_info(){
  local port scheme host adminpath subdomain
  port=$(get_env_val PORT); scheme="http"; host="$(server_ip):${port}"
  if [[ -n "$(get_env_val SSL_CERT_PATH)" ]]; then
    # the certificate is for the domain, not the IP — show the address that works
    scheme="https"; host=$(get_env_val CERT_DOMAIN); host=${host:-$(server_ip)}
    [[ "$port" == "443" ]] || host="${host}:${port}"
  fi
  adminpath=$(get_env_val ADMIN_PATH); adminpath=${adminpath:-admin}
  subdomain=$(get_env_val ADMIN_SUBDOMAIN)
  echo
  if https_locked; then ok "Stage:                                PRODUCTION — HTTPS lock ON (plain HTTP refused)"
  else warn "Stage:                                SETUP — plain HTTP allowed. Not for real use: get a certificate, then \"dotinschool-challenge lock on\""; fi
  ok "User panel:                           ${scheme}://${host}/"
  ok "Admin panel (secret path — you only): ${scheme}://${host}/${adminpath}/"
  [[ -n "$subdomain" ]] && ok "Admin panel (dedicated subdomain):    ${scheme}://${subdomain}/"
  if systemctl is-active --quiet dotinschool-challenge-tunnel 2>/dev/null; then
    local turl; turl=$(journalctl -u dotinschool-challenge-tunnel --no-pager -n 300 2>/dev/null | grep -oE 'https://[a-z0-9-]+\.trycloudflare\.com' | tail -n1 || true)
    [[ -n "$turl" ]] && ok "With a VPN on (Cloudflare Tunnel):    ${turl}/   admin: ${turl}/${adminpath}/"
  fi
  ok "Admin username:                       $(get_env_val ADMIN_USERNAME)"
  ok "Admin password:                       $(get_env_val ADMIN_PASSWORD)"
  echo
  if is_wsl; then
    warn "Running inside WSL: open these from this Windows PC. The address is WSL's own IP and can change"
    warn "after a reboot (check with: hostname -I). If the page stays blank, let local addresses bypass your VPN/proxy."
  fi
  warn "Save this information somewhere safe — it can only be viewed again with \"dotinschool-challenge admin\"."
}

cmd_install(){
  require_root
  install_prereqs
  fetch_bundle
  install_binary
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
  [[ -f "$ENV_FILE" ]] || die "Not installed yet — run \"dotinschool-challenge install\" first."
  local before after
  before=$(cat "${INSTALL_DIR}/VERSION" 2>/dev/null || echo "")
  fetch_bundle
  after=$(cat "${INSTALL_DIR}/VERSION" 2>/dev/null || echo "")
  install_cli
  migrate_stage_env
  # (re)written every time: installs from before the release repo rebuilt the
  # binary from source on every start, which would undo this update
  write_systemd_unit
  if [[ -n "$before" && "$before" == "$after" ]] && [[ -x "${INSTALL_DIR}/${BIN_NAME}" ]]; then
    ok "Already up to date (${after})."; return
  fi
  install_binary
  systemctl restart "$SERVICE_NAME"
  ok "Updated ${before:-?} -> ${after} and the service restarted."
}

# Installs from before the HTTPS lock existed have no APP_ENV: keep them working
# exactly as before (setup stage) and point at the lock.
migrate_stage_env(){
  grep -qE '^APP_ENV=' "$ENV_FILE" 2>/dev/null && return 0
  set_env_val APP_ENV setup
  set_env_val ALLOW_INSECURE_ADMIN 1
  warn "This server is in the SETUP stage (plain HTTP allowed). For real use: get a certificate, then run \"dotinschool-challenge lock on\"."
}

cmd_update_menu(){
  require_root
  info "Fetching the latest management script..."
  local tmp; tmp=$(mktemp)
  local url="${DOTINSCHOOL_CLI_SOURCE_URL:-https://raw.githubusercontent.com/${PUBLIC_REPO}/main/challenge.sh}"
  curl -fsSL "$url" -o "$tmp" || { rm -f "$tmp"; die "Download failed."; }
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
  systemctl disable --now "$TUNNEL_SERVICE" 2>/dev/null || true
  rm -f "$SERVICE_FILE" "$TUNNEL_UNIT"; systemctl daemon-reload
  rm -rf "$INSTALL_DIR" "$CONFIG_DIR" "$CLI_PATH"
  ok "dotinschool-challenge fully removed."
}

cmd_admin(){
  require_root
  [[ -f "$ENV_FILE" ]] || die "Not installed yet."
  box_top; box_title "dotinschool-challenge — Admin Panel Settings"
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
       [[ -n "$sd" ]] && warn "Add a DNS A record for \"${sd}\" pointing to this server's IP. Run \"dotinschool-challenge cert\" again for SSL."
       print_access_info ;;
    *) warn "Invalid option." ;;
  esac
}

install_acme(){ [[ -x "$HOME/.acme.sh/acme.sh" ]] || { info "Installing acme.sh..."; curl -fsSL https://get.acme.sh | sh -s email="admin@$(hostname -f 2>/dev/null || echo localhost)" >/dev/null 2>&1; }; }

# Common tail shared by every issuance method: hand the fresh cert to the
# app and point it at HTTPS. Assumes acme.sh already issued a cert for $1.
activate_cert(){
  local domain="$1"
  mkdir -p "${SSL_DIR}/${domain}"
  "$HOME/.acme.sh/acme.sh" --install-cert -d "$domain" --ecc \
    --fullchain-file "${SSL_DIR}/${domain}/fullchain.pem" \
    --key-file "${SSL_DIR}/${domain}/privkey.pem" \
    --reloadcmd "chmod 600 ${SSL_DIR}/${domain}/privkey.pem; systemctl restart ${SERVICE_NAME}" >/dev/null
  chmod 600 "${SSL_DIR}/${domain}/privkey.pem"
  use_cert_files "$domain"
}

# point the app at ${SSL_DIR}/<domain>/{fullchain,privkey}.pem on :443
use_cert_files(){
  local domain="$1"
  set_env_val SSL_CERT_PATH "${SSL_DIR}/${domain}/fullchain.pem"
  set_env_val SSL_KEY_PATH "${SSL_DIR}/${domain}/privkey.pem"
  set_env_val CERT_DOMAIN "$domain"
  set_env_val PORT 443
  open_firewall_port 443
}

# the panel answers on its own port (a loopback request is allowed in every stage)
service_answers(){
  local port scheme=http i; port=$(get_env_val PORT)
  [[ -n "$(get_env_val SSL_CERT_PATH)" ]] && scheme=https
  for i in $(seq 1 15); do
    curl -fsk -o /dev/null --max-time 3 "${scheme}://127.0.0.1:${port}/api/version" 2>/dev/null && return 0
    sleep 1
  done
  return 1
}

# after any certificate goes live: check it in a browser, then lock
cert_done(){
  ok "Certificate active — the panel is now on HTTPS."
  print_access_info
  https_locked && return 0
  echo
  info "Open the https:// address above in a browser and check that the padlock shows and the admin panel works."
  local a; read -r -p "Everything OK? Turn the HTTPS lock on now (production mode, plain HTTP refused from then on)? [y/N] " a
  if [[ "$a" == "y" || "$a" == "Y" ]]; then lock_on
  else info "Lock stays off for now. Turn it on later with: dotinschool-challenge lock on"; fi
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

cmd_cert(){ issue_http01 letsencrypt; }
cmd_cert_zerossl(){ issue_http01 zerossl; }

# free certificate over HTTP-01: acme.sh binds :80 for ~20s
issue_http01(){
  local ca="$1"
  require_root
  echo "A free certificate needs a domain whose DNS A record points to this server, and port 80"
  echo "reachable from the internet for ~20 seconds."
  local domain="" domain_args=()
  prompt_cert_domain || { info "Cancelled."; return; }
  install_acme
  if [[ "$ca" == "zerossl" ]]; then
    local email; read -r -p "Your email (ZeroSSL needs an account email): " email
    [[ "$email" == *@*.* ]] || { err "A valid email is required for ZeroSSL."; return 1; }
    "$HOME/.acme.sh/acme.sh" --register-account -m "$email" --server zerossl >/dev/null 2>&1 \
      || { err "Could not register the ZeroSSL account."; return 1; }
  fi
  info "Stopping the service temporarily to free port 80..."
  systemctl stop "$SERVICE_NAME" 2>/dev/null || true
  local rc=0
  "$HOME/.acme.sh/acme.sh" --issue --server "$ca" "${domain_args[@]}" --standalone --httpport 80 --keylength ec-256 || rc=$?
  # rc 2 = a valid certificate already exists and isn't due for renewal
  if [[ $rc -ne 0 && $rc -ne 2 ]]; then
    err "Certificate issuance failed — check the domain(s) and that port 80 is free and reachable."
    systemctl start "$SERVICE_NAME" 2>/dev/null || true; return 1
  fi
  activate_cert "$domain"
  systemctl start "$SERVICE_NAME"
  if service_answers; then cert_done
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
  local rc=0
  CF_Token="$cf_token" CF_Account_ID="$cf_account" "$HOME/.acme.sh/acme.sh" --issue --server letsencrypt --dns dns_cf "${domain_args[@]}" --keylength ec-256 || rc=$?
  if [[ $rc -ne 0 && $rc -ne 2 ]]; then
    err "Certificate issuance failed — check the token's permissions and that this domain's zone is on that Cloudflare account."
    return 1
  fi
  activate_cert "$domain"
  systemctl restart "$SERVICE_NAME"
  if service_answers; then cert_done
  else err "Service failed to start with HTTPS — journalctl -u ${SERVICE_NAME} -n 50"; fi
}

# ---- own / paid certificate (Certum, a company CA, any CA) ----------------

cmd_csr(){
  require_root; load_ssl_tools
  echo "Step 1 of 2 for a certificate you order yourself (Certum, your company CA, ...):"
  echo "this creates the private key ON THIS SERVER and a request (CSR) to give the CA."
  local domain extra org country dir a
  read -r -p "Domain for the certificate (e.g. panel.company.ir, empty = cancel): " domain
  [[ -n "$domain" ]] || { info "Cancelled."; return; }
  read -r -p "Extra names on the same certificate, space-separated (Enter = none): " extra
  read -r -p "Organisation name (Enter = skip): " org
  read -r -p "Country code, 2 letters, e.g. IR (Enter = skip): " country
  dir="${SSL_DIR}/.pending/${domain}"
  if [[ -s "$dir/request.csr" ]]; then
    read -r -p "A request for ${domain} already exists. Make a new one (the old one stops matching)? [y/N] " a
    if [[ "$a" != "y" && "$a" != "Y" ]]; then echo; cat "$dir/request.csr"; return; fi
  fi
  ssl_make_csr "$domain" "$extra" "$dir" "$org" "$country" || die "Could not create the request."
  ok "Private key kept on this server only: ${dir}/privkey.pem (never send it to anyone)."
  ok "Request (CSR) saved to ${dir}/request.csr — paste everything below into the CA's order form:"
  echo; cat "$dir/request.csr"; echo
  info "When the CA sends the certificate, upload it here and run: dotinschool-challenge cert-import"
}

cmd_cert_import(){
  require_root; load_ssl_tools
  [[ -f "$ENV_FILE" ]] || die "Not installed yet."
  echo "Install a certificate you got yourself (Certum, your company CA, any CA)."
  local domain stage
  read -r -p "Domain the certificate is for (e.g. panel.company.ir, empty = cancel): " domain
  [[ -n "$domain" ]] || { info "Cancelled."; return; }
  stage=$(mktemp -d); chmod 700 "$stage"
  if ! ssl_import_interactive "$domain" "${SSL_DIR}/.pending/${domain}" "$stage"; then
    rm -rf "$stage"; err "Nothing was changed."; return 1
  fi
  cp -p "$ENV_FILE" "${ENV_FILE}.pre-cert"
  mkdir -p "${SSL_DIR}/${domain}"; chmod 700 "${SSL_DIR}/${domain}"
  # acme.sh must not overwrite these files on its next renewal run
  if [[ -x "$HOME/.acme.sh/acme.sh" ]]; then
    "$HOME/.acme.sh/acme.sh" --remove -d "$domain" --ecc >/dev/null 2>&1 || true
    "$HOME/.acme.sh/acme.sh" --remove -d "$domain" >/dev/null 2>&1 || true
  fi
  install -m 644 "$stage/fullchain.pem" "${SSL_DIR}/${domain}/fullchain.pem"
  install -m 600 "$stage/privkey.pem" "${SSL_DIR}/${domain}/privkey.pem"
  rm -rf "$stage"
  use_cert_files "$domain"
  systemctl restart "$SERVICE_NAME"
  if ! service_answers; then
    err "The service did not come up with this certificate — restoring the previous settings."
    journalctl -u "$SERVICE_NAME" --no-pager -n 10 || true
    mv -f "${ENV_FILE}.pre-cert" "$ENV_FILE"; systemctl restart "$SERVICE_NAME"; return 1
  fi
  rm -f "${ENV_FILE}.pre-cert"
  rm -rf "${SSL_DIR}/.pending/${domain}"
  warn "This certificate is NOT renewed automatically. Note its expiry date and repeat"
  warn "\"dotinschool-challenge csr\" + \"cert-import\" before then (check: dotinschool-challenge lock status)."
  cert_done
}

# ---- HTTPS lock (setup stage -> production) --------------------------------

https_locked(){ [[ "$(get_env_val APP_ENV)" == "production" ]]; }
del_env_val(){ sed -i "/^${1}=/d" "$ENV_FILE"; }
# security-relevant changes go to syslog (journald), where the SIEM agent picks them up
audit_log(){ logger -t dotinschool-challenge "$*" 2>/dev/null || true; }

cmd_lock(){
  require_root
  [[ -f "$ENV_FILE" ]] || die "Not installed yet."
  case "${1:-status}" in
    on) lock_on ;;
    off) lock_off ;;
    status) lock_status ;;
    *) die "Usage: dotinschool-challenge lock on|off|status" ;;
  esac
}

lock_status(){
  load_ssl_tools
  echo
  if https_locked; then echo -e "HTTPS lock: ${GREEN}ON${NC} — production: HTTPS only, plain HTTP refused"
  else echo -e "HTTPS lock: ${YELLOW}OFF${NC} — setup stage: plain HTTP and admin login over HTTP allowed (not for real use)"; fi
  local cert; cert=$(get_env_val SSL_CERT_PATH)
  if [[ -n "$cert" ]]; then echo "Certificate ($(get_env_val CERT_DOMAIN)):"; ssl_show_info "$cert"
  elif [[ "$(get_env_val TLS_PROXY)" == "1" ]]; then echo "TLS: terminated in front of the app (Cloudflare Tunnel / proxy)"
  else echo "Certificate: none yet — get one from the SSL menu (dotinschool-challenge ssl)"; fi
  if [[ -n "$(get_env_val ADMIN_ALLOWED_CIDRS)" ]]; then echo "Admin panel allowed from: $(get_env_val ADMIN_ALLOWED_CIDRS)"; fi
  echo
}

lock_on(){
  load_ssl_tools
  local cert key cidrs
  cert=$(get_env_val SSL_CERT_PATH); key=$(get_env_val SSL_KEY_PATH)
  if [[ -n "$cert" ]]; then
    [[ -s "$cert" && -s "$key" ]] || die "The certificate files are missing — install a certificate again first."
    openssl x509 -in "$cert" -noout -checkend 0 >/dev/null 2>&1 || die "The certificate has expired — renew it first."
  elif [[ "$(get_env_val TLS_PROXY)" != "1" ]]; then
    die "No HTTPS yet. Get a certificate first (dotinschool-challenge ssl), check https:// in a browser, then lock."
  fi
  cidrs=$(ssl_prompt_admin_cidrs "$(get_env_val ADMIN_ALLOWED_CIDRS)") \
    || die "Invalid list — nothing changed. Use IPs/CIDRs like 10.10.0.0/16,192.168.5.7 or the word any."
  cp -p "$ENV_FILE" "${ENV_FILE}.pre-lock"
  set_env_val APP_ENV production
  set_env_val ADMIN_ALLOWED_CIDRS "$cidrs"
  del_env_val ALLOW_INSECURE_ADMIN
  systemctl restart "$SERVICE_NAME"
  if ! service_answers; then
    err "The service refused to start in production mode:"
    journalctl -u "$SERVICE_NAME" --no-pager -n 30 2>/dev/null | grep -iE "fatal|refusing" | tail -n 3 || true
    mv -f "${ENV_FILE}.pre-lock" "$ENV_FILE"; systemctl restart "$SERVICE_NAME"
    die "Lock NOT applied — back in the setup stage, nothing else changed."
  fi
  rm -f "${ENV_FILE}.pre-lock"
  audit_log "HTTPS lock turned ON by ${SUDO_USER:-root} (admin panel allowed from: ${cidrs})"
  ok "HTTPS lock is ON. The panel now works only over HTTPS; plain HTTP is refused, and the"
  ok "service will not start without a valid certificate. Admin panel only from: ${cidrs}"
  print_access_info
}

lock_off(){
  https_locked || { info "The HTTPS lock is already off (setup stage)."; return; }
  warn "Turning the lock off puts the system back into the SETUP stage: plain HTTP and admin"
  warn "login over HTTP become possible again. This is not allowed for production use."
  local a; read -r -p "Type UNLOCK to confirm: " a
  [[ "$a" == "UNLOCK" ]] || { info "Cancelled — the lock stays on."; return; }
  set_env_val APP_ENV setup
  set_env_val ALLOW_INSECURE_ADMIN 1
  systemctl restart "$SERVICE_NAME"
  audit_log "HTTPS lock turned OFF by ${SUDO_USER:-root}"
  warn "Lock is OFF — setup stage. Turn it back on with: dotinschool-challenge lock on"
}

cmd_ssl_menu(){
  require_root
  [[ -f "$ENV_FILE" ]] || die "Not installed yet."
  lock_status
  box_top; box_title "SSL certificate & HTTPS lock"
  box_item 0 "Back"; box_div
  box_title "Free certificate:"
  box_item 1 "Let's Encrypt (needs port 80)"
  box_item 2 "Let's Encrypt via Cloudflare DNS"
  box_item 3 "ZeroSSL (needs port 80 + email)"; box_div
  box_title "Own / paid (Certum, company CA):"
  box_item 4 "Step 1: create key + CSR"
  box_item 5 "Step 2: install the issued files"; box_div
  box_item 6 "HTTPS lock ON  (production)"
  box_item 7 "HTTPS lock OFF (back to setup)"
  box_bottom; echo
  local c; read -r -p "Choice [0-7]: " c
  case "$c" in
    0) return ;;
    1) cmd_cert ;;
    2) cmd_cert_cf ;;
    3) cmd_cert_zerossl ;;
    4) cmd_csr ;;
    5) cmd_cert_import ;;
    6) lock_on ;;
    7) lock_off ;;
    *) warn "Invalid option." ;;
  esac
}

prompt_domain_setup(){
  echo
  warn "The panel is in the SETUP stage on plain HTTP — check it works using the addresses above."
  info "Next: get an HTTPS certificate (free Let's Encrypt/ZeroSSL, or your own e.g. Certum), then turn the HTTPS lock on."
  local ans; read -r -p "Open the certificate menu now? [y/N] " ans
  if [[ "$ans" == "y" || "$ans" == "Y" ]]; then cmd_ssl_menu
  else info "Later: dotinschool-challenge ssl"; fi
}

cmd_log(){
  require_root
  box_top; box_title "dotinschool-challenge — Logs"
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
  cat > /etc/fail2ban/filter.d/dotinschool-challenge.conf <<'EOF'
[Definition]
failregex = ^.*msg="admin login failed" ip=<HOST>\s*$
ignoreregex =
EOF
  cat > /etc/fail2ban/jail.d/dotinschool-challenge.conf <<EOF
[dotinschool-challenge]
enabled = true
backend = systemd
journalmatch = _SYSTEMD_UNIT=${SERVICE_NAME}.service
filter = dotinschool-challenge
maxretry = 5
findtime = 10m
bantime = 1h
action = %(action_)s
EOF
  systemctl enable --now fail2ban >/dev/null 2>&1
  systemctl restart fail2ban
  ok "fail2ban is watching admin login attempts — 5 failures in 10 minutes bans the IP for 1 hour."
  warn "If you ever lock yourself out: fail2ban-client set dotinschool-challenge unbanip <your-ip>"
}

# ---- access with a VPN on: Cloudflare Tunnel -----------------------------------
# Many Iranian data centres drop *incoming* connections from foreign IPs, and a
# visitor with a VPN on arrives from a foreign IP — so the panel never loads for
# them. A tunnel turns that around: cloudflared on this server dials OUT to
# Cloudflare (outgoing traffic works — it's how updates reach GitHub) and keeps
# the connection open; VPN users open a Cloudflare https address and their
# requests come back down that connection. Users without a VPN keep using the
# normal IP:port address. --protocol http2 (TCP) is used because the default
# QUIC (UDP) is often blocked or throttled on Iranian networks.
#
# Tested from an Iranian network: a *named* tunnel (your own domain on
# Cloudflare + a tunnel token) connects fine — its edge, *.argotunnel.com:7844
# with TLS name h2.cftunnel.com, is reachable. The free *quick* tunnel is not:
# trycloudflare.com is filtered in Iran (poisoned DNS, and connections to it are
# cut by name), so quick mode only works on servers outside Iran.
TUNNEL_SERVICE="dotinschool-challenge-tunnel"
TUNNEL_UNIT="/etc/systemd/system/${TUNNEL_SERVICE}.service"
CLOUDFLARED="/usr/local/bin/cloudflared"

install_cloudflared(){
  if [[ -x "$CLOUDFLARED" ]] && "$CLOUDFLARED" --version >/dev/null 2>&1; then return; fi
  info "Installing cloudflared..."
  local arch; arch=$(arch_tag)
  if curl -fsSL --connect-timeout 15 --max-time 300 -o "${CLOUDFLARED}.tmp" \
      "https://github.com/cloudflare/cloudflared/releases/latest/download/cloudflared-linux-${arch}"; then
    mv -f "${CLOUDFLARED}.tmp" "$CLOUDFLARED"; chmod +x "$CLOUDFLARED"
  else
    rm -f "${CLOUDFLARED}.tmp"
    # GitHub unreachable from this network: fall back to Cloudflare's own package repo
    local pm; pm=$(detect_pkg_manager)
    case "$pm" in
      apt)
        mkdir -p --mode=0755 /usr/share/keyrings
        curl -fsSL https://pkg.cloudflare.com/cloudflare-main.gpg -o /usr/share/keyrings/cloudflare-main.gpg
        echo "deb [signed-by=/usr/share/keyrings/cloudflare-main.gpg] https://pkg.cloudflare.com/cloudflared any main" > /etc/apt/sources.list.d/cloudflared.list
        apt-get update -y >/dev/null && apt-get install -y cloudflared >/dev/null
        [[ -x /usr/bin/cloudflared ]] && ln -sf /usr/bin/cloudflared "$CLOUDFLARED"
        ;;
      dnf|yum)
        curl -fsSL https://pkg.cloudflare.com/cloudflared-ascii.repo -o /etc/yum.repos.d/cloudflared.repo
        "$pm" install -y cloudflared >/dev/null
        [[ -x /usr/bin/cloudflared ]] && ln -sf /usr/bin/cloudflared "$CLOUDFLARED"
        ;;
    esac
  fi
  "$CLOUDFLARED" --version >/dev/null 2>&1 || die "Could not download cloudflared. If GitHub is blocked here, run \"dotinschool-challenge warp\" first and try again."
  ok "cloudflared installed: $("$CLOUDFLARED" --version 2>&1 | head -n1)"
}

# the local address cloudflared forwards to (https + no-verify when the panel terminates TLS itself)
tunnel_origin_args(){
  local port; port=$(get_env_val PORT); port=${port:-8081}
  if [[ -n "$(get_env_val SSL_CERT_PATH)" ]]; then
    echo "--url https://127.0.0.1:${port} --no-tls-verify"
  else
    echo "--url http://127.0.0.1:${port}"
  fi
}

write_tunnel_unit(){
  cat > "$TUNNEL_UNIT" <<EOF
[Unit]
Description=dotinschool-challenge - Cloudflare Tunnel (access with a VPN on)
After=network-online.target ${SERVICE_NAME}.service
Wants=network-online.target

[Service]
Type=simple
ExecStart=${CLOUDFLARED} tunnel --no-autoupdate --protocol http2 $1
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
EOF
  systemctl daemon-reload
  systemctl enable "$TUNNEL_SERVICE" >/dev/null 2>&1
  systemctl restart "$TUNNEL_SERVICE"
}

# the current quick-tunnel address, read from the tunnel's own log
quick_tunnel_url(){
  journalctl -u "$TUNNEL_SERVICE" --no-pager -n 300 2>/dev/null \
    | grep -oE 'https://[a-z0-9-]+\.trycloudflare\.com' | tail -n1 || true
}

print_tunnel_info(){
  local url="$1" adminpath; adminpath=$(get_env_val ADMIN_PATH); adminpath=${adminpath:-admin}
  echo
  ok "Address for users WITH a VPN on:   ${url}/"
  ok "Admin panel through the tunnel:     ${url}/${adminpath}/"
  info "Users without a VPN keep using the normal address (dotinschool-challenge admin -> 1)."
}

# Behind the tunnel cloudflared terminates TLS and reports the visitor in
# CF-Connecting-IP: tell the app so its TLS check and rate limits see the truth.
tunnel_env(){
  set_env_val CLIENT_IP_HEADER CF-Connecting-IP
  set_env_val TLS_PROXY 1
  systemctl restart "$SERVICE_NAME" || true
}

tunnel_quick(){
  require_root
  [[ -f "$ENV_FILE" ]] || die "Not installed yet — run \"dotinschool-challenge install\" first."
  install_cloudflared
  write_tunnel_unit "$(tunnel_origin_args)"
  set_env_val TUNNEL_MODE quick
  tunnel_env
  info "Connecting to Cloudflare (up to 60 seconds)..."
  local url="" i
  for i in $(seq 1 60); do
    url=$(quick_tunnel_url)
    [[ -n "$url" ]] && break
    sleep 1
  done
  if [[ -z "$url" ]]; then
    warn "No free address could be obtained: trycloudflare.com is filtered on Iranian networks."
    warn "Use your own domain instead (works from Iran): dotinschool-challenge tunnel token"
    tunnel_off >/dev/null
    return
  fi
  print_tunnel_info "$url"
  warn "This free address changes whenever the tunnel or the server restarts — check it again with: dotinschool-challenge tunnel status"
  warn "For a permanent address, use your own domain on Cloudflare (dotinschool-challenge tunnel token <TOKEN>)."
}

tunnel_token(){
  require_root
  [[ -f "$ENV_FILE" ]] || die "Not installed yet — run \"dotinschool-challenge install\" first."
  local token="${1:-}"
  if [[ -z "$token" ]]; then
    echo "In the Cloudflare dashboard: Zero Trust -> Networks -> Tunnels -> Create a tunnel (Cloudflared),"
    echo "copy the token, and add a Public Hostname (e.g. game.example.com) pointing to:"
    echo "  $(tunnel_origin_args | sed -e 's/--url //' -e 's/ --no-tls-verify/  (enable \"No TLS Verify\")/')"
    read -r -p "Tunnel token: " token
  fi
  [[ -n "$token" ]] || die "A tunnel token is required."
  install_cloudflared
  write_tunnel_unit "run --token ${token}"
  set_env_val TUNNEL_MODE token
  tunnel_env
  sleep 5
  if systemctl is-active --quiet "$TUNNEL_SERVICE"; then
    ok "Tunnel is running with your Cloudflare token — users with a VPN on can use the Public Hostname you set."
  else
    warn "The tunnel did not start. Check: journalctl -u ${TUNNEL_SERVICE} -e"
  fi
}

tunnel_status(){
  if ! systemctl is-active --quiet "$TUNNEL_SERVICE" 2>/dev/null; then
    warn "The VPN-access tunnel is off. Turn it on with: dotinschool-challenge tunnel"
    return
  fi
  if [[ "$(get_env_val TUNNEL_MODE)" == "token" ]]; then
    ok "Tunnel is running (your own domain — see the Public Hostname in the Cloudflare dashboard)."
    return
  fi
  local url; url=$(quick_tunnel_url)
  if [[ -n "$url" ]]; then print_tunnel_info "$url"; else warn "Tunnel is running but has no address yet — try again in a few seconds."; fi
}

tunnel_off(){
  require_root
  systemctl disable --now "$TUNNEL_SERVICE" >/dev/null 2>&1 || true
  rm -f "$TUNNEL_UNIT"; systemctl daemon-reload
  set_env_val TUNNEL_MODE ""
  ok "VPN-access tunnel turned off. The normal IP:port address keeps working."
}

cmd_tunnel(){
  case "${1:-}" in
    quick|on) tunnel_quick ;;
    token) tunnel_token "${2:-}" ;;
    status) tunnel_status ;;
    off) tunnel_off ;;
    "")
      echo; tunnel_status; echo
      box_top; box_title "Access with a VPN on (Cloudflare Tunnel)"
      box_item 0 "Back"; box_div
      box_item 1 "Use my domain (recommended, works in Iran)"
      box_item 2 "Free trycloudflare URL (often blocked in Iran)"
      box_item 3 "Show the current status / address"
      box_item 4 "Turn off"
      box_bottom
      read -r -p "Choice [0-4]: " c
      case "$c" in 1) tunnel_token ;; 2) tunnel_quick ;; 3) tunnel_status ;; 4) tunnel_off ;; *) return ;; esac
      ;;
    *) die "Usage: dotinschool-challenge tunnel [on|token <TOKEN>|status|off]" ;;
  esac
}

cmd_banlog(){
  require_root
  command -v fail2ban-client >/dev/null 2>&1 || die "fail2ban is not installed — run \"dotinschool-challenge fail2ban\" first."
  fail2ban-client status dotinschool-challenge 2>/dev/null || warn "The dotinschool-challenge jail isn't active yet."
}

cmd_service(){ require_root; systemctl "$1" "$SERVICE_NAME"; [[ "$1" == "status" ]] || ok "Command \"$1\" applied to the service."; }

show_status(){
  if ! [[ -f "$ENV_FILE" ]]; then echo -e "Panel status: ${RED}not installed${NC}"; return; fi
  systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null && echo -e "Panel status: ${GREEN}running${NC}" || echo -e "Panel status: ${YELLOW}stopped${NC}"
  systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null && echo -e "Start on boot: ${GREEN}yes${NC}" || echo -e "Start on boot: ${RED}no${NC}"
  https_locked && echo -e "HTTPS lock:    ${GREEN}ON (production)${NC}" || echo -e "HTTPS lock:    ${YELLOW}OFF (setup stage)${NC}"
}

show_menu(){
  box_top; box_title "dotinschool-challenge — Management Script"
  box_item 0 "Exit"; box_div
  box_item 1 "Install"
  box_item 2 "Update to the latest version"
  box_item 3 "Update this menu script"
  box_item 4 "Uninstall"; box_div
  box_item 5 "Admin panel settings"
  box_item 6 "SSL certificate & HTTPS lock"
  box_item 7 "HTTPS lock status"
  box_item 8 "Logs"
  box_item 9 "Factory reset"
  box_item 10 "Brute-force protection (fail2ban)"
  box_item 11 "Fix GitHub connectivity (Cloudflare WARP)"
  box_item 18 "Access with a VPN on (Cloudflare Tunnel)"; box_div
  box_item 12 "Start"; box_item 13 "Stop"; box_item 14 "Restart"; box_item 15 "Status"; box_div
  box_item 16 "Enable autostart"; box_item 17 "Disable autostart"
  box_bottom; echo; show_status; echo
  read -r -p "Choice [0-18]: " choice
  case "$choice" in
    1) cmd_install ;; 2) cmd_update ;; 3) cmd_update_menu ;; 4) cmd_uninstall ;;
    5) cmd_admin ;; 6) cmd_ssl_menu ;; 7) cmd_lock status ;; 8) cmd_log ;; 9) cmd_reset ;; 10) install_fail2ban ;; 11) install_warp ;;
    12) cmd_service start ;; 13) cmd_service stop ;; 14) cmd_service restart ;; 15) cmd_service status ;;
    16) require_root; systemctl enable "$SERVICE_NAME" >/dev/null; ok "Autostart enabled." ;;
    17) require_root; systemctl disable "$SERVICE_NAME" >/dev/null; ok "Autostart disabled." ;;
    18) cmd_tunnel ;;
    0) exit 0 ;;
    *) warn "Enter a number between 0 and 18." ;;
  esac
}

main(){
  case "${1:-}" in
    install) cmd_install ;;
    update) cmd_update ;;
    update-menu) cmd_update_menu ;;
    uninstall) cmd_uninstall ;;
    admin) cmd_admin ;;
    ssl) cmd_ssl_menu ;;
    cert) cmd_cert ;;
    cert-cf) cmd_cert_cf ;;
    cert-zerossl) cmd_cert_zerossl ;;
    csr) cmd_csr ;;
    cert-import) cmd_cert_import ;;
    lock) cmd_lock "${2:-status}" ;;
    log) cmd_log ;;
    reset) cmd_reset ;;
    fail2ban) install_fail2ban ;;
    banlog) cmd_banlog ;;
    warp) install_warp ;;
    tunnel) shift; cmd_tunnel "$@" ;;
    start|stop|restart|status) cmd_service "$1" ;;
    enable) require_root; systemctl enable "$SERVICE_NAME" >/dev/null; ok "Autostart enabled." ;;
    disable) require_root; systemctl disable "$SERVICE_NAME" >/dev/null; ok "Autostart disabled." ;;
    # first run on a fresh server installs straight away; afterwards: the menu
    "") if [[ -f "$ENV_FILE" ]]; then show_menu; else cmd_install; fi ;;
    *) die "Invalid command: $1" ;;
  esac
}
main "$@"
