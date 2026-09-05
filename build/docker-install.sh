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
#   dotinschool up | down | restart | logs | update | admin | ps | rebuild
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
  curl -fsSL https://get.docker.com | sh >/dev/null
  systemctl enable --now docker >/dev/null 2>&1 || true
  docker compose version >/dev/null 2>&1 || die "The docker compose plugin is not available."
  ok "Docker installed."
}

arch_tag(){ case "$(uname -m)" in x86_64|amd64) echo amd64 ;; aarch64|arm64) echo arm64 ;; *) die "Unsupported architecture: $(uname -m)";; esac; }

# No version is hardcoded as the primary source: go.dev sometimes lists a
# release as stable in its metadata a little before every platform's archive
# has finished uploading, so a pinned version can 404. Ask go.dev's JSON
# endpoint for the actual current release names instead; GO_VERSION is only
# the last-resort fallback if that lookup itself fails.
resolve_go_tarballs(){
  local arch="$1" json
  json=$(curl -fsSL --connect-timeout 15 "https://go.dev/dl/?mode=json" 2>/dev/null) || true
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
  local tmp="" tgz base found=0
  for tgz in "${candidates[@]}"; do
    for base in "https://go.dev/dl" "https://mirrors.aliyun.com/golang" "https://golang.google.cn/dl"; do
      tmp=$(mktemp)
      if curl -fsSL --connect-timeout 15 "${base}/${tgz}" -o "$tmp" && [[ -s "$tmp" ]]; then found=1; break 2; fi
      rm -f "$tmp"; tmp=""
    done
  done
  [[ $found -eq 1 ]] || die "Failed to download Go from any mirror/release."
  rm -rf "$GO_ROOT"; tar -C /usr/local -xzf "$tmp"; rm -f "$tmp"
  export PATH="${GO_ROOT}/bin:$PATH"
  ok "Go $("${GO_ROOT}/bin/go" version | awk '{print $3}') installed."
}

random_password(){ openssl rand -base64 18 | tr -dc 'A-Za-z0-9' | head -c 20; }
random_admin_path(){ echo "panel-$(openssl rand -hex 6)"; }

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
  rm -rf "$INSTALL_DIR"
  git_nc clone --depth 1 --branch "$BRANCH" "$url" "$INSTALL_DIR" >/dev/null
  ok "Source ready at ${INSTALL_DIR}."
}

write_env(){
  local env="${INSTALL_DIR}/.env"
  [[ -f "$env" ]] && { ok ".env already exists — left untouched."; return; }
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
  ok ".env created — random admin password and secret path generated."
}

build_image(){
  info "Compiling the binary on the host + building the image (no go build inside the container)..."
  ( cd "$INSTALL_DIR" && PATH="${GO_ROOT}/bin:$PATH" bash scripts/docker-build.sh "$(arch_tag)" )
  ok "Image dotinschool:local built."
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
  rebuild) PATH="/usr/local/go/bin:$PATH" bash scripts/docker-build.sh "$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')" && "${C[@]}" up -d ;;
  update)  GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null git fetch --depth 1 origin main \
             && GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_SYSTEM=/dev/null git reset --hard origin/main \
             && PATH="/usr/local/go/bin:$PATH" bash scripts/docker-build.sh "$(uname -m | sed 's/x86_64/amd64/;s/aarch64/arm64/')" \
             && "${C[@]}" up -d ;;
  admin)
    e=.env
    echo "Admin panel:  http://<server-ip>:$(grep -E '^APP_PORT=' $e | cut -d= -f2)/$(grep -E '^ADMIN_PATH=' $e | cut -d= -f2)/"
    echo "Username:     $(grep -E '^ADMIN_USERNAME=' $e | cut -d= -f2)"
    echo "Password:     $(grep -E '^ADMIN_PASSWORD=' $e | cut -d= -f2)"
    ;;
  *) echo "usage: dotinschool {up|down|restart|logs|ps|rebuild|update|admin}"; exit 1 ;;
esac
WRAP
  chmod +x "$CLI_PATH"
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
  ( cd "$INSTALL_DIR" && docker compose -f build/docker-compose.yml up -d )
  echo
  ( cd "$INSTALL_DIR" && "$CLI_PATH" admin )
  echo
  warn "For HTTPS with a domain: set DOMAIN in ${INSTALL_DIR}/.env and run:"
  echo "  cd ${INSTALL_DIR} && docker compose -f build/docker-compose.yml -f build/docker-compose.prod.yml up -d"
}
main "$@"
