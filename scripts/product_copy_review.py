"""Apply reviewed Chinese product copy with optimistic checks and full-table audit.

Only title_json, description_json and content_json may change. No network,
purchase, payment, delivery or order actions are performed by this module.
"""
from contextlib import closing
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import sqlite3
import time

FIELDS = ('title', 'description', 'content')

def require(condition, message):
    if not condition:
        raise ValueError(message)

def quote(name):
    return '"' + name.replace('"', '""') + '"'

def snapshot(db):
    tables = [r[0] for r in db.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")]
    return {t: hashlib.sha256(repr([tuple(r) for r in db.execute('SELECT * FROM '+quote(t)+' ORDER BY rowid')]).encode()).hexdigest() for t in tables if t != 'products'}

def plan(db, reviewed):
    require(reviewed and len({r['id'] for r in reviewed}) == len(reviewed), 'Duplicate or missing products')
    changes = []
    for item in reviewed:
        row = db.execute('SELECT * FROM products WHERE id=?', (item['id'],)).fetchone()
        require(row is not None and row['slug'] == item['slug'] and row['deleted_at'] is None, 'Product identity conflict: '+item['slug'])
        actual = 'auto' if row['fulfillment_type'] == 'upstream' else row['fulfillment_type']
        require(actual == item['fulfillment_observed'], 'Delivery mode changed: '+item['slug'])
        for field in FIELDS:
            column = field+'_json'
            old = row[column]
            value = json.loads(old)
            current = value.get('zh-CN', '')
            new = item['proposed'][field]
            require(isinstance(new, str) and new.strip(), 'Empty proposed copy')
            require(current in (item['before'][field], new), 'Newer copy found: '+item['slug']+'/'+field)
            if current != new:
                value['zh-CN'] = new
                encoded = json.dumps(value, ensure_ascii=False, separators=(',', ':'))
                if isinstance(old, bytes):
                    encoded = encoded.encode('utf-8')
                changes.append((item['id'],column,old,encoded))
    return changes

def apply_review(database, reviewed, *, apply=False, backup_root=None):
    path = Path(database).resolve(strict=True)
    with closing(sqlite3.connect(path.as_uri()+('?mode=rw' if apply else '?mode=ro'), uri=True, timeout=8)) as db:
        db.row_factory = sqlite3.Row
        changes = plan(db, reviewed)
        if not apply or not changes:
            return {'status':'planned' if not apply else 'already_applied','planned_fields':len(changes),'changed_fields':0,'product_count':len(reviewed)}
        require(backup_root is not None, 'Backup required')
        backup = Path(backup_root)/('before-product-copy-'+datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S%fZ'))
        backup.mkdir(parents=True,mode=0o700,exist_ok=False)
        deadline = time.monotonic()+25
        def progress(*_):
            if time.monotonic()>deadline:
                raise TimeoutError('Backup deadline exceeded')
        with closing(sqlite3.connect(backup/'shop.db')) as saved:
            db.backup(saved,pages=128,progress=progress,sleep=.05)
            require(saved.execute('PRAGMA quick_check').fetchone()[0]=='ok', 'Backup integrity failed')
        os.chmod(backup/'shop.db',0o600)
        try:
            db.execute('BEGIN IMMEDIATE')
            changes = plan(db, reviewed)
            before_hashes = snapshot(db)
            expected = {r['id']:dict(r) for r in db.execute('SELECT * FROM products ORDER BY id')}
            for pid, column, old, new in changes:
                count = db.execute('UPDATE products SET '+column+'=? WHERE id=? AND '+column+'=?',(new,pid,old)).rowcount
                require(count == 1, 'Optimistic update failed')
                expected[pid][column]=new
            after = {r['id']:dict(r) for r in db.execute('SELECT * FROM products ORDER BY id')}
            require(after == expected,'Unexpected product mutation')
            require(snapshot(db) == before_hashes,'Unexpected protected table mutation')
            require(not plan(db, reviewed),'Incomplete copy update')
            db.commit()
        except BaseException:
            db.rollback()
            raise
        report={'status':'applied','product_count':len(reviewed),'changed_fields':len(changes),'changed_products':sorted({c[0] for c in changes}),'tables_checked':len(before_hashes)+1,'backup':str(backup/'shop.db'),'allowed_fields':list(FIELDS),'all_other_fields_unchanged':True,'protected_table_hashes':before_hashes}
        (backup/'report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2),encoding='utf-8')
        return report
