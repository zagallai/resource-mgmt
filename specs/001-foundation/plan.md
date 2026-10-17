# Implementation plan

## Architecture
Browser → same-origin Next.js rewrite → Go REST service → PostgreSQL.
Go is the sole source of business rules, auth, authorization and writes. Next.js has no database access and never stores auth tokens in localStorage. SMTP delivers links through Mailpit locally. The token travels in a URL fragment and is exchanged only after an explicit sign-in confirmation, reducing accidental mail-scanner consumption and query-string logging.

Each business request looks up a hashed session, checks membership for the URL organization, begins a database transaction and sets transaction-local app.tenant. FORCE RLS and composite tenant references defend against forgotten predicates. Pool connections do not retain tenant settings. Runtime credentials must never be migration-owner/superuser credentials.

Authentication control-plane tables (organizations, users, memberships, invitations, login_tokens, sessions, auth_limits) are not protected by RLS because sign-in precedes tenant selection. Their fixed SQL queries are explicitly scoped by email/session/user/membership; no generic table endpoint exposes them. Business tables have RLS. This distinction requires review whenever auth APIs change.

## Transactions and concurrency
Task mutations and allocations lock their task row. Status and owner changes require a version and reason. Allocation release, task mutation and audit insert commit together. Link consumption, invitation acceptance and session creation commit together. Worklogs are append-only with rate snapshots. A first-schema embedded migration runs atomically; schema_version makes startup idempotent. Later schema changes require ordered incremental migrations; do not alter the original schema in a deployed installation.

## Roles
Admin: all org operations, invitations, rate creation/reading, audit history.
Manager: resources/projects/tasks/sprints/allocations/worklogs and aggregate cost dashboard; no raw rates.
Viewer: operational reads only; no budgets, financial dashboard, raw rates or audit records.
Detailed project/engineer permission scopes are backlog. Resource records are not yet linked to login users.

## Delivery sequence
1. Specification, constitution and domain schema.
2. PostgreSQL isolation + invite-only auth.
3. Resources/projects/tasks + day allocations + hold/reassign transactions.
4. Rates/worklogs + sprint/daily capacity + project cost aggregates.
5. Next.js working forms and dashboards.
6. Integration, browser tests, Compose and CI, documented limitations.
7. Remaining V1 in tasks.md.

## Verification
PostgreSQL integration suite creates/drops its own temporary database and a non-superuser role. Requires an admin test DSN only in the test process. Run go test -race with TEST_DATABASE_URL. Build frontend with npm ci && npm run build. Playwright exercises real email-link login and persisted task hold/reassignment. Docker Compose config validation is distinct from booting the complete stack.

## Sources
- GitHub Spec Kit: https://github.com/github/spec-kit — initialized using specify-cli 1.2.0, Codex integration, bundled scripts/templates/workflow.
- Next.js installation: https://nextjs.org/docs/app/getting-started/installation — App Router and TypeScript.
- PostgreSQL RLS: https://www.postgresql.org/docs/current/ddl-rowsecurity.html — policy enforcement and superuser/owner caveats.
