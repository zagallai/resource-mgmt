# Run Resource Management on another machine

The GitHub repository contains the complete application and SQLite schema. It does not contain your database, local settings, passwords or backups. Cloning or downloading it gives you a clean installation. Transfer a database backup separately if you want to keep existing work.

## Option 1: Docker

Use macOS/Linux, or Windows with WSL2 and Docker Desktop integration enabled. Install Docker with the Compose plugin and start its daemon. You do not need to install PostgreSQL, Go or Node on the host.

1. Clone the repository, or download and extract its ZIP. Open a terminal in the extracted project folder.
2. Prepare settings and start the containers:

```sh
cp .env.example .env
sh scripts/compose-local.sh up --build -d
```

3. For a **new empty installation only**, create the administrator once:

```sh
sh scripts/compose-local.sh --profile setup run --rm bootstrap
```

4. Open http://localhost:3000. Sign in as `admin@resourcemgmt.com`; retrieve the link at http://localhost:8025. You can change the initial administrator email and workspace name in `.env` before bootstrap.

Use `sh scripts/compose-local.sh down` to stop. Restart with `sh scripts/compose-local.sh up -d`. The data remains in the project's `data` directory. Calling the helper through `sh` also works when a ZIP download does not preserve executable permissions.

## Option 2: Run directly

Install Go 1.26 or newer and Node.js 24 with npm. Use the separate terminals in the README's Native start section, including `npm ci` in `apps/web`. The local mail inbox is provided by `node scripts/dev-mail.mjs`; no external email provider is necessary for local testing. Use a POSIX shell on macOS/Linux or inside WSL2 for the documented commands.

All paths are resolved within the downloaded project, or explicitly overridden with `SQLITE_PATH`. The API creates its SQLite schema automatically. The app needs no connection to the original computer or a PostgreSQL server.

## Transfer existing data instead of starting empty

On the original machine, stop the API to freeze further edits, then create the final transfer snapshot:

```sh
cd apps/api
go run . backup ../../backups/resource-management-transfer.db
```

The backup command creates the destination directory if needed and refuses to overwrite an existing snapshot. For an all-Docker installation, create a consistent snapshot with `sh scripts/compose-local.sh exec api compass backup /backups/resource-management-transfer.db`, then stop the source API before further edits. If using Docker only and exact freeze timing matters, run the backup command through a one-off API container after stopping the API service.

On the destination, before starting its API:

1. Download/clone and extract the code.
2. Create `data/` and copy the transfer snapshot into it as `data/resource-compass.db`.
3. For Docker, copy `.env.example` to `.env`, then start through the helper. For native startup, use the README's mail/API/frontend commands.
4. **Skip bootstrap.** Your administrator account and workspace are already inside the transferred database.
5. Sign in with the administrator email saved in that database. Existing browser cookies are not transferred; request a fresh local login link if needed.

Do not overwrite an existing destination database without backing it up first. Keep the source stopped once you continue editing on the destination: two copies will not synchronize. Do not copy just the main database file while the API is running, because its WAL companion may contain recent writes. Use the backup command or copy the complete stopped data directory.

## Deployment notes

- Source code is portable; a compiled Go binary must match the destination OS/CPU. Docker rebuilds for the destination platform.
- Native builds install dependencies from the committed Go modules and npm lockfile; internet access is needed for the initial download/build.
- Ports 3000, 8080 and 8025 must be available for local use.
- SQLite needs writable persistent local storage. Shared network filesystems and disposable hosting filesystems are not suitable for the live database.
- Local email is shown only in the development inbox. Hosting for remote users requires configured SMTP, a reachable application origin, HTTPS and secure cookies.
- Docker configuration validation has passed; Docker image startup has not been verified on this host because its daemon is stopped.
