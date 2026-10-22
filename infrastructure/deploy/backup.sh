#!/usr/bin/env bash
# Nightly Postgres backup: verified pg_dump -> age-encrypted off-site copy
# (rclone) -> local/off-site retention -> heartbeat ping.
#
# Install on the server as the deploy user (no redirect: the script writes its
# own log to $BACKUP_DIR/backup.log):
#   crontab -e
#   0 3 * * * /opt/bookhaven/infrastructure/deploy/backup.sh
#
# Configuration: BACKUP_* keys in $ENV_FILE (see .env.production.example and
# docs/DEPLOYMENT.md "Backups"). A variable already set in the environment wins
# over the file. rclone credentials live in ~/.config/rclone/rclone.conf.
#
# Paths are overridable for local runs (defaults = production):
#   APP_DIR       /opt/bookhaven
#   COMPOSE_FILE  docker-compose.prod.yml (relative to APP_DIR)
#   ENV_FILE      $APP_DIR/.env
#   BACKUP_DIR    $APP_DIR/backups
#
# Local test without off-site upload (warns on every run):
#   APP_DIR=. COMPOSE_FILE=docker-compose.yml ENV_FILE=/tmp/b.env \
#   BACKUP_DIR=/tmp/backups BACKUP_OFFSITE=off bash infrastructure/deploy/backup.sh
set -Eeuo pipefail
umask 077

APP_DIR="${APP_DIR:-/opt/bookhaven}"
COMPOSE_FILE="${COMPOSE_FILE:-docker-compose.prod.yml}"
ENV_FILE="${ENV_FILE:-$APP_DIR/.env}"
BACKUP_DIR="${BACKUP_DIR:-$APP_DIR/backups}"

TS="$(date -u +%Y%m%dT%H%M%SZ)"
NAME="bookhaven-$TS.dump"
FINAL="$BACKUP_DIR/$NAME"
TMP="$BACKUP_DIR/.inprogress-$TS.dump"
TMP_AGE="$BACKUP_DIR/.inprogress-$TS.dump.age"
LOG="$BACKUP_DIR/backup.log"
HEARTBEAT_URL=""

log() { printf '%s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$*"; }

ping_heartbeat() { # $1 = "" (success) or "/fail"
  if [ -z "$HEARTBEAT_URL" ]; then
    log "WARNING: BACKUP_HEARTBEAT_URL is empty, no heartbeat sent"
    return 0
  fi
  if ! command -v curl >/dev/null 2>&1; then
    log "WARNING: curl not found, heartbeat${1:+ $1} not sent"
    return 0
  fi
  curl -fsS -m 10 --retry 3 -o /dev/null "${HEARTBEAT_URL%/}$1" \
    || log "WARNING: heartbeat ping${1:+ $1} failed (backup result unchanged)"
}

on_err() {
  # Explicit target: the trap can run inside a command whose stdout is
  # redirected (e.g. the dump into $TMP), which would swallow the message.
  log "ERROR: command failed (line $1, exit $2): $3" >>"$LOG"
}

on_exit() {
  local rc=$?
  set +e
  rm -f -- "$TMP" "$TMP_AGE"
  if [ "$rc" -ne 0 ]; then
    log "FAILED: backup run exited with $rc; no new backup was kept"
    ping_heartbeat /fail
    log "=== backup end (FAILED) ==="
  fi
  exit "$rc"
}

# Read KEY=value from ENV_FILE without executing it: last matching line wins,
# one layer of surrounding single or double quotes is stripped.
env_get() {
  local line val
  line="$(grep -E "^$1=" "$ENV_FILE" | tail -n 1 || true)"
  val="${line#*=}"
  val="${val%$'\r'}"
  if [ "${#val}" -ge 2 ]; then
    case "$val" in
      \"*\") val="${val:1:${#val}-2}" ;;
      \'*\') val="${val:1:${#val}-2}" ;;
    esac
  fi
  printf '%s' "$val"
}

# Join an rclone remote base ("backup:bucket/path" or "/local/dir") with a subpath.
remote_path() {
  case "$BACKUP_RCLONE_REMOTE" in
    *:) printf '%s%s' "$BACKUP_RCLONE_REMOTE" "$1" ;;
    *) printf '%s/%s' "${BACKUP_RCLONE_REMOTE%/}" "$1" ;;
  esac
}

compose() { docker compose -f "$COMPOSE_FILE" "$@"; }

mkdir -p "$BACKUP_DIR"
chmod 700 "$BACKUP_DIR"
exec >>"$LOG" 2>&1
chmod 600 "$LOG"
trap 'on_err "$LINENO" "$?" "$BASH_COMMAND"' ERR
trap on_exit EXIT

SECONDS=0
log "=== backup start: $NAME ==="

# --- configuration + preflight (fails before dumping) ---------------------
if [ ! -r "$ENV_FILE" ]; then
  log "ERROR: env file not readable: $ENV_FILE"
  exit 1
fi
BACKUP_HEARTBEAT_URL="${BACKUP_HEARTBEAT_URL:-$(env_get BACKUP_HEARTBEAT_URL)}"
HEARTBEAT_URL="$BACKUP_HEARTBEAT_URL"
BACKUP_OFFSITE="${BACKUP_OFFSITE:-$(env_get BACKUP_OFFSITE)}"
BACKUP_OFFSITE="${BACKUP_OFFSITE:-on}"
BACKUP_RCLONE_REMOTE="${BACKUP_RCLONE_REMOTE:-$(env_get BACKUP_RCLONE_REMOTE)}"
BACKUP_AGE_RECIPIENT="${BACKUP_AGE_RECIPIENT:-$(env_get BACKUP_AGE_RECIPIENT)}"
BACKUP_LOCAL_RETENTION_DAYS="${BACKUP_LOCAL_RETENTION_DAYS:-$(env_get BACKUP_LOCAL_RETENTION_DAYS)}"
BACKUP_LOCAL_RETENTION_DAYS="${BACKUP_LOCAL_RETENTION_DAYS:-7}"

case "$BACKUP_LOCAL_RETENTION_DAYS" in
  '' | *[!0-9]*)
    log "ERROR: BACKUP_LOCAL_RETENTION_DAYS must be a non-negative integer, got '$BACKUP_LOCAL_RETENTION_DAYS'"
    exit 1
    ;;
