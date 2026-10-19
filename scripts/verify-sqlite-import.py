#!/usr/bin/env python3
"""Compare every exported field against an imported SQLite copy without printing records."""
import datetime,json,sqlite3,sys
if len(sys.argv)!=3: raise SystemExit('Usage: verify-sqlite-import.py EXPORT.json DATABASE.db')
source=json.load(open(sys.argv[1]));db=sqlite3.connect(sys.argv[2]);db.row_factory=sqlite3.Row
scoped={'resources','projects','sprints','tasks','allocations','rates','worklogs','audit_events','project_activities'}
def normalize(row):
 result=dict(row)
 for k,v in result.items():
  if k=='detail' and isinstance(v,str): result[k]=json.loads(v)
  elif k.endswith('_at') and v:
   result[k]=datetime.datetime.fromisoformat(v.replace('Z','+00:00')).astimezone(datetime.timezone.utc).strftime('%Y-%m-%dT%H:%M:%S.%fZ')
 return result
for table,rows in source.items():
 target='base_'+table if table in scoped else table
 actual=[normalize(r) for r in db.execute('SELECT * FROM '+target)]
 expected=[normalize(r) for r in rows]
 key=lambda r:json.dumps(r,sort_keys=True,separators=(',',':'))
 if sorted(map(key,actual))!=sorted(map(key,expected)):
  # Numeric values may change JSON spelling (e.g. 8.00 versus 8), so compare parsed objects.
  identity=lambda r:tuple(str(r.get(k,'')) for k in ('org_id','id','hash','key','user_id'))
  a=sorted(actual,key=identity);e=sorted(expected,key=identity)
  if a!=e: raise SystemExit('Field verification failed: '+table)
 print(table+': '+str(len(rows))+' records verified')
if list(db.execute('PRAGMA foreign_key_check')): raise SystemExit('Foreign key check failed')
if db.execute('PRAGMA integrity_check').fetchone()[0]!='ok': raise SystemExit('Integrity check failed')
print('Every field, relationship and integrity check passed.')
