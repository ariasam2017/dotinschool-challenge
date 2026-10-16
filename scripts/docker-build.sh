#!/usr/bin/env bash
# Build the dotinschool-challenge Docker image reliably: compile the Linux binary on the
# HOST (Go does this natively in seconds, CGO is off, web/assets is committed),
# then package it into a tiny Alpine image. No in-container `go build`, so it
# never fails on memory, a blocked module proxy, or a BuildKit RPC drop.
#
#   scripts/docker-build.sh                 # linux/amd64 -> dotinschool-challenge:local
#   scripts/docker-build.sh "amd64 arm64"   # multi-arch (needs buildx)
#   DOCKER_TAG=myrepo/dotinschool-challenge:1.0 scripts/docker-build.sh
set -euo pipefail

cd "$(dirname "$0")/.."
ARCHS="${1:-amd64}"
TAG="${DOCKER_TAG:-dotinschool-challenge:local}"

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
    -o "dist/dotinschool-challenge_linux_${a}" ./cmd/server
done

if [ "$(echo "$ARCHS" | wc -w)" -gt 1 ]; then
  platforms="$(for a in $ARCHS; do printf 'linux/%s,' "$a"; done)"
  platforms="${platforms%,}"
  echo "==> docker buildx (${platforms})"
  docker buildx build --platform "$platforms" -f build/Dockerfile -t "$TAG" --load .
else
  echo "==> docker build (linux/${ARCHS})"
  # This Dockerfile only copies the already-compiled binary in (no `go build`
  # here), so it should never fail on memory - but Docker's BuildKit backend
  # can still drop its own internal grpc connection mid-build on some hosts
  # ("rpc error: code = Unavailable ... EOF"), even on a trivial RUN step.
  # That's a transient BuildKit hiccup, not a problem with this Dockerfile -
  # retry a couple of times, then fall back to the classic (non-BuildKit)
  # builder, which doesn't use that grpc protocol at all.
  max_attempts=3
  built=0
  for attempt in $(seq 1 "$max_attempts"); do
    if [ "$attempt" -gt 1 ]; then
      msg="==> docker build failed (attempt $((attempt - 1)) of $max_attempts) — retrying"
      [ "$attempt" -eq "$max_attempts" ] && msg="$msg with the classic (non-BuildKit) builder"
      echo "$msg..."
      sleep 3
    fi
    if [ "$attempt" -eq "$max_attempts" ]; then
      DOCKER_BUILDKIT=0 docker build --build-arg "TARGETARCH=${ARCHS}" -f build/Dockerfile -t "$TAG" . && { built=1; break; }
    else
      docker build --build-arg "TARGETARCH=${ARCHS}" -f build/Dockerfile -t "$TAG" . && { built=1; break; }
    fi
  done
  if [ "$built" -ne 1 ]; then
    echo "docker build failed after $max_attempts attempts (BuildKit and the classic builder) — this looks like a genuine Docker/BuildKit problem, not this Dockerfile. Restart Docker (or the docker daemon), then re-run." >&2
    exit 1
  fi
fi

echo "==> done: $TAG"
docker image inspect "$TAG" --format '    size: {{.Size}} bytes' 2>/dev/null || true
