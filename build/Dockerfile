# dotinschool — multi-stage build to a single static binary with the frontend
# embedded. Final image ships only the binary + CA certs (pure-Go SQLite, so
# CGO is off and no libc is needed).
#
# Build context is the repository root:
#   docker build -f build/Dockerfile -t dotinschool:local .

# ---- 1. build the Vue frontend ----
FROM node:22-alpine AS web
WORKDIR /web
COPY web/package.json web/package-lock.json ./
RUN npm ci --no-audit --no-fund
COPY web/ ./
RUN npm run build:fast

# ---- 2. build the Go binary with the frontend embedded ----
FROM golang:1.27-alpine AS build
WORKDIR /src
# Module proxy + checksum DB. Defaults reach networks where proxy.golang.org /
# sum.golang.org are blocked; override with --build-arg for a normal network.
ARG GOPROXY=https://goproxy.cn,direct
ARG GOSUMDB=sum.golang.google.cn
ENV CGO_ENABLED=0 GOFLAGS=-trimpath GOPROXY=${GOPROXY} GOSUMDB=${GOSUMDB}
COPY go.mod go.sum ./
RUN go mod download
COPY . .
# use the freshly built assets, not whatever was committed
RUN rm -rf web/assets
COPY --from=web /web/assets ./web/assets
RUN go build -ldflags="-s -w" -o /out/dotinschool ./cmd/server

# ---- 3. minimal runtime ----
FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata wget && adduser -D -u 10001 app
WORKDIR /app
COPY --from=build /out/dotinschool /app/dotinschool

# Create the data dir with app ownership *before* declaring the VOLUME so a
# freshly-mounted named volume inherits it — otherwise the non-root user cannot
# open /app/data/arena.db (SQLITE_CANTOPEN).
RUN mkdir -p /app/data && chown -R app:app /app

ENV PORT=8080 \
    DATA_DIR=/app/data \
    DOTINSCHOOL_DOCKER=1 \
    TZ=Asia/Tehran

# arena.db + ai-config.json live here — mount a volume over it
VOLUME ["/app/data"]
EXPOSE 8080

HEALTHCHECK --interval=30s --timeout=4s --start-period=5s --retries=3 \
  CMD wget -qO- "http://127.0.0.1:${PORT}/api/config" >/dev/null 2>&1 || exit 1

USER app
ENTRYPOINT ["/app/dotinschool"]
