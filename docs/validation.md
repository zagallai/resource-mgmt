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

This is an initial vertical slice, not the complete V1. Leave/holidays, remaining sprint capacity, monthly cost reports, revised estimate history, fine-grained permissions, CSV/Jira import/sync, membership revocation, production mail delivery and operational hardening remain in specs/001-foundation/tasks.md. Current assumptions: currency-neutral amounts, UTC calendar days, three broad roles, day-level release semantics and scheduled-commitment forecasts.

## 2026-10-17 — Working hours and amount-entry update

Production frontend build and Go vet passed. PostgreSQL race-enabled integration suite passed with an additional default/configurable working-hours scenario: omitted hours default to 8, editing to 6 recalculates daily/full-sprint capacity, invalid hours fail, and changes preserve an audit record. Browser workflow passed with default 8-hour entry, editing to 6 and reloading to verify persistence, entering rates/budgets in normal units, and currency-neutral cost output. Local frontend and API restarted from the project directory; existing resource, allocation, rate and worklog data retained.

## 2026-10-17 — Separate resource and project management screens

- Frontend production build and strict TypeScript compilation passed; Go vet and race-enabled PostgreSQL regression tests passed.
- New database coverage includes resource type/company/initial rate, profile edits with versions, six default project activities, project metadata edits, omit/restore, custom activities, per-project separation, date-window validation, stale project/activity edits, duplicate names and cross-tenant references. Viewer reads do not expose hourly rates.
- Real-browser workflow passed: create onshore/offshore resources with company and hourly rate, change hours/reload, create project with PM/dates, schedule LLD, omit/restore PTE, add penetration testing, reload to verify persistence, then exercise the existing Jira/allocation/cost/hold/reassignment flow.
- Schema version 002 was applied after a local backup. Before and immediately after the upgrade, existing counts were identical: 10 resources, 5 projects, 4 Jira tasks and 4 worklogs. Migration reruns are covered for idempotence. Old projects receive undated planning activities; no existing work records are removed.
- Desktop screenshots of Resources and Projects are in resources.png and projects.png. These contain local browser-test fixtures.

## 2026-10-17 — Project prime resource

Migration 003 adds a composite tenant-scoped resource reference to projects; existing projects stay unassigned until selected. New project creation requires a prime, edits preserve it when omitted, and changes retain optimistic-version/audit protection. PostgreSQL integration tests cover creation, persistence, reassignment, missing selection and cross-tenant denial. Frontend production build, TypeScript compilation and Go vet passed. Project prime designation does not create allocations or change Jira task owners.

Resource ordering update: schema 004 adds creation timestamps recovered from creation audit events for existing resources. PostgreSQL tests passed for newest-first ordering and stable order after editing an older resource. Existing resources and history retained.

## 2026-10-17 — Project page organization and closed date

Production frontend build, strict TypeScript compilation, Go vet and the PostgreSQL race-enabled suite passed. New coverage verifies projects start without automatic activities, optional closed-date persistence, preservation when omitted, clearing and rejection of dates before project start. Existing activity create/update/omit/restore tests continue to pass.

The browser workflow passed (6.7 seconds including runner startup): create project, select it from Existing projects, save/reload closed date, keep activities hidden while editing details, open one shared activity form, add and update activities, omit/restore an existing activity and confirm saved dates after reloading. Prime-resource editing and the existing allocation/cost/hold flow also passed. Reviewed docs/projects.png for layout.

Migration 005 was applied after a local backup. It only adds a nullable closed date and validation constraint. No existing project, activity, rate, allocation or worklog is deleted.

Inline project expansion: production build and browser workflow passed (7.1 seconds including runner startup). Browser assertions cover expanding/collapsing the project name, verifying the edit form is inside the expanded entry, saving and reloading closed date/prime resource, and the existing single activity form and allocation workflows. No database migration was needed.

Activity Add/Update/Delete: production build, TypeScript, Go vet and race-enabled PostgreSQL tests passed. Tests cover stale deletion, tenant isolation, viewer denial, a deletion audit snapshot, repeated deletion and reusing a deleted activity name. Browser workflow passed (7.5 seconds including runner startup), including editing, deleting the selected activity, deleting the final activity, resetting the form and updating the activity count. Include in plan is absent. Before and after migration 006, 67 existing activity records remained; only the obsolete flag was removed.

