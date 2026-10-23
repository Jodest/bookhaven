# Deployment

Status: planned

Target: a single Russian VPS, minimal budget, no domain yet. This document is both the architecture decision record and the operational runbook.

## Architecture

```
Internet
   |
   v
 Caddy (80/443, automatic Let's Encrypt TLS)
   |-- WEB_DOMAIN   -> web:3000     (Next.js, apps/web)
   |-- ADMIN_DOMAIN -> admin:3000   (Next.js, apps/admin)
   |-- API_DOMAIN   -> backend:8080 (Go/Echo API)
                          |
                          |-- postgres:5432
                          |-- redis:6379
```

One VPS, one `docker compose` stack (`docker-compose.prod.yml`), five containers, one internal Docker network. Only Caddy publishes ports to the internet; everything else is reachable only from other containers. This is deliberately the simplest thing that works for an MVP — no Kubernetes, no managed database, no CDN, no load balancer. Scale up (separate DB host, more app replicas, managed Postgres) only when real usage demands it.

**Images are built in CI, never on the server.** GitHub Actions builds the backend/web/admin production images and pushes them to GHCR (`ghcr.io/<owner>/bookhaven-{backend,web,admin}`); the server only ever runs `docker compose pull` + `up -d`. This keeps the VPS small (no Go toolchain, no Node/pnpm, no ~2GB build context) and keeps deploys fast and identical to what CI validated.

## What was fixed to make this possible

The following blocked any production build or deploy and were fixed as prerequisites (see git history for the exact diffs):

- Backend CORS was hardcoded to `localhost:3000/3001` — now reads `CORS_ALLOWED_ORIGINS` from env (`backend/internal/config/config.go`, `backend/cmd/api/main.go`).
- `backend/Dockerfile`'s production stage copied migrations to `/migrations` while `cmd/migrate` reads `file:///app/migrations` — migrations could never run against the production image. Fixed by setting `WORKDIR /app` in the production stage and building/copying a `migrate` binary alongside `api`.
- `infrastructure/docker/Dockerfile.node` and both apps' `next.config.js` still referenced a `@bookhaven/types` package that no longer exists (removed in the FSD migration) — every frontend Docker build failed at `COPY packages/types/package.json`.
- No `.dockerignore` existed at the repo root or in `backend/` — the frontend build context was ~2.4GB (all of `node_modules`, `.pnpm-store`, `.git`) and took 4+ minutes just to transfer. Added both.
- `apps/web`'s homepage is a server component that fetches from the API at render time; Next.js tried to statically prerender it at *build* time, which requires a live backend and fails in CI/Docker with no network path to one. Marked it `export const dynamic = 'force-dynamic'` so it renders per-request instead (this page's content is inherently dynamic anyway).
- Neither app had a `public/` directory; the Dockerfile's `COPY .../public ...` failed. Added `public/.gitkeep` to both.

All three production images (`backend`, `web`, `admin`) were built and smoke-tested locally (backend against the dev Postgres/Redis, including the `migrate` binary) before this plan was written.

## Prerequisites

