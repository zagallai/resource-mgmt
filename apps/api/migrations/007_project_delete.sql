ALTER TABLE projects ADD COLUMN deleted_at timestamptz;
INSERT INTO schema_version VALUES(7);
