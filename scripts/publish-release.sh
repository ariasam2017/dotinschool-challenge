#!/usr/bin/env bash
# Publish the current commit as the installable release of this project.
#
# The source repo is private; servers install from the PUBLIC repo
# ariasam2017/dotinschool-install, which only ever holds:
#   - <short>.sh / <short>-docker.sh    the installers (copied from here)
#   - a rolling "<short>-latest" GitHub release with one bundle per CPU arch:
#       <name>_linux_<arch>.tar.gz = prebuilt binary + Docker files, NO source
#
#   bash scripts/publish-release.sh
#
# Needs: gh (logged in), git, tar, and Go - or Docker, which it then uses to
# compile (so it also works on a Windows box with no Go installed).
set -euo pipefail
cd "$(dirname "$0")/.."

NAME="$(basename "$(git rev-parse --show-toplevel)")"   # dotinschool-game | dotinschool-challenge
SHORT="${NAME#dotinschool-}"                            # game | challenge
PUBLIC_REPO="ariasam2017/dotinschool-install"
TAG="${SHORT}-latest"
ARCHS="amd64 arm64"

die(){ echo "[x] $*" >&2; exit 1; }
command -v gh >/dev/null 2>&1 || die "gh (GitHub CLI) is not installed."
gh auth status >/dev/null 2>&1 || die "gh is not logged in - run: gh auth login"
[[ -z "$(git status --porcelain --untracked-files=no)" ]] || die "Uncommitted changes - commit them first, a release must match a commit."
[[ -f web/assets/index.html ]] || die "web/assets is not built - run: cd web && npm run build"

VERSION="$(git describe --tags --always)"
echo "==> publishing ${NAME} ${VERSION} -> ${PUBLIC_REPO} (${TAG})"

# ---- 1. compile the Linux binaries -----------------------------------------
LDFLAGS="-s -w -X main.version=${VERSION}"
mkdir -p dist
for a in $ARCHS; do
  out="dist/${NAME}_linux_${a}"
  echo "==> compiling linux/${a}"
  if command -v go >/dev/null 2>&1; then
    GOOS=linux GOARCH="$a" CGO_ENABLED=0 go build -trimpath -ldflags="$LDFLAGS" -o "$out" ./cmd/server
  else
    command -v docker >/dev/null 2>&1 || die "Neither Go nor Docker is available to compile with."
    src="$(pwd -W 2>/dev/null || pwd)"   # Git Bash needs the C:/... form for -v
    # A cached image keeps whatever Go patch release it was pulled with, and the
    # standard library ships inside the binary — refresh it so security fixes
    # land. Falls back to the cache where Docker Hub is unreachable.
    docker pull -q golang:1.27-alpine >/dev/null 2>&1 || echo "  (could not refresh golang:1.27-alpine - using the cached image)"
    MSYS_NO_PATHCONV=1 docker run --rm -v "${src}:/src" -w /src \
      -v dotin-gomod:/go/pkg/mod -v dotin-gocache:/root/.cache/go-build \
      -e CGO_ENABLED=0 -e GOOS=linux -e GOARCH="$a" -e GOFLAGS=-buildvcs=false \
      -e GOPROXY="${GOPROXY:-https://proxy.golang.org,https://goproxy.cn,direct}" \
      golang:1.27-alpine go build -trimpath -ldflags="$LDFLAGS" -o "$out" ./cmd/server
  fi
  [[ -s "$out" ]] || die "build produced no ${out}"
done

# ---- 2. pack one bundle per arch -------------------------------------------
stage="$(mktemp -d)"
trap 'rm -rf "$stage"' EXIT
assets=()
for a in $ARCHS; do
  d="${stage}/${a}/${NAME}"
  mkdir -p "$d/dist"
  # files come from the commit (git archive), not the working tree: on Windows
  # the working tree may hold CRLF copies, and a CRLF script won't run on Linux
  git archive HEAD build scripts/install.sh scripts/docker-build.sh .dockerignore | tar -x -C "$d"
    cp "dist/${NAME}_linux_${a}" "$d/dist/"
  echo "$VERSION" > "$d/VERSION"
  tar -C "${stage}/${a}" -czf "${stage}/${NAME}_linux_${a}.tar.gz" "$NAME"
  assets+=("${stage}/${NAME}_linux_${a}.tar.gz")
done

# ---- 3. installers in the public repo --------------------------------------
echo "==> updating ${SHORT}.sh / ${SHORT}-docker.sh in ${PUBLIC_REPO}"
gh repo clone "$PUBLIC_REPO" "${stage}/pub" -- --depth 1 -q
git show HEAD:scripts/install.sh > "${stage}/pub/${SHORT}.sh"
git show HEAD:build/docker-install.sh > "${stage}/pub/${SHORT}-docker.sh"
(
  cd "${stage}/pub"
  git add "${SHORT}.sh" "${SHORT}-docker.sh"
  if git diff --cached --quiet; then
    echo "    (installers unchanged)"
  else
    git commit -q -m "${SHORT}: installers for ${VERSION}"
    git push -q origin HEAD
  fi
)

# ---- 4. the rolling release -------------------------------------------------
if ! gh release view "$TAG" -R "$PUBLIC_REPO" >/dev/null 2>&1; then
  gh release create "$TAG" -R "$PUBLIC_REPO" --title "${NAME} (latest)" --notes "-" --latest=false
fi
gh release upload "$TAG" "${assets[@]}" -R "$PUBLIC_REPO" --clobber
gh release edit "$TAG" -R "$PUBLIC_REPO" \
  --notes "${NAME} ${VERSION} — $(date -u '+%Y-%m-%d %H:%M UTC'). Install: bash <(curl -Ls https://raw.githubusercontent.com/${PUBLIC_REPO}/main/${SHORT}.sh)"

echo "==> done: ${NAME} ${VERSION} is live. Servers get it with: ${NAME} update"
