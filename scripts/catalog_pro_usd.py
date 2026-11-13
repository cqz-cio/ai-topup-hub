"""Rename the eight reviewed GPT Pro products and six covers, without repricing.

The reviewed before/after file is required. All other product columns and every
other business table are audited in the same write transaction. Historical
order/delivery snapshots and supplier identifiers are never rewritten.
"""
from contextlib import closing
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import re
import sqlite3
import time

PRODUCTS = {31: 'qzweb-55', 32: 'qzweb-7', 33: 'qzweb-56', 35: 'qzweb-58',
            37: 'qzweb-57', 45: 'qzweb-6', 54: '86gamestore-82', 55: '86gamestore-83'}
COVERS = {31: '200-ph', 32: '200-renew-ph', 33: '500-ph', 35: '500-eg',
          37: '100-eg', 45: '100-ph'}


def rename(value):
    return re.sub(r'(?i)(Pro\s*)(5|10|25)x\b',
                  lambda m: m[1] + {'5': '100', '10': '200', '25': '500'}[m[2]] + '美金', value)


def require(condition, message):
    if not condition:
        raise ValueError(message)


def protected_snapshot(db):
    result = {}
    for name, in db.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' AND name != 'products'"):
        quoted = '"' + name.replace('"', '""') + '"'
        result[name] = hashlib.sha256(repr([tuple(r) for r in db.execute('SELECT * FROM '+quoted+' ORDER BY rowid')]).encode()).hexdigest()
    return result


def plan(db, reviewed):
    require(len(reviewed) == len(PRODUCTS) and {x['id']: x['slug'] for x in reviewed} == PRODUCTS, 'Unexpected product scope')
    changes = []
    for item in reviewed:
        row = db.execute('SELECT * FROM products WHERE id=?', (item['id'],)).fetchone()
        require(row is not None and row['slug'] == item['slug'] and row['deleted_at'] is None and row['is_active'] == 1, 'Product identity/status changed')
        expected_fields = {'title_json', 'images'} if item['id'] in COVERS else {'title_json'}
        before, after = item['before'], item['after']
        require(set(before) == set(after) == expected_fields, 'Unexpected fields')
        require(after['title_json'] == {k: rename(v) for k, v in before['title_json'].items()}, 'Title is not the approved rename')
        require(after['title_json'] != before['title_json'], 'Missing tier label')
        if 'images' in after:
            require(after['images'] == ['/catalog-products/pro-usd-20261111/pro-'+COVERS[item['id']]+'.svg', *before['images'][1:]], 'Unexpected cover')
        for field in sorted(expected_fields):
            raw = row[field]
            current = json.loads(raw)
            require(current in (before[field], after[field]), 'Concurrent edit: '+item['slug']+'/'+field)
            if current != after[field]:
                encoded = json.dumps(after[field], ensure_ascii=False, separators=(',', ':'))
                changes.append((item['id'], field, raw, encoded.encode('utf-8') if isinstance(raw, bytes) else encoded))
    return changes


def apply_transaction(db, reviewed):
    try:
        db.execute('BEGIN IMMEDIATE')
        changes = plan(db, reviewed)
        protected = protected_snapshot(db)
        expected = {r['id']: dict(r) for r in db.execute('SELECT * FROM products')}
        for pid, field, old, new in changes:
            require(db.execute('UPDATE products SET '+field+'=? WHERE id=? AND '+field+'=?', (new, pid, old)).rowcount == 1, 'Concurrent update')
            expected[pid][field] = new
        require({r['id']: dict(r) for r in db.execute('SELECT * FROM products')} == expected, 'Unexpected product mutation')
        require(protected_snapshot(db) == protected, 'Protected table changed')
        require(not plan(db, reviewed), 'Incomplete update')
        db.commit()
        return {'changed_fields': len(changes), 'changed_products': sorted({x[0] for x in changes}), 'tables_checked': len(protected)+1, 'all_other_fields_preserved': True}
    except BaseException:
        db.rollback()
        raise


def apply_review(database, reviewed, backup_dir):
    path = Path(database).resolve(strict=True)
    backup = Path(backup_dir)
    backup.mkdir(mode=0o700, parents=True, exist_ok=False)
    with closing(sqlite3.connect(path.as_uri()+'?mode=rw', uri=True, timeout=5)) as db:
        db.row_factory = sqlite3.Row
        plan(db, reviewed)
        deadline = time.monotonic()+25
        def progress(*_):
            if time.monotonic() > deadline:
                raise TimeoutError('Database backup exceeded 25 seconds')
        with closing(sqlite3.connect(backup/'shop.db')) as saved:
            db.backup(saved, pages=128, progress=progress, sleep=.05)
            require(saved.execute('PRAGMA quick_check').fetchone()[0] == 'ok', 'Invalid backup')
            with closing(sqlite3.connect(':memory:')) as rehearsal:
                saved.backup(rehearsal)
                rehearsal.row_factory = sqlite3.Row
                simulated = apply_transaction(rehearsal, reviewed)
                require(apply_transaction(rehearsal, reviewed)['changed_fields'] == 0, 'Repeated application changed data')
        (backup/'shop.db').chmod(0o600)
        (backup/'reviewed.json').write_text(json.dumps(reviewed, ensure_ascii=False, indent=2), encoding='utf-8')
        result = apply_transaction(db, reviewed)
        result.update(backup=str(backup), rehearsal=simulated, completed_at=datetime.now(timezone.utc).isoformat())
        (backup/'report.json').write_text(json.dumps(result, indent=2), encoding='utf-8')
        return result


if __name__ == '__main__':
    import argparse
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--database', required=True)
    parser.add_argument('--reviewed', required=True)
    parser.add_argument('--apply', action='store_true')
    parser.add_argument('--backup-dir')
    args = parser.parse_args()
    reviewed = json.loads(Path(args.reviewed).read_text(encoding='utf-8'))
    if args.apply:
        parser.error('--backup-dir is required with --apply') if not args.backup_dir else None
        result = apply_review(args.database, reviewed, args.backup_dir)
    else:
        with closing(sqlite3.connect(Path(args.database).resolve(strict=True).as_uri()+'?mode=ro', uri=True, timeout=5)) as db:
            db.row_factory = sqlite3.Row
            result = {'planned_fields': len(plan(db, reviewed)), 'changed_fields': 0}
    print(json.dumps(result, ensure_ascii=False))