1. **VPS** — a Russian provider (Timeweb Cloud, Selectel, or VK Cloud). For this MVP: 2 vCPU / 4 GB RAM / 40+ GB SSD, Ubuntu 22.04 or 24.04 LTS. This runs Postgres + Redis + 3 app containers + Caddy comfortably; upgrade RAM first if Postgres/Redis start swapping.
2. **Domain** — register one (via the VPS provider or a registrar like reg.ru). You need it before Caddy can issue TLS certificates. Plan for three DNS A-records: `bookhaven.<domain>`, `admin.bookhaven.<domain>`, `api.bookhaven.<domain>` (or `www`/`admin`/`api` subdomains of your root domain — pick names, put them in `.env`, they're not hardcoded anywhere else).
3. **GitHub repository** — this repo needs a `git remote` on GitHub (none is configured yet: `git remote add origin git@github.com:<owner>/bookhaven.git && git push -u origin master` — rename to `main` first if you want CI to trigger on `main` as configured, or adjust the workflow branch names).
4. **SSH key pair** dedicated to deploys (don't reuse your personal key): `ssh-keygen -t ed25519 -f deploy_key -C bookhaven-deploy`.

## One-time server setup

1. Provision the VPS, note its public IP, point the three DNS A-records at it.
2. As root on the fresh server, run `infrastructure/deploy/server-setup.sh` (copy it up first: `scp infrastructure/deploy/server-setup.sh root@<ip>:/root/`). It installs Docker Engine + Compose plugin, creates a non-root `deploy` user in the `docker` group, disables SSH password auth and root login, configures `ufw` (22/80/443 only) and `fail2ban`, installs `age` + `rclone` for backups, and creates `/opt/bookhaven` and `/opt/bookhaven/backups`.
3. Append `deploy_key.pub` to `/home/deploy/.ssh/authorized_keys` on the server (the script copies root's existing `authorized_keys`, but add the dedicated deploy key explicitly if it's not already there).
4. From your machine, copy the compose files up:
   ```
   scp docker-compose.prod.yml Caddyfile deploy@<ip>:/opt/bookhaven/
   scp .env.production.example deploy@<ip>:/opt/bookhaven/.env
   ```
5. SSH in as `deploy` and edit `/opt/bookhaven/.env`: fill in the real domains, `ACME_EMAIL`, a generated `POSTGRES_PASSWORD` (`openssl rand -base64 24`), a generated `JWT_SECRET` (`openssl rand -base64 48`), and `CORS_ALLOWED_ORIGINS` matching the web/admin domains exactly (scheme + host, no trailing slash). Fill in the SMTP block (`SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `MAIL_FROM`) from your email provider — see [Email (SMTP)](#email-smtp); leaving `SMTP_HOST` blank disables sending. Optionally set `SENTRY_DSN_BACKEND` / `SENTRY_DSN_FRONTEND` (see [Error tracking](#error-tracking-sentry); empty = off). Then `chmod 600 .env`, and put an encrypted copy of it in your password manager (see [Backups](#env-and-keys-off-server)).
6. If the GHCR packages end up private (private repo, default), log in once on the server with a PAT scoped to `read:packages`: `docker login ghcr.io -u <github-user> -p <PAT>`. If the repo is public, make the three GHCR packages public instead (Package settings on GitHub) and skip this — one less credential to manage on the server.
7. Set up backups before real data arrives: [Backup setup](#backup-setup-once-as-deploy) (scripts, rclone remote, age key, heartbeat, cron).

## GitHub setup

1. Push this repo to GitHub (`main` branch — rename `master` if needed, or edit the workflow branch filters).
2. Add repository secrets (Settings → Secrets and variables → Actions):
   - `DEPLOY_HOST` — the VPS IP or hostname
   - `DEPLOY_USER` — `deploy`
   - `DEPLOY_SSH_KEY` — the **private** deploy key (`deploy_key`, not `.pub`)
3. Create a GitHub Environment named `production` (Settings → Environments) and, optionally, require manual approval on it if you want a human click before every deploy rather than fully automatic deploy-on-push.
4. `GITHUB_TOKEN` (used to push to GHCR) is provided automatically by Actions — no setup needed, just confirm Settings → Actions → General → Workflow permissions is set to "Read and write permissions" so it can push packages.

## First deploy

1. Actions tab → Deploy → Run workflow on `main` (or push to `main` once deploy-on-push is re-enabled — see [Ongoing deploys](#ongoing-deploys)). `.github/workflows/deploy.yml` runs CI, builds and pushes the three images tagged with the short commit SHA and `latest`, then SSHes in and runs the compose stack.
2. The very first run also needs the database migrated: the deploy workflow already runs `docker compose run --rm --entrypoint /app/migrate backend up` before `up -d`, so this happens automatically — including on the first deploy.
3. Seed data (optional, for a demo account) is not run automatically. `cmd/seed` is dev/demo-only tooling and refuses to run when `APP_ENV=production` unless you pass `-force` — evaluate first whether seed data belongs in a real production database, and only pass `-force` if you genuinely want the same fixed demo seed as dev. If wanted: `docker compose -f docker-compose.prod.yml run --rm --entrypoint /app/migrate backend version` to confirm migrations landed, then adapt `backend/cmd/seed` into a one-off run the same way (with `-force`) if you go ahead.
4. Verify: `https://<API_DOMAIN>/health/ready` returns `{"status":"ok","checks":{"database":"ok","redis":"ok"}}`, `https://<WEB_DOMAIN>` loads the catalog, `https://<ADMIN_DOMAIN>` redirects to login.

## Ongoing deploys

**Current state: manual only.** Until a production server exists, `deploy.yml` has only the `workflow_dispatch` trigger (issue #523): deploy-on-push burned ~13 billed Actions minutes per merge to `main` (full CI re-run + three image builds) and then always failed at the SSH step because there is no host, which exhausted the Free-plan 2000 min/month. Deploy with Actions tab → Deploy → Run workflow on `main` — it still runs the full CI first (path filters and the same-SHA skip in `ci.yml` never apply to `workflow_call`), then builds, pushes and deploys. The same button redeploys without a new commit (e.g. after only changing `.env` on the server, or to retry a flaky deploy).

**Re-enabling deploy-on-push** (once the server, DNS and the `DEPLOY_*` secrets above are in place and a manual run has succeeded): add the `push` trigger back in `.github/workflows/deploy.yml`:

```yaml
on:
  push:
    branches: [main]
  workflow_dispatch:
```

From then on every push to `main` (after CI passes) automatically builds, pushes, and deploys. Optionally add `paths-ignore: ["docs/**", "openspec/**", ".claude/**", "*.md"]` under `push` so docs-only merges don't rebuild images. Update the "Ongoing deploys" wording here and the branch descriptions in `docs/PIPELINE.md`, `.claude/commands/f.md` and `.claude/commands/feature.md` (Step "main merged") back to "deploys automatically".

Treat `main` as production — land features via the existing `/feature` workflow and PRs, don't push directly unless that's a deliberate choice for a one-person project.

### CI minutes budget

`ci.yml` is tuned for the Free plan (2000 min/month, private repo):
- PRs touching only `docs/**`, `openspec/**`, `.claude/**`, `.opencode/**`, `opencode.json` or root `*.md` don't start CI at all.
- Otherwise a small `precheck` job (~1 billed minute) lists the PR's files via the API and runs `backend` only for `backend/**`, `frontend` only for `apps/**`, `packages/**`, root `package.json` / `pnpm-lock.yaml` / `pnpm-workspace.yaml` / `turbo.json` / `tsconfig*.json`; any `.github/workflows/**` change runs both.
- If the PR head SHA already has a successful CI run (the PR into `dev` validated it), the PR into `main` from the same branch skips both jobs. If the dev run is still in progress when the main PR opens, the main PR runs CI normally.
- `concurrency` cancels a stale run when a new commit is pushed to the same PR.
- Skipped jobs block nothing: there are no required checks (branch protection isn't available on Free for private repos).

## Rollback

Images are tagged with the short commit SHA, not just `latest`, so rollback is:

```
ssh deploy@<ip>
cd /opt/bookhaven
sed -i 's/^IMAGE_TAG=.*/IMAGE_TAG=<previous-sha>/' .env
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
```

If the rollback also needs a schema rollback, run the down migration explicitly first — `docker compose -f docker-compose.prod.yml run --rm --entrypoint /app/migrate backend down` rolls back **one** migration step (`down N` rolls back N steps, N must be a positive integer); check `backend/migrations` before running it, and prefer forward-fixing over rolling back a schema in a database with real user data whenever possible. The command prints the resulting schema version.

A full rollback (every migration, i.e. dropping the whole schema) is only available as `down-all --yes-drop-everything`; without that exact flag it refuses, and with `APP_ENV=production` it additionally requires `--force`. Never use it on a database with real data — restore from a backup instead (see [Restore](#restore)). (Before #186, plain `down` rolled back everything; it is now one step.)

## Backups

**What is backed up:** PostgreSQL only — one `pg_dump -Fc` per night. Not backed up by design: the covers volume (a cache, re-mirrored from `books.cover_url`, see [Self-hosted covers](#self-hosted-covers)), Redis (caches, rate-limit counters, queued jobs) and Caddy's certificates (re-issued by Let's Encrypt). **RPO 24 h** (one run at 03:00 server time); WAL archiving / point-in-time recovery is out of scope. RTO is not enforced; a rehearsed [Restore](#restore) takes minutes for a database of this size.

`infrastructure/deploy/backup.sh` (run by cron as `deploy`):

1. Dumps through `docker compose exec -T postgres` (user/database come from the container's own `POSTGRES_USER`/`POSTGRES_DB`) into a temp file `backups/.inprogress-<ts>.dump`.
2. Verifies it: non-empty, `pg_restore --list` succeeds, and a full `pg_restore -f /dev/null` read succeeds (`--list` alone only reads the archive's table of contents and accepts a truncated file). Only then is it renamed to `backups/bookhaven-<UTC timestamp>.dump` (`chmod 600`, directory `chmod 700` — the dump holds password hashes and refresh tokens). A failed or truncated dump never gets the final name and is deleted on exit.
3. Off-site: encrypts the verified dump with [`age`](https://age-encryption.org) to a **public** recipient and uploads only the `.dump.age` with `rclone` to `<remote>/daily/`, plus `weekly/` on Sundays and `monthly/` on the 1st (UTC). After a successful upload it prunes **only** `bookhaven-*.dump.age` under those three prefixes (never the remote root): daily 14 d, weekly 63 d, monthly 400 d. The plaintext dump is never uploaded.
4. Local retention: `bookhaven-*.dump` (and legacy `*.sql.gz`) older than `BACKUP_LOCAL_RETENTION_DAYS` are deleted.
5. Heartbeat: pings `BACKUP_HEARTBEAT_URL` on success and `BACKUP_HEARTBEAT_URL/fail` on any failure. A failed ping is logged as a warning and never turns a good backup into a failure.

**Fail loud.** With off-site on (the default), an empty `BACKUP_RCLONE_REMOTE` or `BACKUP_AGE_RECIPIENT`, or a missing `age`/`rclone` binary, fails the run before dumping (non-zero exit, `/fail` ping). The only way to skip the off-site copy is `BACKUP_OFFSITE=off`, which is logged as a warning on every run and is meant for local testing only.

**Encryption is asymmetric.** The server holds only the `age1...` public recipient; the private identity lives off-server in the password manager, so a compromised server cannot decrypt old off-site copies. **Losing the identity means losing every off-site backup** — there is no recovery. The [quarterly drill](#quarterly-restore-drill) is what proves the identity still works.

### Configuration

The script reads these keys from `/opt/bookhaven/.env` with a parser that does not execute the file (`KEY=value` lines, last one wins, one layer of quotes stripped); a variable already set in the environment takes precedence. Nothing else in the stack uses them.

| Key | Default | Meaning |
|---|---|---|
| `BACKUP_RCLONE_REMOTE` | empty → **error** | rclone destination base, e.g. `backup:bookhaven-backups`. `daily/`, `weekly/`, `monthly/` are appended. |
| `BACKUP_AGE_RECIPIENT` | empty → **error** | `age1...` public key. |
| `BACKUP_OFFSITE` | `on` | `off` skips encryption + upload with a warning (local testing only). |
| `BACKUP_HEARTBEAT_URL` | empty → warning | healthchecks.io-compatible ping URL; `/fail` is appended on failure. |
| `BACKUP_LOCAL_RETENTION_DAYS` | `7` | Local `*.dump` retention in days. |

rclone credentials live in the deploy user's `~/.config/rclone/rclone.conf` (`chmod 600`), not in `.env`. For local runs the paths are overridable: `APP_DIR` (default `/opt/bookhaven`), `COMPOSE_FILE` (`docker-compose.prod.yml`), `ENV_FILE` (`$APP_DIR/.env`), `BACKUP_DIR` (`$APP_DIR/backups`).

### Backup setup (once, as `deploy`)

`server-setup.sh` already installs `age` and `rclone` and creates `/opt/bookhaven/backups` (`700`, owned by `deploy`).

1. Copy the scripts next to the compose file, keeping the path, and make them executable (the repo's `.gitattributes` keeps `*.sh` LF even on a Windows checkout):
   ```
   ssh deploy@<ip> mkdir -p /opt/bookhaven/infrastructure
   scp -r infrastructure/deploy deploy@<ip>:/opt/bookhaven/infrastructure/
   ssh deploy@<ip> 'chmod +x /opt/bookhaven/infrastructure/deploy/*.sh'
   ```
2. Create an S3-compatible bucket (Timeweb S3 / Selectel S3) with an access key limited to that bucket, then on the server run `rclone config` to create a remote named e.g. `backup`; `chmod 600 ~/.config/rclone/rclone.conf`. Check with `rclone lsd backup:`.
3. On **your own machine** (not the server): `age-keygen -o bookhaven-backup.key`. Store the whole file (it contains the `AGE-SECRET-KEY-...` identity) in the password manager, note the printed `age1...` public key, then delete the local file.
4. Create a check on healthchecks.io (free tier; period 1 day, grace ~2 h) and copy its ping URL.
5. Fill `BACKUP_RCLONE_REMOTE`, `BACKUP_AGE_RECIPIENT`, `BACKUP_HEARTBEAT_URL` in `/opt/bookhaven/.env`.
6. Run once by hand and read the log: `/opt/bookhaven/infrastructure/deploy/backup.sh; echo $?; tail /opt/bookhaven/backups/backup.log`. Confirm `rclone ls backup:bookhaven-backups/daily/` lists the new `.dump.age` and the healthchecks check turned green.
7. Install the cron line (`crontab -e`). No output redirect — the script writes its own timestamped log to `/opt/bookhaven/backups/backup.log`:
   ```
   0 3 * * * /opt/bookhaven/infrastructure/deploy/backup.sh
   ```

### `.env` and keys off-server

`.env` is **not** in the dump. Keep an encrypted copy of `/opt/bookhaven/.env`, the `age` identity and the rclone credentials (bucket access key) in the password manager, and update it after every `.env` change. Losing `JWT_SECRET` logs everyone out; losing `POSTGRES_PASSWORD` is recoverable (reset it inside the container) but annoying; losing the age identity loses every off-site backup.

## Restore

1. **Pick the file.**
   - Local (disk survived): `ls -l /opt/bookhaven/backups/bookhaven-*.dump`.
   - Off-site: list and download it, then decrypt it with the identity copied temporarily from the password manager:
     ```
     rclone lsl backup:bookhaven-backups/daily/
     rclone copyto backup:bookhaven-backups/daily/bookhaven-<ts>.dump.age ./x.dump.age
     install -m 600 /dev/null ~/bh-age.key && nano ~/bh-age.key   # paste the identity
     age -d -i ~/bh-age.key -o x.dump x.dump.age
     shred -u ~/bh-age.key
     ```
2. **Dry run first:** `/opt/bookhaven/infrastructure/deploy/restore-drill.sh x.dump` (or `x.dump.age <identity-file>`). It restores into a throwaway `postgres:16-alpine` container (no published port, no named volume, removed afterwards), never touches the compose stack, and prints the `schema_migrations` version/dirty flag and row counts of `users`, `books`, `authors`. Continue only on `DRILL PASSED`.
3. **Production restore — into a fresh database, then swap; never restore over the live database in place.** The commands assume `POSTGRES_DB=bookhaven`. Run them in `/opt/bookhaven`:
   ```
   docker compose -f docker-compose.prod.yml stop backend worker
   docker compose -f docker-compose.prod.yml exec -T postgres sh -c 'createdb -U "$POSTGRES_USER" bookhaven_restore'
   docker compose -f docker-compose.prod.yml exec -T postgres sh -c 'pg_restore -U "$POSTGRES_USER" --no-owner --exit-on-error -d bookhaven_restore' < x.dump
   docker compose -f docker-compose.prod.yml exec -T postgres sh -c 'psql -U "$POSTGRES_USER" -d bookhaven_restore -c "SELECT version, dirty FROM schema_migrations" -c "SELECT count(*) FROM users" -c "SELECT count(*) FROM books"'
   docker compose -f docker-compose.prod.yml exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d postgres -c "ALTER DATABASE bookhaven RENAME TO bookhaven_old" -c "ALTER DATABASE bookhaven_restore RENAME TO bookhaven"'
   docker compose -f docker-compose.prod.yml up -d backend worker
   ```
   If the first rename fails with "database is being accessed by other users" (backend and worker are stopped, so usually a backup cron run or a stray `psql` session), end those sessions and repeat the rename: `... psql -U "$POSTGRES_USER" -d postgres -c "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname='bookhaven' AND pid<>pg_backend_pid()"`.

   Smoke-check `https://<API_DOMAIN>/health/ready` and the catalog. Drop `bookhaven_old` only after a few days (`... psql -U "$POSTGRES_USER" -d postgres -c "DROP DATABASE bookhaven_old"`). If any step before the rename fails, nothing has changed: drop `bookhaven_restore` and start the services again.

   **Rollback after the swap** (the smoke-check fails): put the old database back.
   ```
   docker compose -f docker-compose.prod.yml stop backend worker
   docker compose -f docker-compose.prod.yml exec -T postgres sh -c 'psql -v ON_ERROR_STOP=1 -U "$POSTGRES_USER" -d postgres -c "ALTER DATABASE bookhaven RENAME TO bookhaven_failed" -c "ALTER DATABASE bookhaven_old RENAME TO bookhaven"'
   docker compose -f docker-compose.prod.yml up -d backend worker
   ```
   Keep `bookhaven_failed` for investigation and drop it once the cause is known.
   - **Image vs dump version:** if the running image is older than the dump's migration `version` (its `backend/migrations` has no file with that number), first set `IMAGE_TAG` in `.env` to a build that has it and `pull`. An older binary against a newer schema is not supported.
4. **Full VPS loss:** provision a new VPS → `server-setup.sh` → restore `/opt/bookhaven/.env` from the password manager → copy the compose file, `Caddyfile` and `infrastructure/deploy/` (see [Backup setup](#backup-setup-once-as-deploy)) and re-create `rclone.conf` → `docker compose -f docker-compose.prod.yml up -d postgres` → fetch and decrypt the newest off-site copy (step 1) and dry-run it (step 2). Then restore straight into the empty database the container created. No rename is needed, and the migrate step is skipped because the dump carries the schema and `schema_migrations`:
   ```
   docker compose -f docker-compose.prod.yml exec -T postgres sh -c 'pg_restore -U "$POSTGRES_USER" --no-owner --exit-on-error -d "$POSTGRES_DB"' < x.dump
   docker compose -f docker-compose.prod.yml up -d
   ```
   Point DNS at the new IP; covers re-mirror automatically. Re-install the backup cron line and run one backup by hand.

### Quarterly restore drill

Once a quarter, and after any change to the backup setup: download the latest `monthly/` copy and run `restore-drill.sh x.dump.age <identity-file>`. Do it on your own machine with Docker + `age` + `rclone`, or on the server with the identity copied in temporarily and `shred`-ed afterwards. Record the date and result as one line in the operator log or a GitHub issue comment. This is the only proof that the age identity in the password manager and the off-site copy still work.

## Background jobs (worker)

The `worker` container runs four asynq servers (harvest, `email`, `similarity`, `auth_maintenance`; plus `harvest_schedule` only when the scheduled harvest is on, `ai_moderation` only when AI pre-moderation is on and `ai_curator` only when AI author curation is on) plus an in-process `asynq.Scheduler`. The harvest server processes exactly one harvest job at a time (`Concurrency: 1`, #191), manual and scheduled jobs alike. The scheduler enqueues the daily similar-books recompute (`similarity:recompute` on the `similarity` queue, 03:30 UTC; see [docs/features/similar-books-stage2.md](features/similar-books-stage2.md)). The recompute reads shelves/ratings, builds the "readers also read" ranking and fully replaces the `book_similarities` table in one transaction. Books without rows keep the stage-1 content-based list, so an empty table is always safe.

> **Do not run more than one worker replica** without moving the scheduler out of the worker. Prod runs exactly one (`container_name: bookhaven-worker`). The task's `asynq.Unique(6h)` rejects a duplicate enqueue if two schedulers ever run, but that is a safety net, not a design.

The worker never recomputes on startup. Run the recompute by hand once right after the first deploy of this feature (and after a large data import); it runs in-process and needs no Redis or worker:

```
docker compose -f docker-compose.prod.yml run --rm --entrypoint /app/similarity backend
```

It prints the run stats and exits non-zero on error. If the worker is unhealthy (see the known config issue with the worker's environment), the nightly run never fires and every book silently stays on the stage-1 list: check `docker compose -f docker-compose.prod.yml logs worker` for `asynq scheduler started` and `similar books recomputed`.

### Auth token purge

Always on (no flag). The scheduler enqueues `auth:purge_tokens` (queue `auth_maintenance`, its own server with Concurrency 1; `MaxRetry(2)`, `Timeout(10m)`, `Unique(6h)`) daily at 03:45 UTC. It deletes, in batches of 1000 by primary key, so each statement stays short:

- `refresh_tokens` revoked or expired more than 7 days ago;
- `email_verification_tokens` and `password_reset_tokens` expired more than 30 days ago (used or not).

Live tokens are never touched. During the 30 days a used or expired verification/reset link keeps its specific answer; after that it is unknown and the page shows «Ссылка недействительна». The retention windows are constants (`RefreshTokenRetention`, `OneTimeTokenRetention` in `internal/modules/auth/domain/tokens.go`), not env vars. A failure on one table is logged (`auth token purge failed`) and the other tables still run; the task then fails and asynq retries it (the purge is idempotent). Migration `000038` only adds indexes for the purge filters.

Check `docker compose -f docker-compose.prod.yml logs worker` for `auth token purge scheduled` at startup and a daily `auth tokens purged` line with `refresh_deleted`, `verification_deleted` and `reset_deleted` (logged even when zero; no tokens, emails or user ids are logged). The single-replica rule applies here too: `Unique(6h)` is only a safety net.

### Scheduled harvest

Off by default. When `HARVEST_SCHEDULE_ENABLED=true`, the scheduler enqueues a `harvest:schedule_tick` task (queue `harvest_schedule`) on `HARVEST_SCHEDULE_CRON` (UTC). The tick picks up to `HARVEST_SCHEDULE_BATCH` authors that an admin flagged «Автоматически пополнять книги из Wikidata», that are published and have a Wikidata QID, and that were not auto-harvested in the last 30 days (never-harvested first, then the oldest). It queues **one** ordinary harvest job for them (shown as «Авто» on `/import/jobs`). Everything that job creates is `pending` and goes through `/moderation/books`; nothing is published automatically. A run is skipped while an earlier scheduled job is still pending, active or retrying. Worst case per run: one SPARQL query plus up to 500 Open Library lookups, throttled at about 1 request per second.

| Variable | Meaning |
|---|---|
| `HARVEST_SCHEDULE_ENABLED` | `true` turns the scheduled harvest on. Default `false` (in dev too: no surprise outbound traffic). When off the worker logs `scheduled harvest disabled`. |
| `HARVEST_SCHEDULE_CRON` | Cron spec, UTC. Default `0 4 * * *` (30 min after the similarity recompute). An invalid spec makes the worker exit at startup. |
| `HARVEST_SCHEDULE_BATCH` | Max authors per run. Default `10`, clamped to 1..50; a non-integer value means the default. |

The same single-replica rule applies: two workers would double the ticks. The tick's `asynq.Unique(1h)` and the "scheduled job already queued" check are a safety net only. Check `docker compose -f docker-compose.prod.yml logs worker` for `scheduled harvest enabled` and `scheduled harvest job queued`.

### Scheduled catalog adaptation sync

Off by default. The admin action «Найти экранизации для всего каталога» on `/import/jobs` always works; this schedule only repeats it. When `ADAPTATION_SYNC_SCHEDULE_ENABLED=true`, the scheduler enqueues a `screenworks:sync_catalog_tick` task (queue `harvest_schedule`, the same small server as the harvest tick) on `ADAPTATION_SYNC_SCHEDULE_CRON` (UTC). The tick queues **one** catalog adaptation sync job (shown as «Авто» and «Поиск экранизаций» on `/import/jobs`) on the `harvest` queue, so it shares the single worker slot with harvest jobs. The job asks Wikidata about every published book with a QID in chunks of 50 and only adds links: hidden links stay hidden and hand-edited works are untouched. A run is skipped while a catalog sync job is still pending, active or retrying.

| Variable | Meaning |
|---|---|
| `ADAPTATION_SYNC_SCHEDULE_ENABLED` | `true` turns the scheduled catalog sync on. Default `false`. When off the worker logs `scheduled adaptation sync disabled`. |
| `ADAPTATION_SYNC_SCHEDULE_CRON` | Cron spec, UTC. Default `0 5 * * 0` (Sunday 05:00, an hour after the harvest tick). An invalid spec makes the worker exit at startup. |

The `harvest_schedule` server starts when either schedule is enabled. The single-replica rule applies here too. Check `docker compose -f docker-compose.prod.yml logs worker` for `scheduled adaptation sync enabled` and `scheduled adaptation sync job queued`.

### Automatic collections

**On by default**, unlike the harvest schedule: without it automatic «Подборки» (kind `auto`) never refresh. The scheduler enqueues a `curated:recompute_auto` task (queue `curated`, its own server with Concurrency 1) on `AUTO_COLLECTIONS_SCHEDULE_CRON` (UTC). The task recomputes the stored snapshot of every automatic collection, draft or published, from ratings and the «Прочитано» / «Хочу прочитать» / «Читаю» shelves, and keeps each book's previous position for the «↑/↓/Новинка» indicators. It makes no external calls and costs one aggregate query per collection. A collection that fails keeps its old snapshot and does not stop the others (see the worker log `curated: scheduled recompute failed`). Admins can also press «Пересчитать сейчас» on a collection (`POST /api/v1/admin/collections/{id}/recompute`), which works whether or not the schedule is on.

| Variable | Meaning |
|---|---|
| `AUTO_COLLECTIONS_SCHEDULE_ENABLED` | Default `true`; unset or unparsable also means on. Only an explicit falsy value (`false`, `0`) turns it off, e.g. in dev or tests. When off the worker logs `scheduled auto collections recompute disabled` and automatic collections change only through admin actions. |
| `AUTO_COLLECTIONS_SCHEDULE_CRON` | Cron spec, UTC. Default `0 1 * * *` (04:00 MSK, before the 03:30 UTC similarity recompute). An invalid spec makes the worker exit at startup. |

The single-replica rule applies here too: two workers would run the recompute twice (harmless, the second only moves the movement baseline). The task's `asynq.Unique(1h)` is a safety net only.

### AI pre-moderation of books

Off by default. AI moderation is active only when `AI_MODERATION_ENABLED=true` **and** `ANTHROPIC_API_KEY` is set (a flag without a key logs `AI_MODERATION_ENABLED is set but ANTHROPIC_API_KEY is empty; AI moderation disabled`; otherwise the worker logs `AI moderation disabled`). When active, the worker runs its own asynq server for it (queue `ai_moderation`, `Concurrency: 1`, at least 500 ms between model calls) and the scheduler enqueues `ai_moderation:sweep` on `AI_MODERATION_CRON` (UTC). A sweep gives up to `AI_MODERATION_BATCH` pending books a verdict (no verdict first, then oldest first; also books whose verdict has an older prompt version or that were edited after it). Admins can also queue a check from `/moderation/books` («Проверить ИИ (N)», «Проверить ИИ все непроверенные», up to 1000 books). Verdicts (`publish` / `reject` / `needs_human` with reason codes) live in `book_ai_verdicts`; the model never edits a book and nothing is published automatically. A temporary API failure (429, 5xx, 529 overloaded, network) stores nothing and stops the run; the next run continues.

**Both `backend` (api) and `worker` need the flag and the key.** The api never calls the model, but it uses the same switch for `GET /admin/books/ai-review/status` and the trigger endpoint (503 when off), so the UI only offers buttons the worker would act on. `ANTHROPIC_API_KEY` is a secret: keep it in `.env` only, never commit it; it is never logged.

| Variable | Meaning |
|---|---|
| `AI_MODERATION_ENABLED` | `true` turns AI moderation on (with a key). Default `false`, also in dev and CI. |
| `ANTHROPIC_API_KEY` | Anthropic API key. Empty = off. |
| `AI_MODERATION_MODEL` | Exact model id. Default `claude-haiku-5-5`; `claude-sonnet-5-5` is about 20x more expensive. No date-suffixed ids. |
| `AI_MODERATION_CRON` | Cron spec of the sweep, UTC. Default `*/30 * * * *`. An invalid spec makes the worker exit at startup. |
| `AI_MODERATION_BATCH` | Max books per scheduled sweep. Default `50`, clamped to 1..200; a non-integer value means the default. |

The single-replica rule applies here too: two workers would double the sweeps (the sweep's `asynq.Unique(1h)` is a safety net only). Check `docker compose -f docker-compose.prod.yml logs worker` for `AI moderation enabled`, `AI verdict stored` and `AI sweep finished`.

Cost (assumptions: about 2,200 input tokens per book — a ~1,500-token system prompt without caching plus ~700 tokens of book JSON — and about 400 output tokens including low-effort thinking):

| Model | Input / 1000 books | Output / 1000 books | Total / 1000 books |
|---|---|---|---|
| `claude-haiku-5-5` ($0.10 / $0.50 per 1M) | 2.2M tok → $0.22 | 0.4M tok → $0.20 | **≈ $0.42** (≤ $0.60 worst case) |
| `claude-sonnet-5-5` ($2 / $10 per 1M) | $4.40 | $4.00 | **≈ $8.40** |

Upper bound with the defaults: 50 books × 48 sweeps per day = 2,400 books per day, about $1 per day on Haiku; in practice the inflow is the daily harvest (tens of books). Token counts are stored per verdict, so the real spend is one query away: `SELECT model, sum(input_tokens), sum(output_tokens) FROM book_ai_verdicts GROUP BY model;` (note that a recomputed verdict replaces the previous row, so this counts the latest computation per book).

### AI author curation

Off by default. The AI curator (Импорт → Авторы → «Подбор ИИ», `/import/authors/ai`) is active only when `AI_CURATOR_ENABLED=true` **and** `ANTHROPIC_API_KEY` is set — the same key as AI pre-moderation, but a separate switch, so either feature can be turned on or rolled back alone. A flag without a key logs `AI_CURATOR_ENABLED is set but ANTHROPIC_API_KEY is empty; AI curation disabled`; otherwise the worker logs `AI curation disabled`. When active, the worker runs its own asynq server (queue `ai_curator`, `Concurrency: 1`, one request at a time) for `ai_curator:curate`: one model call per request returns at most N author **names**, and each name is resolved through the worker's shared `WikidataClient` (the same rate limiter as the harvest), keeping only people with a Russian label. The model never supplies catalog data; admins take candidates through the existing author import / harvest / `auto_harvest` flows and everything lands `pending`. Limits: prompt ≤ 500 characters, 1–30 names (default 20), one active request per admin, 20 requests per admin per rolling 24 h.

**Both `backend` (api) and `worker` need the flag and the key.** The api never calls the model; it stores the request and queues the task, and uses the same switch for `GET /admin/ai-curator/status` and `POST /admin/ai-curator/requests` (503 when off). If only the api has the flag, requests stay queued and show as failed/`timeout` after one hour.

| Variable | Meaning |
|---|---|
| `AI_CURATOR_ENABLED` | `true` turns AI curation on (with `ANTHROPIC_API_KEY`). Default `false`, also in dev and CI. |
| `AI_CURATOR_MODEL` | Exact model id. Default `claude-haiku-5-5` (hallucinated names are filtered by the Wikidata check); `claude-sonnet-5-5` if recall is poor. Blank means the default. |

Requests and their candidates live in `ai_curation_requests` / `ai_curation_candidates` (migration 000023) and are deleted after **30 days** (the purge runs at the start of every curate task; catalog data is never touched). Check `docker compose -f docker-compose.prod.yml logs worker` for `AI curation enabled`, `AI curation model answer` and `AI curation finished`; the prompt text is never logged.

Cost per request (assumptions: about 0.9k input tokens — a ~700-token cached system prompt plus the request — and about 2.4k output tokens for 30 names including low-effort thinking):

| Model | Per request | 20 requests/day (the per-admin limit) |
|---|---|---|
| `claude-haiku-5-5` ($0.10 / $0.50 per 1M) | ≈ $0.0013 | ≈ $0.03 |
| `claude-sonnet-5-5` ($2 / $10 per 1M) | ≈ $0.026 | ≈ $0.52 |

Token counts are stored per request: `SELECT model, sum(input_tokens), sum(output_tokens) FROM ai_curation_requests GROUP BY model;` (only the last 30 days are kept).

### AI book descriptions

Off by default. AI description drafts are active only when `AI_DESCRIPTIONS_ENABLED=true` **and** `ANTHROPIC_API_KEY` is set — the same key as AI pre-moderation and curation, but a separate switch. A flag without a key logs `AI_DESCRIPTIONS_ENABLED is set but ANTHROPIC_API_KEY is empty; AI descriptions disabled`; otherwise the worker logs `AI descriptions disabled`. There is no schedule: an admin queues generation for one book («Описание ИИ» panel on the book edit page), or for up to 200 published books with no description and no AI description record yet (Модерация → «Описания ИИ», `/moderation/descriptions`). When active, the worker runs its own asynq server (queue `ai_descriptions`, `Concurrency: 1`, at least 500 ms between model calls) for `ai_descriptions:generate` and `ai_descriptions:sweep`. The model sees only title, author names, year, ISBN, language and genre names, and answers either a 2–4 sentence Russian annotation or "not enough knowledge" (stored as `unknown`).

Every result is a **draft** in `book_ai_descriptions` (migration 000024), never written to `books.description`. An admin approves (as is or edited) or discards each draft one at a time; there is no bulk approval. The public book page shows an approved AI description only while the book has no description of its own, under «Описание создано ИИ и может содержать неточности.»; a sourced or admin-written description always takes over, and AI text appears nowhere else (home, search, lists, AI verdict input). Approving and discarding work while the feature is off.

**Both `backend` (api) and `worker` need the flag and the key.** The api never calls the model; it queues the task and uses the same switch for `enabled` in `GET /admin/ai-descriptions/status` and the 503 of `POST /admin/ai-descriptions/generate`. If only the api has the flag, the button is shown but nothing is generated.

| Variable | Meaning |
|---|---|
| `AI_DESCRIPTIONS_ENABLED` | `true` turns AI description generation on (with `ANTHROPIC_API_KEY`). Default `false`, also in dev and CI. |
| `AI_DESCRIPTIONS_MODEL` | Exact model id. Default `claude-sonnet-5-5` (the only generative task: better knowledge of Russian works and better "I don't know" calibration); `claude-haiku-5-5` to cut cost. Blank means the default. |

Review notes: the model can still hallucinate plausible details or reproduce a known publisher blurb nearly verbatim; neither can be detected automatically. Read every draft before approving, prefer «Изменить и одобрить» to fix wording, and do not paste AI text into the book's own description (that would drop the public AI label). An approved text whose book data changed later is marked «устарел» in the admin app and stays public until regenerated or discarded. Check `docker compose -f docker-compose.prod.yml logs worker` for `AI descriptions enabled`, `AI description stored` and `AI description sweep finished`; the prompt and the text are never logged.

Cost (assumptions: about 1.05k input tokens per book — a ~0.9k system prompt counted uncached plus ~0.15k of book JSON — and about 0.6k output tokens including low-effort thinking; `unknown` answers cost about the same):

| Model | Input / 1000 books | Output / 1000 books | Total / 1000 books |
|---|---|---|---|
| `claude-haiku-5-5` ($0.10 / $0.50 per 1M) | $0.11 | $0.30 | **≈ $0.41** |
| `claude-sonnet-5-5` ($2 / $10 per 1M), default | $2.10 | $6.00 | **≈ $8.10** |

One bulk run (200 books) costs about $1.60 on Sonnet. Token counts are stored per book: `SELECT model, sum(input_tokens), sum(output_tokens) FROM book_ai_descriptions GROUP BY model;` (a regeneration replaces the previous row, so this counts the latest generation per book).

### AI collection drafts

Off by default. AI drafts of manual curated collections are active only when `AI_COLLECTIONS_ENABLED=true` **and** `ANTHROPIC_API_KEY` is set — the same key as the other AI features, but a separate switch. A flag without a key logs `AI_COLLECTIONS_ENABLED is set but ANTHROPIC_API_KEY is empty; AI collection drafts disabled`; otherwise the worker logs `AI collection drafts disabled`. An admin opens a manual collection, uses «Черновик ИИ» next to «Вставить список», describes the collection and asks for a ranked list (1–1000 books; the default is the collection size plus a fifth). The worker runs its own asynq server (queue `ai_collections`, `Concurrency: 1`, at least 500 ms between model calls) for `ai_collections:draft`, one draft at a time, in chunks of 100 books that continue the ranking and exclude the books already proposed. A transient model failure is retried and resumes after the stored chunks; a draft with at least one stored proposal ends `done` even when a later chunk failed (`error_code` then names the failure).

The model only proposes books (title, author, original title, a short note). Matching against the catalog is computed when an admin reads the draft, with the bulk-paste rules (plus a retry by the original title); a book missing from the catalog is only listed under «Нет в каталоге» and is never created or added. The admin applies the chosen books through the existing bulk add, appended at the end or inserted at a position; applying never removes or reorders the books already in the collection.

**Both `backend` (api) and `worker` need the flag and the key.** The api never calls the model; it queues the task and uses the same switch for `enabled` in `GET /admin/ai-collections/status` and the 503 of `POST /admin/ai-collections/drafts`. If only the api has the flag, a draft is queued and then shown as failed with `timeout` after an hour; if only the worker has it, the admin sees the feature as off.

Limits: one active draft per collection (409), 20 drafts per admin per rolling 24 hours, failed ones included (429), a brief of 1–500 characters. Drafts (and their proposals) are kept for 30 days and are deleted with their collection (migration 000028).

| Variable | Meaning |
|---|---|
| `AI_COLLECTIONS_ENABLED` | `true` turns AI collection drafts on (with `ANTHROPIC_API_KEY`). Default `false`, also in dev and CI. |
| `AI_COLLECTIONS_MODEL` | Exact model id. Default `claude-sonnet-5-5` (recall of real, correctly named and attributed works is the whole value; Haiku invents more titles, which only turn into «Нет в каталоге» noise); `claude-haiku-5-5` to cut cost. Blank means the default. |

Cost per chunk of 100 books (assumptions: about 1k input tokens of system prompt plus the brief and context, plus about 15 tokens per already-proposed book in the exclusion list, at most about 13.5k at rank 901; about 6k output tokens including low-effort thinking):

| Model | Top-100 (1 chunk) | Top-1000 (10 chunks) |
|---|---|---|
| `claude-sonnet-5-5` ($2 / $10 per 1M), default | ≈ $0.06 | ≈ $0.75 |
| `claude-haiku-5-5` ($0.10 / $0.50 per 1M) | ≈ $0.003 | ≈ $0.04 |

The daily cap of 20 drafts per admin bounds the worst case at about $15 per admin per day on Sonnet. Token counts are stored per draft: `SELECT model, sum(input_tokens), sum(output_tokens) FROM ai_collection_drafts GROUP BY model;` (only the last 30 days are kept).

### AI genre suggestions

Off by default. The AI analysis of the genre catalog is active only when `AI_GENRES_ENABLED=true` **and** `ANTHROPIC_API_KEY` is set — the same key as the other AI features, but a separate switch. A flag without a key logs `AI_GENRES_ENABLED is set but ANTHROPIC_API_KEY is empty; AI genre suggestions disabled` in the worker and the feature stays off.

The analysis proposes three kinds of suggestions on `/moderation/genres/ai-suggestions`: duplicate genres to merge (spelling, language, synonyms), overly narrow genres to merge into a general one, and "genres" that are not genres (a year, a marketing label, a format) to reject. **The AI never changes a genre by itself, whatever its confidence**: an admin applies a suggestion (through the existing batch genre merge with its preview, or the genre bulk status) or dismisses it. A dismissed suggestion is not proposed again for the same set of genres; a suggestion whose genres changed since the analysis (merged, rejected, deleted) is marked stale and cannot be applied.

**Both `backend` (api) and `worker` need the flag and the key.** The api never calls the model; it records a run and queues `ai_genres:analyze`, and uses the same switch for `enabled` in `GET /admin/genres/ai-suggestions/status` and the 503 of `POST /admin/genres/ai-suggestions/analyze`. When the feature is off, existing suggestions stay visible and can still be applied or dismissed.

A run goes on its own queue `ai_genres` (Concurrency 1, one model call per second at most): the 100 most popular published genres go into every request without sample books, the other pending and published genres are sent in batches of `AI_GENRES_BATCH` with up to two book titles each. **At most 10 requests per run**, so a very large catalog is covered over several runs. A transient API failure (rate limit, overload, network) stops the run; suggestions of earlier batches are kept and the next scheduled or manual run continues. A scheduled run makes no model call at all when no candidate genre was added, renamed or changed status or QID since the last clean run. A run that stays queued or running for more than 2 hours is treated as failed (`interrupted`). Runs older than 180 days are deleted; suggestions are kept (dismissed ones are the memory).

| Variable | Meaning |
|---|---|
| `AI_GENRES_ENABLED` | `true` turns the AI genre analysis on (with `ANTHROPIC_API_KEY`). Default `false`, also in dev and CI. |
| `AI_GENRES_MODEL` | Exact model id. Default `claude-sonnet-5-5` (judging whether two names mean one genre, across languages, needs language knowledge); `claude-haiku-5-5` to cut cost. Blank means the default. |
| `AI_GENRES_CRON` | UTC cron spec of the scheduled run. Default `0 3 * * 1` (Monday 03:00 UTC). Set it empty to disable the schedule (manual runs only); in compose the empty value is passed through. |
| `AI_GENRES_BATCH` | Non-anchor genres per model request. Default `300`, at most `500` (larger values are clamped). |

Cost: the system prompt is cached; a full run is at most 10 requests of about 100 anchor genres (≈ 12 tokens each) plus a batch of up to 500 genres with samples (≈ 45 tokens each), so ≈ 240k input and up to ≈ 80k output tokens in the worst case — about $1.3 per run on Sonnet ($2 / $10 per 1M), about $0.07 on Haiku. A small catalog is one request (a few cents). With the weekly default and the change check, a quiet catalog costs nothing. Token counts are stored per run: `SELECT created_at, model, batches_done, suggestions_created, input_tokens, output_tokens FROM genre_ai_runs ORDER BY created_at DESC;` (migration 000032).

## Self-hosted covers

Book covers are copied from their source link (mostly Open Library) to our own storage and served by the backend as `GET /covers/{shard}/{hash}.{jpg|png|webp}` with `Cache-Control: public, max-age=31536000, immutable`. A cold cover then costs tens of milliseconds instead of 2.5 s through Open Library and archive.org. `books.cover_url` stays the **source** link; the copy state lives in the `books.cover_*` columns (migration `000037_book_cover_mirror`). While a copy is not stored the API returns the source link, so nothing breaks if the worker is down. OpenSpec: `catalog/book-covers`.

- **Volume `covers-data`**: mounted at `/data/covers`, **read-write in `worker`** (the only writer) and **read-only in `backend`** (it only serves files). Both compose files declare it. The production image pre-creates `/data/covers` owned by `nonroot` (65532), so a fresh volume is writable by the worker without a manual `chown`.
- **Web and admin** proxy `/covers/*` to the backend (a Next `rewrite`, like `/api`, resolved from `API_URL` when the image is built; see "Rewrite target" below). Set `SITE_URL` (prod compose: `https://${WEB_DOMAIN}`) on `web` so the Open Graph and structured-data images are absolute URLs.
- **Queue `covers`** (own asynq server, `Concurrency: 1`, downloads at least 1 s apart). `covers:mirror` is queued after any save that writes a cover link (admin create/update, CSV/JSON import, Wikidata harvest). A `covers:sweep` cron (default every 15 minutes) backfills books without a copy, retries transient failures (10 min, doubling, up to 24 h; after 6 failures the copy is `failed` and tried once more after 7 days), re-copies files that vanished from the volume and deletes unreferenced files older than 1 hour.
- **No backup needed.** The files are a cache of the cover links: if the volume is lost or new, the first sweep sees the new storage (marker file `.bookhaven-covers`), sets every stored copy back to `pending` and re-downloads them (about 1 per second). Books show the source link meanwhile. The same happens after restoring the database onto a fresh volume.
- **Mixed runs.** A native backend (`./storage/covers`, gitignored under `backend/storage/`) and the docker worker use different storages: a native backend does not see files the docker worker stored and shows the source links. Run both natively or both in docker.

| Variable | Meaning |
|---|---|
| `COVERS_DIR` | Storage root. Default `./storage/covers` for native runs; `/data/covers` in compose. |
| `COVERS_MIRROR_ENABLED` | Default `true`. `false` on api and worker: nothing is queued and no sweep runs, files already stored are still served. |
| `COVERS_SWEEP_CRON` | UTC cron spec of the sweep (worker). Default `*/15 * * * *`; set it empty to disable the sweep (in compose the empty value is passed through). |
| `COVERS_SWEEP_BATCH` | Books copied per sweep. Default `50`, at most `500`. |
| `COVERS_ALLOWED_HOSTS` | Comma-separated download allowlist; `*.example.org` allows subdomains of `example.org` but not the bare domain. Default `covers.openlibrary.org,archive.org,*.archive.org,upload.wikimedia.org`. |

Downloads are limited to `http`/`https` on ports 80/443, hosts on the allowlist, public addresses only (private, loopback and link-local addresses are refused when connecting, also after redirects; at most 5 redirects), 30 s and 5 MB, and the bytes must be a JPEG, PNG or WebP. A link that breaks a rule is `failed` and the book keeps showing the source link.

**Adding a host.** Add it to `COVERS_ALLOWED_HOSTS` of the **worker** (`covers.example.org`, or `*.example.org` for subdomains; keep the default hosts in the list) and restart the worker. Books that were `failed` with «сайт не входит в список разрешённых» are retried when an admin saves them again; the sweep does not retry permanent failures on its own. Any `next/image` remote host used for fallback links still needs a `remotePatterns` entry in `apps/web/next.config.js`.

**Rewrite target (build time).** Next resolves `rewrites()` when the image is built and stores the destinations in the routes manifest, so the `/api/:path*` and `/covers/:path*` rewrites of the production `web`/`admin` images point wherever `API_URL` pointed during `next build`. The builder stage of `infrastructure/docker/Dockerfile.node` therefore has `ARG API_URL=http://backend:8080` (the compose service on the `internal` network), and the Deploy workflow passes the same value as a `build-args` entry; override it with `--build-arg API_URL=...` only if the backend service is renamed. `turbo.json` lists `API_URL` in the `env` of the build tasks: Turborepo 2 runs tasks in strict env mode and drops undeclared variables, so without it `next build` would never see the value (it also makes `API_URL` part of the build cache key). Admin's `/api` rewrite prefers `API_URL` over `NEXT_PUBLIC_API_URL` for the same reason. The runtime `API_URL` in `docker-compose.prod.yml` is still needed: server-side API calls read it when they run. Without the build value the rewrites would point at `http://localhost:8080` inside the container, and `/covers/*`, the image optimizer's fetch of `/covers/...` and browser `/api` calls through `web`/`admin` would fail (#537). Verify on the first deploy: `curl -I https://$WEB_DOMAIN/covers/<path of a stored cover>` returns `200` with `immutable`, and `https://$WEB_DOMAIN/_next/image?url=%2Fcovers%2F<path>&w=384&q=75` returns `200`.

Check the state: `SELECT cover_status, cover_error, count(*) FROM books WHERE cover_url <> '' GROUP BY 1, 2;` and `docker compose logs worker | grep -i cover`.

## Email (SMTP)

Transactional email (verification, password reset, later notifications) is sent asynchronously: the api enqueues an `email:send` task on the `email` asynq queue and the **worker** delivers it over SMTP (see `backend/internal/pkg/mailer` and [docs/features/mailer-infrastructure.md](features/mailer-infrastructure.md)). Only the `worker` container receives the SMTP variables; the `backend` (api) container never talks to SMTP.

> **Configure real SMTP (and SPF/DKIM/DMARC) on the worker *before* go-live.** Registration sends a verification link by email, and since #160 ([email-verification-login-gate.md](features/email-verification-login-gate.md)) users with an unverified email **cannot log in at all** (register issues no session; login answers 403 `email_not_verified`). With `SMTP_HOST` empty the worker only logs emails, so every new user would be locked out. Before opening registration, register a test account against production and confirm the link arrives (and passes SPF/DKIM/DMARC, see below); watch the worker logs for send errors. Password reset (#161, [password-reset.md](features/password-reset.md)) is also email-only: with broken SMTP nobody can recover a forgotten password.

### Environment variables (backend / api)

| Variable | Meaning |
|---|---|
| `AUTH_COOKIE_SECURE` | `Secure` attribute on the auth cookies (`access_token`, `refresh_token`, `has_session`). Default: on when `APP_ENV=production` (prod compose sets `true`), off in development; an explicit `true`/`false` always wins (an unparsable value falls back to the default). **TLS must terminate in front of the api/web**, otherwise browsers drop Secure cookies and login appears to succeed but the session is lost; set `false` only for an HTTP-only test box (the api logs a warning at startup). |
| `DB_MAX_OPEN_CONNS` | Max open PostgreSQL connections per process (api, worker, similarity). Integer >= 1. Default `20`. Invalid values fall back to the default. |
| `DB_MAX_IDLE_CONNS` | Max idle connections kept per process. Integer >= 0, capped at `DB_MAX_OPEN_CONNS`. Default `5`. |
| `DB_CONN_MAX_LIFETIME` | Recycle a connection after this age (Go duration, e.g. `30m`; `0` = no limit). Default `30m`. |
| `DB_CONN_MAX_IDLE_TIME` | Close a connection idle this long (Go duration; `0` = no limit). Default `5m`. |
| `WEB_BASE_URL` | Public origin of the web app, used to build absolute links in emails (e.g. `https://<WEB_DOMAIN>/verify-email?token=...`). Set in `docker-compose.prod.yml` as `https://${WEB_DOMAIN}` — no separate `.env` key. Must be an absolute `http(s)` URL; `config.Load` refuses to start in production when it is not set explicitly (the worker gets the same value for that reason). Dev default: `http://localhost:3000`. |

The `DB_*` pool limits apply per process. Connection budget: (number of api replicas + worker + one similarity run) x `DB_MAX_OPEN_CONNS` must stay below PostgreSQL `max_connections` (100 by default); with the defaults that is 60 for one api, one worker and one similarity run.

The api also uses Redis directly (not only via asynq): rate limiting of the public "resend verification email" endpoint — `rl:verify-resend-ip:*` (20/hour per client IP) and `rl:verify-resend-email:*` (1/minute and 5/day per address; the key is the sha256 of the lowercased email, no plain addresses in Redis). The public "forgot password" endpoint (#161, [password-reset.md](features/password-reset.md)) has its own, separately counted limits with the same shape: `rl:pwreset-ip:*` (20/hour per client IP) and `rl:pwreset-email:*` (1/minute and 5/day per address, same sha256 key). Login throttling (epic #565): `rl:login-ip:*` (60 attempts of any outcome / 15 min per client IP; correct passwords count too, so an own-account login cannot keep the counter flat) and `rl:login-email:*` (10 failed attempts / 15 min per address, sha256 key as above; cleared by a correct password or a password reset). The window counter key is named `rl:login-*:day:<key>` by the ratelimit package regardless of the 15-minute window. Unlike the email-sending endpoints (which fail closed with 500), login fails open on Redis errors (warn log) so a Redis outage does not lock everyone out. It reuses the existing `REDIS_URL`; no extra configuration.

**Client IP behind Caddy.** The per-IP limits (login, resend-verification and forgot-password) use the client IP from `X-Forwarded-For` (`e.IPExtractor = echo.ExtractIPFromXFFHeader()` in `backend/cmd/api/main.go`). Echo only honours hops from trusted proxies (default: loopback, link-local and private networks): Caddy reaches the api over the private Docker network and, by default, does not trust a client-supplied `X-Forwarded-For`, so the header carries the real client IP and cannot be spoofed by calling the api directly. Do not put another proxy/CDN in front of Caddy (or expose the api port publicly) without revisiting the `IPExtractor` trust settings — otherwise either all requests share one IP (the 20/hour limit hits everyone) or the header becomes spoofable.

### Environment variables (worker)

| Variable | Meaning |
|---|---|
| `SMTP_HOST` | Provider's SMTP submission host. **Empty = sending disabled**: emails are only logged (recipient + subject; bodies are never logged in production) and the worker logs a `SMTP not configured` warning at startup. |
| `SMTP_PORT` | `587` (default, plain connect + STARTTLS) or `465` (implicit TLS). Any port other than 465 upgrades via STARTTLS when the server offers it. |
| `SMTP_USER` / `SMTP_PASSWORD` | Credentials (PLAIN auth). Credentials are only ever sent over an encrypted connection. `SMTP_PASSWORD` is a secret: never log or paste it outside the server's `.env`. |
| `MAIL_FROM` | Sender, e.g. `BookHaven <noreply@bookhaven.example.com>`. Must be a valid address, and must be set explicitly in production (the worker refuses to start otherwise when `SMTP_HOST` is set) and must be on the domain that carries the SPF/DKIM/DMARC records below. |

Failed sends are retried by asynq (up to 8 retries with backoff, roughly a few hours of provider outage); permanent rejections (SMTP 5xx) are not retried, and neither is the misconfiguration "credentials set but the connection to a non-local server is unencrypted (port is not 465 and the server offers no STARTTLS)". Failed/archived tasks can be inspected with asynqmon / `asynq.Inspector` against the production Redis.

### Choosing a provider

Use a transactional SMTP service with an endpoint reachable from the VPS: for example Unisender Go, SendPulse, or Yandex 360 (fine for small volume); Postmark, Mailgun or Amazon SES if they are reachable and payable from your jurisdiction. Use port **587** (STARTTLS) or **465** (TLS) — many VPS providers block outbound port 25, and port 25 is not meant for authenticated submission anyway.

### DNS records (deliverability)

Without these, mail lands in spam or is rejected. All records go on the domain used in `MAIL_FROM`:

1. **SPF** — a TXT record on the sender domain authorizing the provider, e.g. `v=spf1 include:<provider-spf-domain> ~all`. A domain may have **only one** SPF record: if one already exists (e.g. for Yandex 360 mailboxes), merge the `include:` into it instead of adding a second record.
2. **DKIM** — the provider issues one or more DKIM records (TXT or CNAME, e.g. `<selector>._domainkey.<domain>`); add them exactly as given and complete the domain verification in the provider's dashboard.
3. **DMARC** — a TXT record at `_dmarc.<domain>`, starting in monitoring mode: `v=DMARC1; p=none; rua=mailto:dmarc-reports@<domain>`. After a few weeks of clean aggregate reports, tighten to `p=quarantine` (and later `p=reject` if desired).

### Verifying

Send a real email to a Gmail and a Yandex inbox — either through the first real flow (registration: the verification email) or by enqueuing a test email from a machine that can reach the production Redis (the dev-only `go run ./cmd/mailtest -to you@example.com` CLI is not shipped in the production image). In the received message open "Show original" / "Message properties" and check for `spf=pass`, `dkim=pass` and `dmarc=pass`. Watch `docker compose -f docker-compose.prod.yml logs -f worker` for send errors.

## Monitoring

Kept intentionally minimal for an MVP with no ops budget:

- **Uptime**: point a free external checker (UptimeRobot, or Yandex/VK equivalents) at `https://<API_DOMAIN>/health/ready` and `https://<WEB_DOMAIN>/`. Free tier is enough for one project. Liveness vs readiness: `/health` is liveness (static `200 {"status":"ok"}`, the process serves HTTP; Caddy's active health check uses it and must keep doing so, otherwise a PostgreSQL/Redis blip would turn into 502/503 for the whole API), while `/health/ready` is readiness (pings PostgreSQL and Redis, 2 s timeout each; `503` with `"unavailable"` for the failing dependency, details only in the backend logs). The deploy workflow and uptime monitoring use `/health/ready`.
- **Logs**: `docker compose -f docker-compose.prod.yml logs -f backend|web|admin|caddy` over SSH. The backend already logs structured JSON (zap) with method/URI/status per request.
- **Errors**: opt-in Sentry error tracking, see [Error tracking (Sentry)](#error-tracking-sentry) below.

Revisit this section once there's a real incident or real user volume — don't build observability infrastructure speculatively on a minimal-budget MVP.

### Error tracking (Sentry)

Opt-in and off by default: with no DSN every process behaves exactly as before and makes no network calls (the backend logs one `error tracking disabled` line at startup). Use two Sentry projects (platform Go and Next.js) and put the DSNs **only in the server `.env`**:

| `.env` variable | Passed to | Meaning |
|---|---|---|
| `SENTRY_DSN_BACKEND` | `backend`, `worker` as `SENTRY_DSN` | DSN of the Go project. Empty = off. |
| `SENTRY_DSN_FRONTEND` | `web`, `admin` as `SENTRY_DSN` | DSN of the Next.js project. Empty = off. Read at runtime on the server; it is not a `NEXT_PUBLIC_*` variable and never reaches the browser bundle. |
| `SENTRY_ENVIRONMENT` (optional) | all, set it in the compose `environment` if you need it | Overrides the environment tag (default `APP_ENV` for the backend, `NODE_ENV` for Next.js, i.e. `production`). |

`SENTRY_RELEASE` is filled by `docker-compose.prod.yml` from `IMAGE_TAG` (the 12-character git SHA the deploy workflow writes to `.env`), so the release always matches the deployed image. Every event has `environment`, `release` and a `component` tag (`api`, `worker`, `web`, `admin`); web and admin share one project and are told apart by that tag.

What is reported:

- **API**: requests that end with status 5xx and panics, one issue per method + route template + status (for example `HTTP 500 GET /api/v1/shelves/:id`). Most handlers write the 500 JSON and drop the underlying error, so these events show the route and status but no stack; the request log line holds the detail. Never reported: 4xx, `/health` and `/health/ready` (including a readiness 503).
- **Worker**: a task is reported only when it fails for good (asynq retries are used up, or the error is `SkipRetry`), with tags `task_type`, `queue`, `retried`, `max_retry`. Intermediate failures that asynq will retry are not reported. The task payload is never sent.
- **web/admin**: Next.js server-side errors (server components, route handlers, server-side data fetching) from the Node.js runtime; `notFound()`, `redirect()` and 4xx errors from the API are dropped. The edge `middleware.ts` and browser errors are not covered.

Privacy: no default PII, no request headers/cookies/body/query/IP, no user, no breadcrumbs, no task payload, no performance tracing, profiling or session replay. Emails, `Bearer` tokens, JWTs and `token=`/`password=`/`secret=` values are redacted from messages and exception text (best effort).

Quota: during an outage (for example PostgreSQL down) every route answers 5xx. Grouping by route keeps the number of issues small, but the event count can still hit the free monthly quota, so enable spike protection / a rate limit in each Sentry project.

Follow-up (not implemented): browser-side reporting. It adds roughly 25-30 kB gzip to every first load and needs a way to reach the ingest host. Prefer the SDK's `tunnelRoute` (same-origin POST through the app, `connect-src 'self'` keeps working, CSP unchanged) over adding the Sentry ingest host to the CSP.

## Security headers

- **Who sets what**: the apps own their headers (web/admin via `headers()` in `next.config.js` from `security-headers.js`; the API via the `SecurityHeaders()` Echo middleware), so they are identical in local dev and prod. Caddy adds **only** `Strict-Transport-Security: max-age=31536000` (the `(hsts)` snippet in the Caddyfile), because only Caddy knows the connection is TLS. Caddy must not set any header the apps set.
- **CSP is enforced** and pragmatic: `script-src 'unsafe-inline'` is required by the App Router inline bootstrap scripts and the JSON-LD block, so the policy guards framing, objects, base-uri, forms and third-party scripts but is not an XSS stopper. A nonce-based CSP is a possible follow-up. `img-src https:` is intentional (Wikimedia photos, arbitrary avatar URLs).
- **HSTS has no `preload` and no `includeSubDomains`** until prod has run on TLS for a while; it is easy to back out by lowering `max-age`.
- **`connect-src` API origin** comes from `NEXT_PUBLIC_API_URL` at image build time. The prod images are built with only `API_URL` as a build arg (see `deploy.yml` / `Dockerfile.node`), so `NEXT_PUBLIC_API_URL` is empty at build, the browser client uses the same-origin `/api` rewrite and `connect-src 'self'` is sufficient. If you ever pass `NEXT_PUBLIC_API_URL` as a build arg, the API origin is added automatically.

## Security checklist

- [x] SSH: key-only, no root login, no password auth (server-setup.sh)
- [x] Firewall: only 22/80/443 exposed; Postgres/Redis/backend/web/admin are on the internal Docker network only
- [x] `fail2ban` enabled for SSH brute-force protection
- [x] Backend production image is distroless, non-root (`nonuser:nonuser`)
- [x] CORS restricted to the real production origins, not `*`
- [ ] `.env` on the server is `chmod 600`, root/deploy-owned only, never committed — verify after every manual edit
- [ ] Rotate `JWT_SECRET`/`POSTGRES_PASSWORD`/`SMTP_PASSWORD` if they were ever pasted anywhere outside the server's `.env` (chat, ticket, etc.)
- [ ] `SENTRY_DSN_BACKEND` / `SENTRY_DSN_FRONTEND` are set only in the server's `.env` (never committed, never `NEXT_PUBLIC_*`); empty is fine
- [ ] `SMTP_PASSWORD` is set only in the server's `.env` and only passed to the `worker` container; SPF/DKIM/DMARC records are in place for the `MAIL_FROM` domain before real users receive mail
- [ ] Confirm GHCR package visibility matches intent (public if repo is public, private + PAT otherwise) so images aren't unintentionally exposed or, conversely, undeployable
- [ ] Backups (pre-launch, see [Backups](#backups)): off-site rclone remote configured and `BACKUP_RCLONE_REMOTE`/`BACKUP_AGE_RECIPIENT` set; healthchecks.io check created (period 1 day, grace ~2 h) and `BACKUP_HEARTBEAT_URL` set; cron line installed and one manual run green; first [restore drill](#quarterly-restore-drill) passed from an off-site copy
- [ ] `.env`, the `age` identity and the rclone bucket credentials are stored in the password manager (and the identity is **not** on the server)
- [ ] Credential-stuffing against `/api/v1/auth/login`: the app-layer throttle is in place (60 attempts / 15 min per IP, 10 failed per address); an additional Caddy `rate_limit` (plugin) is optional before opening registration to the public
- [x] Security headers on web/admin/api; [ ] after the first TLS deploy, `curl -I` each domain and confirm HSTS + CSP; consider `includeSubDomains`/`preload` later

## Cost estimate (minimal budget)

- VPS (2 vCPU/4GB, RU provider): roughly 500–1000 ₽/month depending on provider/promo
- Domain (.ru or .com): roughly 200–1500 ₽/year
- Everything else in this plan (GitHub Actions minutes on a public repo, GHCR storage, Let's Encrypt, UptimeRobot free tier) is free at this scale

## Agent/subagent setup for ongoing infra work

A new `devops` subagent (`.claude/agents/devops.md`) was added alongside the existing `feature-planner` / `developer` / `review-qa` trio, scoped to infrastructure-as-code: Dockerfiles, `docker-compose.prod.yml`, `Caddyfile`, `.github/workflows/*.yml`, and the `infrastructure/deploy/` scripts.

It is deliberately **not** wired into a `/deploy` slash command that executes real deploys. Deployment already has a safe, auditable trigger — the Deploy workflow in GitHub Actions (manual `workflow_dispatch` for now; push to `main` once re-enabled) — so an agent doesn't need Bash access to `ssh`/`docker push`/real credentials to get infra changes live; it only needs to edit the files that CI/CD then acts on. Its rules explicitly forbid running anything against the real server or a real registry (see the agent file) and require it to validate changes locally instead (`docker build`, `docker compose config`, `bash -n`). Use it like the other subagents: give it a scoped infra request (e.g. "add a staging environment variable for feature-flag X", "add a healthcheck to the redis service"), review its diff, and commit yourself — same pattern as `/feature`, minus the auto-commit, since infra changes to what's live in production deserve a manual look before merging to `main`.

For application feature work, nothing changes — keep using `/feature` with `feature-planner` → `developer` → `review-qa`.
