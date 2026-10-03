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
2. As root on the fresh server, run `infrastructure/deploy/server-setup.sh` (copy it up first: `scp infrastructure/deploy/server-setup.sh root@<ip>:/root/`). It installs Docker Engine + Compose plugin, creates a non-root `deploy` user in the `docker` group, disables SSH password auth and root login, configures `ufw` (22/80/443 only) and `fail2ban`, and creates `/opt/bookhaven`.
3. Append `deploy_key.pub` to `/home/deploy/.ssh/authorized_keys` on the server (the script copies root's existing `authorized_keys`, but add the dedicated deploy key explicitly if it's not already there).
4. From your machine, copy the compose files up:
   ```
   scp docker-compose.prod.yml Caddyfile deploy@<ip>:/opt/bookhaven/
   scp .env.production.example deploy@<ip>:/opt/bookhaven/.env
   ```
5. SSH in as `deploy` and edit `/opt/bookhaven/.env`: fill in the real domains, `ACME_EMAIL`, a generated `POSTGRES_PASSWORD` (`openssl rand -base64 24`), a generated `JWT_SECRET` (`openssl rand -base64 48`), and `CORS_ALLOWED_ORIGINS` matching the web/admin domains exactly (scheme + host, no trailing slash). Fill in the SMTP block (`SMTP_HOST`, `SMTP_PORT`, `SMTP_USER`, `SMTP_PASSWORD`, `MAIL_FROM`) from your email provider — see [Email (SMTP)](#email-smtp); leaving `SMTP_HOST` blank disables sending. Then `chmod 600 .env`.
6. If the GHCR packages end up private (private repo, default), log in once on the server with a PAT scoped to `read:packages`: `docker login ghcr.io -u <github-user> -p <PAT>`. If the repo is public, make the three GHCR packages public instead (Package settings on GitHub) and skip this — one less credential to manage on the server.

## GitHub setup

1. Push this repo to GitHub (`main` branch — rename `master` if needed, or edit the workflow branch filters).
2. Add repository secrets (Settings → Secrets and variables → Actions):
   - `DEPLOY_HOST` — the VPS IP or hostname
   - `DEPLOY_USER` — `deploy`
   - `DEPLOY_SSH_KEY` — the **private** deploy key (`deploy_key`, not `.pub`)
3. Create a GitHub Environment named `production` (Settings → Environments) and, optionally, require manual approval on it if you want a human click before every deploy rather than fully automatic deploy-on-push.
4. `GITHUB_TOKEN` (used to push to GHCR) is provided automatically by Actions — no setup needed, just confirm Settings → Actions → General → Workflow permissions is set to "Read and write permissions" so it can push packages.

## First deploy

1. Push to `main`. `.github/workflows/deploy.yml` runs CI, builds and pushes the three images tagged with the short commit SHA and `latest`, then SSHes in and runs the compose stack.
2. The very first run also needs the database migrated: the deploy workflow already runs `docker compose run --rm --entrypoint /app/migrate backend up` before `up -d`, so this happens automatically — including on the first deploy.
3. Seed data (optional, for a demo account) is not run automatically. `cmd/seed` is dev/demo-only tooling and refuses to run when `APP_ENV=production` unless you pass `-force` — evaluate first whether seed data belongs in a real production database, and only pass `-force` if you genuinely want the same fixed demo seed as dev. If wanted: `docker compose -f docker-compose.prod.yml run --rm --entrypoint /app/migrate backend version` to confirm migrations landed, then adapt `backend/cmd/seed` into a one-off run the same way (with `-force`) if you go ahead.
4. Verify: `https://<API_DOMAIN>/health` returns `{"status":"ok"}`, `https://<WEB_DOMAIN>` loads the catalog, `https://<ADMIN_DOMAIN>` redirects to login.

## Ongoing deploys

Every push to `main` (after CI passes) automatically builds, pushes, and deploys. Treat `main` as production — land features via the existing `/feature` workflow and PRs, don't push directly unless that's a deliberate choice for a one-person project.

Manual re-run: Actions tab → Deploy → Run workflow (`workflow_dispatch` is enabled) — useful to redeploy without a new commit (e.g. after only changing `.env` on the server, or to retry a flaky deploy).

## Rollback

Images are tagged with the short commit SHA, not just `latest`, so rollback is:

```
ssh deploy@<ip>
cd /opt/bookhaven
sed -i 's/^IMAGE_TAG=.*/IMAGE_TAG=<previous-sha>/' .env
docker compose -f docker-compose.prod.yml pull
docker compose -f docker-compose.prod.yml up -d
```

If the rollback also needs a schema rollback, run the down migration explicitly first — `docker compose -f docker-compose.prod.yml run --rm --entrypoint /app/migrate backend down` rolls back **one** migration step; check `backend/migrations` before running it, and prefer forward-fixing over rolling back a schema in a database with real user data whenever possible.

## Backups

`infrastructure/deploy/backup.sh` runs `pg_dump` inside the `postgres` container, gzips it, and keeps 7 days locally. Install it via cron on the `deploy` user:

```
crontab -e
0 3 * * * /opt/bookhaven/infrastructure/deploy/backup.sh >> /var/log/bookhaven-backup.log 2>&1
```

For real durability, uncomment the `rclone` line in the script and configure an S3-compatible off-site remote (Timeweb S3 or Selectel S3 both work) — a backup that lives only on the same disk as the database it's backing up doesn't survive a disk failure.

## Email (SMTP)

Transactional email (verification, password reset, later notifications) is sent asynchronously: the api enqueues an `email:send` task on the `email` asynq queue and the **worker** delivers it over SMTP (see `backend/internal/pkg/mailer` and [docs/features/mailer-infrastructure.md](features/mailer-infrastructure.md)). Only the `worker` container receives the SMTP variables; the `backend` (api) container never talks to SMTP.

> **Configure real SMTP (and SPF/DKIM/DMARC) on the worker *before* go-live.** Registration sends a verification link by email, and since #160 ([email-verification-login-gate.md](features/email-verification-login-gate.md)) users with an unverified email **cannot log in at all** (register issues no session; login answers 403 `email_not_verified`). With `SMTP_HOST` empty the worker only logs emails, so every new user would be locked out. Before opening registration, register a test account against production and confirm the link arrives (and passes SPF/DKIM/DMARC, see below); watch the worker logs for send errors.

### Environment variables (backend / api)

| Variable | Meaning |
|---|---|
| `WEB_BASE_URL` | Public origin of the web app, used to build absolute links in emails (e.g. `https://<WEB_DOMAIN>/verify-email?token=...`). Set in `docker-compose.prod.yml` as `https://${WEB_DOMAIN}` — no separate `.env` key. Must be an absolute `http(s)` URL; `config.Load` refuses to start in production when it is not set explicitly (the worker gets the same value for that reason). Dev default: `http://localhost:3000`. |

The api also uses Redis directly (not only via asynq): rate limiting of the public "resend verification email" endpoint — `rl:verify-resend-ip:*` (20/hour per client IP) and `rl:verify-resend-email:*` (1/minute and 5/day per address; the key is the sha256 of the lowercased email, no plain addresses in Redis). It reuses the existing `REDIS_URL`; no extra configuration.

**Client IP behind Caddy.** The per-IP limit uses the client IP from `X-Forwarded-For` (`e.IPExtractor = echo.ExtractIPFromXFFHeader()` in `backend/cmd/api/main.go`). Echo only honours hops from trusted proxies (default: loopback, link-local and private networks): Caddy reaches the api over the private Docker network and, by default, does not trust a client-supplied `X-Forwarded-For`, so the header carries the real client IP and cannot be spoofed by calling the api directly. Do not put another proxy/CDN in front of Caddy (or expose the api port publicly) without revisiting the `IPExtractor` trust settings — otherwise either all requests share one IP (the 20/hour limit hits everyone) or the header becomes spoofable.

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

- **Uptime**: point a free external checker (UptimeRobot, or Yandex/VK equivalents) at `https://<API_DOMAIN>/health` and `https://<WEB_DOMAIN>/`. Free tier is enough for one project.
- **Logs**: `docker compose -f docker-compose.prod.yml logs -f backend|web|admin|caddy` over SSH. The backend already logs structured JSON (zap) with method/URI/status per request.
- **Errors** (optional, recommended once there's real traffic): Sentry has a free tier and SDKs for both Go/Echo and Next.js — add it as a follow-up, not a blocker for launch.

Revisit this section once there's a real incident or real user volume — don't build observability infrastructure speculatively on a minimal-budget MVP.

## Security checklist

- [x] SSH: key-only, no root login, no password auth (server-setup.sh)
- [x] Firewall: only 22/80/443 exposed; Postgres/Redis/backend/web/admin are on the internal Docker network only
- [x] `fail2ban` enabled for SSH brute-force protection
- [x] Backend production image is distroless, non-root (`nonuser:nonuser`)
- [x] CORS restricted to the real production origins, not `*`
- [ ] `.env` on the server is `chmod 600`, root/deploy-owned only, never committed — verify after every manual edit
- [ ] Rotate `JWT_SECRET`/`POSTGRES_PASSWORD`/`SMTP_PASSWORD` if they were ever pasted anywhere outside the server's `.env` (chat, ticket, etc.)
- [ ] `SMTP_PASSWORD` is set only in the server's `.env` and only passed to the `worker` container; SPF/DKIM/DMARC records are in place for the `MAIL_FROM` domain before real users receive mail
- [ ] Confirm GHCR package visibility matches intent (public if repo is public, private + PAT otherwise) so images aren't unintentionally exposed or, conversely, undeployable
- [ ] Add rate limiting at Caddy (`rate_limit` via a plugin, or keep it at the app layer) before opening registration to the public, to blunt credential-stuffing against `/api/v1/auth/login`

## Cost estimate (minimal budget)

- VPS (2 vCPU/4GB, RU provider): roughly 500–1000 ₽/month depending on provider/promo
- Domain (.ru or .com): roughly 200–1500 ₽/year
- Everything else in this plan (GitHub Actions minutes on a public repo, GHCR storage, Let's Encrypt, UptimeRobot free tier) is free at this scale

## Agent/subagent setup for ongoing infra work

A new `devops` subagent (`.claude/agents/devops.md`) was added alongside the existing `feature-planner` / `developer` / `review-qa` trio, scoped to infrastructure-as-code: Dockerfiles, `docker-compose.prod.yml`, `Caddyfile`, `.github/workflows/*.yml`, and the `infrastructure/deploy/` scripts.

It is deliberately **not** wired into a `/deploy` slash command that executes real deploys. Deployment already has a safe, auditable trigger — a push to `main` through GitHub Actions — so an agent doesn't need Bash access to `ssh`/`docker push`/real credentials to get infra changes live; it only needs to edit the files that CI/CD then acts on. Its rules explicitly forbid running anything against the real server or a real registry (see the agent file) and require it to validate changes locally instead (`docker build`, `docker compose config`, `bash -n`). Use it like the other subagents: give it a scoped infra request (e.g. "add a staging environment variable for feature-flag X", "add a healthcheck to the redis service"), review its diff, and commit yourself — same pattern as `/feature`, minus the auto-commit, since infra changes to what's live in production deserve a manual look before merging to `main`.

For application feature work, nothing changes — keep using `/feature` with `feature-planner` → `developer` → `review-qa`.
