#!/bin/sh

set -euo pipefail

export readonly USER_ID="$(id -u)"

mkdir -p "$DB_ROOT_DIR/db"

chown -R "$USER_ID:$USER_ID" "$DB_ROOT_DIR"

exec /app/grype-server "$@"
