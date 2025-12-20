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
