.PHONY: all build web go test clean run-server dev tidy

VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo 0.0.0-dev)
LDFLAGS := -X github.com/nees/envvar/internal/version.Version=$(VERSION)
EMBED   := internal/server/dist

all: build

## build: build the web SPA, embed it, and compile the single Go binary
build: web go

## web: build the TanStack Start SPA and copy it into the Go embed dir
web:
	cd web && pnpm install --frozen-lockfile && pnpm build
	rm -rf $(EMBED)
	mkdir -p $(EMBED)
	cp -r web/dist/client/. $(EMBED)/

## go: compile the binary (expects $(EMBED) to be populated)
go:
	go build -ldflags "$(LDFLAGS)" -o bin/envvar ./cmd/envvar

## test: run Go tests
test:
	go test ./...

## tidy: tidy go modules
tidy:
	go mod tidy

## run-server: run the API + UI locally
run-server:
	go run ./cmd/envvar server

## dev: run the Vite dev server (proxies /v1 to a running `envvar server`)
dev:
	cd web && pnpm dev

clean:
	rm -rf bin web/dist web/node_modules
