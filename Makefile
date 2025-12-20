.PHONY: run build

run:
	templ generate
	go run ./cmd/server

build:
	templ generate
	go build ./...

test:
	go test ./...

migrate:
	go run ./cmd/migrate

migrate-create:
	@if [ -z "$(name)" ]; then echo "usage: make migrate-create name=add_table"; exit 1; fi
	go run ./cmd/migrate-create -name "$(name)"
