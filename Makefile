.PHONY: run build init test migrate migrate-create load-prompts joke-audio-venv joke-audio-deps generate-joke-audio e2e-test deploy

run:
	templ generate
	go run ./cmd/server

build:
	templ generate
	go build ./...

init:
	@mkdir -p static/sounds static/vendor
	curl -L -o static/vendor/htmx.min.js https://unpkg.com/htmx.org@1.9.12/dist/htmx.min.js
	curl -L -o static/sounds/join.ogg https://opengameart.org/sites/default/files/audio_preview/pop2.wav.ogg
	curl -L -o static/sounds/round_start.ogg https://opengameart.org/sites/default/files/pop1.wav
	curl -L -o static/sounds/timer_end.ogg https://opengameart.org/sites/default/files/pop9.wav
	curl -L -o static/sounds/voting_start.mp3 https://opengameart.org/sites/default/files/Accept.mp3

test:
	GOCACHE="$(GOCACHE)" go test ./...

migrate:
	go run ./cmd/migrate

migrate-create:
	@if [ -z "$(name)" ]; then echo "usage: make migrate-create name=add_table"; exit 1; fi
	go run ./cmd/migrate-create -name "$(name)"

load-prompts:
	go run ./cmd/load-prompts -file prompts.csv

JOKE_AUDIO_VENV ?= .venv-joke-audio
JOKE_AUDIO_REQUIREMENTS ?= scripts/requirements-joke-audio.txt
JOKE_AUDIO_PYTHON ?= $(JOKE_AUDIO_VENV)/bin/python

joke-audio-venv:
	@if ! command -v python3.11 >/dev/null 2>&1; then \
		echo "python3.11 is required (brew install python@3.11)"; \
		exit 1; \
	fi
	@if [ ! -x "$(JOKE_AUDIO_PYTHON)" ]; then \
		python3.11 -m venv "$(JOKE_AUDIO_VENV)"; \
	fi

joke-audio-deps: joke-audio-venv
	"$(JOKE_AUDIO_PYTHON)" -m pip install --upgrade pip setuptools
	"$(JOKE_AUDIO_PYTHON)" -m pip install -r "$(JOKE_AUDIO_REQUIREMENTS)"

generate-joke-audio: joke-audio-deps
	@set -a; \
	if [ -f .env ]; then . ./.env; fi; \
	set +a; \
	"$(JOKE_AUDIO_PYTHON)" scripts/generate_joke_audio.py $(ARGS)

DATABASE_URL_TEST ?= postgres:///picture_this_test?sslmode=disable
PORT_TEST ?= 8081
GOCACHE ?= $(CURDIR)/.gocache

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

deploy:
	@set -eu; \
	SRC_DIR="$$(pwd)"; \
	DEST_DIR="/opt/picture-this"; \
	mkdir -p "$$DEST_DIR/bin"; \
	rsync -a --delete --exclude ".env" --exclude "bin" "$$SRC_DIR"/ "$$DEST_DIR"/; \
	cd "$$DEST_DIR"; \
	GO111MODULE=on go build -o "$$DEST_DIR/bin/picture-this" ./cmd/server; \
	go run ./cmd/migrate; \
	supervisorctl restart picture-this
