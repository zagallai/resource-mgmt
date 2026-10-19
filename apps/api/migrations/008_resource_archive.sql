ALTER TABLE resources ADD COLUMN archived_at timestamptz;
INSERT INTO schema_version VALUES(8);
