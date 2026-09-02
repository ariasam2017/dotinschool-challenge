#!/usr/bin/env bash
# =============================================================================
# dotinschool — Linux installer / management script (x-ui / Sanaei-panel style)
#
# Install (one line):
#   bash <(curl -Ls https://raw.githubusercontent.com/YOUR_ORG/dotinschool/main/scripts/install.sh)
#
# After install the "dotinschool" command is available:
#   dotinschool                 interactive management menu
#   dotinschool install         install / reinstall
#   dotinschool update          pull the latest version + rebuild + restart
#   dotinschool update-menu     update just this CLI script
#   dotinschool admin           show / change admin address, credentials, subdomain
#   dotinschool cert            issue / renew a Let's Encrypt certificate
#   dotinschool start|stop|restart|status
#   dotinschool enable|disable  auto-start on boot
#   dotinschool uninstall
# =============================================================================
set -euo pipefail

REPO_URL="${DOTINSCHOOL_REPO_URL:-https://github.com/YOUR_ORG/dotinschool.git}"
BRANCH="${DOTINSCHOOL_BRANCH:-main}"
CLI_SOURCE_URL="${DOTINSCHOOL_CLI_SOURCE_URL:-https://raw.githubusercontent.com/YOUR_ORG/dotinschool/main/scripts/install.sh}"

INSTALL_DIR="/opt/dotinschool"
CONFIG_DIR="/etc/dotinschool"
ENV_FILE="${CONFIG_DIR}/dotinschool.env"
SSL_DIR="${CONFIG_DIR}/ssl"
SERVICE_NAME="dotinschool"
SERVICE_FILE="/etc/systemd/system/${SERVICE_NAME}.service"
CLI_PATH="/usr/local/bin/dotinschool"
GO_VERSION="1.27.1"
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

require_root(){ [[ ${EUID:-$(id -u)} -eq 0 ]] || die "این اسکریپت باید با کاربر root اجرا شود (sudo bash install.sh)."; }

detect_pkg_manager(){
  if command -v apt-get >/dev/null 2>&1; then echo apt
  elif command -v dnf >/dev/null 2>&1; then echo dnf
  elif command -v yum >/dev/null 2>&1; then echo yum
  else die "توزیع لینوکس شما پشتیبانی نمی‌شود (فقط apt/dnf/yum)."; fi
}

install_prereqs(){
  local pm; pm=$(detect_pkg_manager)
  info "نصب پیش‌نیازها (curl, git, tar, openssl)..."
  case "$pm" in
    apt) apt-get update -y >/dev/null; apt-get install -y curl git tar openssl ca-certificates >/dev/null ;;
    dnf) dnf install -y curl git tar openssl ca-certificates >/dev/null ;;
    yum) yum install -y curl git tar openssl ca-certificates >/dev/null ;;
  esac
  ok "پیش‌نیازها آماده شد."
}

arch_tag(){
  case "$(uname -m)" in
    x86_64|amd64) echo amd64 ;;
    aarch64|arm64) echo arm64 ;;
    *) die "معماری پردازنده پشتیبانی نمی‌شود: $(uname -m)" ;;
  esac
}

go_ok(){ [[ -x "${GO_ROOT}/bin/go" ]] && "${GO_ROOT}/bin/go" version 2>/dev/null | grep -q "go${GO_VERSION%.*}"; }

install_go(){
  if go_ok; then ok "Go از قبل نصب است ($("${GO_ROOT}/bin/go" version | awk '{print $3}'))."; return; fi
  local arch tgz url mirrors
  arch=$(arch_tag)
  tgz="go${GO_VERSION}.linux-${arch}.tar.gz"
  mirrors=(
    "https://go.dev/dl/${tgz}"
    "https://mirrors.aliyun.com/golang/${tgz}"
    "https://golang.google.cn/dl/${tgz}"
  )
  info "دانلود Go ${GO_VERSION} (${arch})..."
  local tmp; tmp=$(mktemp)
  for url in "${mirrors[@]}"; do
    if curl -fsSL --connect-timeout 15 "$url" -o "$tmp"; then break; fi
    tmp=""; tmp=$(mktemp)
  done
  [[ -s "$tmp" ]] || die "دانلود Go از هیچ‌کدام از منابع ممکن نشد."
  rm -rf "$GO_ROOT"
  tar -C /usr/local -xzf "$tmp"
  rm -f "$tmp"
  go_ok || die "نصب Go ناموفق بود."
  ok "Go $("${GO_ROOT}/bin/go" version | awk '{print $3}') نصب شد."
}

random_password(){
  local p; p=$(openssl rand -base64 18 2>/dev/null | tr -dc 'A-Za-z0-9' | head -c 20) || true
  [[ -n "$p" ]] || die "ساخت رمز تصادفی ممکن نشد (openssl سالم است؟)."
  echo "$p"
}
random_admin_path(){ echo "panel-$(openssl rand -hex 6)"; }

