ALTER TABLE resources ADD COLUMN location_type text CHECK(location_type IN ('offshore','onshore')), ADD COLUMN company_name text NOT NULL DEFAULT '', ADD COLUMN version int NOT NULL DEFAULT 1;
ALTER TABLE projects ADD COLUMN start_date date, ADD COLUMN end_date date, ADD COLUMN pm_name text NOT NULL DEFAULT '', ADD COLUMN version int NOT NULL DEFAULT 1, ADD CONSTRAINT project_dates CHECK((start_date IS NULL AND end_date IS NULL) OR (start_date IS NOT NULL AND end_date IS NOT NULL AND end_date>=start_date));
CREATE TABLE project_activities(org_id uuid NOT NULL,id uuid NOT NULL DEFAULT gen_random_uuid(),project_id uuid NOT NULL,name text NOT NULL CHECK(length(trim(name))>0),start_date date,end_date date,included boolean NOT NULL DEFAULT true,version int NOT NULL DEFAULT 1,PRIMARY KEY(org_id,id),FOREIGN KEY(org_id,project_id) REFERENCES projects(org_id,id),CONSTRAINT activity_dates CHECK((start_date IS NULL AND end_date IS NULL) OR (start_date IS NOT NULL AND end_date IS NOT NULL AND end_date>=start_date)));
CREATE UNIQUE INDEX project_activity_name ON project_activities(org_id,project_id,lower(trim(name)));
ALTER TABLE project_activities ENABLE ROW LEVEL SECURITY;
ALTER TABLE project_activities FORCE ROW LEVEL SECURITY;
CREATE POLICY tenant_scope ON project_activities USING(org_id=nullif(current_setting('app.tenant',true),'')::uuid) WITH CHECK(org_id=nullif(current_setting('app.tenant',true),'')::uuid);
-- Backfill only blank, undated planning rows; preserve all existing projects and work items.
DO $$ DECLARE tenant uuid; BEGIN FOR tenant IN SELECT id FROM organizations LOOP PERFORM set_config('app.tenant',tenant::text,true); INSERT INTO project_activities(org_id,project_id,name) SELECT p.org_id,p.id,n.name FROM projects p CROSS JOIN unnest(ARRAY['LLD','SIT','UAT','RB','RF','PTE']) n(name) WHERE p.org_id=tenant; END LOOP; END $$;
INSERT INTO schema_version VALUES(2);
