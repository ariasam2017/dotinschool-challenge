#!/usr/bin/env bash
# Build the dotinschool Docker image reliably: compile the Linux binary on the
# HOST (Go does this natively in seconds, CGO is off, web/assets is committed),
# then package it into a tiny Alpine image. No in-container `go build`, so it
# never fails on memory, a blocked module proxy, or a BuildKit RPC drop.
#
#   scripts/docker-build.sh                 # linux/amd64 -> dotinschool:local
#   scripts/docker-build.sh "amd64 arm64"   # multi-arch (needs buildx)
#   DOCKER_TAG=myrepo/dotinschool:1.0 scripts/docker-build.sh
set -euo pipefail

cd "$(dirname "$0")/.."
ARCHS="${1:-amd64}"
TAG="${DOCKER_TAG:-dotinschool:local}"

command -v go >/dev/null 2>&1 || { echo "go not found on PATH"; exit 1; }

if [ ! -f web/assets/index.html ] || ! grep -q 'DOTIN_HEAD' web/assets/index.html; then
  echo "==> web/assets looks unbuilt — building the frontend"
  ( cd web && npm ci --no-audit --no-fund && npm run build:fast )
fi

VERSION="$(git describe --tags --always --dirty 2>/dev/null || echo dev)"
mkdir -p dist

for a in $ARCHS; do
  echo "==> host-compiling linux/$a"
  GOOS=linux GOARCH="$a" CGO_ENABLED=0 \
    go build -trimpath -ldflags="-s -w -X main.version=${VERSION}" \
    -o "dist/dotinschool_linux_${a}" ./cmd/server
done

if [ "$(echo "$ARCHS" | wc -w)" -gt 1 ]; then
  platforms="$(for a in $ARCHS; do printf 'linux/%s,' "$a"; done)"
  platforms="${platforms%,}"
  echo "==> docker buildx (${platforms})"
  docker buildx build --platform "$platforms" -f build/Dockerfile -t "$TAG" --load .
else
  echo "==> docker build (linux/${ARCHS})"
  docker build --build-arg "TARGETARCH=${ARCHS}" -f build/Dockerfile -t "$TAG" .
fi

echo "==> done: $TAG"
docker image inspect "$TAG" --format '    size: {{.Size}} bytes' 2>/dev/null || true
