# Run Resource Management on another machine

The GitHub repository contains the complete application and SQLite schema. It does not contain your database, local settings, passwords or backups. Cloning or downloading it gives you a clean installation. Transfer a database backup separately if you want to keep existing work.

## Install and build from source

Use macOS/Linux or a POSIX shell in Windows WSL2. Install Go 1.26 or newer and Node.js 24 with npm. Download/clone the repository and open a terminal in its project folder:

```sh
cd apps/api
go mod download
go build -o compass .
cd ../web
npm ci
npm run build
```

Follow the [README's run steps](../README.md#run-locally) to start the local inbox, API and frontend in separate terminals. For a new empty installation, bootstrap the administrator once. For transferred data, skip bootstrap. The API creates the SQLite schema automatically; it needs no connection to the original computer or a database server.

## Transfer existing data instead of starting empty

On the original machine, stop the API to freeze further edits, then create the final transfer snapshot:

```sh
cd apps/api
./compass backup ../../backups/resource-management-transfer.db
```

The backup command creates the destination directory if needed and refuses to overwrite an existing snapshot. Build the API first if you have not already done so.

On the destination, before starting its API:

1. Download/clone and extract the code.
2. Create `data/` and copy the transfer snapshot into it as `data/resource-compass.db`.
3. Install and build using the commands above, then start the mail inbox, API and frontend using the README commands.
4. **Skip bootstrap.** Your administrator account and workspace are already inside the transferred database.
5. Sign in with the administrator email saved in that database. Existing browser cookies are not transferred; request a fresh local login link if needed.

Do not overwrite an existing destination database without backing it up first. Keep the source stopped once you continue editing on the destination: two copies will not synchronize. Do not copy just the main database file while the API is running, because its WAL companion may contain recent writes. Use the backup command or copy the complete stopped data directory.

## Deployment notes

- Source code is portable; build the Go binary on the destination machine for its OS/CPU.
- The build installs dependencies from the committed Go modules and npm lockfile; internet access is needed for the initial download/build.
- Ports 3000, 8080 and 8025 must be available for local use.
- SQLite needs writable persistent local storage. Shared network filesystems and disposable hosting filesystems are not suitable for the live database.
- Local email is shown only in the development inbox. Hosting for remote users requires configured SMTP, a reachable application origin, HTTPS and secure cookies.
