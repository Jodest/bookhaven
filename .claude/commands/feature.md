---
description: Implement a BookHaven feature end-to-end via planner -> issue/PR tracking -> developer -> checks -> reviewer -> dev PR -> main PR
argument-hint: "<feature request>" [--small] [--hotfix] [--no-commit]
---

Implement the following BookHaven feature request from planning through a reviewed, merged pull request, orchestrating the project's `feature-planner`, `developer`, and `review-qa` subagents (via the Agent tool). Follow every step below in order; do not skip steps, do not invoke the reviewer before mechanical checks pass, and **never merge a pull request yourself** — every PR is opened, reported, and left for the human to merge on GitHub.

Request: $ARGUMENTS

Parse `$ARGUMENTS` for the quoted feature request text and the optional flags `--small`, `--hotfix`, and `--no-commit`.

This command is also the engine behind `/f`'s feature-routing and resume handling — see `.claude/commands/f.md`. If you were invoked with a resume-style request instead of a fresh feature request (e.g. "PR #12 in dev merged, continue" or "main PR #14 approved and merged"), skip to **Resuming after the dev PR is merged** or **Resuming after the main PR is merged** below instead of Step 1.

## OpenSpec

Every product area is specified as living OpenSpec specs under `openspec/specs/` (see `docs/PIPELINE.md` → "OpenSpec"). For a change in one of those domains the planner returns `FORMAT: openspec` and a change folder `openspec/changes/<slug>/` instead of a Feature Contract. Run the CLI only through this pinned, telemetry-off invocation (referred to as `$OPENSPEC` below):

```bash
OPENSPEC_TELEMETRY=0 npx -y @fission-ai/openspec@1.13.2
```

Never use `/opsx:*` commands (none are installed) and never edit `openspec/specs/` by hand — main specs change only through `archive` in Step 9. That includes fixing a mistake in a living spec: write a small change folder whose delta corrects it, validate, archive.

## Step 0 — Prerequisites check

Run `gh auth status`. If `gh` is missing or not authenticated, STOP immediately and report:

```
STATUS: NEEDS_HUMAN_ACTION
ACTION_NEEDED:
Install and authenticate the GitHub CLI (see docs/PIPELINE.md) before this pipeline can create issues/PRs.
```

Do not attempt any workaround (no direct API calls with a hardcoded token, no skipping tracking silently).

## Step 1 — Git safety

Run `git status`. If there are pre-existing uncommitted changes unrelated to this feature that could get mixed into the feature commit, STOP and report them instead of proceeding — never mix unrelated pre-existing changes into the feature commit.

Run `git fetch origin` so `main` and `dev` are current before branching.

## Step 2 — Classify

Decide the workflow:

- **small** — `--small` was passed, or the change is obviously local and low risk (isolated UI tweak, copy/text change, small styling fix, trivial bug fix).
- **normal** — anything touching the API, database, auth, user-owned resources, a new backend module, or a significant frontend feature.
- **hotfix** — `--hotfix` was passed. Orthogonal to small/normal: it changes the *branch target* (skip `dev`, PR straight into `main`), not whether planning happens. Use only for urgent production fixes. The PR into `main` is still required and still not auto-merged.

## Step 3 — Create the feature branch

```bash
git checkout main && git pull origin main
git checkout -b <prefix>/<slug>
```

`<prefix>` is `feat` for normal, `fix` for small, or `hotfix` when `--hotfix` was passed (regardless of small/normal). `<slug>` is a short kebab-case description.

## Step 4 — Planning (normal workflow only)

Skip the planner for `--small`. If a `--small` change alters behavior described in `openspec/specs/`, write a minimal change folder yourself instead — `openspec/changes/<slug>/` with `proposal.md` (why, what changes, issue link), the delta spec (usually one MODIFIED requirement that keeps every existing scenario name) and `tasks.md` — and run `$OPENSPEC validate <slug> --strict`. The rest of the workflow then treats it like a planner-made change (developer ticks `tasks.md`, review traces the scenarios, Step 9 archives it). A `--small` change that alters no specified behavior needs no change folder.

Invoke the `feature-planner` subagent (Agent tool, `subagent_type: "feature-planner"`) with the full feature request as its prompt — it starts with zero context, so give it the complete request text, not a summary.

- If it returns `STATUS: READY`, continue with the `FORMAT`, `PLAN` path and `SUMMARY` it reports.
  - `FORMAT: openspec` — run `$OPENSPEC validate <slug> --strict`. If it reports errors, re-invoke the planner once with the plan path and the exact validator output; if it still fails, STOP and report. Then continue to Step 5.
  - `FORMAT: contract` — continue to Step 5.
- If it returns `STATUS: BLOCKED`, STOP and report the reason.

## Step 5 — GitHub issue tracking

Skip entirely if `--no-commit` was passed (dry run — nothing gets tracked or pushed).

