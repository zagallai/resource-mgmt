ALTER TABLE sprints ADD COLUMN created_at timestamptz NOT NULL DEFAULT now();
UPDATE sprints s SET created_at=x.created_at FROM (SELECT org_id,entity_id,min(created_at) created_at FROM audit_events WHERE action='sprints' GROUP BY org_id,entity_id) x WHERE s.org_id=x.org_id AND s.id=x.entity_id;
INSERT INTO schema_version(version) VALUES(12);