port_in_use(){ (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null && { exec 3<&- 2>/dev/null; exec 3>&- 2>/dev/null; return 0; }; return 1; }
find_free_port(){ local port="$1" tries=0; while port_in_use "$port"; do port=$((port+1)); tries=$((tries+1)); [[ $tries -lt 50 ]] || { echo "$1"; return; }; done; echo "$port"; }

fetch_app(){
  if [[ -d "$INSTALL_DIR/.git" ]]; then
    info "به‌روزرسانی سورس از ${REPO_URL} (${BRANCH})..."
    git -C "$INSTALL_DIR" fetch --depth 1 origin "$BRANCH" >/dev/null
    git -C "$INSTALL_DIR" reset --hard "origin/${BRANCH}" >/dev/null
  else
    info "دریافت سورس از ${REPO_URL} (${BRANCH})..."
    rm -rf "$INSTALL_DIR"
    git clone --depth 1 --branch "$BRANCH" "$REPO_URL" "$INSTALL_DIR" >/dev/null
  fi
  ok "سورس در ${INSTALL_DIR} آماده است."
}

build_binary(){
  info "کامپایل باینری (CGO خاموش، فرانت از قبل بیلد و داخل مخزن است)..."
  ( cd "$INSTALL_DIR" && CGO_ENABLED=0 "${GO_ROOT}/bin/go" build -trimpath -ldflags="-s -w" -o "${BIN_NAME}.new" ./cmd/server )
  mv "${INSTALL_DIR}/${BIN_NAME}.new" "${INSTALL_DIR}/${BIN_NAME}"
  chmod +x "${INSTALL_DIR}/${BIN_NAME}"
  ok "باینری ساخته شد: ${INSTALL_DIR}/${BIN_NAME}"
}

write_env_file(){
  mkdir -p "$CONFIG_DIR"
  if [[ ! -f "$ENV_FILE" ]]; then
    local pass adminpath port
    pass=$(random_password); adminpath=$(random_admin_path); port=$(find_free_port 8080)
    [[ "$port" == "8080" ]] || warn "پورت ۸۰۸۰ اشغال بود — از ${port} استفاده می‌شود."
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
    ok "فایل کانفیگ ساخته شد — رمز ادمین تصادفی و مسیر مخفی تولید شد."
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
  ok "یونیت systemd نوشته شد."
}

open_firewall_port(){
  local port="$1"
  if command -v ufw >/dev/null 2>&1 && ufw status 2>/dev/null | grep -q "Status: active"; then ufw allow "${port}/tcp" >/dev/null 2>&1 || true; fi
  if command -v firewall-cmd >/dev/null 2>&1 && systemctl is-active --quiet firewalld 2>/dev/null; then
    firewall-cmd --permanent --add-port="${port}/tcp" >/dev/null 2>&1 || true; firewall-cmd --reload >/dev/null 2>&1 || true
  fi
}

install_cli(){ cp "${INSTALL_DIR}/scripts/install.sh" "$CLI_PATH"; chmod +x "$CLI_PATH"; ok "دستور «dotinschool» نصب شد."; }

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
  ok "پنل کاربر (آدرس عمومی):        ${scheme}://${ip}:${port}/"
  ok "پنل ادمین (مسیر مخفی — فقط شما): ${scheme}://${ip}:${port}/${adminpath}/"
  [[ -n "$subdomain" ]] && ok "پنل ادمین (ساب‌دامین اختصاصی):  ${scheme}://${subdomain}/"
  ok "نام کاربری ادمین:              $(get_env_val ADMIN_USERNAME)"
  ok "رمز عبور ادمین:                $(get_env_val ADMIN_PASSWORD)"
  echo
  warn "این اطلاعات را جایی امن ذخیره کنید — دوباره فقط با «dotinschool admin» قابل مشاهده است."
}

cmd_install(){
  require_root
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
      warn "پورت ${oldport} اشغال است — تغییر به ${newport}."
      set_env_val PORT "$newport"; systemctl restart "$SERVICE_NAME"; sleep 2; tries=$((tries+1))
    else break; fi
  done
  open_firewall_port "$(get_env_val PORT)"
  if systemctl is-active --quiet "$SERVICE_NAME"; then ok "نصب کامل شد و سرویس در حال اجراست."
  else err "سرویس بالا نیامد — لاگ:"; journalctl -u "$SERVICE_NAME" --no-pager -n 30 || true; exit 1; fi
  print_access_info
}

