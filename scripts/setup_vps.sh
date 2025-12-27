#!/usr/bin/env bash
set -euo pipefail

if [[ ${EUID:-$(id -u)} -ne 0 ]]; then
  echo "This script must run as root." >&2
  exit 1
fi

DOMAIN=${DOMAIN:-}
LETSENCRYPT_EMAIL=${LETSENCRYPT_EMAIL:-}
APP_USER=${APP_USER:-picturethis}
APP_DIR=${APP_DIR:-/opt/picture-this}
APP_PORT=${APP_PORT:-8080}
APP_ENV=${APP_ENV:-production}
DB_NAME=${DB_NAME:-picture_this}
DB_USER=${DB_USER:-picture_this}
DB_PASS=${DB_PASS:-}
REPO_DIR=${REPO_DIR:-$(pwd)}
SKIP_BUILD=${SKIP_BUILD:-}

if [[ -z "$DOMAIN" ]]; then
  echo "DOMAIN is required (e.g. DOMAIN=example.com)." >&2
  exit 1
fi

if [[ -z "$LETSENCRYPT_EMAIL" ]]; then
  echo "LETSENCRYPT_EMAIL is required for certbot." >&2
  exit 1
fi

if [[ -z "$DB_PASS" ]]; then
  echo "DB_PASS is required for the database user." >&2
  exit 1
fi

export DEBIAN_FRONTEND=noninteractive
apt-get update
apt-get install -y --no-install-recommends \
  ca-certificates \
  curl \
  git \
  rsync \
  make \
  nginx \
  postgresql \
  supervisor \
  certbot \
  python3-certbot-nginx \
  golang-go

systemctl enable --now postgresql

if ! id -u "$APP_USER" >/dev/null 2>&1; then
  useradd --system --create-home --shell /bin/bash "$APP_USER"
fi

mkdir -p "$APP_DIR"
if [[ "$REPO_DIR" != "$APP_DIR" ]]; then
  rsync -a --delete "$REPO_DIR"/ "$APP_DIR"/
fi

mkdir -p /var/log/picture-this
chown -R "$APP_USER":"$APP_USER" /var/log/picture-this

# Database setup
sudo -u postgres psql -v ON_ERROR_STOP=1 <<SQL
DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = '${DB_USER}') THEN
    CREATE ROLE ${DB_USER} LOGIN PASSWORD '${DB_PASS}';
  END IF;
END
$$;

DO $$
BEGIN
  IF NOT EXISTS (SELECT 1 FROM pg_database WHERE datname = '${DB_NAME}') THEN
    CREATE DATABASE ${DB_NAME} OWNER ${DB_USER};
  END IF;
END
$$;
SQL

# App env
cat > "$APP_DIR/.env" <<ENV
PORT=${APP_PORT}
DATABASE_URL=postgres://${DB_USER}:${DB_PASS}@localhost:5432/${DB_NAME}?sslmode=disable
PROMPTS_PER_PLAYER=2
DB_MAX_OPEN_CONNS=10
DB_MAX_IDLE_CONNS=10
DB_CONN_MAX_LIFETIME_SECONDS=300
DB_CONN_MAX_IDLE_SECONDS=60
ENV

chown "$APP_USER":"$APP_USER" "$APP_DIR/.env"
chmod 600 "$APP_DIR/.env"

# Build
if [[ -z "$SKIP_BUILD" ]]; then
  cd "$APP_DIR"
  if [[ ! -d bin ]]; then
    mkdir -p bin
  fi
  GO111MODULE=on go build -o "$APP_DIR/bin/picture-this" ./cmd/server
  chown "$APP_USER":"$APP_USER" "$APP_DIR/bin/picture-this"
fi

# Nginx config
install -d /etc/nginx/sites-available /etc/nginx/sites-enabled
sed \
  -e "s#{{DOMAIN}}#${DOMAIN}#g" \
  -e "s#{{APP_DIR}}#${APP_DIR}#g" \
  -e "s#{{APP_PORT}}#${APP_PORT}#g" \
  "$APP_DIR/deploy/nginx/picture-this.conf" \
  > /etc/nginx/sites-available/picture-this.conf

ln -sf /etc/nginx/sites-available/picture-this.conf /etc/nginx/sites-enabled/picture-this.conf
rm -f /etc/nginx/sites-enabled/default
nginx -t
systemctl reload nginx

# Supervisor config
sed \
  -e "s#{{APP_USER}}#${APP_USER}#g" \
  -e "s#{{APP_DIR}}#${APP_DIR}#g" \
  -e "s#{{APP_ENV}}#${APP_ENV}#g" \
  -e "s#{{APP_PORT}}#${APP_PORT}#g" \
  "$APP_DIR/deploy/supervisor/picture-this.conf" \
  > /etc/supervisor/conf.d/picture-this.conf

supervisorctl reread
supervisorctl update
supervisorctl restart picture-this

# TLS via Let's Encrypt
certbot --nginx -d "$DOMAIN" --non-interactive --agree-tos -m "$LETSENCRYPT_EMAIL"

systemctl reload nginx

echo "Setup complete. App should be live at https://${DOMAIN}"