Initial activity rows at project creation: production build, TypeScript, Go vet and PostgreSQL race-enabled tests passed. Tests cover multiple/custom activities, undated names, skipping blank rows, duplicate/out-of-range/incomplete entries and no partial saves on failure. Browser workflow passed (8.1 seconds including runner startup), entering two initial activities, removing an extra draft row, verifying the saved names/dates across reloads and exercising existing activity update/delete and allocation flows. No migration or automatic activity defaults were introduced.

Guarded project deletion: production build, TypeScript, Go vet and race-enabled PostgreSQL tests passed. Deletion is permitted only without Jira tasks; records are retained with deleted_at and an audit snapshot. Tests verify stale/repeated/foreign/viewer deletion, hiding project-only activities, rejecting additions after deletion and unchanged allocations/worklogs when a linked project deletion is blocked. Browser workflow passed (8.7 seconds including runner startup), checking blocked linked-work deletion and successful removal of a test project containing only a planning activity. Broader deletion that releases allocations was rejected by automatic approval review and is not implemented. No linked-work or capacity changes are performed by this feature.

Resource Archive/Restore: production build, TypeScript, Go vet and PostgreSQL race-enabled tests passed. Tests verify stale versions, viewer/cross-tenant denial, active/archived lists, new-assignment denial, keeping existing archived prime references, restoration and byte-for-byte unchanged allocations/worklogs/rates. Browser workflow passed (10.0 seconds including runner startup), including archive, reload, archived-list discovery, exclusion from project prime choices, historical name visibility and restore. Migration 008 adds only archived_at; it does not archive any records automatically or change existing work/allocations.

Unused resource deletion: production build, TypeScript, Go vet and PostgreSQL race-enabled tests passed. Covered stale/cross-tenant/viewer denial, deletion with an initial hourly rate, archived-unused deletion, active/archived list removal, blocked restore/reassignment after deletion, audit retention and unchanged linked work. Browser workflow passed (11.2 seconds including runner startup): newly added unused resource disappears from both directories across reloads, while deleting an assigned resource is blocked with Archive guidance. Schema 009 adds only deleted_at; no resources or financial/work records are automatically removed.

### Project deletion confirmation and override
Validated frontend production build, backend race tests and go vet. Added isolated linked-project deletion tests covering incorrect confirmation, released future allocations, retained worklogs and allocation labels, hidden tasks and rejected new work on deleted projects. Browser workflow covers confirmation, disabled destructive action and cancellation, plus confirmed deletion of an unused project.

Browser confirmation/cancel and confirmed unused-project deletion workflow passed (10.2 seconds). Backend race tests include the linked-task override; no user project was deleted during these checks.

### Central task workflow
Frontend production build and browser end-to-end workflow passed (12.0 seconds), including owner-defaulted inline allocation, two contributors, day-to-hour conversion, actual work, hold/resume and reassignment. Optional-Jira-key migration applied after local database backup; backend race tests passed with UTC database timezone, including multiple tasks without keys. An earlier test run with the host database timezone exposed an existing UTC/local-date boundary issue in the effective-rate assertion; that timezone inconsistency is not resolved by this UI change.

Sprint-boundary backend race tests and frontend production build passed. Tests reject tasks before/after sprint boundaries, accept matching boundary dates and block sprint shrinkage excluding linked tasks.

Project-owned sprint migration applied after local database backup. Frontend production build and backend race tests passed, including rejection of missing-project sprint creation, cross-project task/sprint selection and moving an occupied sprint to another project. Validation used disposable backend databases; no additional workspace demonstration records were created for this change. Browser workflow source was updated for the new Project selector; full live workflow was not rerun to avoid adding more test data to the user's workspace.

## SQLite runtime conversion
The active local runtime now uses project-local SQLite. See validation-sqlite.md for complete migration, field verification, race tests, backup/restore and isolated browser evidence. Historical PostgreSQL results above remain as prior implementation evidence, not the current runtime configuration.

Administrator login email updated to admin@resourcemgmt.com; user ID, memberships and existing sessions retained. Old-email magic links invalidated.

## Fresh-machine preparation
A clean export of the staged Git index to an isolated directory was validated without the working repository's node_modules, build output, settings or database. npm ci installed 34 packages with zero reported vulnerabilities; the frontend production build passed. Backend race tests, a CGO-disabled API binary build, fresh administrator bootstrap and backup into a previously missing directory passed. The fresh database and its backup both passed integrity checks and retained the new test workspace. Shell syntax and Compose configuration passed; Docker container startup remains unverified because the local daemon is stopped. This is clean-checkout evidence on this Mac, not a claim of testing a second physical machine. No test records were added to the user's working database.
