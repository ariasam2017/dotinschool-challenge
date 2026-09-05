#!/usr/bin/env bash
# dotinschool — update a Docker install to the latest version and restart:
#
#   ./update.sh
#
# Works for ANY Docker install of this project (the one-line `docker compose
# up -d --build`, the host-compile `./docker-up.sh`, or the Linux
# `docker-install.sh` flow) — run it from the project's root folder.
#
# Pulls the latest source from GitHub (git fetch + hard reset, so a dirty
# working tree never causes a merge conflict), then hands off to
# ./docker-up.sh to rebuild the host-compiled binary, repackage the image,
# and restart the stack — no in-container `go build`, so it can't fail on
# memory / a blocked proxy / a BuildKit RPC drop.
set -euo pipefail
cd "$(dirname "$0")"

say(){ printf '\033[36m==> %s\033[0m\n' "$*"; }
die(){ printf '\033[31m[x] %s\033[0m\n' "$*" >&2; exit 1; }

command -v git >/dev/null 2>&1 || die "git not found — install git and try again."
[ -d .git ] || die "this folder is not a git checkout — clone the repo with git to use ./update.sh."

BRANCH="$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo main)"
[ "$BRANCH" = "HEAD" ] && BRANCH="main"   # detached HEAD (e.g. a shallow clone) — assume main

say "fetching the latest source (branch: $BRANCH)"
git fetch --depth 1 origin "$BRANCH" || die "could not reach GitHub — check this server's internet connection."

BEFORE="$(git rev-parse HEAD)"
git reset --hard "origin/$BRANCH" >/dev/null
AFTER="$(git rev-parse HEAD)"

if [ "$BEFORE" = "$AFTER" ]; then
  say "already up to date — no new version on GitHub."
  exit 0
fi

say "new version pulled ($BEFORE -> $AFTER) — rebuilding and restarting"
exec bash docker-up.sh
