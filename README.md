# Resource Compass

An initial persistent vertical slice of a multi-user engineering planning app, built with Next.js/TypeScript, Go REST and PostgreSQL. This directory is the application; the parent project's sources remain untouched.

## Start with Docker

Requires Docker Compose and a running Docker daemon. Local services bind to loopback. Credentials are development-only.

```sh
cp .env.example .env
docker compose up --build -d
docker compose --profile setup run --rm bootstrap
```

Run bootstrap once for the first organization. Set ADMIN_EMAIL and ORG_NAME in .env beforehand. Each additional bootstrap invocation deliberately creates another organization. Open http://localhost:3000 and request a sign-in link for that admin email. Open the email in Mailpit at http://localhost:8025, follow the link and confirm sign-in. No public registration and no seeded login bypass.

1. People & projects: create resources and a project.
2. Administration: add a rate for each resource, effective on/before work dates. Invite colleagues here; after saving, colleagues request their own sign-in email.
3. Planning: create a sprint.
4. Tasks: create a Jira task with a primary owner and optional sprint.
5. Planning: allocate different contributors by day and record actual work.
6. Tasks: hold/resume or reassign, providing a reason. Released allocations remain visible in Activity. Resuming/reassigning requires explicit new scheduling.
7. Overview: inspect full-sprint capacity, daily overload count and project costs.

Stop with `docker compose down`; the database volume remains. Avoid removing the volume if you need its data.

## Native development

Go 1.26+, Node 24, PostgreSQL 16+, SMTP test inbox (or run `node scripts/dev-mail.mjs` for a loopback-only inbox at port 8025). Start PostgreSQL and create a database plus separate migration and application roles. Apply scripts/init-db.sql adapting the owner name and database name for your local environment. Run the embedded migration with the owner DSN, then bootstrap once:

```sh
cd apps/api
export DATABASE_URL='postgres://OWNER:PASSWORD@localhost:5432/compass?sslmode=disable'
go run . migrate
ADMIN_EMAIL=admin@example.com ORG_NAME=Engineering go run . bootstrap
export DATABASE_URL='postgres://compass_app:compass_app_local@localhost:5432/compass?sslmode=disable'
APP_ORIGIN=http://localhost:3000 COOKIE_SECURE=false SMTP_ADDR=localhost:1025 go run .
```

In another terminal:

```sh
cd apps/web
npm ci
npm run dev
```

Never run the API as a database superuser: superusers bypass RLS. Secure cookies default to true; COOKIE_SECURE=false is for local HTTP only. Production requires HTTPS, secure cookies, real mail delivery, managed secrets, backups and the remaining operational tasks in the backlog. Compose is a local environment, not a production deployment.

## Tests

```sh
cd apps/api
TEST_DATABASE_URL='postgres://ADMIN:PASSWORD@localhost:5432/postgres?sslmode=disable' go test -race -v ./...
go vet ./...
cd ../web
npm ci
npm run build
npx playwright install chromium
npm run test:e2e
```

The integration test creates a new isolated database and restricted runtime role, then removes them. Its admin connection needs CREATEDB/CREATEROLE. Without TEST_DATABASE_URL the integration test explicitly skips. Browser tests require a running app, fresh bootstrap admin (admin@example.com by default), and a local Mailpit-compatible inbox at port 8025. They create uniquely named records in that local organization. See docs/validation.md for actual execution results and limits.

## Specifications

Official GitHub Spec Kit scaffolding is under .specify and .agents/skills (specify-cli 1.2.0, Codex integration). Project-specific baseline: specs/001-foundation/{spec,plan,data-model,tasks}.md. API contract: specs/001-foundation/contracts/openapi.json. The first migration is embedded in apps/api/schema.sql and versioned as schema_version=1; subsequent versions need incremental migrations.

Current limits: USD/UTC day-based planning; three broad roles; weekday capacity without leave/holidays; forecast covers scheduled future commitments only and does not automatically reconcile worklogs against today's allocations. Monthly reports, estimate history, CSV/Jira integration and detailed permissions remain backlog.
