# Production Backups

Status: implemented (2026-10-22, verified locally; operator-only items below remain open until the production server exists)

Issue: epic #570, devops sub-issue #571 (supersedes proposal #555). Area: devops only (no backend/frontend code, no migrations). Implementer: `devops` agent.

## Goal

Make the nightly Postgres backup trustworthy before the first production launch: every copy is verified, encrypted and pushed off-site, a stopped backup raises an alert, and there is a documented and rehearsed restore. RPO stays 24 h; WAL archiving / PITR is out of scope.

## User Stories

- As the operator, I want a backup that survives the loss of the VPS disk, so one failure cannot take the DB and all its copies.
- As the operator, I want to be alerted when the nightly backup fails or stops running.
- As the operator, I want a step-by-step restore I have actually rehearsed, so a restore under stress works the first time.

## Acceptance Criteria

- [x] `backup.sh` dumps with `pg_dump -Fc` into a temp file, verifies it (size > 0 and `pg_restore --list` succeeds), and only then renames it to `bookhaven-<UTC timestamp>.dump`. A failed or truncated dump never gets the final name, is removed by an `EXIT` trap, and never counts toward retention.
- [x] `backup.sh` no longer `source`s `.env`. DB user/name come from the container's own env (`sh -c 'pg_dump ... -U "$POSTGRES_USER" "$POSTGRES_DB"'` inside the `postgres` service). The `BACKUP_*` keys are read by a small parser that does not execute anything (`grep '^KEY='`, strip one layer of surrounding quotes).
- [x] Off-site: the verified dump is encrypted with `age` to a public recipient (`BACKUP_AGE_RECIPIENT`) and uploaded with `rclone` to `BACKUP_RCLONE_REMOTE`. The plaintext dump is never uploaded.
- [x] Fail loud: if `BACKUP_RCLONE_REMOTE` or `BACKUP_AGE_RECIPIENT` is empty, or `age`/`rclone` is missing, the script exits non-zero and sends a failure ping. The only way to skip off-site is an explicit `BACKUP_OFFSITE=off`, which is logged as a warning on every run (used for local verification only).
- [x] Retention: local `*.dump` keeps `BACKUP_LOCAL_RETENTION_DAYS` (default 7). Off-site uses prefixes `daily/` (every run), `weekly/` (Sundays, UTC), `monthly/` (1st of month, UTC); the script prunes with `rclone delete --min-age` at 14d / 63d / 400d. Pruning runs only after a successful upload. Legacy `*.sql.gz` files are cleaned by the same local rule.
- [x] Heartbeat: on success the script `curl`s `BACKUP_HEARTBEAT_URL` (healthchecks.io-compatible); on any failure (ERR trap) it calls `${BACKUP_HEARTBEAT_URL}/fail`. If the URL is empty, it logs a warning and continues (not an error, so local runs work). Ping failures never change the exit code of a successful backup (`curl -fsS -m 10 --retry 3 ... || log warning`).
- [x] Logging: the script appends its own output to `$BACKUP_DIR/backup.log` (timestamped lines, start/end, file name, size, upload target). The cron line has no `/var/log` redirect.
- [x] Paths are overridable for local runs: `APP_DIR` (default `/opt/bookhaven`), `COMPOSE_FILE` (default `docker-compose.prod.yml`), `ENV_FILE` (default `$APP_DIR/.env`), `BACKUP_DIR` (default `$APP_DIR/backups`). Defaults keep the production behavior.
- [x] A new `infrastructure/deploy/restore-drill.sh <file.dump|file.dump.age> [age-identity-file]` restores into a throwaway container (`postgres:16-alpine`, unique name, no published port, no volume, `--rm`), runs `pg_restore --no-owner --exit-on-error`, prints sanity checks (`schema_migrations` version and `dirty`, row counts of `users`, `books`, `authors`), and always removes the container (trap). It never touches the compose stack or its volume.
- [x] `server-setup.sh` installs `age` and `rclone` (apt) and creates `$APP_DIR/backups` owned by the deploy user; its "Next steps" lists configuring the rclone remote, the `BACKUP_*` keys and the cron line. It does not install the crontab itself (needs `.env` first) — the cron install is a documented operator step.
- [x] `docs/DEPLOYMENT.md`: rewritten **Backups** section, a new **Restore** section, a quarterly restore-drill procedure, and the `.env` off-server copy note; the pre-launch checklist gets the backup items. The dangling "(see below)" at the `down-all` paragraph points to the Restore section.
- [x] `.env.production.example` gets the `BACKUP_*` keys with comments (empty values, no secrets).
- [x] Both scripts pass `shellcheck` (run via `docker run --rm -v ...:/mnt koalaman/shellcheck:stable`), and keep LF line endings.

