ALTER TABLE projects ADD COLUMN prime_resource_id uuid;
ALTER TABLE projects ADD CONSTRAINT project_prime_resource_tenant FOREIGN KEY(org_id,prime_resource_id) REFERENCES resources(org_id,id);
INSERT INTO schema_version VALUES(3);
