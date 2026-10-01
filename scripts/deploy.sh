#!/usr/bin/env bash
set -euo pipefail

# Run locally on the VPS after setup_vps.sh has provisioned the service.
SRC_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd -P)
APP_DIR=${APP_DIR:-/opt/picture-this}
if [[ "$APP_DIR" != /* || "$APP_DIR" == / ]]; then
  echo "APP_DIR must be an absolute application directory, not /." >&2
  exit 1
fi
if [[ ! -d "$APP_DIR" ]]; then
  echo "APP_DIR does not exist; run scripts/setup_vps.sh first." >&2
  exit 1
fi
APP_DIR=$(cd "$APP_DIR" && pwd -P)
if [[ "$APP_DIR" == / || "$SRC_DIR" == "$APP_DIR/"* || "$APP_DIR" == "$SRC_DIR/"* ]]; then
  echo "APP_DIR and the checkout must be identical or separate directories." >&2
  exit 1
fi
if [[ ! -r "$APP_DIR/.env" ]]; then
  echo "A readable production .env is required in APP_DIR." >&2
  exit 1
fi
for command in go rsync supervisorctl install; do
  command -v "$command" >/dev/null || { echo "$command is required." >&2; exit 1; }
done
supervisorctl status picture-this

STAGE_DIR=$(mktemp -d)
trap 'rm -rf "$STAGE_DIR"' EXIT
EXCLUDES=(
  --exclude='/.env' --exclude='/.env.*' --exclude='/bin/'
  --exclude='/.git/' --exclude='/.gocache/' --exclude='/.cache/'
  --exclude='/.agents/' --exclude='/.codex/' --exclude='/.aws/'
  --exclude='/.venv*/' --exclude='/node_modules/' --exclude='/exp/'
  --exclude='/playwright-report/' --exclude='/test-results/'
  --exclude='/screenshots/' --exclude='__pycache__/' --exclude='.DS_Store'
)
rsync -a "${EXCLUDES[@]}" "$SRC_DIR/" "$STAGE_DIR/"
cd "$STAGE_DIR"
go tool templ generate
mkdir -p bin
go build -o bin/picture-this ./cmd/server
go build -o bin/migrate ./cmd/migrate

# Both commands use godotenv. Use the production file, never the checkout's
# credentials or an inherited development DATABASE_URL. Do not source .env as
# shell code: godotenv syntax and shell syntax differ.
install -m 600 "$APP_DIR/.env" "$STAGE_DIR/.env"
env -u DATABASE_URL bin/migrate

# In-place deployments already have their assets. For a separate checkout,
# remove obsolete source files but retain server-generated sounds and narration.
if [[ "$SRC_DIR" != "$APP_DIR" ]]; then
  rsync -a --delete --chmod=D755,F644 "${EXCLUDES[@]}" \
    --filter='P /static/audio/***' --filter='P /static/sounds/***' \
    "$STAGE_DIR/" "$APP_DIR/"
fi
mkdir -p "$APP_DIR/bin"
# Rename on the destination filesystem so the running executable is never
# truncated. Build/migration failures above leave the installed binary alone.
install -m 755 bin/picture-this "$APP_DIR/bin/picture-this.new"
mv -f "$APP_DIR/bin/picture-this.new" "$APP_DIR/bin/picture-this"
supervisorctl restart picture-this
supervisorctl status picture-this
