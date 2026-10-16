#!/usr/bin/env bash
# Build the frontend, then cross-compile the single static binary for every
# release target into ./dist/. Run from the repo root.
#
#   scripts/build.sh            # build all targets
#   scripts/build.sh local      # just build for the host OS/arch
set -euo pipefail

cd "$(dirname "$0")/.."
ROOT="$(pwd)"
DIST="${ROOT}/dist"
VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
LDFLAGS="-s -w -X main.version=${VERSION}"

echo "==> building frontend (web/)"
( cd web && npm ci --no-audit --no-fund && npm run build )

mkdir -p "$DIST"
rm -f "$DIST"/dotinschool-challenge-*

build_one() {
  local goos="$1" goarch="$2" ext="${3:-}"
  local out="dotinschool-challenge-${goos}-${goarch}${ext}"
  echo "==> $out"
  CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
    go build -trimpath -ldflags="$LDFLAGS" -o "${DIST}/${out}" ./cmd/server
  ( cd "$DIST" && tar -czf "${out%.exe}.tar.gz" "$out" && rm -f "$out" ) 2>/dev/null || \
    ( cd "$DIST" && zip -q "${out%.exe}.zip" "$out" && rm -f "$out" )
}

if [[ "${1:-}" == "local" ]]; then
  go build -trimpath -ldflags="$LDFLAGS" -o "${ROOT}/dotinschool-challenge" ./cmd/server
  echo "==> ${ROOT}/dotinschool-challenge"
  exit 0
fi

build_one linux amd64
build_one linux arm64
build_one windows amd64 .exe

echo "==> release artifacts:"
ls -lh "$DIST"