Implementation notes (deviations/additions, all within the contract's intent):
- Verification adds a full `pg_restore -f /dev/null` read after `--list`: tested locally, `--list` alone returns 0 on a dump truncated to 100 KB (it reads only the TOC); the full read fails it.
- Failure handling: the `ERR` trap logs the failing command (written straight to the log, since it can fire inside the `>"$TMP"` redirect); the `/fail` ping and temp cleanup run in the `EXIT` trap on any non-zero exit, so preflight `exit 1` paths (which do not raise `ERR`) also ping `/fail`.
- `BACKUP_*` values already set in the process environment take precedence over `ENV_FILE` (used for `BACKUP_OFFSITE=off` in local runs). Remote pruning additionally filters `--include 'bookhaven-*.dump.age'`. Stale `.inprogress-*` files older than a day are removed.
- `restore-drill.sh` uses no named volume; the anonymous volume from the image's `VOLUME` is removed with the container (`--rm` + `docker rm -f -v` in the trap). It exits non-zero if `schema_migrations` is dirty.
- Root `.gitattributes` (`*.sh text eol=lf`) keeps scripts LF under `core.autocrlf=true`.

## Business Rules

- What is backed up: Postgres only. Not backed up by design: covers volume (re-mirrored from `cover_url`), Redis (caches, rate limits, queued jobs), Caddy certs (re-issued). State this in the Backups section.
- Encryption is asymmetric: the server holds only the `age` public recipient; the private identity lives off-server (password manager). A compromised server cannot decrypt old off-site copies. Losing the identity means losing every off-site backup — say so explicitly.
- `.env` is not in the dump. The operator keeps an encrypted copy of `.env` (and the `age` identity and rclone credentials) in a password manager and updates it after every `.env` change. Losing `JWT_SECRET` logs everyone out; losing `POSTGRES_PASSWORD` is recoverable but annoying.
- RPO 24 h (one run per night, 03:00 server time). RTO target: documented, not enforced.

## Configuration (`.env` keys, read by backup.sh only)

| Key | Default | Meaning |
|---|---|---|
| `BACKUP_RCLONE_REMOTE` | empty → error | rclone destination base, e.g. `backup:bookhaven-backups`. Subprefixes `daily/`, `weekly/`, `monthly/` are appended. |
| `BACKUP_AGE_RECIPIENT` | empty → error | `age1...` public key. |
| `BACKUP_OFFSITE` | `on` | `off` skips encryption+upload with a warning (local testing only). |
| `BACKUP_HEARTBEAT_URL` | empty → warning | Ping URL; `/fail` suffix on failure. |
| `BACKUP_LOCAL_RETENTION_DAYS` | `7` | Local `*.dump` retention. |

rclone remote credentials live in the deploy user's `~/.config/rclone/rclone.conf` (chmod 600), not in `.env`.

## Script flow (backup.sh)

1. `set -euo pipefail`; traps: `EXIT` removes temp files; `ERR` logs and sends `/fail` ping.
2. Redirect output to `$BACKUP_DIR/backup.log` (`exec >>"$LOG" 2>&1`), log start.
3. Read `BACKUP_*` keys from `ENV_FILE` (non-executing parser); preflight: when off-site is on, require both keys and `command -v age rclone`, else fail before dumping.
4. `docker compose -f "$COMPOSE_FILE" exec -T postgres sh -c 'pg_dump -Fc -U "$POSTGRES_USER" "$POSTGRES_DB"' > "$TMP"` (temp file inside `$BACKUP_DIR`, e.g. `.inprogress-<ts>.dump`).
5. Verify: `[ -s "$TMP" ]` and `docker compose ... exec -T postgres pg_restore --list < "$TMP" > /dev/null`. Then `mv` to `bookhaven-<ts>.dump`, `chmod 600`.
6. Off-site (if on): `age -r "$BACKUP_AGE_RECIPIENT" -o "$TMP_AGE" file.dump`; `rclone copyto` to `daily/`; on Sunday/1st also `rclone copyto` to `weekly/`/`monthly/` (server-side copy is fine); then `rclone delete --min-age` pruning per prefix. Remove the local `.age` file.
7. Local retention: `find "$BACKUP_DIR" -maxdepth 1 \( -name 'bookhaven-*.dump' -o -name '*.sql.gz' \) -mtime +N -delete`.
8. Success ping; log end with size and duration.

## Restore (docs/DEPLOYMENT.md → Restore)

Document as numbered commands:
1. Pick the file: local `$APP_DIR/backups/bookhaven-*.dump`, or `rclone copy` from `daily/|weekly/|monthly/` and `age -d -i <identity> -o x.dump x.dump.age` (identity copied temporarily from the password manager, deleted after).
2. Dry run first: `restore-drill.sh x.dump` — confirms the file restores cleanly.
3. Production restore (fresh database, then swap — never restore over the live DB in place):
   - `docker compose -f docker-compose.prod.yml stop backend worker`
   - create `bookhaven_restore` database, `pg_restore --no-owner --exit-on-error -d bookhaven_restore < x.dump` via `exec -T postgres`
   - check `schema_migrations` and row counts
   - `ALTER DATABASE bookhaven RENAME TO bookhaven_old; ALTER DATABASE bookhaven_restore RENAME TO bookhaven;` (connected to `postgres` DB)
   - `up -d backend worker`, smoke-check, drop `bookhaven_old` only after a few days.
   - Note: if the image tag running is older than the dump's migration version, set `IMAGE_TAG` accordingly first.
4. Full VPS loss: `server-setup.sh` → restore `.env` from the password manager → `up -d postgres` → steps 3 (into the empty DB the container created; the migrate step is skipped because the dump carries the schema) → `up -d`. Covers re-mirror automatically.

Quarterly drill: download the latest `monthly/` copy, decrypt, run `restore-drill.sh`, record date + result (one line in the operator's log / issue). It is the only proof the identity key and the off-site copy still work.

## Verification (local, devops agent)

Never touch the dev database destructively: no `down -v`, no `migrate down`, no `DROP`, no restore into the `bookhaven-postgres` container. Use the scratchpad for temp dirs.

1. `shellcheck` both scripts (dockerized).
2. Local backup against the dev stack: `APP_DIR=<repo> COMPOSE_FILE=docker-compose.yml ENV_FILE=<scratch>/backup.env BACKUP_DIR=<scratch>/backups BACKUP_OFFSITE=off bash infrastructure/deploy/backup.sh` → exit 0, one `bookhaven-*.dump`, no temp leftovers, log written. (`pg_dump` is read-only.)
3. Fail-loud: same run with `BACKUP_OFFSITE` unset and empty remote → non-zero exit, no off-site attempt, failure logged.
4. Truncated dump: simulate a failing dump (e.g. point at a non-existent service name or DB) → non-zero, no `bookhaven-*.dump` created, temp file removed.
5. Restore drill: `bash infrastructure/deploy/restore-drill.sh <scratch>/backups/bookhaven-*.dump` → prints migration version (not dirty) and non-zero `books` count; afterwards `docker ps -a` shows no drill container.
6. Off-site path, if `age`/`rclone` are available (natively or in a throwaway `alpine` container with `apk add age rclone`): generate a test identity with `age-keygen` in the scratchpad, set `BACKUP_RCLONE_REMOTE` to a local scratch directory path (rclone accepts local paths), run → `daily/*.dump.age` exists, decrypts with the test identity, and `restore-drill.sh` on the `.age` file succeeds. If this is not feasible on the Windows box, report it as unverified rather than skipping silently.
7. Heartbeat: point `BACKUP_HEARTBEAT_URL` at an unreachable URL → backup still exits 0 with a warning logged.

Operator-only (documented, not verified here): real S3 bucket + rclone remote, real age keypair in the password manager, healthchecks.io check (period 1 day, grace ~2 h), crontab install on the VPS, first real drill.

## Documentation

- `docs/DEPLOYMENT.md`: Backups (what/why, config table, cron line `0 3 * * * /opt/bookhaven/infrastructure/deploy/backup.sh` with no redirect, log location, copying `infrastructure/deploy/` to the server alongside the compose file), Restore, quarterly drill, `.env`/identity off-server copy; pre-launch checklist items: off-site remote configured, heartbeat check created, first drill passed, `.env` + age identity in password manager.
- `.env.production.example`: `BACKUP_*` block.
- `docs/ROADMAP.md`: mark the item if listed.

## Risks

- `pg_restore --list` from stdin inside the container: works for custom format; if it misbehaves, copy the temp file into the container (`docker compose cp`) instead — keep the verification, not the mechanism.
- Windows dev box: scripts must stay LF (check `.gitattributes` covers `*.sh`); run them with Git Bash. Do not `gofmt`/reformat unrelated files.
- Lost age identity = no off-site restores; mitigated only by the password-manager copy and the quarterly drill.
- Dump contains PII and secrets (password hashes, refresh tokens): local copies `chmod 600`, `backups/` `chmod 700`.
- `rclone delete --min-age` on a misconfigured remote path could delete unrelated objects — prune only under the three explicit prefixes, never the remote root.
- Restoring a dump newer than the running image's migrations: covered by the `IMAGE_TAG` note in Restore.