If the request names an existing open issue (e.g. "do #379"), reuse it instead of creating new ones: it is the issue for the small/hotfix workflow, or the epic for the normal workflow (create only the missing sub-issues and link them from it).

**Normal workflow** — create an epic + one sub-issue per touched area, inferred from the planner's `SUMMARY` (`backend`, `frontend`, `devops`):

```bash
EPIC_URL=$(gh issue create --title "[epic] <feature>" \
  --body "$(cat <<'EOF'
## Summary
<2-3 bullets from the planner SUMMARY>

## Plan
<PLAN path: openspec/changes/<slug>/ or docs/features/<feature>.md>
EOF
)")
```

For each touched area, create a sub-issue tagged with the area in brackets, referencing the epic:

```bash
gh issue create --title "[backend] <scoped task>" \
  --body "Parent epic: <EPIC_URL>

<scoped detail from the contract>"
```

Then edit the epic body to list the sub-issues as a checklist (`gh issue edit <epic-number> --body "..."`). Record every issue number — they feed the `Closes #N` PR bodies later.

**Small / hotfix workflow** — create a single issue, no epic:

```bash
gh issue create --title "[fix] <short description>"
```

**Project board (best-effort)** — read the "Project board constants" section of `docs/PIPELINE.md`. If it still shows placeholders (not filled in), skip board wiring entirely and note it in the final report instead of failing. If real values are present:

```bash
ITEM_ID=$(gh project item-add <project-number> --owner <owner> --url "<issue-url>" --format json | jq -r '.id')
gh project item-edit --id "$ITEM_ID" --field-id <status-field-id> --project-id <project-id> --single-select-option-id <status-backlog-id>
```

