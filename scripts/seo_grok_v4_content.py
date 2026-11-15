"""Apply only the reviewed Chinese Grok copy; never infer product entitlements.

Default: read-only plan. --apply-local accepts a pending draft only in this
checkout's work/ database. Production --apply requires an exact content digest
in an operator approval file. Existing copy changes cause a closed failure.
"""
import argparse
from copy import deepcopy
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import sqlite3

ROOT = Path(__file__).resolve().parents[1]
CATALOG = ROOT / 'docs/catalog/seo-v4/grok-copy.json'
ALLOWED = {66: 'ellaix-grok-plus-monthly', 67: 'ellaix-grok-plus-yearly',
           68: 'ellaix-grok-premium-monthly', 69: 'ellaix-grok-premium-yearly',
           70: 'ellaix-grok-super-monthly', 71: 'ellaix-grok-super-yearly',
           72: 'ellaix-grok-ultra-monthly', 73: 'ellaix-grok-ultra-yearly'}
COPY_COLUMNS = {'description_json', 'content_json', 'seo_meta_json', 'updated_at'}

def require(condition, message):
    if not condition:
        raise ValueError(message)

def load_catalog(path=CATALOG):
    path = Path(path).resolve(strict=True)
    data = json.loads(path.read_text('utf-8'))
    require({p['id']: p['slug'] for p in data['products']} == ALLOWED and len(data['products']) == 8, 'Unexpected product scope')
    digest = hashlib.sha256(path.read_bytes())
    for p in data['products']:
        source = (path.parent / p['file']).resolve(strict=True)
        require(source.parent == path.parent and source.suffix == '.html', 'Content escaped the reviewed folder')
        raw = source.read_bytes();digest.update(raw)
        p['content'] = raw.decode('utf-8').strip()
    return data, digest.hexdigest()

def approved(approval_file, digest):
    require(approval_file is not None, 'Unconfirmed draft: operator approval is required before production publication')
    approval = json.loads(Path(approval_file).read_text('utf-8'))
    require(approval.get('status') == 'approved' and approval.get('copy_sha256') == digest and approval.get('confirmed_by'), 'Approval does not match this exact copy')

def encode(value, old):
    raw = json.dumps(value, ensure_ascii=False, separators=(',', ':'))
    return raw.encode('utf-8') if isinstance(old, bytes) else raw

def localized(old, before, after, context):
    value = json.loads(old or '{}')
    require(isinstance(value, dict), 'Unexpected localized data: ' + context)
    require(value.get('zh-CN', '') in (before, after), 'Newer copy found: ' + context)
    if value.get('zh-CN') == after:
        return old
    value['zh-CN'] = after
    return encode(value, old)

def plan(db, catalog, baseline, now):
    changes = []
    for p in catalog['products']:
        row = db.execute('SELECT * FROM products WHERE id=?', (p['id'],)).fetchone()
        require(row is not None and row['slug'] == p['slug'] and row['category_id'] == 23 and row['is_active'] == 1 and row['deleted_at'] is None, 'Product identity/state changed: ' + p['slug'])
        require(json.loads(row['title_json']).get('zh-CN') == p['title'], 'Product title changed: ' + p['slug'])
        before = baseline[p['slug']]
        values = {}
        for name in ['description', 'content']:
            values[name+'_json'] = localized(row[name+'_json'], before[name], p[name], p['slug']+'/'+name)
        old = row['seo_meta_json']
        seo = json.loads(old or '{}'); updated = deepcopy(seo)
        for field, target in [('title', p['seo_title']), ('description', p['seo_description'])]:
            current = updated.setdefault(field, {})
            require(isinstance(current, dict), 'Unexpected SEO metadata shape')
            expected = before.get('seo_meta', {}).get(field, {}).get('zh-CN', '')
            require(current.get('zh-CN', '') in (expected, target), 'Newer SEO copy found: '+p['slug'])
            current['zh-CN'] = target
        values['seo_meta_json'] = old if seo == updated else encode(updated, old)
        item_changes = [{'id': p['id'], 'column': column, 'before': row[column], 'after': value} for column,value in values.items() if row[column] != value]
        if item_changes:
            item_changes.append({'id':p['id'],'column':'updated_at','before':row['updated_at'],'after':now})
        changes.extend(item_changes)
    return changes

