# Resource Management SQLite schema

The live database is `data/resource-compass.db`. Organization-owned physical tables have a `base_` prefix; the API accesses them through organization-filtered queries. Composite `(org_id, id)` keys and foreign keys keep related records in the same organization. This diagram shows the actual table names and principal relationships; `apps/api/sqlite_schema.sql` defines every column, constraint and append-only trigger.

```mermaid
erDiagram
    organizations ||--o{ memberships : has
    users ||--o{ memberships : joins
    organizations ||--o{ invitations : invites
    users ||--o{ sessions : authenticates
    organizations ||--o{ base_resources : owns
    organizations ||--o{ base_projects : owns
    base_resources |o--o{ base_projects : prime_resource
    base_projects |o--o{ base_sprints : plans
    base_projects ||--o{ base_project_activities : contains
    base_projects ||--o{ base_tasks : contains
    base_sprints |o--o{ base_tasks : groups
    base_resources ||--o{ base_tasks : primary_owner
    base_tasks ||--o{ base_allocations : schedules
    base_resources ||--o{ base_allocations : contributes
    base_tasks ||--o{ base_worklogs : records
    base_resources ||--o{ base_worklogs : works
    base_resources ||--o{ base_rates : rate_history
    users ||--o{ base_audit_events : performs

    organizations {
        text id PK
    }
    users {
        text id PK
        string email UK
    }
    memberships {
        text org_id PK,FK
        text user_id PK,FK
        string role
    }
    invitations {
        text id PK
        text org_id FK
        string email
        string expires_at
    }
    sessions {
        string hash PK
        text user_id FK
        string expires_at
    }
    login_tokens {
        string hash PK
        string email
        string expires_at
        string used_at
    }
    auth_limits {
        string key PK
        int hits
        string resets_at
    }
    schema_version {
        int version PK
    }
    base_resources {
        text org_id PK,FK
        text id PK
        string name
        real daily_hours
        string archived_at
        string deleted_at
    }
    base_projects {
        text org_id PK,FK
        text id PK
        text prime_resource_id FK
        string name
        string start_date
        string end_date
    }
    base_sprints {
        text org_id PK,FK
        text id PK
        text project_id FK
        string name
        string start_date
        string end_date
    }
    base_project_activities {
        text org_id PK,FK
        text id PK
        text project_id FK
        string name
        string start_date
        string end_date
    }
    base_tasks {
        text org_id PK,FK
        text id PK
        text project_id FK
        text sprint_id FK
        text primary_owner_id FK
        string jira_key
        string title
        real estimate_hours
        string status
    }
    base_allocations {
        text org_id PK,FK
        text id PK
        text task_id FK
        text resource_id FK
        string day
        real hours
        string released_at
    }
    base_rates {
        text org_id PK,FK
        text id PK
        text resource_id FK
        string effective_from
        int hourly_cents
    }
    base_worklogs {
        text org_id PK,FK
        text id PK
        text task_id FK
        text resource_id FK
        string day
        real hours
        int hourly_cents
    }
    base_audit_events {
        text org_id PK,FK
        text id PK
        text actor_id FK
        text entity_id
        string action
        string detail
    }
```

- Every task belongs to a project; sprint membership is optional. New sprints require a project, while legacy unassigned sprints can be linked through the editor.
- The primary owner is responsible for a task. Multiple resources can contribute through separate dated allocations and worklogs.
- Rates are append-only effective-date records. Worklogs snapshot the applicable rate so later rate changes do not rewrite recorded cost.
- Hold/reassignment/deletion releases future allocations while retaining history. Project/resource removal uses markers; unused project activities can be deleted with audit snapshots.
- Audit entity IDs intentionally refer to different entity types without a single-table foreign key.
- Login tokens refer to email addresses before an invited user necessarily exists; they do not have a user foreign key.
- SQLite schema version 1 is independent of the historical PostgreSQL migration sequence.
