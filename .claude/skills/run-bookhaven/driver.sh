#!/usr/bin/env bash
# Drives the BookHaven docker-compose stack: ensures Docker Desktop is up,
# starts services, applies migrations + account-only seed (admin fixture +
# test login) on a fresh DB, and smoke-tests each HTTP surface. Does NOT
# seed the demo catalog (books/genres/authors) automatically — run
# `cmd/seed` yourself (no flags) if you want that. Safe to re-run — every
# step is idempotent.
set -uo pipefail
cd "$(dirname "$0")/../../.."   # repo root (this file lives in .claude/skills/run-bookhaven/)

# --- 1. Find the docker CLI (Docker Desktop 4.90+ installs per-user, not in Program Files) ---
find_docker() {
  if command -v docker >/dev/null 2>&1; then command -v docker; return; fi
  local localappdata_posix
  localappdata_posix="$(cygpath -u "$LOCALAPPDATA" 2>/dev/null)"
  for candidate in \
    "$localappdata_posix/Programs/DockerDesktop/resources/bin/docker.exe" \
    "/c/Program Files/Docker/Docker/resources/bin/docker.exe"; do
    [ -x "$candidate" ] && { echo "$candidate"; return; }
  done
}
DOCKER_BIN="$(find_docker)"
if [ -z "${DOCKER_BIN:-}" ]; then
  echo "FATAL: docker CLI not found (checked \$LOCALAPPDATA/Programs/DockerDesktop and Program Files)" >&2
  exit 1
fi
export PATH="$(dirname "$DOCKER_BIN"):$PATH"
echo "Using docker: $DOCKER_BIN"

# --- 2. Ensure the Docker Desktop daemon is responsive; launch it if not ---
if ! timeout 6 docker info >/dev/null 2>&1; then
  echo "Docker daemon not responding — launching Docker Desktop..."
  DESKTOP_EXE="$(dirname "$DOCKER_BIN")/../../Docker Desktop.exe"
  [ -x "$DESKTOP_EXE" ] || DESKTOP_EXE="/c/Program Files/Docker/Docker/Docker Desktop.exe"
  "$DESKTOP_EXE" >/dev/null 2>&1 &
  disown 2>/dev/null || true

  stable=0
  for i in $(seq 1 40); do
    sleep 5
    if timeout 5 docker info >/dev/null 2>&1; then
      stable=$((stable+1))
    else
      stable=0
    fi
    [ "$stable" -ge 3 ] && break
  done
  if [ "$stable" -lt 3 ]; then
    echo "FATAL: Docker daemon did not become ready after ~200s." >&2
    echo "If this persists, the WSL 'docker-desktop' distro may be corrupt:" >&2
    echo "  wsl --shutdown && wsl --unregister docker-desktop   # then relaunch Docker Desktop" >&2
    exit 1
  fi
  echo "Docker daemon ready."
fi

# --- 3. Start services (uses cached images; add --build if compose files or Dockerfiles changed) ---
echo "Starting services..."
docker compose up -d
docker compose ps --format "table {{.Name}}\t{{.State}}\t{{.Ports}}"

# --- 4. Wait for backend to answer, then check the DB actually has tables ---
echo "Waiting for backend API..."
ready=0
for i in $(seq 1 30); do
  code=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/swagger/index.html 2>/dev/null)
  [ "$code" = "200" ] && { ready=1; break; }
  sleep 3
done
if [ "$ready" -ne 1 ]; then
  echo "FATAL: backend did not become ready. Recent logs:" >&2
  docker compose logs --tail 40 backend >&2
  exit 1
fi

books_code=$(curl -s -o /dev/null -w "%{http_code}" "http://localhost:8080/api/v1/books?limit=1")
if [ "$books_code" != "200" ]; then
  echo "Books endpoint returned $books_code — applying migrations..."
  docker compose exec -T backend go run cmd/migrate/main.go up
  echo "Seeding accounts only (admin fixture + test login; no demo books)..."
  docker compose exec -T backend go run cmd/seed/main.go -accounts-only
fi
# Demo catalog (books/genres/authors) is NOT seeded automatically — run
# yourself if wanted: docker compose exec backend go run cmd/seed/main.go

# --- 5. Wait for web/admin dev servers (first boot runs `pnpm install`, can take minutes) ---
echo "Waiting for web (:3000) and admin (:3001) — first boot reinstalls deps, can take a few minutes..."
web_code=000; admin_code=000
for i in $(seq 1 60); do
  web_code=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:3000 2>/dev/null)
  admin_code=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:3001 2>/dev/null)
  # admin redirects unauthenticated requests (307) — that counts as "up"
  if [ "$web_code" = "200" ] && { [ "$admin_code" = "200" ] || [ "$admin_code" = "307" ]; }; then
    break
  fi
  sleep 5
done

echo ""
echo "=== Smoke test results ==="
echo "backend (http://localhost:8080/swagger/index.html): 200"
echo "books API sample: $(curl -s 'http://localhost:8080/api/v1/books?limit=1' | head -c 200)"
echo "web (http://localhost:3000): $web_code"
echo "admin (http://localhost:3001): $admin_code"

if [ "$web_code" != "200" ] || { [ "$admin_code" != "200" ] && [ "$admin_code" != "307" ]; }; then
  echo "WARNING: web or admin did not become ready in time. Recent logs:" >&2
  docker compose logs --tail 30 web admin >&2
  exit 1
fi

echo ""
echo "All services up:"
echo "  Web:   http://localhost:3000"
echo "  Admin: http://localhost:3001"
echo "  API:   http://localhost:8080  (Swagger: http://localhost:8080/swagger/index.html)"
echo "  Test account: test@bookhaven.local / test1234"
echo "  No demo catalog seeded automatically. To seed books/genres/authors:"
echo "    docker compose exec backend go run cmd/seed/main.go"
