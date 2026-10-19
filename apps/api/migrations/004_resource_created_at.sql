ALTER TABLE resources ADD COLUMN created_at timestamptz;
-- Recover existing creation dates from the original creation audit event.
-- Unknown legacy dates sort behind known dates instead of appearing newly created.
DO $$ DECLARE tenant uuid; BEGIN FOR tenant IN SELECT id FROM organizations LOOP
PERFORM set_config('app.tenant',tenant::text,true);
UPDATE resources r SET created_at=coalesce((SELECT min(a.created_at) FROM audit_events a WHERE a.org_id=r.org_id AND a.entity_id=r.id AND a.action='resources'),'epoch'::timestamptz) WHERE r.org_id=tenant;
END LOOP; END $$;
ALTER TABLE resources ALTER COLUMN created_at SET NOT NULL, ALTER COLUMN created_at SET DEFAULT clock_timestamp();
CREATE INDEX resource_creation_order ON resources(org_id,created_at DESC,id DESC);
INSERT INTO schema_version VALUES(4);
