# dotinschool-challenge — dev & release tasks.
# `make dev` runs the Go server and the Vite dev server together (Vite proxies
# /api to :8080). `make build` produces the embedded single binary.

SHELL := /bin/bash
BIN   := dotinschool-challenge
GO    ?= go

.PHONY: help
help:
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | sort | \
	  awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: web
web: ## build the frontend into web/assets
	cd web && npm ci --no-audit --no-fund && npm run build

.PHONY: web-fast
web-fast: ## build the frontend without typecheck
	cd web && npm run build:fast

.PHONY: build
build: web-fast ## build the embedded single binary for this host
	CGO_ENABLED=0 $(GO) build -trimpath -ldflags="-s -w" -o $(BIN) ./cmd/server

.PHONY: run
run: build ## build then run
	./$(BIN)

.PHONY: dev
dev: ## Go server (:8080) + Vite dev server (:5173) together
	( $(GO) run ./cmd/server & echo $$! > .devpid ) ; \
	cd web && npm run dev ; \
	kill $$(cat .devpid) 2>/dev/null ; rm -f .devpid

.PHONY: release
release: ## cross-compile release artifacts into dist/
	bash scripts/build.sh

.PHONY: image
image: ## reliable Docker image: host-compile the binary, then package it
	bash scripts/docker-build.sh

.PHONY: image-hermetic
image-hermetic: ## fully in-container build (needs ~2GB free for `go build`)
	docker build -f build/Dockerfile.hermetic -t dotinschool-challenge:local .

.PHONY: docker
docker: image ## alias for `make image`

.PHONY: seed
seed: build ## run the standalone demo-data seeder against ./data
	$(GO) run ./cmd/seed

.PHONY: test
test: ## run Go tests + frontend typecheck
	$(GO) test ./... && cd web && npm run typecheck

.PHONY: tidy
tidy: ## gofmt + go mod tidy
	gofmt -w . && $(GO) mod tidy

.PHONY: clean
clean:
	rm -rf $(BIN) $(BIN).exe dist web/assets/assets web/node_modules
