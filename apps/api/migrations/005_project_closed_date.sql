ALTER TABLE projects ADD COLUMN closed_date date;
ALTER TABLE projects ADD CONSTRAINT project_closed_date_order CHECK(closed_date IS NULL OR start_date IS NULL OR closed_date>=start_date);
INSERT INTO schema_version VALUES(5);
