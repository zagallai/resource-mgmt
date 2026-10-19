# Resource Management

Engineering resource and project planning with a Next.js/TypeScript frontend, Go REST API and a portable SQLite database. No PostgreSQL service is required. The database defaults to `data/resource-compass.db` inside this project and is excluded from Git.

## Native start

Requires Go 1.26+, Node 24 and an SMTP inbox. Schema initialization runs automatically when the API starts.

```sh
# Terminal 1, from the project root
node scripts/dev-mail.mjs
# Terminal 2
cd apps/api
# New installation only: run bootstrap once, not after copying an existing database.
ADMIN_EMAIL=admin@resourcemgmt.com ORG_NAME=Engineering go run . bootstrap
COOKIE_SECURE=false APP_ORIGIN=http://localhost:3000 SMTP_ADDR=127.0.0.1:1025 go run .
# Terminal 3
cd apps/web
npm ci
npm run dev
```

Open http://localhost:3000 and request a sign-in link. The local inbox is http://localhost:8025. Links expire in 10 minutes; sessions last 24 hours. Set `SQLITE_PATH` to an absolute file location to override the default. The API resolves the default inside the source project's data directory; a standalone binary uses `data/` under its working directory.

## Docker start

```sh
cp .env.example .env
./scripts/compose-local.sh up --build -d
# New installations only:
./scripts/compose-local.sh --profile setup run --rm bootstrap
```

The wrapper uses your local user/group IDs and prepares private `data/` and `backups/` directories. SQLite is bind-mounted from this project; there is no database-server container. Stop with `./scripts/compose-local.sh down`. Existing data is retained. Docker is an optional local environment; use HTTPS, secure cookies and proper SMTP when hosting.

## Working in the app

- Resources manages profiles, working hours (default eight), hourly rates, archive/restore and deletion of unused people.
- Projects manages project details, prime resource, optional activity rows and confirmed deletion with explicit linked-task override.
- Planning creates/edits project-owned sprints, newest first.
- Tasks creates and assigns work. Jira keys are optional; estimates are days (including quarters). A standard effort day is eight hours. Sprint task dates must stay inside the sprint. Allocate resource time defaults to the task's owner; actual-work logging is optional.
- Overview shows team availability, workload and sprint capacity. Project costs are hidden for now.
- Activity retains bookings, actual work and audit history. Admin manages invitations.

Current limitation: an allocation still books all entered days against its single selected date. A three-day booking therefore appears as 24 hours on that date; multi-date distribution remains to be implemented. Leave/holiday calendars, early-finish forecasts, pagination and Jira integration remain backlog.

## Backup and move

Create a consistent standalone database snapshot; the destination must not already exist:

```sh
cd apps/api
go run . backup ../../backups/resource-compass-transfer.db
```

For Docker, use `./scripts/compose-local.sh exec api compass backup /backups/resource-compass-transfer.db`.

For a move, stop the source API after its final backup, copy the application and snapshot to the destination, place the snapshot at `data/resource-compass.db`, restore runtime/SMTP settings, install dependencies and start. Do not bootstrap again. Rebuild binaries for a different operating system/CPU. Do not copy only the main file while the API is running: WAL companion files can contain recent writes. Use the backup command, or stop the API and copy the entire data directory. Use persistent local disk, not a network share or an ephemeral deployment filesystem. Separate writable copies do not synchronize.

Database and backup files contain private user/session/history data; keep them private and out of Git. SQLite does not encrypt the file automatically.

## Tests

```sh
cd apps/api
go test -race -v ./...
go vet ./...
cd ../web
npm run build
```

Backend tests use temporary SQLite files with no database server and no skipped PostgreSQL prerequisites. They cover login, roles, tenant boundaries, multiple contributors, rates, history, sprint guards, archive/delete, concurrent login reuse, atomic import and backup/restore. Browser tests must run against a dedicated disposable database/organization, never the user's working data. `E2E_BASE_URL` sets the browser target; `MAIL_URL` sets the local inbox.

## Architecture and specifications

See [the SQLite decision](docs/decisions/0002-sqlite-portability.md), [schema diagram](docs/sqlite-schema.md), and [validation](docs/validation.md). The active embedded schema is `apps/api/sqlite_schema.sql`, with a separate SQLite schema-version history. `apps/api/schema.sql` and PostgreSQL migration files are historical references, not executed by the SQLite API. Existing Spec Kit files under `.specify`, `.agents/skills` and `specs` remain available; the SQLite decision supersedes earlier PostgreSQL/RLS assumptions.

## GitHub repository

SQLite databases, backups, local environment files, generated screenshots and build output are excluded from Git. A fresh checkout starts without working data; follow the bootstrap steps above. Never add a database or export to source control. CI runs backend race tests, Go vet, a production frontend build and Compose configuration validation.
