# Plan and implementation

- [x] Add version 002 without rewriting or deleting version 001 data.
- [x] Add a transactional, serialized, idempotent migration runner.
- [x] Resource creation atomically saves profile and initial dated rate; editing saves profile and capacity with version protection.
- [x] Project creation atomically saves metadata plus six default planning activities.
- [x] Separate project_activities from Jira tasks; validate dates against a locked project row; audit edits and omission/restoration.
- [x] Dedicated resource/project components and distinct navigation entries; move rate management from Administration to Resources.
- [x] Preserve prior broad roles and financial privacy. Preserve existing allocations, worklogs, rates and budgets.
- [x] PostgreSQL integration coverage, frontend build and real-browser persistence workflow.

Data additions: resources.location_type/company_name/version; projects.start_date/end_date/pm_name/version; project_activities with org/id/project/name/start/end/included/version. No third-party service dependency or external data source is introduced.
