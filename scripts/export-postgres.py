#!/usr/bin/env python3
"""Export one consistent PostgreSQL snapshot for the SQLite importer.
Run with POSTGRES_SOURCE_URL and an output path; never commit the export.
"""
import json, os, pathlib, subprocess, sys, urllib.parse
TABLES = ['organizations','users','memberships','invitations','login_tokens','sessions','auth_limits','resources','projects','sprints','tasks','allocations','rates','worklogs','audit_events','project_activities']
if len(sys.argv) != 2 or not os.environ.get('POSTGRES_SOURCE_URL'):
    raise SystemExit('Usage: POSTGRES_SOURCE_URL=... python3 export-postgres.py OUTPUT.json')
p = pathlib.Path(sys.argv[1])
query = 'SELECT json_build_object(' + ','.join("'%s',(SELECT coalesce(json_agg(row_to_json(x)), '[]') FROM %s x)" % (t,t) for t in TABLES) + ')'
# Passing the source URL through the environment avoids exposing credentials in command arguments.
source = urllib.parse.urlparse(os.environ['POSTGRES_SOURCE_URL'])
if source.scheme not in ('postgres','postgresql'): raise SystemExit('PostgreSQL source URL required')
env = dict(os.environ, PGDATABASE=source.path.lstrip('/'), PGHOST=source.hostname or 'localhost', PGPORT=str(source.port or 5432), PGUSER=urllib.parse.unquote(source.username or ''))
if source.password: env['PGPASSWORD']=urllib.parse.unquote(source.password)
params=urllib.parse.parse_qs(source.query)
if 'sslmode' in params: env['PGSSLMODE']=params['sslmode'][0]
result = subprocess.run([os.environ.get('PSQL','psql'), '-X', '-At', '-v', 'ON_ERROR_STOP=1', '-c', query], env=env, stdout=subprocess.PIPE, stderr=subprocess.PIPE, check=True)
data = json.loads(result.stdout)
with open(p, 'x', opener=lambda path, flags: os.open(path, flags, 0o600)) as f:
    json.dump(data,f,separators=(',',':'))
print('Exported snapshot:', ', '.join('%s=%s' % (t,len(data[t])) for t in TABLES))
