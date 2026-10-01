---
description: Ranks the open backlog, asks you which item(s) to work on and in what order, then works through them one at a time via the normal /f feature pipeline
argument-hint: (none — interactive; or name a specific open proposal issue/number to skip straight to confirming it)
---

Run BookHaven's "what's next" flow: rank the open backlog (GitHub issues labeled `proposal`, created by `backlog-researcher`), ask the human which one(s) to do and in what order, then hand items to the normal feature pipeline **one at a time, strictly sequentially** — never start item 2 before item 1's PR is open and this command has stopped for the human. **This command needs a human answer partway through — never run it unattended/headless.**

Request: $ARGUMENTS

## Step 0 — Prerequisites

Run `gh auth status`. If `gh` is missing or not authenticated, STOP and report:

```
STATUS: NEEDS_HUMAN_ACTION
ACTION_NEEDED:
Install and authenticate the GitHub CLI (see docs/PIPELINE.md) before this pipeline can read/create issues.
```

## Step 1 — Check for an existing queue first

Read `docs/ROADMAP.md`. If a `## Up Next (queued order)` section exists and is non-empty, **skip ranking and asking entirely** — a human already chose this order in a previous run. Take the first item listed, remove it from the section (committing that removal the same way Step 4 below commits queue changes), and go straight to **Step 5 — Hand off**.

If the section doesn't exist or is empty, continue to Step 2.

## Step 2 — Rank

Invoke the `backlog-prioritizer` subagent (Agent tool, `subagent_type: "backlog-prioritizer"`) with no extra arguments.

If it returns `STATUS: EMPTY`, tell the user there's nothing pending prioritization and suggest running `/backlog-groom` (or `backlog-researcher` directly) first, then stop — do not fabricate candidates yourself.

## Step 3 — Ask what and in what order

If `$ARGUMENTS` clearly names a specific open `proposal` issue (by number or an unambiguous title match), confirm your understanding in one line and treat that as a single-item selection — skip to Step 4.

Otherwise, use `AskUserQuestion` (`multiSelect: true`) with the ranked candidates from Step 2 as options (issue number + title as the label, the "why now" rationale as the description). Always include a "none of these right now" option. If Step 2 reported `FLAGGED_STALE` items, mention them briefly before asking — the human may say "reject #N" instead of picking.

- If the user picks **one** item, that's the whole selection — no ordering question needed.
- If the user picks **more than one**, ask a plain follow-up question (no need for another `AskUserQuestion`) asking them to state the order by issue number, e.g. *"You picked #12, #15, #20 — what order? (e.g. '15, 12, 20')"*. Wait for their reply before continuing.
- If the user says "reject #N" for a stale/duplicate/moot item instead of selecting it: `gh issue close <n> --comment "Rejected during backlog prioritization: <reason>."` and stop — do not proceed to Step 4.

## Step 4 — Persist the queue (only if more than one item was selected)

Update `docs/ROADMAP.md`: add (or extend) a `## Up Next (queued order)` section directly above `## Proposed (awaiting prioritization)`, listing the chosen items in the stated order:

```
1. #<n> <title>
2. #<n> <title>
...
```

Remove these same items from the `## Proposed` section — they're no longer "awaiting prioritization," they're queued. Commit and open a doc-only PR the same way `/backlog-groom` does (short branch off `main`, PR into `dev`, never merged) — this keeps the queue visible and durable across sessions even if implementation of item 1 takes a while.

If only one item was selected, skip this step — there's no queue to persist for a single pick, go straight to Step 5.

## Step 5 — Hand off exactly one item to the feature pipeline

Take the single item now in hand (from Step 1's dequeue, Step 3's single pick, or Step 4's first queued item):

1. `gh issue view <n> --json title,body` — this becomes the feature request text passed onward. Do not re-summarize or reinterpret it; pass the actual title/body through.
2. Run the exact procedure in `.claude/commands/feature.md`, treating the proposal issue's title + body as `$ARGUMENTS` (no `--small` unless the item is obviously trivial — most backlog proposals warrant full planning through `feature-planner`).
3. `feature.md`'s own Step 5 creates the epic (or single issue for a small/hotfix classification). Once that issue/epic number is known, close the original proposal issue and link forward: `gh issue close <proposal-n> --comment "Superseded by #<epic-or-issue-number> — picked up via /backlog-next."`
4. Continue following `feature.md` through to its own stopping point (PR opened into `dev`, never merged).

**Stop there.** Do not automatically continue to the next queued item, even if one is waiting — that would violate the strictly-sequential, one-thing-in-flight rule this command exists to enforce. The next queued item starts only when the human runs `/backlog-next` again (Step 1 will find it waiting).

## If the user picks "none of these"

Stop after Step 3. Report:

```
STATUS: SKIPPED
REASON: user declined all ranked candidates this run.
```

Do not proceed to planning, issue creation, or implementation.

## Final output

For a hand-off that reached the pipeline, report using `.claude/commands/feature.md`'s own final-output format (`STATUS: PR_OPEN`, `STATUS: FAIL`, etc.) — this command does not invent a separate report format, it inherits whatever the pipeline it handed off to produced. Additionally note:

- which proposal issue was closed/superseded;
- if a queue was persisted in Step 4, the remaining items and their order, plus: *"Run `/backlog-next` again when ready for the next one — it'll pick up #<next> directly, no re-ranking needed."*

For a reject-only run (Step 3's "reject #N" path), report:

```
STATUS: REJECTED
ISSUE: #<n>
REASON: <reason given>
```
