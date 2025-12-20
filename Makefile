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

load-prompts:
	go run ./cmd/load-prompts -file prompts.csv

DATABASE_URL_TEST ?= postgres://user:password@localhost:5432/picture_this_test?sslmode=disable
PORT_TEST ?= 8081

e2e-test:
	@set -e; \
	DATABASE_URL="$(DATABASE_URL_TEST)" psql "$$DATABASE_URL" -v ON_ERROR_STOP=1 -c "drop schema public cascade; create schema public;"; \
	DATABASE_URL="$(DATABASE_URL_TEST)" make migrate; \
	PORT="$(PORT_TEST)" DATABASE_URL="$(DATABASE_URL_TEST)" make run > /tmp/picture_this_test.log 2>&1 & \
	SERVER_PID=$$!; \
	trap 'kill $$SERVER_PID' EXIT; \
	sleep 1; \
	DATABASE_URL="$(DATABASE_URL_TEST)" make load-prompts; \
	BASE_URL="http://localhost:$(PORT_TEST)" python3 scripts/e2e_game.py; \
	kill $$SERVER_PID; \
	wait $$SERVER_PID 2>/dev/null || true; \
	trap - EXIT;
