---
description: Periodic backlog grooming — researches new candidate work, files each as a trackable GitHub issue, and notifies the human; never plans, ranks, or implements anything itself
argument-hint: "[focus] [N cycles] — optional, e.g. 'security', 'ops', 'deps', 'spec-drift', '3 cycles'; headless-safe, does not need a human answer mid-run"
---

Run BookHaven's backlog-grooming pass. This command only researches, files, and notifies — it never invokes `feature-planner`, never ranks candidates, and never implements anything. Picking and implementing the next item happens later, in `/backlog-next` (a separate, interactive command — do not run that one headless).

## Step 1 — Git safety

Run `git status`. If there are pre-existing uncommitted changes unrelated to this run, STOP and report them instead of proceeding.

Run `git fetch origin` and make sure you're starting from an up-to-date `main`.

## Step 2 — Research

Invoke the `backlog-researcher` subagent (Agent tool, `subagent_type: "backlog-researcher"`) — it knows its own sources, opens a `proposal`-labeled GitHub issue per new candidate, and writes a matching line into `docs/ROADMAP.md`. Pass `$ARGUMENTS`' focus (if any) as `Focus: <focus>`; with no focus, pass no extra arguments.

If `$ARGUMENTS` asks for several cycles, run them strictly one after another, giving each cycle a different focus (the one named, or rotate through `spec`, `spec-drift`, `review-followup`, `deps`, `code`, `ops`, `competitive`), all on one branch with one PR at the end. Stop early after two consecutive cycles return `STATUS: NO_NEW_CANDIDATES`.

## Step 3 — Read back the full pending list

Read `docs/ROADMAP.md`'s `## Proposed (awaiting prioritization)` section in full, not just what this run added — earlier runs' candidates may still be untriaged.

If the section doesn't exist or is empty after Step 2, skip to the no-op report below.

## Step 4 — Commit the doc change

If `docs/ROADMAP.md` changed:

```bash
git checkout main && git pull origin main
git checkout -b docs/backlog-groom-<YYYY-MM-DD>
git add docs/ROADMAP.md
git commit -m "docs(roadmap): backlog grooming — N new candidate(s)"
git push -u origin docs/backlog-groom-<YYYY-MM-DD>
gh pr create --base dev --title "docs(roadmap): backlog grooming <YYYY-MM-DD>" --body "..."
```

**Do not merge.** This is a documentation-only PR like any other — same dev → main gate, left for the human.

If nothing changed (backlog-researcher returned `STATUS: NO_NEW_CANDIDATES`), skip this step entirely — no branch, no commit, no PR.

## Step 5 — Notify

Call `PushNotification` (`status: "proactive"`) with a one-line summary, e.g. `"BookHaven backlog: N new proposal issues, M awaiting prioritization"`.

Skip the notification if Step 2 returned `STATUS: NO_NEW_CANDIDATES` **and** the pending list was already empty before this run — nothing changed, nothing worth interrupting for.

## Final output

If candidates were proposed:

```
STATUS: GROOMED
NEW_CANDIDATES: <count>
PENDING_TOTAL: <count of all open `proposal` issues>
PR: <url, if one was opened for the ROADMAP.md update>
PENDING:
- #<n> <item 1>
- #<n> <item 2>
...
NEXT: Run /backlog-next when you're ready to pick one — it ranks these, asks you to choose, and hands the pick to the normal feature pipeline. Nothing gets planned or implemented until then.
```

If nothing new was found and the pending list is empty:

```
STATUS: NO_OP
REASON: No new candidates found; nothing pending prioritization.
```

If nothing new was found but items are still pending from earlier runs:

```
STATUS: NO_NEW_CANDIDATES
PENDING_TOTAL: <count>
PENDING:
- <item 1>
...
NEXT: These are still waiting on your prioritization from a previous run.
```
