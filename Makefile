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

proto:
	PATH="$(shell go env GOPATH)/bin:$$PATH" protoc \
		--go_out=. --go_opt=module=github.com/Radiushina/GophKeeper \
		--go-grpc_out=. --go-grpc_opt=module=github.com/Radiushina/GophKeeper \
		api/file/v1/file.proto

wire:
	go tool wire ./cmd/server/di ./cmd/client

mock:
	go tool mockery

lint:
	golangci-lint run --config .golangci.yml

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

CERTS_DIR := certs

certs: certs-ca certs-csr certs-server
# создаёт корневой CA (ca.key + ca.crt)
certs-ca:
	mkdir -p $(CERTS_DIR)
	openssl req -x509 -newkey rsa:2048 -nodes \
		-keyout $(CERTS_DIR)/ca.key -out $(CERTS_DIR)/ca.crt -days 365 \
		-subj "/CN=GophKeeper Dev CA"

# создаёт ключ сервера и заявку (server.key + server.csr)
certs-csr:
	mkdir -p $(CERTS_DIR)
	openssl req -newkey rsa:2048 -nodes \
		-keyout $(CERTS_DIR)/server.key -out $(CERTS_DIR)/server.csr \
		-subj "/CN=localhost"

# CA подписывает заявку → server.crt
certs-server:
	mkdir -p $(CERTS_DIR)
	printf "subjectAltName=DNS:localhost,IP:127.0.0.1\n" > $(CERTS_DIR)/san.cnf
	openssl x509 -req -in $(CERTS_DIR)/server.csr -CA $(CERTS_DIR)/ca.crt -CAkey $(CERTS_DIR)/ca.key \
		-CAcreateserial -out $(CERTS_DIR)/server.crt -days 365 \
		-extfile $(CERTS_DIR)/san.cnf
