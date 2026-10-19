ALTER TABLE sprints ADD COLUMN project_id uuid;
ALTER TABLE sprints ADD CONSTRAINT sprints_project_fk FOREIGN KEY(org_id,project_id) REFERENCES projects(org_id,id);
UPDATE sprints s SET project_id=x.project_id FROM (SELECT org_id,sprint_id,(array_agg(DISTINCT project_id))[1] project_id FROM tasks WHERE sprint_id IS NOT NULL GROUP BY org_id,sprint_id HAVING count(DISTINCT project_id)=1) x WHERE s.org_id=x.org_id AND s.id=x.sprint_id;
INSERT INTO schema_version(version) VALUES(11);
