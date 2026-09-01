# BookHaven AI Development Rules

## Project

BookHaven is a social book catalog for the Russian/CIS market.

Core features:

- Book catalog
- Authors and genres
- Authentication
- User shelves
- Ratings and reviews
- User lists
- User profiles
- Admin panel

## Stack

Backend:

- Go 1.23
- Echo
- GORM
- PostgreSQL 16
- Redis 7

Frontend:

- Next.js 14
- React 18
- TypeScript
- Tailwind CSS
- shadcn/ui

Monorepo:

- Turborepo
- pnpm workspaces

Infrastructure:

- Docker
- Docker Compose

Migrations:

- SQL migrations
- golang-migrate compatible

## Repository Structure

- apps/web: public Next.js application
- apps/admin: admin Next.js application
- backend: Go backend
- packages/api-client: API client
- packages/types: shared TypeScript types
- packages/config: shared TypeScript configuration
- packages/ui: shared UI components
- docs: project documentation

## General Rules

1. Prefer existing project patterns over introducing new patterns.
2. Reuse existing abstractions when appropriate.
3. Do not introduce abstractions without a concrete need.
4. Do not perform unrelated refactoring.
5. Keep changes scoped to the requested feature.
6. Do not modify existing migrations. Create a new migration when schema changes are required.
7. Do not change dependencies unless necessary.
8. Preserve existing API contracts unless the feature explicitly requires a change.
9. Follow existing naming and directory conventions.
10. Prefer simple solutions over speculative extensibility.

## Exploration Budget

Agents must minimize repository exploration.

Default limits:

- Read at most 2 similar implementations.
- Read only files relevant to the feature.
- Read only migrations affecting relevant tables.
- Do not inspect the entire migration history unless schema evolution is relevant.
- Do not read the entire repository to understand a local feature.
- Stop exploring once enough information is available to implement safely.

Use progressive disclosure:

1. Read the relevant roadmap entry.
2. Read the feature document.
3. Inspect 1–2 relevant implementations.
4. Inspect relevant migrations only if needed.
5. Implement.

If the existing pattern is clear, stop searching.

## Documentation

Documentation hierarchy:

- README.md: setup and developer usage
- docs/PROJECT.md: project-wide description and stack
- docs/ROADMAP.md: feature status and priorities
- docs/features/<feature>.md: requirements and behavior of a feature
- docs/architecture/OVERVIEW.md: architecture overview
- docs/adr/: significant architectural decisions only

Do not duplicate detailed feature requirements in ROADMAP.md.

ROADMAP.md is an index and status document.

One feature should normally have one feature document.

Update documentation when implementation changes behavior, API, architecture, or feature status.

## Feature Documentation

Feature documents are the source of truth for feature requirements.

Use:

docs/features/<feature>.md

Do not create multiple documents for business, UX, API, database, and testing unless the feature is unusually large.

## Security

For authenticated or user-owned resources always verify:

- authentication
- authorization
- resource ownership
- IDOR risks
- input validation
- sensitive data exposure

Never assume that having an authenticated user is sufficient authorization.

## Testing

Use automated tools instead of LLM reasoning whenever possible.

Backend:

- go test ./...
- go vet ./...

Frontend:

- pnpm type-check
- pnpm lint

Run only the checks relevant to changed areas when possible.

Do not repeat checks that have already passed unless changes were made afterward.

## Mechanical vs Reasoning Checks

Mechanical checks belong to tools:

- compilation
- type checking
- linting
- tests
- formatting
- git diff validation

Reasoning checks belong to agents:

- business rules
- architecture
- authorization logic
- UX behavior
- acceptance criteria
- unintended behavior

Do not ask an agent to reproduce mechanical checks manually.

## Definition of Done

A feature is done only when:

- acceptance criteria are satisfied;
- implementation follows existing architecture;
- relevant tests exist and pass;
- relevant automated checks pass;
- authentication and authorization are correct;
- documentation is updated;
- ROADMAP.md reflects the current state;
- no unrelated changes were introduced;
- review status is PASS.

## Feature Workflow

Normal feature workflow:

Planner
→ Developer
→ automated checks
→ Reviewer
→ PASS
→ commit

Small feature workflow:

Developer
→ automated checks
→ Reviewer
→ PASS
→ commit

Do not add unnecessary agent stages.

## Failure States

CODE_FAIL:

The implementation needs correction.

Return to Developer.

PLAN_INVALID:

The feature contract is technically or architecturally invalid.

Return to Planner.

Do not ask Developer to solve an invalid plan by silently changing the architecture.

## Review Cycles

Maximum review/fix cycles: 3.

If the feature still fails after 3 cycles, stop and report the remaining issues.

Do not enter an infinite fix/review loop.

## Git

The feature command may create the commit after Review returns PASS.

Never commit a failed feature.

Never push automatically.

Before starting a feature, check git status.

Do not mix pre-existing user changes with feature changes.

Recommended commit format:

<type>(<scope>): <description>

Examples:

feat(shelves): add book shelves
fix(reviews): prevent duplicate likes
refactor(api): simplify catalog client

Do not create a commit containing unrelated changes.

## No-Go List

Do not:

- rewrite existing modules without need;
- redesign the architecture without a feature requirement;
- modify unrelated files;
- rewrite migration history;
- add dependencies without justification;
- create documentation that duplicates existing documentation;
- create an agent for a task that can be handled by an existing agent;
- repeat repository exploration unnecessarily;
- repeat automated checks unnecessarily;
- produce long explanations when a structured result is sufficient.

## AI Output

Prefer concise structured output.

Avoid unnecessary explanations, summaries, and repetition.

Use English for agent-to-agent communication unless a project document explicitly requires another language.

User-facing responses may use Russian.

## Workflow Optimization

The workflow optimizer is not part of the normal feature execution path.

It is invoked separately to analyze:

- token usage;
- context size;
- execution time;
- repository exploration;
- repeated work;
- retry/rework rate;
- agent responsibilities;
- documentation overhead;
- unnecessary workflow stages.

Optimizer recommendations must be evidence-based when metrics are available.

Do not automatically change workflow configuration without explicit approval.