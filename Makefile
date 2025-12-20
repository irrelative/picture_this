.PHONY: run build

run:
	templ generate
	go run ./...

build:
	templ generate
	go build ./...

test:
	go test ./...
