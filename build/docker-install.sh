#!/usr/bin/env bash
# =============================================================================
# dotinschool — Docker installer for a Linux host.
#
#   bash <(curl -Ls https://raw.githubusercontent.com/YOUR_ORG/dotinschool/main/build/docker-install.sh)
#
# Installs Docker if missing, clones the repo to /opt/dotinschool, generates a
# .env with a random admin password + secret admin path, brings the stack up,
# and installs a "dotinschool" wrapper command:
#
#   dotinschool up | down | restart | logs | update | admin | ps
# =============================================================================
set -euo pipefail

REPO_URL="${DOTINSCHOOL_REPO_URL:-https://github.com/YOUR_ORG/dotinschool.git}"
BRANCH="${DOTINSCHOOL_BRANCH:-main}"
INSTALL_DIR="/opt/dotinschool"
CLI_PATH="/usr/local/bin/dotinschool"
COMPOSE=(docker compose -f build/docker-compose.yml)

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[0;33m'; CYAN='\033[0;36m'; NC='\033[0m'
info(){ echo -e "${CYAN}[i]${NC} $*"; }
ok(){ echo -e "${GREEN}[✓]${NC} $*"; }
warn(){ echo -e "${YELLOW}[!]${NC} $*"; }
die(){ echo -e "${RED}[x]${NC} $*" >&2; exit 1; }

require_root(){ [[ ${EUID:-$(id -u)} -eq 0 ]] || die "با کاربر root اجرا کنید (sudo)."; }

install_docker(){
  if command -v docker >/dev/null 2>&1 && docker compose version >/dev/null 2>&1; then
    ok "Docker از قبل نصب است."; return
  fi
  info "نصب Docker..."
  curl -fsSL https://get.docker.com | sh >/dev/null
  systemctl enable --now docker >/dev/null 2>&1 || true
  docker compose version >/dev/null 2>&1 || die "افزونه‌ی docker compose در دسترس نیست."
  ok "Docker نصب شد."
}

random_password(){ openssl rand -base64 18 | tr -dc 'A-Za-z0-9' | head -c 20; }
random_admin_path(){ echo "panel-$(openssl rand -hex 6)"; }

fetch_repo(){
  if [[ -d "$INSTALL_DIR/.git" ]]; then
    git -C "$INSTALL_DIR" fetch --depth 1 origin "$BRANCH" >/dev/null
    git -C "$INSTALL_DIR" reset --hard "origin/${BRANCH}" >/dev/null
  else
    rm -rf "$INSTALL_DIR"
    git clone --depth 1 --branch "$BRANCH" "$REPO_URL" "$INSTALL_DIR" >/dev/null
  fi
  ok "سورس در ${INSTALL_DIR} آماده شد."
}

write_env(){
  local env="${INSTALL_DIR}/.env"
  [[ -f "$env" ]] && { ok ".env از قبل وجود دارد — دست‌نخورده ماند."; return; }
  cat > "$env" <<EOF
APP_PORT=8080
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
  ok ".env ساخته شد — رمز ادمین تصادفی و مسیر مخفی تولید شد."
}

install_cli(){
  cat > "$CLI_PATH" <<'WRAP'
#!/usr/bin/env bash
set -euo pipefail
cd /opt/dotinschool
C=(docker compose -f build/docker-compose.yml)
case "${1:-}" in
  up)      "${C[@]}" up -d ;;
  down)    "${C[@]}" down ;;
  restart) "${C[@]}" restart ;;
  logs)    "${C[@]}" logs -f app ;;
  ps)      "${C[@]}" ps ;;
  update)  git fetch --depth 1 origin main && git reset --hard origin/main && "${C[@]}" up -d --build ;;
  admin)
    env=.env
    echo "پنل ادمین:  http://<server-ip>:$(grep -E '^APP_PORT=' $env | cut -d= -f2)/$(grep -E '^ADMIN_PATH=' $env | cut -d= -f2)/"
    echo "نام کاربری:  $(grep -E '^ADMIN_USERNAME=' $env | cut -d= -f2)"
    echo "رمز عبور:    $(grep -E '^ADMIN_PASSWORD=' $env | cut -d= -f2)"
    ;;
  *) echo "usage: dotinschool {up|down|restart|logs|ps|update|admin}" ; exit 1 ;;
esac
WRAP
  chmod +x "$CLI_PATH"
  ok "دستور «dotinschool» نصب شد."
}

main(){
  require_root
  command -v git >/dev/null 2>&1 || { curl -fsSL https://get.docker.com >/dev/null 2>&1 || true; }
  install_docker
  command -v git >/dev/null 2>&1 || die "git نصب نیست — آن را نصب و دوباره اجرا کنید."
  fetch_repo
  write_env
  install_cli
  ( cd "$INSTALL_DIR" && "${COMPOSE[@]}" up -d --build )
  echo
  ( cd "$INSTALL_DIR" && "$CLI_PATH" admin )
  echo
  warn "برای HTTPS با دامنه: DOMAIN را در ${INSTALL_DIR}/.env تنظیم کنید و اجرا کنید:"
  echo "  cd ${INSTALL_DIR} && docker compose -f build/docker-compose.yml -f build/docker-compose.prod.yml up -d"
}
main "$@"
