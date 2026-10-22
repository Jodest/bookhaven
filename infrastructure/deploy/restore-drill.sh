#!/usr/bin/env bash
# Restore drill: proves a backup file actually restores, without touching the
# compose stack or its volume.
#
#   restore-drill.sh <file.dump|file.dump.age> [age-identity-file]
#
# Restores into a throwaway postgres:16-alpine container (unique name, no
# published port, no named volume, --rm), runs
# pg_restore --no-owner --exit-on-error, prints sanity checks
# (schema_migrations version/dirty, row counts of users/books/authors) and
# always removes the container. An .age file is decrypted into a private temp
# dir with the given identity first; the plaintext copy is deleted on exit.
# Exit code is non-zero if the restore fails or the schema is marked dirty.
set -Eeuo pipefail
umask 077

usage() {
  echo "usage: $0 <file.dump|file.dump.age> [age-identity-file]" >&2
  exit 2
}

[ $# -ge 1 ] && [ $# -le 2 ] || usage
SRC="$1"
IDENTITY="${2:-}"
[ -r "$SRC" ] || { echo "ERROR: cannot read $SRC" >&2; exit 2; }

IMAGE="postgres:16-alpine"
CONTAINER="bookhaven-restore-drill-$(date -u +%Y%m%d%H%M%S)-$$"
WORKDIR=""
STARTED=0

cleanup() {
  local rc=$?
  set +e
  if [ "$STARTED" = 1 ]; then
    docker rm -f -v "$CONTAINER" >/dev/null 2>&1
    echo "drill container $CONTAINER removed"
  fi
  [ -n "$WORKDIR" ] && rm -rf -- "$WORKDIR"
  exit "$rc"
}
trap cleanup EXIT

DUMP="$SRC"
case "$SRC" in
  *.age)
    [ -n "$IDENTITY" ] || { echo "ERROR: $SRC is encrypted, pass the age identity file as the 2nd argument" >&2; exit 2; }
    [ -r "$IDENTITY" ] || { echo "ERROR: cannot read identity $IDENTITY" >&2; exit 2; }
    command -v age >/dev/null 2>&1 || { echo "ERROR: age is not installed" >&2; exit 2; }
    WORKDIR="$(mktemp -d "${TMPDIR:-/tmp}/bookhaven-drill.XXXXXX")"
    DUMP="$WORKDIR/drill.dump"
    echo "decrypting $SRC"
    age -d -i "$IDENTITY" -o "$DUMP" "$SRC"
    ;;
esac

echo "starting throwaway container $CONTAINER ($IMAGE)"
STARTED=1
docker run -d --rm --name "$CONTAINER" \
  -e POSTGRES_USER=drill -e POSTGRES_PASSWORD=drill -e POSTGRES_DB=drill \
  "$IMAGE" >/dev/null

# TCP check: the image's init phase runs a socket-only temporary server, so a
# socket pg_isready could succeed before the real server is up.
ready=0
for _ in $(seq 1 60); do
  if docker exec "$CONTAINER" pg_isready -q -h 127.0.0.1 -U drill -d drill; then
    ready=1
    break
  fi
  sleep 1
done
[ "$ready" = 1 ] || { echo "ERROR: drill postgres did not become ready in 60s" >&2; exit 1; }

echo "restoring $SRC"
start=$SECONDS
docker exec -i "$CONTAINER" pg_restore --no-owner --exit-on-error -U drill -d drill <"$DUMP"
echo "restore OK in $((SECONDS - start))s"

q() { docker exec "$CONTAINER" psql -X -At -v ON_ERROR_STOP=1 -U drill -d drill -c "$1"; }

migration="$(q 'SELECT version, dirty FROM schema_migrations')"
version="${migration%%|*}"
dirty="${migration#*|}"
echo "schema_migrations: version=$version dirty=$dirty"
for t in users books authors; do
  echo "rows in $t: $(q "SELECT count(*) FROM $t")"
done

if [ "$dirty" != f ]; then
  echo "ERROR: schema_migrations is dirty (or missing) - this dump is not a clean restore point" >&2
  exit 1
fi
echo "DRILL PASSED: $SRC"
