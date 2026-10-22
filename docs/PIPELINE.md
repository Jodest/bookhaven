# AI development pipeline

Status: active

How feature work moves from request to production in this repo, and what to set up once so the automation actually works. Adapted from a multi-agent Claude Code pipeline pattern (product/backend/devops sub-agents + GitHub Issues/Projects + a gated `dev`→`main` PR flow) to BookHaven's actual shape: one repo, one maintainer, one VPS.

## Components

| File | Role |
|------|------|
| `.claude/commands/f.md` | `/f` — universal entry point. Classifies a request and routes it. |
| `.claude/commands/feature.md` | `/feature` — the full pipeline engine: plan → GitHub issue tracking → implement → check → review → PR into `dev` → PR into `main`. `/f` delegates feature-shaped work here; you can also invoke it directly. |
| `.claude/agents/feature-planner.md` | Business/UX/architecture analysis → an OpenSpec change (`openspec/changes/<slug>/`) for domains with living specs, otherwise a Feature Contract (`docs/features/<feature>.md`). |
| `openspec/` | OpenSpec: living specs (`specs/<domain>/<capability>/spec.md`), in-flight changes (`changes/<slug>/`), archive (`changes/archive/`), project context/rules (`config.yaml`). See "OpenSpec" below. |
| `.claude/agents/developer.md` | Implementation (backend/frontend/migrations/tests). |
| `.claude/agents/review-qa.md` | Code review, acceptance/security/QA pass on the diff. |
| `.claude/agents/devops.md` | Infra-as-code only (Dockerfiles, compose, Caddy, CI/CD workflows). Never touches the real server or a real cloud API. |
| `.claude/commands/backlog-groom.md` | `/backlog-groom` — headless-safe: researches new backlog candidates, files each as a `proposal`-labeled GitHub issue, notifies you. Never plans or implements. |
| `.claude/agents/backlog-researcher.md` | Surveys tech debt, prior business analyses, and competitors for backlog candidates; opens the `proposal` issues. |
| `.claude/commands/backlog-next.md` | `/backlog-next` — interactive only: ranks open `proposal` issues, asks you to pick one, hands the pick to `/feature.md`. |
| `.claude/agents/backlog-prioritizer.md` | Ranks open `proposal` issues with a one-line rationale each. Read-only — never decides for you. |

## Branches