cmd_update(){
  require_root
  [[ -d "$INSTALL_DIR" ]] || die "هنوز نصب نشده — اول «dotinschool install» را بزنید."
  local before after
  before=$(git -C "$INSTALL_DIR" rev-parse HEAD 2>/dev/null || echo "")
  fetch_app
  after=$(git -C "$INSTALL_DIR" rev-parse HEAD 2>/dev/null || echo "")
  install_cli
  if [[ -n "$before" && "$before" == "$after" ]]; then ok "از قبل به‌روز است — نسخه‌ی جدیدی نیست."; return; fi
  build_binary
  systemctl restart "$SERVICE_NAME"
  ok "به‌روزرسانی انجام و سرویس ری‌استارت شد."
}

cmd_update_menu(){
  require_root
  info "دریافت آخرین نسخه‌ی اسکریپت مدیریتی..."
  local tmp; tmp=$(mktemp)
  curl -fsSL "$CLI_SOURCE_URL" -o "$tmp" || { rm -f "$tmp"; die "دانلود ناموفق بود."; }
  bash -n "$tmp" 2>/dev/null || { rm -f "$tmp"; die "اسکریپت دانلودی از تست نحو رد نشد — چیزی تغییر نکرد."; }
  install -m 755 "$tmp" "$CLI_PATH"; rm -f "$tmp"
  ok "اسکریپت مدیریتی به‌روز شد."
}

cmd_uninstall(){
  require_root
  read -r -p "$(echo -e "${YELLOW}سرویس، فایل‌ها و کانفیگ حذف می‌شوند. مطمئن هستید؟ [y/N] ${NC}")" ans
  [[ "$ans" == "y" || "$ans" == "Y" ]] || { info "لغو شد."; return; }
  systemctl stop "$SERVICE_NAME" 2>/dev/null || true
  systemctl disable "$SERVICE_NAME" 2>/dev/null || true
  rm -f "$SERVICE_FILE"; systemctl daemon-reload
  rm -rf "$INSTALL_DIR" "$CONFIG_DIR" "$CLI_PATH"
  ok "dotinschool کاملاً حذف شد."
}

cmd_admin(){
  require_root
  [[ -f "$ENV_FILE" ]] || die "هنوز نصب نشده."
  box_top; box_title "dotinschool — تنظیمات پنل ادمین"
  box_item 0 "بازگشت"; box_div
  box_item 1 "نمایش اطلاعات دسترسی فعلی"
  box_item 2 "تغییر نام کاربری"
  box_item 3 "رمز عبور تصادفی جدید"
  box_item 4 "تغییر پورت"
  box_item 5 "تغییر مسیر مخفی"
  box_item 6 "تنظیم ساب‌دامین ادمین"
  box_bottom; echo
  read -r -p "انتخاب [0-6]: " choice
  case "$choice" in
    0) return ;;
    1) print_access_info ;;
    2) read -r -p "نام کاربری جدید: " u; set_env_val ADMIN_USERNAME "$u"; systemctl restart "$SERVICE_NAME"; ok "تغییر یافت."; print_access_info ;;
    3) local p; p=$(random_password); set_env_val ADMIN_PASSWORD "$p"; systemctl restart "$SERVICE_NAME"; ok "رمز جدید ساخته شد."; print_access_info ;;
    4) read -r -p "پورت جدید: " p; set_env_val PORT "$p"; open_firewall_port "$p"; systemctl restart "$SERVICE_NAME"; ok "تغییر یافت."; print_access_info ;;
    5) local np; np=$(random_admin_path); set_env_val ADMIN_PATH "$np"; systemctl restart "$SERVICE_NAME"; ok "مسیر مخفی عوض شد — آدرس قبلی دیگر کار نمی‌کند."; print_access_info ;;
    6) read -r -p "ساب‌دامین کامل (خالی = غیرفعال): " sd; set_env_val ADMIN_SUBDOMAIN "$sd"; systemctl restart "$SERVICE_NAME";
       [[ -n "$sd" ]] && warn "یک رکورد DNS از نوع A برای «${sd}» به IP این سرور اضافه کنید. برای SSL دوباره «dotinschool cert» را اجرا کنید."
       print_access_info ;;
    *) warn "گزینه نامعتبر." ;;
  esac
}

