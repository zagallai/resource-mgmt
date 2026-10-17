# Validation — 2026-10-16

## Passed

- Official Spec Kit initialized with specify-cli 1.2.0, Codex integration and bundled workflow/scripts/templates.
- PostgreSQL 16 integration suite with Go race detector: 9 subtests passed. Tests use a real disposable database and a non-superuser application role.
- Tested: hashed single-use magic links, expiry/replay, invitation acceptance, role enforcement, logout, origin validation and throttling; tenant access denial, RLS fail-closed writes and cross-tenant foreign-key rejection; multiple contributors, daily overload, weekday sprint capacity; hold/resume/reassignment, stale-version rejection, previous-owner audit data and preserved allocation/work history; historical actual rate snapshots, dated forecast repricing, immutable rate/worklog/audit rows and missing-rate reporting.
- Playwright Chromium: one complete browser scenario passed (3.9 seconds including runner startup): real SMTP email-link sign-in → resources/project/rates/sprint/task → two contributors → actual and forecast cost assertions → hold → reload/persisted release history → resume → reassignment. Screenshot reviewed for desktop legibility in docs/dashboard.png. Test fixture names include timestamps; screenshot contains local test data.
- Next.js production build and strict TypeScript check passed.
- Go vet passed.
- npm dependency audit reported zero vulnerabilities at install time.
- Docker Compose configuration validates.

## Running local development session

Frontend: http://localhost:3000
Go API/database health: http://localhost:8080/health
Development email inbox: http://localhost:8025
Bootstrap admin email: admin@example.com

Request a link in the frontend, open the development inbox, copy the latest link and confirm sign-in. Email links are single-use, so request a fresh one rather than reusing a test link.

Because the host Docker daemon was stopped and Go was absent from PATH, validation used a temporary downloaded Go 1.27.2 toolchain, an isolated native PostgreSQL instance on 127.0.0.1:55432 with data in /tmp/rc-pgdata, and the loopback-only scripts/dev-mail.mjs inbox. The API runs as compass_app, not a database superuser. These local processes are temporary; the README/Compose setup is the reproducible handoff.

## Not yet verified / not complete

Full Docker image builds and service startup were not run: the Docker daemon was stopped. Mailpit itself was not used in this native session; the local SMTP sink exposes the same read endpoints used by the test. CI configuration is provided but has not run on GitHub. No remote repository, deployment or production email provider was created.

This is an initial vertical slice, not the complete V1. Leave/holidays, remaining sprint capacity, monthly cost reports, revised estimate history, fine-grained permissions, CSV/Jira import/sync, membership revocation, production mail delivery and operational hardening remain in specs/001-foundation/tasks.md. Current assumptions: USD, UTC calendar days, three broad roles, day-level release semantics and scheduled-commitment forecasts.
