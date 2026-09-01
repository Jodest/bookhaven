# Feature Command

Implement a BookHaven feature from request to reviewed commit.

## Input

The user provides:

/feature "<feature request>"

Optional:

/feature --small "<feature request>"

/feature --no-commit "<feature request>"

## Goal

Produce:

- implemented feature;
- passing automated checks;
- successful review;
- updated documentation;
- git commit unless --no-commit is specified.

## Step 1: Git Safety

Check git status.

If there are pre-existing changes that may conflict with the feature:

STOP.

Report the existing changes.

Never mix unrelated user changes into the feature commit.

## Step 2: Classify

Determine whether the feature is small or normal.

Use --small when explicitly requested or when the change is clearly local and low risk.

Examples of small changes:

- isolated UI change;
- copy/text change;
- small styling fix;
- trivial bug fix.

Normal workflow is required for:

- API changes;
- database changes;
- authentication;
- authorization;
- user-owned resources;
- new backend modules;
- significant frontend features.

## Step 3: Planning

For normal features:

Run Feature Planner.

Planner creates or updates:

docs/features/<feature>.md

Planner must return:

STATUS: READY

If:

STATUS: BLOCKED

STOP.

## Step 4: Implementation

Run Developer.

Developer implements the Feature Contract.

Developer updates relevant documentation.

Developer does not commit.

If Developer returns:

STATUS: PLAN_INVALID

return to Planner.

## Step 5: Automated Checks

Run only checks relevant to changed areas.

Backend:

go test ./...
go vet ./...

Frontend:

pnpm type-check
pnpm lint

Also run:

git diff --check

If any check fails:

return to Developer.

Do not invoke Reviewer until mechanical checks pass.

## Step 6: Review

Run Review and QA.

Provide:

- Feature Contract;
- git diff;
- automated check results.

If:

STATUS: PASS

continue.

If:

CATEGORY: CODE_FAIL

return to Developer.

If:

CATEGORY: PLAN_INVALID

return to Planner.

Maximum fix/review cycles: 3.

After 3 failed cycles:

STOP.

Do not commit.

## Step 7: Documentation

Documentation is handled by Developer.

After Reviewer PASS, verify that:

- docs/features/<feature>.md exists or was appropriately updated;
- docs/ROADMAP.md reflects feature status.

Do not invoke a separate documentation agent.

## Step 8: Commit

Only after Review PASS.

If --no-commit was specified:

STOP and report PASS without committing.

Otherwise:

1. inspect git diff;
2. inspect git status;
3. stage only feature-related files;
4. create one focused commit.

Use:

<type>(<scope>): <description>

Do not push.

## Final Output

Return:

STATUS: PASS

FEATURE:
<name>

CHECKS:
- ...

REVIEW:
PASS

DOCS:
- ...

COMMIT:
<hash>

If the workflow stops:

STATUS: FAIL

STAGE:
<stage>

REASON:
...

COMMIT:
NOT CREATED