- `main` — production. Deploy (`.github/workflows/deploy.yml`) is currently **manual only** (`workflow_dispatch`) until a production server exists; push-to-deploy is disabled to save Actions minutes (#523) — re-enable steps in `docs/DEPLOYMENT.md` → "Ongoing deploys".
- `dev` — review gate only. **There is no separate dev deployment** — this project runs on a single minimal-budget VPS by design (`docs/DEPLOYMENT.md`), and a second environment isn't worth the cost for a solo-maintained app. `dev`'s only job is: `ci.yml` runs on PRs into it (path-filtered; the PR into `main` skips CI when the same head SHA already passed here — see `docs/DEPLOYMENT.md` → "CI minutes budget"), and it's a place to look at a diff before it reaches `main`.
- `feat/<slug>` / `fix/<slug>` / `hotfix/<slug>` — short-lived work branches.

Flow: `feat/<slug>` → PR into `dev` (CI + your own look) → merge → PR from the same branch into `main` (`Closes #N`) → merge (no automatic deploy until a prod server exists; then a manual Deploy run, later deploy-on-push again).

No agent ever merges a PR or pushes directly to `main`/`dev`. You review and merge on GitHub; then tell `/f` to continue (e.g. *"PR #12 in dev merged, continue"*).

## GitHub issue tracking

Every normal-workflow feature gets:
- one epic issue (`[epic] <feature>`), body links to the plan: the OpenSpec change `openspec/changes/<slug>/` or the Feature Contract `docs/features/<feature>.md`
- one sub-issue per touched area, tagged `[backend]` / `[frontend]` / `[devops]`, referencing the epic

Small/hotfix changes get a single `[fix]` issue instead. The plan (OpenSpec change or Feature Contract) stays the source of truth for *what* to build; issues exist for status/traceability, not as a second spec.

## OpenSpec

Status: standard workflow since 2026-10-13. Every product area has a living spec, and every behavior change goes through an OpenSpec change. Framework: [OpenSpec](https://github.com/Fission-AI/OpenSpec) (MIT).

**Why.** Feature Contracts describe a *change*. After a few iterations nobody can tell how a domain behaves *now* without stacking several contracts plus their "Update (#N)" notes (the import domain had about 12). OpenSpec keeps a second layer: living specs of current behavior, updated by merging each change's deltas in the same PR as the code.

**What is used and what isn't.** Only the artifact format and the CLI. Orchestration stays in `/feature`: GitHub issues, the `dev`→`main` gate, mechanical checks, the independent `review-qa` pass. `openspec init --tools none` was used, so no `/opsx:*` commands or OpenSpec skills are installed. They would compete with `/f` and skip all of the above.

### Layout

- `openspec/specs/<domain>/<capability>/spec.md` — current behavior: `### Requirement` (SHALL/MUST) + `#### Scenario` (WHEN/THEN). Observable behavior only, no code identifiers. **This is where to look to learn how something works today.**
- `openspec/changes/<slug>/` — a change in flight: `proposal.md`, delta `specs/` (`ADDED / MODIFIED / REMOVED / RENAMED Requirements`), `design.md` (API/DB/frontend detail), `tasks.md`.
- `openspec/changes/archive/<date>-<slug>/` — archived changes (history).
- `openspec/config.yaml` — project context and per-artifact rules, fed to the planner.
- `docs/features/*.md` — older Feature Contracts, kept as frozen history and never updated.

### Domains

| Domain | Capabilities |
|---|---|
| `catalog-import` | book lookup, book file import, Wikidata book harvest, Wikidata author import, Wikidata genre import, Wikidata matching rules, the admin «Импорт» section, Wikidata author enrichment |
| `catalog-moderation` | entity status and public visibility, bulk status, moderation queues, duplicate hints, author merge, genre merge |
| `catalog-admin` | dashboard, catalog lists, book editing, author editing, genre editing, admin help |
| `auth` | registration, session (login, tokens, refresh, logout), email verification, password reset, authorization (bearer/cookie, roles, page guards) |
| `users` | own profile, public profile, admin user list, admin role change |
| `reviews` | book ratings, book reviews, review likes, review moderation, review-like notifications |
| `collections` | shelves, shelf books, public collections |
| `home` | home page (payload, cache, layout), book of the day, literary calendar, catalog rows, random book, personal blocks |
| `reading-challenges` | goals, progress |
| `screen-works` | screen adaptations of books (films, series, cartoons, teleplays), the public book-page block, admin hide/show of an adaptation link |

A Feature Contract is used only for work outside these domains (for example infra or tooling). A new product area gets a baseline spec, verified against the code, before or together with its first change.

### How to run a task

Nothing OpenSpec-specific to type: start work through `/f` as always.

| What you want | What to type |
|---|---|
| A feature or behavior change | `/f "<request>"` |
| A small local fix, no planning | `/f "<request>" --small` |
| An urgent fix straight into `main` | `/f "<request>" --hotfix` |
| A dry run without commit or PR | `/f "<request>" --no-commit` |
| An existing GitHub issue | `/f #379` (or "do #379"): the issue is reused, not duplicated |
| Pick from the backlog | `/backlog-next` (ranks the open `proposal` issues and asks) |

After each merge tell `/f` what happened ("PR #N in dev merged", then "PR #N in main merged") and it continues.

### Lifecycle inside `/feature`

1. **Plan (Step 4).** The planner finds the affected domain and writes `openspec/changes/<slug>/`; the orchestrator runs `validate <slug> --strict`. For `--small` there is no planner: if the fix alters specified behavior, the orchestrator writes a minimal change folder itself (proposal, delta, tasks) and validates it.
2. **Issues (Step 5).** An epic plus area sub-issues, or one `[fix]` issue; an existing issue named in the request is reused.
3. **Implementation (Step 6).** The developer implements and ticks `tasks.md`; it never archives.
4. **Checks (Step 7).** Build, vet, tests, type-check, lint, `validate <slug> --strict`, `git diff --check`.
5. **Review (Step 8).** `review-qa` traces every delta scenario to code or tests and checks that `tasks.md` is complete.
6. **Archive (Step 9).** `archive <slug> -y` merges the deltas into `openspec/specs/` on the feature branch, then `validate --specs --strict`. Code and spec update reach `dev` and `main` in the same PR.

### Rules

- Never edit `openspec/specs/` by hand. Living specs change only through a change and `archive`, including fixes to a spec's own mistakes (for example `auth-roles-contributor`).
- A MODIFIED requirement keeps every existing scenario **under its exact name**; the CLI treats a renamed scenario as dropped. Rewrite WHEN/THEN under the old name and add new scenarios alongside.
- Baseline specs describe current behavior as-is, bugs included; the bug is filed as an issue and fixed through a normal change (for example #366, #378).
- No `/opsx:*` commands, no global install, no devDependency.
- **Admin guide.** A feature that changes admin UX (labels, pages, flows, roles) updates the matching chapter in `docs/admin-guide/` in the same PR: the planner lists the chapter in `tasks.md`, review-qa checks it. The same files are shown in the admin app at `/help`. A new admin section or tab also gets a `helpChapter` in `apps/admin/src/shared/config/navigation.ts` (and a chapter in the registry `shared/config/help.ts` if it needs a new one).

### CLI

Always pinned and with telemetry off. No global install and no devDependency: a root devDependency changed the frontend lockfile resolution and would have entered the Docker images.

```bash
OPENSPEC_TELEMETRY=0 npx -y @fission-ai/openspec@1.13.2 validate --specs --strict   # all living specs
OPENSPEC_TELEMETRY=0 npx -y @fission-ai/openspec@1.13.2 list --specs                # capability inventory
OPENSPEC_TELEMETRY=0 npx -y @fission-ai/openspec@1.13.2 show <capability> --type spec
OPENSPEC_TELEMETRY=0 npx -y @fission-ai/openspec@1.13.2 validate <slug> --strict    # one change
```

Bump the pinned version deliberately (OpenSpec releases weekly) and re-run `validate --specs --strict` after a bump.

### History

- 2026-10-09: pilot started on `catalog-import` (#337); two features went through cleanly (#340, #345).
- 2026-10-12: decision to extend domain by domain, each with a baseline spec verified against the code: `catalog-moderation` (#349), `auth` (#357), `reviews` (#360), `collections` (#363).
- 2026-10-13: `users` (#369), `home` and `reading-challenges` (#372), `catalog-admin` and Wikidata author enrichment (#375). First fixes through changes: #366, #378. Pilot ended and OpenSpec documented as the standard workflow (#392).

## Prerequisites (one-time setup)

### 1. GitHub CLI

The whole pipeline shells out to `gh`. Install and authenticate it before using `/f` or `/feature` for anything beyond a dry run (`--no-commit`):

```powershell
winget install --id GitHub.cli
gh auth login
```

Verify with `gh auth status`. `/feature` checks this itself at Step 0 and stops with `STATUS: NEEDS_HUMAN_ACTION` if it's missing.

### 2. Push the `dev` branch

A local `dev` branch (mirroring `main` at the time of setup) already exists in this repo. It still needs to be pushed once:

```bash
git push -u origin dev
```

Do this yourself (or ask the assistant to, once you're ready) — deliberately not done as part of this setup, since pushing a new branch is a shared-repo change.

### 3. (Optional) Branch protection on `main` and `dev`

Nothing technical stops a manual `git push origin main`. If you want GitHub itself to enforce "no direct pushes, PR + passing CI required," set branch protection rules in **Settings → Branches** for both `main` and `dev` (require the `CI` check, require a PR before merging). This is a repo setting, not something `gh` scripts in this pipeline touch — configure it yourself if you want the hard guarantee, not just the convention.

### 4. (Optional) GitHub Project board

Only needed if you want the Backlog/In Progress/Blocked/PR Review/Done status board, not just issues. Skip this section entirely and the pipeline will still create issues/PRs — it just won't set a board status (it checks the constants below and no-ops if they're still placeholders).

```bash
# Create a project (or use an existing one) and note its number
gh project create --owner <your-github-username> --title "BookHaven"
gh project list --owner <your-github-username>

# Get the project ID and the Status field's ID + option IDs
gh project field-list <project-number> --owner <your-github-username> --format json
gh project field-list <project-number> --owner <your-github-username> --format json \
  | jq '.fields[] | select(.name=="Status")'
```

Once you have real values, fill in the **Project board constants** section below (this file is what `/feature` reads to decide whether board wiring is configured).

## Project board constants

Replace every `<...>` below with the real value once you've run the commands above. Leave them as-is to skip board wiring.

```
owner:              <your-github-username>
project-number:     <project-number>
project-id:         <project-id>
status-field-id:    <status-field-id>
status-backlog-id:      <status-backlog-id>
status-in-progress-id:  <status-in-progress-id>
status-blocked-id:      <status-blocked-id>
status-pr-review-id:    <status-pr-review-id>
status-done-id:         <status-done-id>
```

## Flags on `/feature`

- `--small` — skip planning and the epic/sub-issue split; one `[fix]` issue instead. If the fix alters specified behavior, the orchestrator still writes a minimal OpenSpec change (see "OpenSpec" → Lifecycle).
- `--hotfix` — skip the `dev` gate; PR goes straight into `main` (still requires review, still never auto-merged).
- `--no-commit` — dry run: implement, check, review, then stop. No issue, no branch push, no PR.

## Resuming a gated pipeline

`/feature` (and `/f`, which delegates to it) stops after opening each PR and waits. To move it forward, just say what happened:

- *"PR #12 in dev merged, continue"* → opens the PR from the same branch into `main`.
- *"main PR #14 approved and merged"* → closes sub-issues/epic, marks the board Done, confirms the real deploy will run automatically.

## Backlog loop (who decides what to work on)

Two extra commands sit in front of `/feature`, splitting "what could we do" from "what do we do next" so a human always makes the actual prioritization call:

1. **`/backlog-groom`** (safe to run unattended, e.g. from an external scheduler) — `backlog-researcher` surveys tech debt / open business-analysis questions / competitor gaps, files up to 5 new candidates as `proposal`-labeled GitHub issues, logs them in `docs/ROADMAP.md`'s `## Proposed (awaiting prioritization)` section, and sends a push notification. It never plans or implements anything.
2. **`/backlog-next`** (interactive — never run headless, it asks a question mid-run) — `backlog-prioritizer` ranks the open `proposal` issues, you're asked which one(s) to do next (multi-select allowed). Pick one and it's handed straight to `feature.md`. Pick several and state an order, and only the first is handed to `feature.md`; the rest are queued on the issues themselves (label `queued` plus a `Queue position: N` comment — no file edit, no doc PR, so the queue is the same whichever branch you look at) — items are always worked **one at a time, strictly sequentially**, never as parallel branches. Running `/backlog-next` again picks up the next queued item directly, no re-ranking. The original `proposal` issue is closed and linked to the new epic once planning starts.

**Real weekly automation for step 1 needs an external scheduler**, not this session's `CronCreate` tool — `CronCreate` jobs are session-only, die when this CLI session ends, and auto-expire after 7 days regardless, so they can't stand in for "runs every week forever." Use the OS's own scheduler (e.g. Windows Task Scheduler) to run `claude -p "/backlog-groom"` headlessly on a weekly cron, and make sure the `gh`/`git` commands it needs are pre-approved (see your permission settings) so the unattended run doesn't hang waiting for a click nobody will make.

## What's deliberately not automated

- No agent ever pushes to `main`/`dev` directly, merges a PR, or triggers/verifies a real production deploy — `.claude/agents/devops.md` is explicitly barred from touching real infrastructure, and `/feature` only ever opens PRs.
- No second (dev) deployment environment — see `docs/DEPLOYMENT.md`'s single-VPS, minimal-budget rationale. If real usage later justifies a staging environment, that's a `devops`-agent task in its own right, not a side effect of this pipeline doc.
