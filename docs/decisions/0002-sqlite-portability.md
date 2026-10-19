# SQLite for a portable local deployment

Status: accepted by the user, implemented and switched locally. PostgreSQL remains an unchanged rollback copy.

## Decision and reason
Use SQLite as Resource Management's active database. The user's current priority is moving the complete application and its data between machines or servers. Shared multi-user deployment is deferred. PostgreSQL was the original choice; this change follows the user's explicit portability preference, rather than a claim that PostgreSQL cannot be moved.

The database lives at `data/resource-compass.db` under the project and is excluded from Git. `SQLITE_PATH` can set an absolute location. Go uses the pure-Go modernc SQLite driver, so there is no separately installed database server or C compiler requirement for the API binary.

## Preserved behavior
Keep the frontend, REST routes, invite-only magic links, 24-hour sessions, roles, organizations, project-owned sprints, date guards, allocations, multiple contributors, primary owners, historical rates, worklogs, audit history and archive/delete rules. Original IDs and every source record are copied. No test/demo records are removed as part of this conversion.

PostgreSQL row-level security is replaced with fail-closed application-scoped queries and organization predicates on writes. Composite foreign keys enforce organization-consistent references. Append-only SQLite triggers protect rates, worklogs and audit events. Raw access to the SQLite file remains privileged access; it is not protected by PostgreSQL-style database roles or RLS.

## Storage and concurrency
Use WAL, foreign keys on every connection, a busy timeout, full synchronous writes and immediate transactions. One application database connection serializes operations. This favors the current lightweight deployment, not horizontally scaled concurrent writers. Timestamps use UTC with fixed microsecond formatting. Monetary rates remain integer minor units; effort remains numeric hours internally, with days in the task UI.

## Migration and rollback
1. Validate a separate candidate against disposable test databases.
2. Export one consistent PostgreSQL snapshot into a private JSON file; keep credentials and row contents out of logs.
3. Import only into an empty SQLite file, atomically; reject missing tables/columns, broken relationships and integrity failures.
4. Compare every exported field with the SQLite copy, including historical and deleted records.
5. Run the full browser workflow on a separate SQLite test instance, not the user's workspace.
6. Stop the PostgreSQL API briefly, take a final PostgreSQL backup/export, import and verify the final SQLite database, then start the SQLite API.
7. Keep PostgreSQL and the pre-conversion code snapshot intact for rollback. Switching back after new SQLite edits requires migrating those edits; the old PostgreSQL copy will not automatically contain them.

## Moving and backups
Use the API `backup` command to create a consistent standalone SQLite snapshot with `VACUUM INTO`. It refuses to replace an existing file. To move an active installation, stop the source API after taking the final backup, move the project and database snapshot, set the destination path and start the destination. Do not run two writable copies and expect them to synchronize. A stopped installation can be copied with its whole data directory; never discard a remaining WAL file. Keep the live database on local persistent disk, not a network share or ephemeral hosting filesystem.

The app still needs its Go backend, frontend runtime, SMTP configuration and HTTPS when hosted. Moving only the database file does not deploy the app. Database files and backups contain private data and are not encrypted by SQLite automatically.

## Evidence
- SQLite intended uses: https://www.sqlite.org/whentouse.html
- WAL and companion files: https://www.sqlite.org/wal.html
- Consistent standalone backups: https://www.sqlite.org/lang_vacuum.html
- Driver: https://pkg.go.dev/modernc.org/sqlite
