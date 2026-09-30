---
description: Ranks the open backlog, asks you which item to work on next, then hands your pick to the normal /f feature pipeline
argument-hint: (none — interactive; or name a specific open proposal issue/number to skip straight to confirming it)
---

Run BookHaven's "what's next" flow: rank the open backlog (GitHub issues labeled `proposal`, created by `backlog-researcher`), ask the human which one to do, then hand the chosen item to the normal feature pipeline. **This command needs a human answer partway through — never run it unattended/headless.**

Request: $ARGUMENTS

## Step 0 — Prerequisites

Run `gh auth status`. If `gh` is missing or not authenticated, STOP and report:

```
STATUS: NEEDS_HUMAN_ACTION
ACTION_NEEDED:
Install and authenticate the GitHub CLI (see docs/PIPELINE.md) before this pipeline can read/create issues.
```

## Step 1 — Rank

Invoke the `backlog-prioritizer` subagent (Agent tool, `subagent_type: "backlog-prioritizer"`) with no extra arguments.

If it returns `STATUS: EMPTY`, tell the user there's nothing pending prioritization and suggest running `/backlog-groom` (or `backlog-researcher` directly) first, then stop — do not fabricate candidates yourself.

## Step 2 — Ask (skip only if $ARGUMENTS already names one specific open proposal issue)

If `$ARGUMENTS` clearly names a specific open `proposal` issue (by number or an unambiguous title match), confirm your understanding in one line and skip to Step 3 with that issue.

Otherwise, use `AskUserQuestion` with the ranked candidates from Step 1 as options (issue number + title as the label, the "why now" rationale as the description). Always include a "none of these right now" option. If Step 1 reported `FLAGGED_STALE` items, mention them briefly in your message before asking (they're not automatically excluded from the choices — the human may still want one) so the human can also say "reject #N" instead of picking.

If the user says "reject #N" (for a stale/duplicate/moot one) instead of picking a candidate to implement: `gh issue close <n> --comment "Rejected during backlog prioritization: <reason>."` and stop — do not proceed to planning.

## Step 3 — Hand off to the feature pipeline

Once an item is chosen:

1. `gh issue view <n> --json title,body` — this becomes the feature request text passed onward. Do not re-summarize or reinterpret it; pass the actual title/body through.
2. Run the exact procedure in `.claude/commands/feature.md`, treating the proposal issue's title + body as `$ARGUMENTS` (no `--small` unless the item is obviously trivial — most backlog proposals warrant full planning through `feature-planner`).
3. `feature.md`'s own Step 5 creates the epic (or single issue for a small/hotfix classification). Once that issue/epic number is known, close the original proposal issue and link forward: `gh issue close <proposal-n> --comment "Superseded by #<epic-or-issue-number> — picked up via /backlog-next."`
4. Continue following `feature.md` through to its own stopping point (PR opened into `dev`, never merged).

## If the user picks "none of these"

Stop after Step 2. Report:

```
STATUS: SKIPPED
REASON: user declined all ranked candidates this run.
```

Do not proceed to planning, issue creation, or implementation.

## Final output

For a hand-off that reached the pipeline, report using `.claude/commands/feature.md`'s own final-output format (`STATUS: PR_OPEN`, `STATUS: FAIL`, etc.) — this command does not invent a separate report format, it inherits whatever the pipeline it handed off to produced. Additionally note which proposal issue was closed/superseded.

For a reject-only run (Step 2's "reject #N" path), report:

```
STATUS: REJECTED
ISSUE: #<n>
REASON: <reason given>
```