install_acme(){ [[ -x "$HOME/.acme.sh/acme.sh" ]] || { info "نصب acme.sh..."; curl -fsSL https://get.acme.sh | sh -s email="admin@$(hostname -f 2>/dev/null || echo localhost)" >/dev/null 2>&1; }; }

cmd_cert(){
  require_root
  echo "برای گرفتن گواهی SSL به یک دامنه نیاز است که رکورد A آن به IP این سرور اشاره کند."
  warn "اگر فقط با IP خام کار می‌کنید و دامنه ندارید، این مرحله را رد کنید — پنل روی HTTP معمولی بدون خطا کار می‌کند."
  read -r -p "دامنه (خالی = انصراف): " domain
  [[ -n "$domain" ]] || { info "لغو شد."; return; }
  local domain_args=(-d "$domain") subdomain; subdomain=$(get_env_val ADMIN_SUBDOMAIN)
  if [[ -n "$subdomain" && "$subdomain" != "$domain" ]]; then
    read -r -p "ساب‌دامین ادمین (${subdomain}) هم زیر همین گواهی باشد؟ [Y/n] " inc
    [[ "$inc" != "n" && "$inc" != "N" ]] && domain_args+=(-d "$subdomain")
  fi
  install_acme
  mkdir -p "${SSL_DIR}/${domain}"
  info "توقف موقت سرویس برای آزادسازی پورت ۸۰..."
  systemctl stop "$SERVICE_NAME" 2>/dev/null || true
  if ! "$HOME/.acme.sh/acme.sh" --issue "${domain_args[@]}" --standalone --httpport 80; then
    err "صدور گواهی ناموفق بود — دامنه‌ها را و آزاد بودن پورت ۸۰ را بررسی کنید."
    systemctl start "$SERVICE_NAME" 2>/dev/null || true; return 1
  fi
  "$HOME/.acme.sh/acme.sh" --install-cert -d "$domain" \
    --fullchain-file "${SSL_DIR}/${domain}/fullchain.pem" \
    --key-file "${SSL_DIR}/${domain}/privkey.pem" \
    --reloadcmd "systemctl restart ${SERVICE_NAME}" >/dev/null
  set_env_val SSL_CERT_PATH "${SSL_DIR}/${domain}/fullchain.pem"
  set_env_val SSL_KEY_PATH "${SSL_DIR}/${domain}/privkey.pem"
  set_env_val PORT 443
  open_firewall_port 443
  systemctl start "$SERVICE_NAME"; sleep 1
  systemctl is-active --quiet "$SERVICE_NAME" && ok "گواهی صادر و فعال شد — پنل روی HTTPS است (تمدید خودکار با acme.sh)." || err "سرویس با HTTPS بالا نیامد — journalctl -u ${SERVICE_NAME} -n 50"
}

cmd_service(){ require_root; systemctl "$1" "$SERVICE_NAME"; [[ "$1" == "status" ]] || ok "دستور «$1» روی سرویس اعمال شد."; }

show_status(){
  if ! [[ -f "$ENV_FILE" ]]; then echo -e "وضعیت پنل: ${RED}نصب‌نشده${NC}"; return; fi
  systemctl is-active --quiet "$SERVICE_NAME" 2>/dev/null && echo -e "وضعیت پنل: ${GREEN}در حال اجرا${NC}" || echo -e "وضعیت پنل: ${YELLOW}متوقف${NC}"
  systemctl is-enabled --quiet "$SERVICE_NAME" 2>/dev/null && echo -e "اجرای خودکار در بوت: ${GREEN}بله${NC}" || echo -e "اجرای خودکار در بوت: ${RED}خیر${NC}"
}

show_menu(){
  box_top; box_title "dotinschool — اسکریپت مدیریت"
  box_item 0 "خروج"; box_div
  box_item 1 "نصب"
  box_item 2 "به‌روزرسانی به آخرین نسخه"
  box_item 3 "به‌روزرسانی خودِ منو"
  box_item 4 "حذف"; box_div
  box_item 5 "تنظیمات پنل ادمین"
  box_item 6 "گواهی SSL"; box_div
  box_item 7 "شروع"; box_item 8 "توقف"; box_item 9 "ری‌استارت"; box_item 10 "وضعیت"; box_div
  box_item 11 "فعال‌سازی اجرای خودکار"; box_item 12 "غیرفعال‌سازی اجرای خودکار"
  box_bottom; echo; show_status; echo
  read -r -p "انتخاب [0-12]: " choice
  case "$choice" in
    1) cmd_install ;; 2) cmd_update ;; 3) cmd_update_menu ;; 4) cmd_uninstall ;;
    5) cmd_admin ;; 6) cmd_cert ;;
    7) cmd_service start ;; 8) cmd_service stop ;; 9) cmd_service restart ;; 10) cmd_service status ;;
    11) require_root; systemctl enable "$SERVICE_NAME" >/dev/null; ok "اجرای خودکار فعال شد." ;;
    12) require_root; systemctl disable "$SERVICE_NAME" >/dev/null; ok "اجرای خودکار غیرفعال شد." ;;
    0) exit 0 ;;
    *) warn "عددی بین ۰ تا ۱۲ وارد کنید." ;;
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
    start|stop|restart|status) cmd_service "$1" ;;
    enable) require_root; systemctl enable "$SERVICE_NAME" >/dev/null; ok "اجرای خودکار فعال شد." ;;
    disable) require_root; systemctl disable "$SERVICE_NAME" >/dev/null; ok "اجرای خودکار غیرفعال شد." ;;
    "") show_menu ;;
    *) die "دستور نامعتبر: $1" ;;
  esac
}
main "$@"
