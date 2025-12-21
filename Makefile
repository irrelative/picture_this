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

DATABASE_URL_TEST ?= postgres:///picture_this_test?sslmode=disable
PORT_TEST ?= 8081

e2e-test:
	@set -e; \
	PORT="$(PORT_TEST)"; \
	if ! python3 -c 'import socket,sys; port=int(sys.argv[1]); sock=socket.socket(); sock.bind(("127.0.0.1", port)); sock.close()' "$$PORT" 2>/dev/null; then \
		PORT=$$(python3 -c 'import socket; sock=socket.socket(); sock.bind(("127.0.0.1", 0)); print(sock.getsockname()[1]); sock.close()'); \
		echo "PORT_TEST $$PORT_TEST in use, using $$PORT"; \
	fi; \
	DATABASE_URL="$(DATABASE_URL_TEST)" psql "$$DATABASE_URL" -v ON_ERROR_STOP=1 -c "drop schema public cascade; create schema public;"; \
	DATABASE_URL="$(DATABASE_URL_TEST)" make migrate; \
	PORT="$$PORT" DATABASE_URL="$(DATABASE_URL_TEST)" make run > /tmp/picture_this_test.log 2>&1 & \
	SERVER_PID=$$!; \
	trap 'kill $$SERVER_PID 2>/dev/null || true' EXIT; \
	sleep 1; \
	if ! kill -0 $$SERVER_PID 2>/dev/null; then \
		echo "server failed to start; log output:"; \
		cat /tmp/picture_this_test.log; \
		exit 1; \
	fi; \
	DATABASE_URL="$(DATABASE_URL_TEST)" make load-prompts; \
	BASE_URL="http://localhost:$$PORT" python3 scripts/e2e_game.py; \
	kill $$SERVER_PID 2>/dev/null || true; \
	wait $$SERVER_PID 2>/dev/null || true; \
	trap - EXIT;
