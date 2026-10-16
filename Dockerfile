# dotinschool-challenge — self-contained image: the frontend and the Go binary are both
# built inside this file, so a plain
#
#   docker compose up -d --build
#
# from the project root just works — one container, no host toolchain.
#
# The `go build` step is the memory hog. It is tuned (-p 1, GOMEMLIMIT, GOGC) to
# fit a small Docker Desktop VM. If it still dies with
# `rpc error: code = Unavailable ... EOF`, raise Docker Desktop's memory
# (Settings -> Resources, 4 GB is plenty) OR use the host-build path:
# `.\docker-up.ps1` / `make image`.

# ---- 1. build the Vue frontend ----
FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build:fast

# ---- 2. compile the Go binary with the frontend embedded ----
FROM golang:1.27-alpine AS build
WORKDIR /src
# proxy.golang.org / sum.golang.org are unreachable on some networks; these
# mirrors work globally. Override with --build-arg on a normal network.
ARG GOPROXY=https://goproxy.cn,direct
ARG GOSUMDB=sum.golang.google.cn
ENV CGO_ENABLED=0 GOFLAGS=-trimpath GOPROXY=${GOPROXY} GOSUMDB=${GOSUMDB} \
    GOMEMLIMIT=900MiB GOGC=40
COPY go.mod go.sum ./
# goproxy.cn's own DNS lookup is occasionally flaky inside Docker Desktop's
# build network (intermittent "i/o timeout" from the internal resolver, not a
# block) — retry a few times instead of failing/appearing to hang on one bad
# lookup. Already-fetched modules are cached, so a retry only redoes the ones
# that failed.
RUN n=0; until [ $n -ge 5 ]; do \
      go mod download && break; \
      n=$((n + 1)); \
      echo "go mod download failed (attempt $n/5), retrying in 5s..." >&2; \
      sleep 5; \
    done; \
    if [ $n -ge 5 ]; then echo "go mod download failed after 5 attempts" >&2; exit 1; fi
COPY . .
RUN rm -rf web/assets
COPY --from=web /web/assets ./web/assets
# -p 1 compiles one package at a time -> much lower peak memory.
RUN go build -p 1 -ldflags="-s -w" -o /out/dotinschool-challenge ./cmd/server

# ---- 3. minimal runtime ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget && adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /out/dotinschool-challenge /app/dotinschool-challenge
# data dir must be app-owned before VOLUME so a fresh named volume inherits it
# (else the non-root user cannot open arena.db -> SQLITE_CANTOPEN).
RUN mkdir -p /app/data && chown -R app:app /app

ENV PORT=8080 DATA_DIR=/app/data DOTINSCHOOL_DOCKER=1 TZ=Asia/Tehran
VOLUME ["/app/data"]
EXPOSE 8080
HEALTHCHECK --interval=30s --timeout=4s --start-period=5s --retries=3 \
  CMD wget -qO- "http://127.0.0.1:${PORT}/api/config" >/dev/null 2>&1 || exit 1
USER app
ENTRYPOINT ["/app/dotinschool-challenge"]
