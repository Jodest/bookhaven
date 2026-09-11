#!/usr/bin/env bash
# Nightly Postgres backup. Install via cron on the deploy user, e.g.:
#   crontab -e
#   0 3 * * * /opt/bookhaven/infrastructure/deploy/backup.sh >> /var/log/bookhaven-backup.log 2>&1
#
# Optional off-site copy: install rclone and configure a remote named "backup"
# (works with any S3-compatible provider, e.g. Timeweb S3 / Selectel S3), then
# uncomment the rclone line below.
set -euo pipefail

APP_DIR="/opt/bookhaven"
BACKUP_DIR="$APP_DIR/backups"
RETENTION_DAYS=7
TIMESTAMP="$(date +%Y%m%d-%H%M%S)"

mkdir -p "$BACKUP_DIR"
cd "$APP_DIR"

set -a; source .env; set +a

docker compose -f docker-compose.prod.yml exec -T postgres \
  pg_dump -U "$POSTGRES_USER" "$POSTGRES_DB" \
  | gzip > "$BACKUP_DIR/bookhaven-$TIMESTAMP.sql.gz"

find "$BACKUP_DIR" -name "*.sql.gz" -mtime "+$RETENTION_DAYS" -delete

# rclone copy "$BACKUP_DIR/bookhaven-$TIMESTAMP.sql.gz" backup:bookhaven-backups/
