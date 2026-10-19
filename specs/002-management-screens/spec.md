# 002 — Separate Resources and Projects
Date: 2026-10-17

## Resources screen
Directory plus add/edit forms. Resource name, type (onshore/offshore), company name, working hours per day (default 8, >0 and <=24), and hourly rate with effective date. Admins manage rates here; managers manage profiles and working hours; viewers read operational details without rates. Historical dated rates remain append-only and actual-work snapshots remain unchanged. Existing resources retain unknown type/company until filled in, rather than inventing a classification. Profile edits use optimistic versions and audit before/after values.

## Projects screen
Project name, start date, end date, PM name. New project forms require all four. Legacy imported records without dates/PM remain readable and editable. Existing financial budgets are retained, but budget entry is outside this screen’s current scope.

Every project gets its own LLD, SIT, UAT, RB, RF and PTE planning activities with initially blank dates. The abbreviations are retained exactly as supplied. Activities support name changes, start/end dates, omission and restoration. Omission hides the item from the active plan without deleting it or its audit history. Suggested additions: Penetration testing, Security Code Scan, DAST, SCA/SAST; arbitrary names are also allowed. Dates are either both blank or both valid, ordered and within the project window when included. Project dates cannot be shortened past included activities. Project names need not be unique, but activity names are unique within a project ignoring case/outer whitespace.

## Domain boundary
Project planning activities are separate from Jira tasks, resource assignments and sprints. They have no implicit engineering estimate, owner or allocations. Existing Jira work, holds, contributor allocations and actual costs remain intact. Further screen design is deferred until these two workflows are reviewed.

## Migration and validation
Version 002 adds nullable legacy metadata, profile/project versions and tenant-isolated project_activities with composite foreign keys and FORCE RLS. Existing projects get undated defaults. A transactional migration runner serializes upgrades with an advisory lock; re-running it is safe. New installations apply version 001 then 002. Audit, version checks and role controls apply to all edits. Tests cover persistence, default activities, per-project isolation, omit/restore, date validation, stale edits and cross-tenant denial.

## Project prime resource
Each new project requires one prime resource chosen from the organization’s resource directory. It is independent of PM name and Jira task owners. Project details support changing it with existing optimistic version/audit protection. No allocations or costs are created by this designation. Migration 003 adds a nullable tenant-scoped resource reference; existing projects remain intact and show no prime until one is chosen. Legacy metadata edits that omit the field preserve any existing prime.

Resources list newest additions first using immutable creation timestamps; profile edits do not move resources up. Migration 004 recovers legacy creation times from creation audit events. Unrecorded legacy dates sort last.

## Current project workflow — supersedes initial prefilled activity design
Keep Add project. Below it, display a searchable Existing projects table with project name, PM, prime resource, planned start/end, optional closed date and an Edit details action. No project details or activity editor opens until a project is chosen (creating a project selects it). Selected details contain a closed-date field, which can be cleared to reopen the record; it must not precede the project start. Planned end and actual closed date are independent.

Activity editing is hidden until Manage activities is selected. One form creates an activity or edits the activity selected in a dropdown; no separate form per activity and no prefilled rows for new projects. LLD/SIT/UAT/RB/RF/PTE and testing/security names remain suggestions. Activities may be added at project setup or later. Existing activities and omission history remain intact and available from the selector. Switching project clears activity selection. Version 005 only adds closed_date and its date-order constraint; no activity records are deleted.

Project list interaction update: projects use the same expandable directory pattern as Resources. Clicking a project name opens its details and activity controls immediately inside that entry; clicking again collapses it. Only one project is expanded at a time. Add project stays at the top, search remains available, and Manage activities retains its single on-demand form.

Saved-activity visibility: Manage activities now shows a compact table of that project’s saved activity names, dates and included/omitted state. Edit/View loads the selected record into the same single form; the dropdown and custom activity entry remain available. The table and form remain hidden while only project details are being edited.

## Current activity actions — Add, update and delete
The single activity form contains only its name and optional start/end dates. Remove Include in plan, included/omitted labels and the inclusion state. Saved rows have Edit and Delete actions; suggested/custom names remain supported. Deleting requires the current version and the same write permissions as editing. It locks the project/activity, records the deleted activity snapshot in audit history and deletes that row in one transaction. Missing or foreign-tenant activities return 404; stale versions return 409; viewers cannot delete. Deleting the selected activity clears the editor, and deleting the last activity shows an empty list and new-entry form. Migration 006 removes only the inclusion flag and retains all saved activity records/dates.

## Optional activity rows during project creation
Add project includes one blank optional activity row (name, start date, end date). Users can add or remove rows, choose suggested names or enter arbitrary activities. No named/default activity is inserted automatically. Entirely blank rows are skipped; dated rows need a name and both valid dates within the project window. A named activity may remain undated. Project and its entered activities/audit events save in one transaction; invalid rows leave no partial project or activities. The creation form resets to one blank row after success. Existing projects retain their single activity add/update editor and saved-list delete actions.

