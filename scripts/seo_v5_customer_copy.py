"""Replace reviewed internal SEO copy markers; default is a read-only plan.

Only eight product identities and the listed Chinese copy paths are writable.
An apply requires the exact manifest SHA256, an explicit backup, and a transaction
that checks every other database value. Never republishes binaries or assets.
"""
import argparse
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import sqlite3

PRODUCTS={60:'telegram-premium-3m',62:'telegram-premium-12m',45:'chatgpt-pro-100-ph',31:'chatgpt-pro-200-ph',33:'chatgpt-pro-500-ph',37:'chatgpt-pro-100-egypt',29:'chatgpt-go-code-egypt',59:'chatgpt-go-manual'}

def allowed(product_id):
    if product_id not in PRODUCTS:return set()
    return {'content_json','updated_at'}|({'description_json','seo_meta_json'} if product_id==29 else set())

def load(path):
    raw=Path(path).read_bytes();data=json.loads(raw)
    assert len(data['products'])==8 and {p['id']:p['slug'] for p in data['products']}==PRODUCTS
    assert sum(len(p['replacements']) for p in data['products'])==13
    for p in data['products']:
        for edit in p['replacements']:
            assert edit['column'] in allowed(p['id'])-{'updated_at'}
            assert edit['path']==(['description','zh-CN'] if edit['column']=='seo_meta_json' else ['zh-CN'])
            assert '需店主确认' in edit['before'] and '店主' not in edit['after']
            assert '购买前' in edit['after'] and '客服' in edit['after']
    return data,hashlib.sha256(raw).hexdigest()

def plan(db,data,now):
    changes=[]
    for product in data['products']:
        row=db.execute('SELECT * FROM products WHERE id=?',(product['id'],)).fetchone()
        assert row is not None and row['slug']==product['slug'] and row['is_active']==1 and row['deleted_at'] is None
        for column in sorted({edit['column'] for edit in product['replacements']}):
            value=json.loads(row[column]);original=json.loads(row[column])
            for edit in (e for e in product['replacements'] if e['column']==column):
                target=value
                for key in edit['path'][:-1]:target=target[key]
                key=edit['path'][-1];text=target[key]
                if edit['before'] in text:
                    assert text.count(edit['before'])==1 and edit['after'] not in text,'Ambiguous copy replacement'
                    target[key]=text.replace(edit['before'],edit['after'])
                else:assert text.count(edit['after'])==1,'Live copy changed; refuse overwrite: '+product['slug']
            assert '需店主确认' not in json.dumps(value,ensure_ascii=False)
            if value!=original:
                encoded=json.dumps(value,ensure_ascii=False,separators=(',',':'))
                if isinstance(row[column],bytes):encoded=encoded.encode('utf-8')
                changes.append(dict(id=row['id'],slug=row['slug'],column=column,before=row[column],after=encoded))
        if any(c['id']==row['id'] for c in changes):changes.append(dict(id=row['id'],slug=row['slug'],column='updated_at',before=row['updated_at'],after=now))
    return changes

def protected(db):
    result={}
    for table, in db.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name"):
        rows=[]
        for record in db.execute('SELECT * FROM "'+table.replace('"','""')+'"'):
            value=dict(record)
            if table=='products':
                for column in allowed(value.get('id')):value.pop(column,None)
            rows.append(json.dumps(value,sort_keys=True,ensure_ascii=True,default=lambda b:{'blob':b.hex()}))
        result[table]={'rows':len(rows),'sha256':hashlib.sha256('\n'.join(sorted(rows)).encode()).hexdigest()}
    return result

def apply(db,data,now):
    try:
        db.execute('BEGIN IMMEDIATE');before=protected(db);changes=plan(db,data,now)
        for c in changes:
            assert c['column'] in allowed(c['id'])
            assert db.execute('UPDATE products SET '+c['column']+'=? WHERE id=? AND '+c['column']+' IS ?',(c['after'],c['id'],c['before'])).rowcount==1
            stored=db.execute('SELECT '+c['column']+' FROM products WHERE id=?',(c['id'],)).fetchone()[0]
            assert type(stored) is type(c['before']),'SQLite storage type changed'
        assert protected(db)==before,'Non-target data changed'
        assert plan(db,data,now)==[],'Not idempotent'
        db.commit()
        return {'status':'passed','changed_fields':len(changes),'products_changed':len({c['id'] for c in changes}),'tables_checked':len(before),'all_other_values_preserved':True,'changes':changes}
    except BaseException:
        db.rollback();raise

def main():
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--database',required=True);p.add_argument('--manifest',required=True);p.add_argument('--apply',action='store_true');p.add_argument('--approved-sha256');p.add_argument('--backup-root');args=p.parse_args()
    data,digest=load(args.manifest);now=datetime.now(timezone.utc).isoformat();database=Path(args.database).resolve(strict=True)
    with sqlite3.connect('file:'+database.as_posix()+'?mode='+('rw' if args.apply else 'ro'),uri=True,timeout=5) as db:
        db.row_factory=sqlite3.Row
        if not args.apply:
            changes=plan(db,data,now);print(json.dumps({'status':'planned','manifest_sha256':digest,'changed_fields':len(changes),'products_changed':len({c['id'] for c in changes})}));return
        assert args.approved_sha256==digest and args.backup_root,'Exact content approval and backup required'
        backup=Path(args.backup_root)/('customer-copy-'+datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S%fZ'));backup.mkdir(parents=True,exist_ok=False,mode=0o700)
        with sqlite3.connect(backup/'before.db') as copy:db.backup(copy)
        report=apply(db,data,now);report.update(manifest_sha256=digest,backup=str(backup),time=now)
        (backup/'report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2,default=lambda value:{'blob':value.hex()}),encoding='utf-8')
        print(json.dumps({k:v for k,v in report.items() if k!='changes'},ensure_ascii=False))

if __name__=='__main__':main()
