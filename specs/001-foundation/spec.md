# 001 — Resource Compass foundation
Status: initial vertical slice implemented; remaining scope tracked in tasks.md
Date: 2026-10-16

## Product boundary
An internal, multi-user engineering planning tool. Next.js/TypeScript frontend, Go REST API, PostgreSQL. A user may belong to many organizations with independent roles. Manual Jira keys first; CSV and live Jira synchronization later. No SaaS billing in V1.

## P1 — Sign in and collaborate
An administrator provisions an organization from the operator command, invites colleagues, and grants admin, manager or viewer access. Invited users request a ten-minute, single-use email link. Confirming it accepts pending unexpired invitations and starts a 24-hour server-side session. No public registration. The same person can switch organizations. Identical request responses for known/unknown emails, IP throttling, secure-cookie default, and same-origin mutations are required.
Acceptance: replayed/expired links fail; uninvited email receives no link; viewer writes fail; unauthorized organization returns 404; logout invalidates the server-side session.

## P1 — Plan at task level
Managers create resources, projects, sprints and tasks. Each task has a Jira key, estimate, start/target dates, one primary owner and an optional sprint. Multiple contributors receive separate allocations by calendar day. A resource can contribute across tasks/projects. Estimates do not determine completion.
Acceptance: two contributors can share a task; an unrelated tenant resource cannot be referenced; dates and positive hours validated; more than daily capacity remains possible with a visible overload warning.

## P1 — Hold and reassign without losing work
A hold, completion or primary-owner reassignment releases all unreleased allocations dated today or later and appends an audit record with actor, time, previous state and reason. Past allocations and actual work remain. Resuming does not resurrect released rows; managers explicitly schedule new commitments. Reassignment releases all future contributor allocations to force intentional replanning. This is a conservative V1 policy, not a Jira synchronization rule.
Acceptance: held/done tasks cannot receive allocations; stale task versions return 409; released allocations and reasons remain visible; actual work remains unchanged.

## P1 — Capacity and money
Daily capacity defaults to eight hours. Sprint capacity uses inclusive weekdays and scheduled allocations in that date range, including allocations for tasks outside that sprint to reflect competing work. Worklogs use the most recent rate effective on work date and snapshot it forever. Forecast prices unreleased allocations today onward at the applicable dated rate. Budgets and all costs are USD cents. Missing rates are shown as unpriced allocations.
Acceptance: 14 consecutive days yield ten workdays; overload is visible; new rate changes forecasts but never old actual costs; logging without a rate fails.

## P2 — Remaining V1
Leave/holidays, partial-day hold timestamps, remaining-sprint capacity, monthly reports, original/revised estimate history, engineer/team-lead/project-scoped permissions, invitations delivered proactively, administrative membership revocation, CSV import, richer resource workload filters, budget/overdue alerts, allocation editing/replacement, production email delivery.

## Non-goals now
Passkeys, Entra SSO, AI recommendations, subscriptions, automated Jira sync.

## Operational assumptions
UTC calendar dates in the slice; organization timezone support later. Whole-day release semantics apply to today. One currency per installation (USD). Manual actual work logging; future-dated actuals rejected. Planned allocations do not automatically decrement when work is logged: managers must hold/complete/replan, and dashboard clearly labels scheduled forecast. Forecast is not a full estimate-to-complete.
