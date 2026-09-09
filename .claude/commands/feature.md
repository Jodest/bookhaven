---
description: Implement a BookHaven feature end-to-end via planner -> developer -> checks -> reviewer -> commit
argument-hint: "<feature request>" [--small] [--no-commit]
---

Implement the following BookHaven feature request from planning through a reviewed commit, orchestrating the project's `feature-planner`, `developer`, and `review-qa` subagents (via the Agent tool). Follow every step below in order; do not skip steps or invoke Reviewer before mechanical checks pass.

Request: $ARGUMENTS

Parse `$ARGUMENTS` for the quoted feature request text and the optional flags `--small` and `--no-commit`.

## Step 1 — Git safety

Run `git status`. If there are pre-existing uncommitted changes unrelated to this feature that could get mixed into the feature commit, STOP and report them to the user instead of proceeding — never mix unrelated pre-existing changes into the feature commit.

## Step 2 — Classify

Decide small vs. normal workflow:

- Use small when `--small` was passed, or the change is obviously local and low risk (isolated UI tweak, copy/text change, small styling fix, trivial bug fix).
- Use normal for anything touching the API, database, auth, user-owned resources, a new backend module, or a significant frontend feature.

## Step 3 — Planning (normal workflow only)

Skip this step for `--small`.

Invoke the `feature-planner` subagent (Agent tool, `subagent_type: "feature-planner"`) with the full feature request as its prompt — it starts with zero context, so give it the complete request text, not a summary.

- If it returns `STATUS: READY`, continue to Step 4 with the `CONTRACT` path it reports.
- If it returns `STATUS: BLOCKED`, STOP and report the reason to the user.

## Step 4 — Implementation

Invoke the `developer` subagent (Agent tool, `subagent_type: "developer"`). Give it:

- the Feature Contract path from Step 3 (normal workflow), or the raw feature request verbatim (small workflow) — it cannot read your conversation, so include the actual request text, not just a reference to it;
- explicit instruction not to commit.

- If it returns `STATUS: IMPLEMENTED`, continue to Step 5.
- If it returns `STATUS: PLAN_INVALID`, go back to Step 3 (re-plan with the reported reason) — for the small workflow, STOP and report instead, since there is no planner to re-invoke.

## Step 5 — Automated checks

Run yourself (do not delegate) only the checks relevant to what changed:

- Backend touched: from `backend/`, run `go build ./...`, `go vet ./...`, `go test ./...`.
- Frontend touched (`apps/web` and/or `apps/admin`): run `tsc --noEmit` and `next lint` for each touched app. On Windows, invoke the app's `node_modules/.bin/next` shim directly (e.g. `cd apps/admin && ./node_modules/.bin/next lint`) rather than `pnpm run`, which can fail to resolve the POSIX shim through cmd.exe.
- Always run `git diff --check` (whitespace/conflict-marker sanity).

If any check fails, go back to Step 4 with the failure output. Do not invoke the reviewer until every relevant mechanical check passes.

## Step 6 — Review

Invoke the `review-qa` subagent (Agent tool, `subagent_type: "review-qa"`). Give it, inline in the prompt (it has zero context):

- the Feature Contract content or path (normal workflow), or the original request (small workflow);
- the full `git diff` output for the changed files;
- the Step 5 automated check results.

- If it returns `STATUS: PASS`, continue to Step 7.
- If it returns `CATEGORY: CODE_FAIL`, go back to Step 4 with the reported issues.
- If it returns `CATEGORY: PLAN_INVALID`, go back to Step 3 (or STOP for the small workflow).

Track review/fix cycles (Steps 4-6). After 3 failed cycles, STOP and report the remaining issues instead of looping further.

## Step 7 — Documentation check

After a PASS, verify yourself (do not invoke another agent):

- `docs/features/<feature>.md` exists or was appropriately updated (normal workflow);
- `docs/ROADMAP.md` reflects the feature's current status.

## Step 8 — Commit

Only after Step 6 returned `STATUS: PASS`.

If `--no-commit` was passed: STOP and report PASS without committing.

Otherwise:

1. Inspect `git diff` and `git status`.
2. Stage only the feature-related files (never `git add -A`/`.` blindly — exclude anything unrelated found in Step 1).
3. Create one focused commit using `<type>(<scope>): <description>` (e.g. `feat(shelves): add book shelves`).
4. Never push.

## Final output

Report to the user:

If successful:

```
STATUS: PASS
FEATURE: <name>
CHECKS: <what ran and passed>
REVIEW: PASS
DOCS: <what was updated>
COMMIT: <hash, or "not created" if --no-commit>
```

If the workflow stopped early:

```
STATUS: FAIL
STAGE: <step where it stopped>
REASON: <why>
COMMIT: NOT CREATED
```
