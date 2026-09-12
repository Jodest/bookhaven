# AI development pipeline

Status: active

How feature work moves from request to production in this repo, and what to set up once so the automation actually works. Adapted from a multi-agent Claude Code pipeline pattern (product/backend/devops sub-agents + GitHub Issues/Projects + a gated `dev`→`main` PR flow) to BookHaven's actual shape: one repo, one maintainer, one VPS.

## Components

| File | Role |
|------|------|
| `.claude/commands/f.md` | `/f` — universal entry point. Classifies a request and routes it. |
| `.claude/commands/feature.md` | `/feature` — the full pipeline engine: plan → GitHub issue tracking → implement → check → review → PR into `dev` → PR into `main`. `/f` delegates feature-shaped work here; you can also invoke it directly. |
| `.claude/agents/feature-planner.md` | Business/UX/architecture analysis → Feature Contract (`docs/features/<feature>.md`). |
| `.claude/agents/developer.md` | Implementation (backend/frontend/migrations/tests). |
| `.claude/agents/review-qa.md` | Code review, acceptance/security/QA pass on the diff. |
| `.claude/agents/devops.md` | Infra-as-code only (Dockerfiles, compose, Caddy, CI/CD workflows). Never touches the real server or a real cloud API. |

## Branches

- `main` — production. Push triggers the real deploy (`.github/workflows/deploy.yml`).
- `dev` — review gate only. **There is no separate dev deployment** — this project runs on a single minimal-budget VPS by design (`docs/DEPLOYMENT.md`), and a second environment isn't worth the cost for a solo-maintained app. `dev`'s only job is: `ci.yml` runs on PRs into it, and it's a place to look at a diff before it reaches `main`.
- `feat/<slug>` / `fix/<slug>` / `hotfix/<slug>` — short-lived work branches.

Flow: `feat/<slug>` → PR into `dev` (CI + your own look) → merge → PR from the same branch into `main` (`Closes #N`) → merge → prod deploys automatically.

No agent ever merges a PR or pushes directly to `main`/`dev`. You review and merge on GitHub; then tell `/f` to continue (e.g. *"PR #12 in dev merged, continue"*).

## GitHub issue tracking

Every normal-workflow feature gets:
- one epic issue (`[epic] <feature>`), body links to the Feature Contract at `docs/features/<feature>.md`
- one sub-issue per touched area, tagged `[backend]` / `[frontend]` / `[devops]`, referencing the epic

Small/hotfix changes get a single `[fix]` issue instead. The Feature Contract stays the source of truth for *what* to build; issues exist for status/traceability, not as a second spec.

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

- `--small` — skip planning and the epic/sub-issue split; one `[fix]` issue instead.
- `--hotfix` — skip the `dev` gate; PR goes straight into `main` (still requires review, still never auto-merged).
- `--no-commit` — dry run: implement, check, review, then stop. No issue, no branch push, no PR.

## Resuming a gated pipeline

`/feature` (and `/f`, which delegates to it) stops after opening each PR and waits. To move it forward, just say what happened:

- *"PR #12 in dev merged, continue"* → opens the PR from the same branch into `main`.
- *"main PR #14 approved and merged"* → closes sub-issues/epic, marks the board Done, confirms the real deploy will run automatically.

## What's deliberately not automated

- No agent ever pushes to `main`/`dev` directly, merges a PR, or triggers/verifies a real production deploy — `.claude/agents/devops.md` is explicitly barred from touching real infrastructure, and `/feature` only ever opens PRs.
- No second (dev) deployment environment — see `docs/DEPLOYMENT.md`'s single-VPS, minimal-budget rationale. If real usage later justifies a staging environment, that's a `devops`-agent task in its own right, not a side effect of this pipeline doc.
