# Resource Management

Engineering resource and project planning with a Next.js/TypeScript frontend, Go REST API and a portable SQLite database. No PostgreSQL service is required. The database defaults to `data/resource-compass.db` inside this project and is excluded from Git.

For a newly downloaded copy, follow [Run on another machine](docs/run-on-another-machine.md). It covers both a clean installation and transferring existing data.

## Install and build

Install Go 1.26 or newer and Node.js 24 with npm. From the project folder, build the API and frontend:

```sh
cd apps/api
go mod download
go build -o compass .
cd ../web
npm ci
npm run build
```

## Run locally

Start these in three separate terminals. Each terminal starts in the project folder.

```sh
# Terminal 1: local email inbox
node scripts/dev-mail.mjs
```

```sh
# Terminal 2: API
cd apps/api
# New installation only; skip this after transferring an existing database:
ADMIN_EMAIL=admin@resourcemgmt.com ORG_NAME=Engineering ./compass bootstrap
COOKIE_SECURE=false APP_ORIGIN=http://localhost:3000 SMTP_ADDR=127.0.0.1:1025 ./compass
```

```sh
# Terminal 3: frontend
cd apps/web
npm start
```

Open http://localhost:3000 and request a sign-in link. The local inbox is http://localhost:8025. Links expire in 10 minutes; sessions last 24 hours. The API initializes the SQLite schema automatically. Set `SQLITE_PATH` to an absolute file location to override the default. The API resolves the default inside the source project's data directory; a standalone binary uses `data/` under its working directory.

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
./compass backup ../../backups/resource-compass-transfer.db
```

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

SQLite databases, backups, local environment files, generated screenshots and build output are excluded from Git. A fresh checkout starts without working data; follow the bootstrap steps above. Never add a database or export to source control. CI runs backend race tests, Go vet and a production frontend build.
