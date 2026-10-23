# Dependency Vulnerability Scanning

Status: implemented

Issue: #254. Area: devops / CI. Not covered by any `openspec/specs/` domain, so this is a Feature Contract.

## Goal

Known-vulnerable Go and npm dependencies fail CI, and new advisories against unchanged code are caught by a weekly run. Every exception goes into a documented ignore list with an expiry date, so CI stays green but never silent. The cheap remediations land here, so the scanners are green on day one. The Next 15 / React 19 migration is split into a follow-up.

Phasing (agreed with the orchestrator):

- **(a) This feature.** Scanners, the gate, Dependabot, the weekly run, and the cheap fixes: the Go toolchain bump, Go module bumps, Next to the latest 14.2.x, and npm patch fixes.
- **(b) Follow-up issue.** The Next 15.5+/React 19 migration of both apps and `packages/ui`. Create it while doing this feature.
- **(c) Ignore list.** Allowlist entries for everything still open after (a), each with an expiry date and a link to the follow-up issue.

**Recommended split: two PRs, in this order.**

- **PR 1 = group 1 (Go remediation).** Backend only. The Go 1.23 → 1.27 jump changes `go vet` checks and GODEBUG defaults. pgx 5.5 → 5.9 changes the database driver under GORM. Both deserve their own revert unit.
- **PR 2 = groups 2–6 (npm fixes, gate, CI, Dependabot, weekly run, docs).**

The Next 14.2.6 → 14.2.x patch bump has lower risk and stays in PR 2. The CI `frontend` build is its gate.

## Baseline findings (local run on main, 2026-10-23)

**govulncheck v1.1.4 with go1.27.1.** 18 reachable vulnerabilities, plus 14 in imported but uncalled packages and 21 in required modules.

| Reachable finding | Current | Fixed in | Action in this feature |
|---|---|---|---|
| stdlib: crypto/x509, crypto/tls, net/http, net/http/internal/http2, net/textproto, html/template | go 1.23 (no longer supported) | go1.27.2 / go1.27.3 | Toolchain bump to go1.27.3 |
| golang.org/x/net | v0.41.0 | v0.60.0 | bump |
| golang.org/x/text | v0.27.0 | v0.41.0 | bump |
| github.com/jackc/pgx/v5 (GO-2026-5004) | v5.5.5 | v5.9.2 | bump |
| github.com/golang-jwt/jwt/v5 (GO-2025-3553) | v5.2.1 | v5.2.2 | bump |
| github.com/golang-jwt/jwt v3.2.2+incompatible (GO-2025-3553), indirect | v3.2.2 | no fix | Run `go mod why -m github.com/golang-jwt/jwt`; most likely Echo v4's deprecated JWT middleware. If govulncheck still reports it as reachable after the bumps, add an allowlist entry with a justification, or remove the import path. |
| github.com/redis/go-redis/v9 | v9.7.0 | v9.7.3 | bump |

**`pnpm audit --prod`.** 4 critical, 18 high, 26 moderate, 5 low. Almost all are `next` 14.2.6.

| Finding | Fixed in | Action |
|---|---|---|
| next: middleware auth bypass (critical) | 14.2.25 | Bump `next` and `eslint-config-next` to the latest 14.2.x (≥ 14.2.35) in both apps |
| next: Image Optimization RCE via AVIF/SVG (critical) | ≥ 15.5.24 / 15.5.28 only | Allowlist, linked to follow-up (b). ~~Mitigation: neither `next.config` sets `images`~~ — wrong for apps/web (see Implementation notes): recorded as exposed, accepted only until the first production deploy. |
| next: RCE on Windows-hosted servers (critical) | ≥ 15.5.24 only | Allowlist, linked to (b). Production runs on Linux containers. |
| next: high DoS / SSRF / cache-poisoning advisories still open after 14.2.x | 15.5.x only | Allowlist each, linked to (b) |
| next-intl < 4.9.2 (open redirect, prototype pollution) | 4.9.2 (major) | **Remove the dependency.** `next-intl` is in `apps/web/package.json` but no file under `apps/web` imports it. |
| postcss < 8.5.23, source-map-js < 1.2.2 (transitive through `next`, which pins postcss) | patch / minor within the same major | Root `package.json` `pnpm.overrides` forcing the fixed versions. If the CI build breaks, drop the override and add allowlist entries instead. |
| vite ≤ 6.4.3, esbuild | dev-only | Excluded by `--prod`. Not gated. |

