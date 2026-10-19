# SQLite conversion validation

The conversion was developed under `/tmp/resource-compass-sqlite-candidate` so the running PostgreSQL application was preserved during implementation.

## Passed
- Full backend integration suite on temporary SQLite databases; no PostgreSQL connection or skipped setup required.
- Race-enabled tests and Go vet.
- Single-use magic-link verification under concurrent requests.
- Backup creates a standalone file, restores correctly and refuses overwrite.
- Import rejects populated destinations and rolls back broken relationships atomically.
- Fail-closed organization reads/writes, cross-organization IDs and compound update predicates.
- A consistent PostgreSQL export imported into SQLite; every field across 16 application tables compared, with foreign-key and integrity checks passing.
- Frontend production build.

## Completed runtime verification
- Full isolated browser workflow passed: 1 test, 9.1 seconds, using a dedicated SQLite database and separate local ports.
- Final source API stopped before a new PostgreSQL backup/export. All 16 application tables imported and every field compared successfully; relationship and integrity checks passed.
- Production frontend rebuilt for the normal local API, then the SQLite API and frontend started on ports 8080 and 3000.
- API health returned OK; the frontend API proxy returned the expected unauthenticated response.
- The user's existing browser session restored; saved project-owned sprints were visible in the live Planning screen without another login.
- PostgreSQL data and pre-conversion code/database backups retained. No demonstration records were added to the user's working database during SQLite validation.

Final snapshot counts: organizations 1, users 1, memberships 1, invitations 0, login tokens 29, sessions 29, auth limits 1, resources 65, projects 37, sprints 26, tasks 24, allocations 47, rates 64, worklogs 23, audit events 661, project activities 89. These include old archived/deleted/demo records, all preserved.

## Limits
Docker daemon is unavailable, so Compose startup itself has not been run. SQLite remains a local-disk deployment with serialized writes, not a shared network-file or multi-server database. Existing demonstration records are preserved rather than removed by the migration.