esac

case "$BACKUP_OFFSITE" in
  off)
    log "WARNING: BACKUP_OFFSITE=off - off-site copy SKIPPED, this backup lives only on this disk (local testing only)"
    ;;
  on)
    missing=""
    [ -n "$BACKUP_RCLONE_REMOTE" ] || missing="$missing BACKUP_RCLONE_REMOTE"
    [ -n "$BACKUP_AGE_RECIPIENT" ] || missing="$missing BACKUP_AGE_RECIPIENT"
    command -v age >/dev/null 2>&1 || missing="$missing age(binary)"
    command -v rclone >/dev/null 2>&1 || missing="$missing rclone(binary)"
    if [ -n "$missing" ]; then
      log "ERROR: off-site backup is on but not configured, missing:$missing (set BACKUP_OFFSITE=off only for local testing)"
      exit 1
    fi
    ;;
  *)
    log "ERROR: BACKUP_OFFSITE must be 'on' or 'off', got '$BACKUP_OFFSITE'"
    exit 1
    ;;
esac

cd "$APP_DIR"

# --- dump into a temp file, verify, then give it the final name -----------
# DB user/name come from the postgres container's own environment.
# shellcheck disable=SC2016 # expanded by sh inside the container, on purpose
compose exec -T postgres sh -c 'pg_dump -Fc -U "$POSTGRES_USER" "$POSTGRES_DB"' >"$TMP"

if [ ! -s "$TMP" ]; then
  log "ERROR: dump is empty"
  exit 1
fi
# --list checks the archive header/TOC; the full read to /dev/null walks every
# data block, so a truncated archive fails here as well.
compose exec -T postgres pg_restore --list <"$TMP" >/dev/null
compose exec -T postgres pg_restore -f /dev/null <"$TMP"

mv -- "$TMP" "$FINAL"
chmod 600 "$FINAL"
SIZE="$(wc -c <"$FINAL" | tr -d ' ')"
log "dump verified: $FINAL ($SIZE bytes)"

# --- off-site: encrypt with age, upload with rclone, prune ----------------
if [ "$BACKUP_OFFSITE" = on ]; then
  age -r "$BACKUP_AGE_RECIPIENT" -o "$TMP_AGE" "$FINAL"
  REMOTE_NAME="$NAME.age"
  prefixes="daily"
  [ "$(date -u +%u)" = 7 ] && prefixes="$prefixes weekly"
  [ "$(date -u +%d)" = 01 ] && prefixes="$prefixes monthly"
  for p in $prefixes; do
    rclone copyto "$TMP_AGE" "$(remote_path "$p/$REMOTE_NAME")"
    log "uploaded: $(remote_path "$p/$REMOTE_NAME")"
  done
  rm -f -- "$TMP_AGE"

  # Prune only our own files under the three explicit prefixes, never the root.
  rclone delete --min-age 14d --include 'bookhaven-*.dump.age' "$(remote_path daily/)"
  rclone delete --min-age 63d --include 'bookhaven-*.dump.age' "$(remote_path weekly/)"
  rclone delete --min-age 400d --include 'bookhaven-*.dump.age' "$(remote_path monthly/)"
  log "off-site retention applied (daily 14d, weekly 63d, monthly 400d)"
fi

# --- local retention (also cleans legacy *.sql.gz and stale temp files) ---
find "$BACKUP_DIR" -maxdepth 1 -type f \
  \( -name 'bookhaven-*.dump' -o -name '*.sql.gz' \) \
  -mtime "+$BACKUP_LOCAL_RETENTION_DAYS" -delete
find "$BACKUP_DIR" -maxdepth 1 -type f -name '.inprogress-*' -mtime +1 -delete

ping_heartbeat ""
log "=== backup end (OK): $NAME, $SIZE bytes, ${SECONDS}s ==="
