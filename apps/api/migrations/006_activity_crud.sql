-- Activity inclusion is no longer a separate state. Preserve every activity and its dates.
ALTER TABLE project_activities DROP COLUMN included;
INSERT INTO schema_version VALUES(6);
