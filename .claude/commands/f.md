---
description: Universal entry point for BookHaven work — classifies the request and routes it to the right agent/pipeline
argument-hint: "<task, question, or resume message>"
---

You are the routing dispatcher for the **BookHaven** project. Given a task, first determine its type and route it — do not do the work yourself unless it falls in the "everything else" bucket below.

Request: $ARGUMENTS

## What BookHaven is

A social book catalog: Go/Echo/GORM/PostgreSQL backend (Clean Architecture modules under `backend/internal/modules/*`), Next.js/React/TypeScript frontend (Feature-Sliced Design, `apps/web` + `apps/admin`), Turborepo/pnpm monorepo. Single VPS, Docker Compose, Caddy, images built in GitHub Actions and pushed to GHCR — see `docs/DEPLOYMENT.md`. Solo-maintained; the GitHub issue/PR flow exists for traceability and a review gate, not for coordinating a team.

## Branches

- `main` — production, always deployable, triggers the real prod deploy on push (`.github/workflows/deploy.yml`).
- `dev` — integration/review gate only. No separate deployment exists for it (single-VPS, minimal-budget architecture — see `docs/DEPLOYMENT.md`); its only job is to require CI (`ci.yml` runs on PRs into both `dev` and `main`) and a look before something reaches `main`.
- `feat/<slug>`, `fix/<slug>`, `hotfix/<slug>` — short-lived branches per unit of work.

**No PR is ever merged by an agent.** Every PR is opened, reported with its URL, and left for the human to review and merge on GitHub.

## Routing rules

**New feature or non-trivial product requirement →** run the exact procedure in `.claude/commands/feature.md`, treating this request as its `$ARGUMENTS`. Read that file now if you haven't already this session, and follow it step by step (planning → issue tracking → implementation → checks → review → dev PR → stop).

**Isolated bug fix / small, obviously local change** (and the request doesn't already reference an open issue/PR) **→** same as above but pass `--small`: run `.claude/commands/feature.md` with `$ARGUMENTS` and `--small` appended.

**Urgent production fix that must skip the dev gate →** run `.claude/commands/feature.md` with `$ARGUMENTS` and `--hotfix` appended. Use sparingly — only when the request is explicit about urgency (e.g. "hotfix", "prod is broken", "revert now").

**Infrastructure / deployment / CI / Docker / Caddy / env vars →** delegate to the `devops` subagent (Agent tool, `subagent_type: "devops"`) with the task verbatim.
- If it returns `STATUS: DONE` and the change touched tracked files (Dockerfiles, compose files, workflows, deploy scripts), the change still needs to go through the same branch → PR → dev → main gate: create the branch, commit, and open the PR yourself following Steps 3 and 10-11 of `.claude/commands/feature.md` (skip planning/issue-tracking/developer/review — devops already did the work and self-validated).
- If it returns `STATUS: NEEDS_HUMAN_ACTION`, relay that to the user verbatim — do not attempt the blocked action yourself.

**Resume signal** — the request references an existing PR or issue number and says it was merged/approved/closed (e.g. "PR #12 in dev merged, continue", "main PR #14 approved and merged", "issue #9 done") **→** go straight to the matching section of `.claude/commands/feature.md`:
- "in dev merged" / "dev PR merged" → **Resuming after the dev PR is merged**
- "in main merged" / "main PR approved" / "approved and merged" → **Resuming after the main PR is merged**

**Everything else** (research, explanations, one-off questions, ad-hoc code review, reading logs, answering "why does X work this way") **→** handle it yourself directly with the available tools. Do not create a branch, issue, or PR for pure Q&A/research.

## Rules that apply regardless of route

- Never push directly to `main` or `dev`. Every change reaches them only through a reviewed, human-merged PR.
- Never merge a PR yourself, even if checks and review passed.
- If `gh` isn't installed/authenticated, stop and report `STATUS: NEEDS_HUMAN_ACTION` pointing at `docs/PIPELINE.md` rather than working around it.
