# Data model

| Entity | Relationships and invariants |
|---|---|
| Organization | Name, USD currency. Operator creates initially. |
| User | Globally unique normalized email. |
| Membership | (org, user), role; permits multiple organizations. |
| Invitation | Org, email, role, seven-day expiry and acceptance timestamp. |
| Login token | SHA-256 hash, email, ten-minute expiry and used timestamp. |
| Session | SHA-256 hash, user, 24-hour expiry. |
| Resource | (org,id), name and decimal daily capacity (0,24]. |
| Project | (org,id), name, nonnegative budget in cents. |
| Sprint | (org,id), inclusive date range. |
| Task | Project, unique Jira key in org, current estimate, start/target, state, primary owner, optional sprint, optimistic version. |
| Allocation | Task + contributor + day + hours; immutable original row, soft release timestamp/reason. Many contributors allowed. |
| Rate | Resource + effective date unique; nonnegative hourly cents. No edit/delete API. |
| Worklog | Task + contributor + work date + hours; immutable rate snapshot. |
| Audit event | Actor, entity, action, detail JSON, timestamp. No edit/delete API. |

All business primary keys include org_id; foreign keys include it too. Identifiers are PostgreSQL-generated UUIDs. Hours use numeric, not floating-point database columns; money uses bigint cents. API accepts JSON numbers. Day allocations are atomic calendar-day commitments. Worklogs may be recorded while held (late reporting); scheduling a held/done task is prohibited. Owner need not be the only contributor and is not implicitly allocated.

Rates can be inserted retroactively but existing actual snapshots never change. Missing rates block actual-work insertion and visibly count as unpriced forecast entries. Costs round to the nearest cent at the project aggregate. Sprint capacity is full-sprint weekday capacity, not remaining capacity; daily overload includes weekends if someone explicitly schedules work then.
