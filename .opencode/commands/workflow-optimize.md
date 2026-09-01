# Workflow Optimize

Analyze and propose improvements to the BookHaven AI workflow.

## Modes

/workflow-optimize

Analyze and propose changes.

Optional:

/workflow-optimize --apply

Apply approved low-risk changes.

## Rules

Use Workflow Optimizer.

Focus only on:

- AGENTS.md
- .opencode/
- docs/workflow/

Do not modify product code.

Do not add agents unless clearly justified.

Prefer removing or shortening instructions over adding complexity.

## Output

For each change provide:

ID:
...

FILE:
...

PROBLEM:
...

CHANGE:
...

EXPECTED IMPACT:
...

RISK:
...

If --apply is not specified:

STATUS: PROPOSAL

Do not modify files.

If --apply is specified:

Apply only low-risk, clearly justified changes.

Then return:

STATUS: APPLIED

CHANGES:
- ...

Do not commit automatically.