---
name: run-bookhaven
description: Build, run, and drive the BookHaven stack (Go backend, Next.js web + admin, Postgres, Redis) via Docker Compose. Use when asked to start/run/launch BookHaven, bring up the app, check it's running, seed the database, or smoke-test its web/admin/API surfaces.
---

BookHaven is a Docker Compose stack (Postgres, Redis, Go/Echo backend, two Next.js apps: web + admin). Drive it via `.claude/skills/run-bookhaven/driver.sh` — it starts every container, waits for each HTTP surface, and auto-applies migrations/seed if the database is empty. Run it from the repo root.

## Prerequisites

- Docker Desktop (Windows). This machine has it installed **per-user**, not in `Program Files` — as of Docker Desktop 4.90 the install lives at `%LOCALAPPDATA%\Programs\DockerDesktop`, and `docker.exe` is at `...\DockerDesktop\resources\bin\docker.exe`. That path is not on `PATH` by default. The driver script finds it automatically (checks `command -v docker` first, then that per-user path, then the old `C:\Program Files\Docker\Docker\...` path) — don't hardcode either path elsewhere.
- `.env` at repo root (already present; copied from `.env.example`, defaults work for local dev — Postgres/Redis ports, `BACKEND_PORT=8080`, `WEB_PORT=3000`, `ADMIN_PORT=3001`).

## Run (agent path)

```bash
cd <repo-root>
bash .claude/skills/run-bookhaven/driver.sh
```

The script is idempotent — safe to re-run any time to check status or recover from a partial start. It:

1. Locates `docker.exe`, launches Docker Desktop if the daemon isn't responding, and polls until it's stably up (3 consecutive successful `docker info` calls — a single success can be a transient blip right after launch).
2. Runs `docker compose up -d` (uses cached images; does **not** pass `--build` — add that yourself if `docker-compose.yml`/Dockerfiles changed).
3. Waits for the backend (`:8080/swagger/index.html` → 200).
4. Checks `/api/v1/books?limit=1`: if it 500s (tables missing), runs `docker compose exec backend go run cmd/migrate/main.go up`; if it returns an empty `items` array (fresh DB), runs `docker compose exec backend go run cmd/seed/main.go`.
5. Waits for web (`:3000` → 200) and admin (`:3001` → 200 or 307 — admin redirects unauthenticated requests to login, that's healthy).
6. Prints a smoke-test summary with all URLs and the seeded test account.

On success you'll see:
```
All services up:
  Web:   http://localhost:3000
  Admin: http://localhost:3001
  API:   http://localhost:8080  (Swagger: http://localhost:8080/swagger/index.html)
  Test account: test@bookhaven.local / test1234
```

On failure the script exits non-zero and dumps the last 30-40 lines of the relevant container's logs — read those first.

**First run after a fresh Docker install/rebuild is slow** (5-10+ min): `web`/`admin` containers run `pnpm install && pnpm dev` as their container command (see `docker-compose.yml`), so every container recreation reinstalls all frontend deps from scratch, not just on first build. There is no faster documented path for this repo — a native (non-Docker) run was not verified this session, so it isn't documented here.

## Test

```bash
docker compose exec backend go test ./...      # or: make test-backend
docker compose exec web pnpm test              # or: make test-web (apps/web now has vitest)
```

## Stop

```bash
docker compose down          # stop + remove containers, keep volumes (DB data persists)
docker compose down -v       # also wipe volumes — next run starts with an empty DB
```

---

## Gotchas

- **`$LOCALAPPDATA` in Git Bash has mixed slashes** (`C:\Users\...\Local/Programs/...`) — using it directly in a path breaks `dirname`/PATH lookups in ways that fail silently (bare `docker` resolves to nothing, `docker info` exits 127, which looks identical to "daemon not responding" and triggers a pointless Docker Desktop relaunch). The driver converts it with `cygpath -u` first. If you're scripting Docker paths elsewhere in this repo, do the same.
- **A single successful `docker info` right after launching Docker Desktop isn't reliable** — it can flap for a bit before actually being ready. The driver requires 3 consecutive successes before proceeding.
- **A fresh/reset Postgres volume has no tables and no data** — `/api/v1/books` and `/api/v1/genres` 500 with `relation "books" does not exist` until migrations run, then return empty until seeded. The driver detects and fixes both automatically; if you're debugging by hand, that 500 is not a bug, it's an unmigrated DB.
- **`docker compose up -d` without `--build`** reuses existing images. If you've changed a `Dockerfile` or `docker-compose.yml` service definition, you need `--build` explicitly — the driver intentionally doesn't force a rebuild every run (this stack's builds take too long to do unconditionally).

## Troubleshooting

- **`failed to connect to the docker API at npipe:////./pipe/dockerDesktopLinuxEngine`**: Docker Desktop isn't running (or hasn't finished starting). The driver auto-launches it; if it keeps failing, launch Docker Desktop manually and watch its tray icon for the "Engine running" state before retrying.
- **`Wsl/Service/CreateInstance/HCS_E_CONNECTION_TIMEOUT`** (visible in Docker Desktop's own error dialog, not the CLI): the underlying `docker-desktop` WSL distro is corrupted. Fix, in order of least destructive first: (1) `wsl --shutdown` then relaunch Docker Desktop, (2) reboot Windows, (3) as a last resort, `wsl --shutdown && wsl --unregister docker-desktop` then relaunch Docker Desktop to let it recreate the distro from scratch (this wipes all local images/volumes — you'll need to `--build` and re-migrate/seed after). All three were needed in sequence once on this machine before a full Docker Desktop reinstall was needed.
- **Docker Desktop directory contains only a `tmp-delete` folder, `docker.exe`/`Docker Desktop.exe` missing entirely**: the installation is broken (can happen if Docker Desktop processes are force-killed mid self-update). Reinstall from https://www.docker.com/products/docker-desktop/ — note it now installs per-user under `%LOCALAPPDATA%\Programs\DockerDesktop`, not `C:\Program Files\Docker\Docker`.
- **`relation "books"/"genres" does not exist (SQLSTATE 42P01)`** in backend logs: DB has no schema yet — see the migrate/seed step above; the driver handles this automatically.
