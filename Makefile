migrate-postgres:
ifneq "$(name)" ""
	migrate create -ext sql -dir internal/migrations/postgres $(name)
else
	echo "\nSpecify migration script name\n";
endif

api-build:
	docker run --rm -v ${PWD}/docs:/spec redocly/cli build-docs --config redocly.yml -o openapi.html openapi.yml

ogen:
	go tool ogen --config ogen.yml --target gen/oas -package oas --clean docs/openapi.yml

wire:
	go tool wire ./cmd/server/di ./cmd/client

mock:
	go tool mockery

BUILDINFO_PKG := github.com/Radiushina/GophKeeper/internal/domains/buildinfo
VERSION ?= 0.1.0
DATE ?= $(shell date -u +%Y-%m-%dT%H:%M:%SZ)
LDFLAGS := -X '$(BUILDINFO_PKG).Version=$(VERSION)' -X '$(BUILDINFO_PKG).Date=$(DATE)'

build-server:
	CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o bin/gophkeeper-server ./cmd/server

build-client:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/gophkeeper-linux-amd64 ./cmd/client
	CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/gophkeeper-windows-amd64.exe ./cmd/client
	CGO_ENABLED=0 GOOS=darwin GOARCH=amd64 go build -ldflags "$(LDFLAGS)" -o bin/gophkeeper-darwin-amd64 ./cmd/client
	CGO_ENABLED=0 GOOS=darwin GOARCH=arm64 go build -ldflags "$(LDFLAGS)" -o bin/gophkeeper-darwin-arm64 ./cmd/client

run-client:
	go run ./cmd/client --server http://localhost:9090 -tui