(substitute the actual values from `docs/PIPELINE.md`; the same substitution applies everywhere else in this file that sets a project item's status)

## Step 6 — Implementation

Invoke the `developer` subagent (Agent tool, `subagent_type: "developer"`). Give it:

- the plan path and format from Step 4 (normal workflow, or a small change with a change folder) — for an OpenSpec change, tell it to tick `tasks.md` checkboxes as it goes and not to run `archive`; or the raw feature request verbatim (small/hotfix workflow, always included even when there is a change folder) — it cannot read your conversation, so include the actual request text, not just a reference to it;
- explicit instruction not to commit.

- If it returns `STATUS: IMPLEMENTED`, continue to Step 7.
- If it returns `STATUS: PLAN_INVALID`, go back to Step 4 (re-plan with the reported reason) — for the small/hotfix workflow, STOP and report instead, since there is no planner to re-invoke.

## Step 7 — Automated checks

Run yourself (do not delegate) only the checks relevant to what changed:

- Backend touched: from `backend/`, run `go build ./...`, `go vet ./...`, `go test ./...`.
- Frontend touched (`apps/web` and/or `apps/admin`): run `tsc --noEmit` and `next lint` for each touched app. On Windows, invoke the app's `node_modules/.bin/next` shim directly (e.g. `cd apps/admin && ./node_modules/.bin/next lint`) rather than `pnpm run`, which can fail to resolve the POSIX shim through cmd.exe.
- OpenSpec change: run `$OPENSPEC validate <slug> --strict` (the developer may have adjusted delta specs).
- Always run `git diff --check` (whitespace/conflict-marker sanity).

If any check fails, go back to Step 6 with the failure output. Do not invoke the reviewer until every relevant mechanical check passes.

## Step 8 — Review

Invoke the `review-qa` subagent (Agent tool, `subagent_type: "review-qa"`). Give it, inline in the prompt (it has zero context):

- the plan path and format (normal workflow) — for an OpenSpec change, tell it to verify every scenario in the change's delta specs and that `tasks.md` is fully ticked — or the original request (small/hotfix workflow);
- the full `git diff` output for the changed files;
- the Step 7 automated check results.

- If it returns `STATUS: PASS`, continue to Step 9.
- If it returns `CATEGORY: CODE_FAIL`, go back to Step 6 with the reported issues.
- If it returns `CATEGORY: PLAN_INVALID`, go back to Step 4 (or STOP for the small/hotfix workflow).

Track review/fix cycles (Steps 6-8). After 3 failed cycles, STOP and report the remaining issues instead of looping further.

## Step 9 — Documentation check

After a PASS, verify yourself (do not invoke another agent):

- contract format: `docs/features/<feature>.md` exists or was appropriately updated (normal workflow);
- OpenSpec format: every `tasks.md` checkbox is ticked (`$OPENSPEC status --change <slug>`), then archive the change so the updated main specs ship in the same commit as the code:
  ```bash
  $OPENSPEC archive <slug> -y
  $OPENSPEC validate --specs --strict
  ```
  `archive` merges the deltas into `openspec/specs/` and moves the change to `openspec/changes/archive/<date>-<slug>/`. If it refuses (for example, a MODIFIED block would drop scenarios), go back to Step 6 with its output — never pass `--no-validate`. Archiving on the feature branch is deliberate: an agent never commits to `main`, so this is the only point where the spec update can travel with its code through the PR gate;
- `docs/ROADMAP.md` reflects the feature's current status.

## Step 10 — Commit

Only after Step 8 returned `STATUS: PASS`.

If `--no-commit` was passed: STOP and report PASS without committing, pushing, or opening anything. Leave the changes uncommitted on the feature branch created in Step 3 and say so.

Otherwise:

1. Inspect `git diff` and `git status`.
2. Stage only the feature-related files (never `git add -A`/`.` blindly — exclude anything unrelated found in Step 1).
3. Create one focused commit using `<type>(<scope>): <description>`.
4. Before pushing, set the issue(s) from Step 5 to **In Progress** on the project board (best-effort, same as Step 5's board wiring).

## Step 11 — Push and open the pull request

```bash
git push -u origin <prefix>/<slug>
```

- **Hotfix** (`--hotfix`): open the PR straight into `main`:
  ```bash
  gh pr create --base main --title "<type>(<scope>): <description>" --body "$(cat <<'EOF'
  ## Summary
  <bullets>

  Closes #<issue-number>
  EOF
  )"
  ```
- **Normal / small** (default): open the PR into `dev`:
  ```bash
  gh pr create --base dev --title "<type>(<scope>): <description>" --body "$(cat <<'EOF'
  ## Summary
  <bullets>

  Related: #<issue-number(s)>
  EOF
  )"
  ```
  Do not write `Closes #N` on the `dev` PR — the issue should only close when it lands in `main`.

Set the issue(s)/project item(s) to **PR Review** (best-effort). Report the PR URL.

**STOP here. Do not merge. Do not proceed to the next PR.** The human reviews and merges on GitHub; you resume only when told to.

End with a handoff message:

> PR #N (`<prefix>/<slug>` → `<dev|main>`) is open: <url>
> **Next after merge:** <what happens next — main PR, or done>
> **To resume:** tell `/f` — "PR #N in <dev|main> merged, continue"

## Resuming after the dev PR is merged

Triggered by a request like "PR #N in dev merged, continue" (normal/small workflow only — hotfix skips `dev` entirely).

1. `git fetch origin && git checkout dev && git pull origin dev`
2. Re-run the Step 7 checks against `dev` as a sanity check (the merge could have introduced a conflict-resolution regression).
3. Checkout the original feature branch (`git checkout <prefix>/<slug>`), rebase or merge `dev` into it if needed so it's up to date, then open a PR from that branch into `main`:
   ```bash
   gh pr create --base main --title "<same title>" --body "$(cat <<'EOF'
   ## Summary
   <bullets>

   ## Dev validation
   - [x] CI passed on dev
   - [x] <manual check performed, if any>

   Closes #<issue-number(s)>
   EOF
   )"
   ```
4. Set the issue(s)/project item(s) to **PR Review**.
5. Report the PR URL and **stop**. Never auto-merge.

## Resuming after the main PR is merged

Triggered by a request like "PR #N in main merged" / "approved and merged".

1. Verify: `gh pr view <N> --json state,mergedAt --jq '.state'` should be `MERGED`.
2. `Closes #N` in the PR body auto-closes the sub-issue(s) on merge — confirm with `gh issue view <N> --json state`. If any are still open, close them explicitly with a short comment.
3. For the normal workflow, check whether every sub-issue under the parent epic is now closed. If so, close the epic:
   ```bash
   gh issue close <epic-number> --comment "All sub-issues complete."
   ```
4. Set the project item(s) to **Done** (best-effort).
5. Production does **not** deploy on push to `main` for now: `.github/workflows/deploy.yml` is manual-only (`workflow_dispatch`) until a production server exists (see `docs/DEPLOYMENT.md` → "Ongoing deploys"). Note this in the report; do not attempt to trigger or verify the real deploy yourself (that crosses into `devops` agent territory, and even it never touches the real server — see `.claude/agents/devops.md`).
6. Report `STATUS: DONE`.

## Final output

Report to the user:

If a PR was opened (end of Step 11, or either resume path):

```
STATUS: PR_OPEN
STAGE: dev | main
FEATURE: <name>
ISSUES: <issue number(s)>
PR: <url>
CHECKS: <what ran and passed>
REVIEW: PASS
NEXT: <what the human needs to do>
```

If the full cycle completed (after the main PR merge resume):

```
STATUS: DONE
FEATURE: <name>
ISSUES: <issue number(s), all closed>
MERGED: <main PR url>
```

If `--no-commit` was passed and review passed:

```
STATUS: PASS
FEATURE: <name>
CHECKS: <what ran and passed>
REVIEW: PASS
COMMIT: not created (--no-commit)
```

If the workflow stopped early:

```
STATUS: FAIL
STAGE: <step where it stopped>
REASON: <why>
```