def protected_fingerprints(db):
    result = {}
    for table, in db.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name"):
        records = []
        for row in db.execute('SELECT * FROM "'+table.replace('"','""')+'"'):
            values = dict(row)
            if table == 'products' and values.get('id') in ALLOWED:
                for column in COPY_COLUMNS: values.pop(column, None)
            records.append(json.dumps(values, sort_keys=True, ensure_ascii=True, default=lambda v: {'blob':v.hex()}))
        result[table] = {'rows':len(records),'sha256':hashlib.sha256('\n'.join(sorted(records)).encode()).hexdigest()}
    return result

def apply_transaction(db, catalog, baseline, now):
    try:
        db.execute('BEGIN IMMEDIATE')
        before = protected_fingerprints(db)
        changes = plan(db,catalog,baseline,now)
        for change in changes:
            require(change['column'] in COPY_COLUMNS and change['id'] in ALLOWED, 'Unexpected update scope')
            count = db.execute('UPDATE products SET '+change['column']+'=? WHERE id=? AND '+change['column']+' IS ?', (change['after'],change['id'],change['before'])).rowcount
            require(count == 1, 'Concurrent copy change')
        require(protected_fingerprints(db) == before, 'Non-copy data changed')
        require(plan(db,catalog,baseline,now) == [], 'Content update is not idempotent')
        db.commit()
        return {'changed_fields':len(changes),'products_changed':len({x['id'] for x in changes}),'tables_checked':len(before),'non_copy_fields_preserved':True,'idempotent':True,'changes':changes}
    except BaseException:
        db.rollback();raise

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--database',required=True)
    parser.add_argument('--catalog',default=str(CATALOG))
    parser.add_argument('--approval-file')
    parser.add_argument('--backup-root')
    modes=parser.add_mutually_exclusive_group();modes.add_argument('--apply',action='store_true');modes.add_argument('--apply-local',action='store_true')
    args=parser.parse_args();database=Path(args.database).resolve(strict=True)
    catalog,digest=load_catalog(args.catalog)
    baseline=json.loads((Path(args.catalog).parent/'before.json').read_text('utf-8'))
    if args.apply: approved(args.approval_file,digest)
    if args.apply_local: require(database.is_relative_to(ROOT/'work'), 'Draft application is restricted to this checkout work/ database')
    now=datetime.now(timezone.utc).isoformat()
    with sqlite3.connect('file:'+database.as_posix()+'?mode='+('rw' if args.apply or args.apply_local else 'ro'),uri=True) as db:
        db.row_factory=sqlite3.Row
        if not args.apply and not args.apply_local:
            changes=plan(db,catalog,baseline,now)
            print(json.dumps({'mode':'plan','copy_sha256':digest,'changed_fields':len(changes),'status':catalog['status']},ensure_ascii=False));return
        require(args.backup_root, 'An explicit backup directory is required')
        backup=Path(args.backup_root).resolve()/('grok-v4-'+datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S%fZ'))
        backup.mkdir(parents=True,exist_ok=False)
        with sqlite3.connect(backup/'shop.db') as dst:db.backup(dst)
        result=apply_transaction(db,catalog,baseline,now)
        result.update(copy_sha256=digest,mode='local-draft' if args.apply_local else 'approved-publication',backup=str(backup),time=now)
        (backup/'report.json').write_text(json.dumps(result,ensure_ascii=False,indent=2,default=lambda b:{'blob':b.hex()}),encoding='utf-8')
        print(json.dumps({k:v for k,v in result.items() if k!='changes'},ensure_ascii=False))

if __name__=='__main__':main()
