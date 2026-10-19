# Resource Compass Constitution
Version: 1.0.0 | Ratified: 2026-10-16

1. Tenant isolation is mandatory. Resolve membership from server-side sessions; scope every business transaction, enforce composite tenant foreign keys and PostgreSQL RLS with a non-superuser runtime role.
2. Planning never overwrites history. Tasks, dated allocations, actual work and effective-dated rates are separate entities. No destructive task switching. One primary owner; any number of contributors.
3. Money and time must be explainable. integer minor units and decimal hours; actual work snapshots applicable rates. Display gaps in pricing and forecast coverage.
4. Authentication is invite-only magic link. Store hashes, expire links in ten minutes, consume once atomically, use HttpOnly cookies, enforce roles and same-origin mutations. Passkeys and Entra SSO are backlog.
5. Test business outcomes with real PostgreSQL and browser flows. A skipped integration suite is not a passing persistence test. Report execution limits explicitly.
6. Keep V1 focused on internal engineering operations. No billing, AI, or Jira sync before trustworthy allocations and cost reporting.
7. Changes to these rules require a dated rationale and corresponding spec/test changes.
