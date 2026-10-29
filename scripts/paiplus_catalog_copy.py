"""Apply reviewed PAIPLUS catalog copy without changing historical deliveries.

This maintenance helper only updates the nine products already enabled in the
PAIPLUS adapter. Each field uses an exact before/after check. A SQLite snapshot
and a complete table audit are required before committing production changes.
"""
from contextlib import closing
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import sqlite3
import time

PRODUCTS = {29: 'qzweb-4', 30: 'qzweb-5', 31: 'qzweb-55', 32: 'qzweb-7',
            33: 'qzweb-56', 35: 'qzweb-58', 37: 'qzweb-57', 45: 'qzweb-6',
            65: 'ellaix-gpt-plus-account-30d'}
FIELDS = ('description_json', 'content_json', 'instructions_json')
ORIGIN = 'https://cdk.qzweb.cloud'


def require(condition, message):
    if not condition:
        raise ValueError(message)


def quote(name):
    return '"' + name.replace('"', '""') + '"'


def snapshot(db):
    tables = [r[0] for r in db.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")]
    return {t: hashlib.sha256(repr([tuple(r) for r in db.execute('SELECT * FROM '+quote(t)+' ORDER BY rowid')]).encode()).hexdigest()
            for t in tables if t != 'products'}


def plan(db, reviewed):
    require({r['id']: r['slug'] for r in reviewed} == PRODUCTS and len(reviewed) == len(PRODUCTS), 'Unexpected product scope')
    changes = []
    for item in reviewed:
        row = db.execute('SELECT * FROM products WHERE id=?', (item['id'],)).fetchone()
        require(row is not None and row['slug'] == item['slug'] and row['deleted_at'] is None,
                'Product identity conflict: '+item['slug'])
        require(row['is_active'] == 1 and row['fulfillment_type'] == 'upstream', 'Delivery scope changed')
        require(set(item['before']) == set(FIELDS) and set(item['after']) == set(FIELDS), 'Unexpected fields')
        for field in FIELDS:
            old = row[field]
            current = json.loads(old)
            before, after = item['before'][field], item['after'][field]
            require(current in (before, after), 'Newer copy found: '+item['slug']+'/'+field)
            require(set(before) == set(after) and 'zh-CN' in after, 'Locale set changed')
            require(all(before[k] == after[k] for k in before if k != 'zh-CN'), 'Other locale changed')
            require(isinstance(after['zh-CN'], str) and after['zh-CN'].strip(), 'Empty proposed copy')
            if field == 'instructions_json':
                require(ORIGIN+'/' in after['zh-CN'] and 'PAIPLUS' in after['zh-CN'], 'Missing local redemption guide')
                require('cdk.bar' not in json.dumps(after), 'Supplier URL retained')
            else:
                require('cdk.bar' not in json.dumps(after) and 'cdk.qzweb.cloud' not in json.dumps(after), 'Public redemption URL introduced')
                require('PAIPLUS' in after['zh-CN'], 'Missing local code description')
            if current != after:
                encoded = json.dumps(after, ensure_ascii=False, separators=(',', ':'))
                if isinstance(old, bytes):
                    encoded = encoded.encode('utf-8')
                changes.append((item['id'], field, old, encoded))
    return changes


def apply_transaction(db, reviewed):
    try:
        db.execute('BEGIN IMMEDIATE')
        changes = plan(db, reviewed)
        before = snapshot(db)
        expected = {r['id']: dict(r) for r in db.execute('SELECT * FROM products ORDER BY id')}
        for pid, field, old, new in changes:
            count = db.execute('UPDATE products SET '+field+'=? WHERE id=? AND '+field+'=?', (new, pid, old)).rowcount
            require(count == 1, 'Optimistic update failed')
            expected[pid][field] = new
        require({r['id']: dict(r) for r in db.execute('SELECT * FROM products ORDER BY id')} == expected, 'Unexpected product mutation')
        require(snapshot(db) == before, 'Protected table changed')
        require(not plan(db, reviewed), 'Incomplete or non-idempotent update')
        db.commit()
    except BaseException:
        db.rollback()
        raise
    return {'changed_fields': len(changes), 'changed_products': sorted({c[0] for c in changes}),
            'tables_checked': len(before)+1, 'other_product_fields_preserved': True,
            'protected_tables_preserved': True, 'idempotent': True}


def apply_review(database, reviewed, *, apply=False, backup_root=None):
    path = Path(database).resolve(strict=True)
    with closing(sqlite3.connect(path.as_uri()+('?mode=rw' if apply else '?mode=ro'), uri=True, timeout=8)) as db:
        db.row_factory = sqlite3.Row
        changes = plan(db, reviewed)
        if not apply or not changes:
            return {'status': 'planned' if not apply else 'already_applied', 'planned_fields': len(changes), 'changed_fields': 0}
        require(backup_root is not None, 'Backup required')
        backup = Path(backup_root)/('before-paiplus-copy-'+datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S%fZ'))
        backup.mkdir(parents=True, mode=0o700, exist_ok=False)
        deadline = time.monotonic()+25
        def progress(*_):
            if time.monotonic() > deadline:
                raise TimeoutError('Backup deadline exceeded')
        with closing(sqlite3.connect(backup/'shop.db')) as saved:
            db.backup(saved, pages=128, progress=progress, sleep=.05)
            require(saved.execute('PRAGMA quick_check').fetchone()[0] == 'ok', 'Backup integrity failed')
            with closing(sqlite3.connect(':memory:')) as rehearsal:
                saved.backup(rehearsal)
                rehearsal.row_factory = sqlite3.Row
                copy_result = apply_transaction(rehearsal, reviewed)
                require(apply_transaction(rehearsal, reviewed)['changed_fields'] == 0, 'Second application changed data')
        os.chmod(backup/'shop.db', 0o600)
        result = apply_transaction(db, reviewed)
        result.update(status='applied', backup=str(backup/'shop.db'), rehearsal=copy_result)
        (backup/'reviewed-copy.json').write_text(json.dumps(reviewed, ensure_ascii=False, indent=2), encoding='utf-8')
        (backup/'report.json').write_text(json.dumps(result, ensure_ascii=False, indent=2), encoding='utf-8')
        return result
