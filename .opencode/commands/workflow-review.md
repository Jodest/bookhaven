# Workflow Review

Analyze the current BookHaven AI development workflow.

This command does not modify files.

## Run

/workflow-review

## Agent

Use Workflow Optimizer.

## Context

Provide the optimizer with:

- AGENTS.md
- .opencode/agents/
- .opencode/commands/
- .opencode/templates/
- docs/workflow/ if available
- recent feature contracts
- recent workflow results
- available metrics

Do not provide the entire product repository.

## Goals

Analyze:

- token consumption;
- execution time;
- exploration;
- retries;
- duplicated work;
- unnecessary context;
- unnecessary stages;
- documentation overhead.

## Output

Return:

STATUS: REVIEWED

TOP_RECOMMENDATIONS:
...

Do not modify files.

Do not commit.