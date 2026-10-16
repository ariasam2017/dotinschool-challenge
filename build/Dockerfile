# dotinschool-challenge — packaging Dockerfile.
#
# This image contains NOTHING but the prebuilt static binary + CA certs, so the
# `docker build` step is a ~2 second file copy that cannot fail on memory, a
# flaky module proxy, or a BuildKit RPC drop.
#
# The binary is compiled on the host first (Go builds Linux binaries natively,
# CGO is off, and web/assets is committed so no Node is needed either):
#
#   make image            # host cross-compile + docker build, one command
#   # or manually:
#   GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" \
#       -o dist/dotinschool-challenge_linux_amd64 ./cmd/server
#   docker build -f build/Dockerfile -t dotinschool-challenge:local .
#
# For a fully hermetic in-container build (needs ~2 GB free for `go build`), use
# build/Dockerfile.hermetic instead.

FROM alpine:3.20

RUN apk add --no-cache ca-certificates tzdata wget && adduser -D -u 10001 app
WORKDIR /app

# TARGETARCH is set automatically by `docker build` / buildx (amd64, arm64).
ARG TARGETARCH=amd64
COPY dist/dotinschool-challenge_linux_${TARGETARCH} /app/dotinschool-challenge
RUN chmod +x /app/dotinschool-challenge \
    && mkdir -p /app/data \
    && chown -R app:app /app

ENV PORT=8080 \
    DATA_DIR=/app/data \
    DOTINSCHOOL_DOCKER=1 \
    TZ=Asia/Tehran

VOLUME ["/app/data"]
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=4s --start-period=5s --retries=3 \
  CMD wget -qO- "http://127.0.0.1:${PORT}/api/config" >/dev/null 2>&1 || exit 1

USER app
ENTRYPOINT ["/app/dotinschool-challenge"]
