#!/usr/bin/env bash
# dotinschool-challenge — bring the whole thing up on Docker with ONE command:
#
#   ./docker-up.sh
#
# Compiles the Linux binary on the host (fast, reliable), packages it into a
# tiny image (a ~5s file copy — no in-container `go build`, so no "rpc ... EOF"),
# and starts the stack. Re-run it any time to rebuild + recreate.
set -euo pipefail
cd "$(dirname "$0")"

say(){ printf '\033[36m==> %s\033[0m\n' "$*"; }
die(){ printf '\033[31m[x] %s\033[0m\n' "$*" >&2; exit 1; }

GO="$(command -v go || true)"
[ -n "$GO" ] || [ -x "$HOME/go/bin/go" ] || die "Go not found — install it or add it to PATH."
[ -n "$GO" ] || GO="$HOME/sdk/go/bin/go"
command -v docker >/dev/null 2>&1 || die "Docker not found."
docker info >/dev/null 2>&1 || die "Docker is not running."

# .env with random admin password + secret path on first run
if [ ! -f .env ]; then
  say "creating .env (random admin password + secret path)"
  rnd(){ LC_ALL=C tr -dc 'A-Za-z0-9' </dev/urandom | head -c "$1"; }
  cat > .env <<EOF
APP_PORT=8081
ADMIN_USERNAME=administrator
ADMIN_PASSWORD=$(rnd 20)
ADMIN_PATH=panel-$(rnd 8)
ADMIN_SUBDOMAIN=
DOMAIN=
# IPPANEL_API_KEY=
# IPPANEL_PATTERN_CODE=
# IPPANEL_LOGIN_PATTERN_CODE=
# IPPANEL_ADMIN_NOTIFY_PATTERN_CODE=
# IPPANEL_ADMIN_PHONE=
# AI_API_KEY=
EOF
  chmod 600 .env
fi

# pick a free host port (exported for compose; .env is not modified)
port_free(){ ! (exec 3<>"/dev/tcp/127.0.0.1/$1") 2>/dev/null; }
PORT=""
for p in 8081 8082 8083 8084 3001 9001; do if port_free "$p"; then PORT="$p"; break; fi; done
export APP_PORT="${PORT:-8081}"
say "host port: $APP_PORT"

# Tell the container which address to advertise in its start-up log (otherwise
# it can only guess and prints 127.0.0.1). A DOMAIN in .env wins over this.
if ! grep -qE '^DOMAIN=[^[:space:]]' .env 2>/dev/null; then
  PUBHOST=$(curl -fsSL -4 --connect-timeout 8 https://api.ipify.org 2>/dev/null \
            || hostname -I 2>/dev/null | awk '{print $1}')
  if [ -n "${PUBHOST:-}" ]; then
    if grep -qE '^PUBLIC_HOST=' .env 2>/dev/null; then
      sed -i "s|^PUBLIC_HOST=.*|PUBLIC_HOST=${PUBHOST}|" .env
    else
      echo "PUBLIC_HOST=${PUBHOST}" >> .env
    fi
    say "public host: $PUBHOST"
  fi
fi

say "compiling the linux binary on the host (first run may take ~1 min)"
mkdir -p dist
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 "$GO" build -trimpath -ldflags="-s -w" \
  -o dist/dotinschool-challenge_linux_amd64 ./cmd/server

say "building the Docker image (packaging — a few seconds)"
docker build -f build/Dockerfile -t dotinschool-challenge:local .

say "starting the stack"
docker compose -f build/docker-compose.yml up -d

sleep 3
PASS=$(grep -E '^ADMIN_PASSWORD=' .env | cut -d= -f2-)
APATH=$(grep -E '^ADMIN_PATH=' .env | cut -d= -f2-)
USER=$(grep -E '^ADMIN_USERNAME=' .env | cut -d= -f2-)
echo
HOST=$(grep -E '^PUBLIC_HOST=' .env 2>/dev/null | cut -d= -f2-); HOST="${HOST:-localhost}"
echo "──────────────────────────────────────────────"
echo " user panel :  http://$HOST:$APP_PORT/"
echo " admin panel:  http://$HOST:$APP_PORT/$APATH/"
echo " username   :  $USER"
echo " password   :  $PASS"
echo "──────────────────────────────────────────────"
echo " logs : docker compose -f build/docker-compose.yml logs -f app"
echo " stop : docker compose -f build/docker-compose.yml down"