Project deletion: expanded project details offer Delete project. Only projects without any Jira tasks may be removed. Projects with linked tasks return 409 without modifying project/work/allocations. Allowed removal sets deleted_at, hides project/activities from active screens and records a snapshot in audit; database project/activity records remain. Creating a Jira task locks and checks the active parent so it cannot race deletion. No allocation release or linked-task deletion is performed. Schema 007 only adds the nullable removal timestamp.

Resource Archive/Restore: choose Archive resource within the expanded resource entry. A Show archived resources toggle reveals archived profiles and Restore resource. Actions require manager/admin membership and the current profile version, and append audit history. Existing tasks, allocations, actuals, dated rates and project prime references remain unchanged. Archived resources cannot receive new allocations, task ownership or new project prime assignments; existing prime references remain editable without forcing a reassignment. Historical names and past work entry remain available. Restoring returns the profile to active lists in its original creation order. Schema 008 only adds nullable archived_at; no records are deleted or allocations released.

Unused resource deletion: expanded Resources entries include Delete resource. Allow only resources without task ownership/current or recorded owner history, allocations, worklogs or any project prime reference. Initial hourly rates do not block deletion: profile/rates/audit records are retained with a deleted_at marker, but the resource disappears from both active and archived lists and cannot be restored through Archive/Restore. Block used resources with 409 and direct users to Archive. New assignments/prime references/worklogs lock the resource to prevent racing deletion. Migration 009 adds the nullable marker and changes no existing records automatically.

### Confirmed project deletion override
Projects open a confirmation dialog showing their activity and linked Jira task counts. The user must type the project name; linked tasks additionally require selecting the explicit override checkbox. The backend verifies the project version and, when overriding, the exact name. Linked tasks and activities disappear from active screens through the parent project's reversible deletion marker. Today and future unreleased allocations are released atomically. Worklogs, historical allocations, rates and audit records remain; history retains Jira keys. Deleted-project tasks reject new allocations, worklogs and task changes. External Jira issues are never deleted.

### Central task workflow and effort days
Jira keys are optional; multiple tasks without keys are permitted and selectors/history fall back to the task title. Supplied keys remain unique within the organization. Task estimates and task-card effort totals use standard eight-hour effort days, with quarter-day input increments; each resource's configured daily capacity remains independent. Tasks now contain allocation and optional actual-work forms. Allocation defaults to the primary owner, fixes the task automatically, accepts quarter-day increments and supports additional contributors. Planning retains sprint creation and daily workload. Existing status, reassignment and immutable history behavior is preserved.

Sprints are listed on Planning with the same expandable controls used for resources/projects. Authorized editors can change name/start/end. Backend compares the submitted previous values under a row lock to prevent stale updates, validates dates and records an audit event. Date changes recalculate date-based capacity but do not move tasks or bookings.

### Sprint date guardrail
Creating a task with a selected sprint requires its start and target dates to fall within the sprint, including boundary dates. Browser validation preserves entered fields and displays the allowed range; backend validation independently enforces the rule under a sprint row lock. Editing a sprint cannot exclude any existing task belonging to a non-deleted project. Existing task dates are never moved automatically. Tasks without a sprint remain supported.

### Overview resource availability
Daily workload moved from Planning to Overview. Team availability lists every active resource for a selected date, unreleased booked effort, remaining configured capacity, booking status, booked task labels and open owned-task target dates. Zero bookings are visible. Standard effort-day conversion is eight hours; individual daily capacity is retained. Target dates are displayed as entered rather than inferred early-completion forecasts. Planning manages sprints.

Task creation appears before the task list so a growing list cannot push the form to the bottom. Project costs are hidden from Overview at the user's request; cost history remains available in persistence for future work. This layout-only update was checked with a production build without generating additional workspace fixtures.

### Project-owned sprints
New sprints require an active project. Creation and inline editing display a project selector; sprint summaries show that project. Task sprint choices are filtered by selected project and changing project resets sprint selection. Backend independently rejects cross-project sprint selection and moving a sprint away from existing linked-task projects. Existing sprints are migrated to a project only where all linked tasks identify one unambiguous project; empty/ambiguous sprints retain their data and display Choose project for manual association. Dates must continue to contain linked tasks.

Sprint lists and task sprint choices sort by creation time descending, with newest added first, matching Resources. Historical creation times are recovered from creation audit events where available; editing dates does not change list order. Backend race tests verify creation order independently of sprint start dates.

### Overview task summary
Who’s working on what groups current/upcoming open tasks by resource, including primary owners and additional contributors with unreleased bookings from today onward. Each entry shows task title, optional Jira key, project, explicit status and dates. Multiple tasks appear together. Active/on-hold overdue tasks remain visible rather than being mistaken for finished work. The summary is independent of the date-specific capacity table.
