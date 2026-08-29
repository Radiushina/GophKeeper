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
	go tool wire ./cmd/server/di

mock:
	go tool mockery

build-server:
	CGO_ENABLED=0 go build -o bin/gophkeeper-server ./cmd/server
