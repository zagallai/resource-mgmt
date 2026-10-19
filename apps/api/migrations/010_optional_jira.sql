ALTER TABLE tasks ALTER COLUMN jira_key DROP NOT NULL;
UPDATE tasks SET jira_key=NULL WHERE trim(jira_key)='';
INSERT INTO schema_version(version) VALUES(10);