## Acceptance Criteria

- [x] Go runs on go1.27.3 everywhere:
  - `backend/go.mod` has `go 1.27.0` and `toolchain go1.27.3`.
  - CI `setup-go` uses `go-version: "1.27.3"` as an exact patch, because the runner toolcache would otherwise satisfy `"1.27"` with an older, vulnerable patch.
  - `backend/Dockerfile` uses `golang:1.27.3-alpine`.
  - `go build`, `go vet`, `go test ./...` pass.
- [x] The Go modules in the table are bumped to at least their fixed versions, and `go mod tidy` is clean. No other upgrades for their own sake: the existing pins on sentry-go, asynq and anthropic-sdk-go stay unless the tidy or the bumps force them.
- [x] `govulncheck ./...` (pinned version) reports no symbol-level (reachable) finding that isn't allowlisted.
- [x] `next` and `eslint-config-next` are the same latest 14.2.x in both apps. `next-intl` is removed. The postcss/source-map-js overrides are present, or allowlisted with a reason. `pnpm install --frozen-lockfile` passes on the updated lockfile.
- [x] `pnpm audit --prod` reports no high/critical advisory that isn't allowlisted.
- [x] `.github/vuln-allowlist.json` exists. Every entry has `id`, `package`, `reason`, `issue` and `expires`. The expiry is at most 90 days out (enforced by the gate). All remaining `next` entries link to the follow-up issue (b).
- [x] The CI `backend` job fails on an un-allowlisted reachable Go vulnerability. The CI `frontend` job fails on an un-allowlisted high/critical production npm advisory. Both fail on an expired allowlist entry or on a scanner that produced no report.
- [x] Both jobs write a findings summary to the job summary, including the non-blocking ones: unreachable Go findings, moderate/low npm advisories, and stale allowlist entries.
- [x] A weekly scheduled workflow, which can also be started manually, runs both scans on `main` without build or test.
- [x] `.github/dependabot.yml` covers `gomod`, `npm` and `github-actions`:
  - monthly schedule, target branch `dev`;
  - grouped into one PR per ecosystem;
  - semver-major ignored for gomod and npm.
- [x] The docs are updated (see Documentation). The ROADMAP entry for #254 is done. A follow-up ROADMAP entry and GitHub issue exist for the Next 15/React 19 migration.

## Business Rules (scan policy)

- **Go blocking set.** Only govulncheck findings at symbol level are blocking, meaning the vulnerable function is reachable from our code (the first trace frame has a `function`). Findings in imported or required modules that are never called are shown in the job summary as information only. Severity is not considered: govulncheck reports reachability, not CVSS.
- **npm blocking set.** `pnpm audit --prod` advisories of severity `high` or `critical` are blocking. `moderate` and `low` are summary only. Dev dependencies are not scanned for gating, because they never ship.
- **Allowlist (escape hatch).** One file for both ecosystems. A finding matches an entry by advisory ID: a `GO-YYYY-NNNN` ID for Go, a `GHSA-…` ID for npm (pnpm's `github_advisory_id`), plus the package name.
  - **Expired entry.** Blocking. This forces a re-triage instead of a silent permanent ignore.
  - **Expiry more than 90 days in the future.** Blocking: the lint fails.
  - **Entry that matches nothing.** A non-blocking notice ("stale, remove it").
- **No scanner output = failure.** This covers an unparsable JSON report, a missing govulncheck/pnpm audit report, or a registry error. The gate exits non-zero with a clear message and never passes on missing data. A re-run is the escape hatch for flaky registry errors.
- **Where the scans run.** Scans run on PRs only when the matching CI job runs, which reuses the existing precheck path filters. They run unconditionally on manual Deploy (`workflow_call`) and weekly. A new advisory can therefore turn a PR red for code it didn't touch. That is intended. The documented escape hatch is a bump, or an allowlist entry with a reason, an issue and an expiry.
- **Go patch updates are manual**, prompted by the weekly scan. A version change touches `go.mod` (`toolchain`), `ci.yml` and the Dockerfile together.
- **Dependabot docker updates are not configured.** `node:22-alpine` and `distroless` already float to patched images on rebuild. Go image bumps must stay in lockstep with `go.mod` and CI.
- **Dependabot security update PRs are not enabled.** GitHub opens them against the default branch (`main`), bypassing `dev`. Dependabot *alerts* should be enabled in repository settings (a manual step, documented). Fixes then flow through feature → dev → main.
- **No secrets or registries.** Nothing pushes images or touches servers. The workflows need only `contents: read`.

## CI

Existing style: actions pinned by major tag (`@v4`/`@v5`), tools installed with `go install …@vX.Y.Z` as in the Dockerfile. Keep that style.

`ci.yml`:

- **`backend` job, after `go test`.**
  - `go install golang.org/x/vuln/cmd/govulncheck@v1.1.4`. Before pinning, check with `go list -m -versions golang.org/x/vuln` whether a newer release exists, and pin the latest.
  - `govulncheck -format json ./... > govulncheck.json`. JSON mode exits 0 even when it finds something.
  - `node ../.github/scripts/vuln-gate.mjs go govulncheck.json`. The runner's preinstalled Node is enough; the script has no dependencies.
- **`frontend` job, after install.**
  - `pnpm audit --prod --json > pnpm-audit.json || true`. pnpm exits non-zero when it finds vulnerabilities; the gate decides.
  - `node .github/scripts/vuln-gate.mjs npm pnpm-audit.json`.
  - `node --test .github/scripts/`, the gate's unit tests.
- **`precheck`.** Extend the `.github/workflows/*` case to also match `.github/scripts/*` and `.github/vuln-allowlist.json`, so both jobs run.
- No new jobs on PRs: on the Free plan, minutes are billed per job (see DEPLOYMENT.md "CI minutes budget"). The scans add roughly 30–60 s to existing jobs.

New `.github/workflows/security-scan.yml`:

- Triggers: `schedule: cron "0 5 * * 1"` (weekly, Monday 05:00 UTC) and `workflow_dispatch`. `permissions: contents: read`.
- Two jobs, `go-scan` and `npm-scan`. Each does checkout, setup (same versions as ci.yml), the scan and the gate. `npm-scan` needs only `pnpm install --frozen-lockfile --ignore-scripts` before `pnpm audit`. In fact `pnpm audit` reads the lockfile only, so try it without install first and keep the install only if needed.
- When the run fails, GitHub notifies by email. No issue automation (keep scope tight).

`.github/scripts/vuln-gate.mjs` (Node ESM, no npm dependencies, runs on Windows too):

- Arguments: `go|npm <report.json>`. Reads `.github/vuln-allowlist.json`, relative to the script.
- `go` mode: parses the govulncheck JSON stream (a sequence of JSON objects). Collects `finding` messages and classifies them as symbol-level or module/package-level. Deduplicates by OSV ID and module. Takes fixed versions from `finding.fixed_version`.
- `npm` mode: parses the `pnpm audit --json` object (`advisories` keyed by ID, each with `github_advisory_id`, `module_name`, `severity`, `patched_versions`). Requires `metadata` to be present.
- Applies the Business Rules above, prints a readable list, and appends a Markdown table to `$GITHUB_STEP_SUMMARY` when that is set. Exits 1 on any blocking condition.
- Uses an injectable "today" for tests.

`.github/vuln-allowlist.json` shape:

```json
{
  "entries": [
    {
      "id": "GHSA-xxxx-xxxx-xxxx",
      "package": "next",
      "reason": "Fixed only in 15.5.x; images optimizer unused for remote sources; tracked in follow-up",
      "issue": "#<follow-up>",
      "expires": "2026-12-31"
    }
  ]
}
```

`.github/dependabot.yml`:

- **`gomod`**: `/backend`. Monthly, `target-branch: dev`, `open-pull-requests-limit: 2`. One group with `update-types: [minor, patch]`. Ignore `version-update:semver-major` for `*`. `commit-message.prefix: "chore(deps)"`.
- **`npm`**: `/`, the pnpm workspace root with its lockfile. Same settings as gomod. Majors are ignored, so `next` 15 and React 19 never arrive here; they come through follow-up (b).
- **`github-actions`**: `/`. Monthly, `target-branch: dev`, one group including majors (e.g. `actions/checkout` v4 → v5). `commit-message.prefix: "ci(deps)"`.
- No labels, because a missing label makes Dependabot error.

## Existing Patterns

- `.github/workflows/ci.yml`: the precheck path filter, `permissions: contents: read`, and the `workflow_call` reuse from `deploy.yml`. The scans therefore also gate manual deploys, which is intended.
- `backend/Dockerfile`: pinned `go install …@vX.Y.Z` tools.
- `docs/features/test-coverage-and-ci.md`: the earlier CI-only contract.

## Tests

- **Unit tests for the gate**, `.github/scripts/vuln-gate.test.mjs` with `node:test` and small JSON fixtures under `.github/scripts/fixtures/`. Cover:
  - a reachable Go finding fails;
  - a module-only Go finding passes but is listed;
  - an allowlisted finding passes;
  - an expired entry fails;
  - expiry more than 90 days out fails;
  - a stale entry gives a notice only;
  - npm high fails, moderate passes;
  - an empty or garbage report fails;
  - a pnpm report without `metadata` fails.
- **Local verification** (the developer runs these and records the before/after counts in the PR description):
  - `cd backend && govulncheck ./...` before and after group 1, then the gate script on the JSON output;
  - `pnpm audit --prod`, then the gate;
  - `go build ./... && go vet ./... && go test ./...`;
  - `pnpm run lint && pnpm run type-check && pnpm run test`.
  - **Do not run native `next build`.** The CI `frontend` job is the build gate for the Next bump.
- Rebuild the local compose backend image (dev stage, `air`/`swag` installs under Go 1.27) and smoke-test the API.
- Prove the gate fails at least once in CI, e.g. with a temporary expired allowlist entry on the branch. Revert it before merge.
- Run `security-scan.yml` with `workflow_dispatch` once after merge to `dev`. A scheduled run only fires from the default branch, so mention this in the PR.

## Documentation

- **`docs/DEPLOYMENT.md`**, new section "Dependency scanning" next to "CI minutes budget":
  - what runs where (PR jobs, Deploy, weekly);
  - the blocking policy;
  - **triaging a red scan**: read the job summary, then bump (`go get module@fixed && go mod tidy` / `pnpm up`), then re-run;
  - **adding an allowlist entry**: required fields, 90-day maximum, link an issue, and justify reachability or mitigation;
  - **Go patch bump procedure**: `go.mod` toolchain, `ci.yml` and Dockerfile together;
  - Dependabot policy, and the manual step to enable Dependabot alerts (security update PRs stay off, with the reason why).
- Add a line to the Security checklist.
- **`docs/PIPELINE.md`**: a one-line pointer to that section ("a red `govulncheck`/`pnpm audit` gate → see DEPLOYMENT.md#dependency-scanning"), and a note that Dependabot PRs target `dev`.
- **`docs/PROJECT.md` and `.opencode/AGENTS.md`**: "Go 1.23" → "Go 1.27". Do not edit frozen feature contracts or archived changes that mention the 1.23 pins.
- **`docs/ROADMAP.md`**: mark the #254 line done with a short note. Add an entry for the Next 15/React 19 migration (follow-up (b), new issue). Its body should list:
  - the open allowlisted advisories;
  - the App Router on Next 15 requiring React 19;
  - async request APIs (`cookies()`, `headers()`, `params`);
  - `packages/ui` peer React 18;
  - whether to target 15.5.x or the current 16.x.
- The allowlist expiry should be set so the follow-up lands before it.

## Implementation notes (PR 2)

- govulncheck is pinned to `v1.9.0`, the newest release at implementation time (`go list -m -versions golang.org/x/vuln`).
- The gate's tests run as `node --test .github/scripts/*.test.mjs`: a bare directory argument is not accepted by every Node version.
- Verified for the Image Optimization advisories: `apps/admin` sets no `images` config, but `apps/web` does (`images.remotePatterns` for covers.openlibrary.org and the Wikimedia hosts) and uses `next/image`, so the "no images config" mitigation does not hold for web. No library-level mitigation is claimed: the two critical Image Optimization RCE entries are recorded as **exposed** in apps/web and accepted temporarily only because there is no production deployment yet. #594 (or setting `images.unoptimized`) must land before the first production deploy.
- After the npm fixes two `vite` high advisories (dev-server only, via vitest through `@sentry/cloudflare`'s optional peer) remain in `pnpm audit --prod`; they are allowlisted against the epic #590.
- The postcss / source-map-js overrides (`^8.5.23`, `^1.2.2`) work: tsc, lint and vitest pass; the CI `next build` remains the final gate.

## Risks

- **Go 1.23 → 1.27 behavior changes.** With the `go` directive at 1.24 or higher, `go vet` flags non-constant printf format strings, which can redden CI. Fix those calls with `"%s"`. Do not disable vet. GODEBUG defaults change with the directive (crypto/x509, TLS defaults such as ML-KEM key exchange, RSA keys under 1024 bits rejected). This should be harmless for us, but it is the reason PR 1 is separate.
- **pgx 5.5 → 5.9 under `gorm.io/driver/postgres`.** Backend unit tests mostly use fakes, so the compose smoke test (migrations, a few API reads and writes) is the real check.
- **x/net and x/text bumps** may raise the `go` directive further via `go mod tidy`. Accept it if it stays ≤ 1.27.
- **pnpm 9.0.0 `audit`** uses the npm registry's audit endpoint. If the request fails with pnpm 9.0.0 in CI, bump `packageManager` and the `pnpm/action-setup` version to the latest 9.x (lockfile v9 unchanged). Do not move to pnpm 10 in this feature.
- **The postcss override against `next`'s exact pin** may break the CSS pipeline. The CI build catches this; the fallback is allowlist entries.
- **The Next 14.2.x patch line includes the middleware hardening** (the internal subrequest header is ignored). `apps/web/src/middleware.ts` and admin middleware are session-only and should be unaffected. Verify login, protected redirects and the token-refresh flow on the compose stack.
- **Advisories open until follow-up (b) lands**, including two critical Image Optimization RCEs that ARE reachable through `next/image` in apps/web; they must be closed (#594 or `images.unoptimized`) before the first production deploy (DEPLOYMENT.md Security checklist). The expiry dates force a revisit. There is no production deployment yet, which lowers the exposure.
- **A new advisory can turn unrelated PRs red.** This is accepted and documented: bump or add an allowlist entry. The weekly run usually surfaces it first.
- **Dependabot PRs cost CI minutes.** The monthly schedule with one grouped PR per ecosystem is about three PRs a month.
