"""Apply the reviewed first SEO content batch to a local SQLite database only.

No network calls, service changes, publication to production, or purchases.
The transaction preserves every field outside the reviewed Chinese copy and
metadata, and inserts exactly three articles using existing page templates.
"""
import argparse
from contextlib import closing
from copy import deepcopy
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import sqlite3
import time

ROOT = Path(__file__).resolve().parents[1]
CATALOG = ROOT / 'docs/catalog/seo-round1/catalog.json'
PRODUCTS = {'qzweb-5', '86gamestore-88'}
POSTS = {'chatgpt-payment-methods', 'chatgpt-plus-code-guide', 'chatgpt-plus-renewal-rules'}


def require(condition, message):
    if not condition:
        raise ValueError(message)


def load_catalog():
    data = json.loads(CATALOG.read_text(encoding='utf-8'))
    require({p['slug'] for p in data['products']} == PRODUCTS and len(data['products']) == 2, 'Unexpected product scope')
    require({p['slug'] for p in data['posts']} == POSTS and len(data['posts']) == 3, 'Unexpected article scope')
    require(set(data['site_seo']) == {'title', 'description'}, 'Unexpected site metadata scope')
    for item in data['products']:
        require(set(item['fields']) == {'description', 'content', 'seo.title', 'seo.description', 'seo.keywords'}, 'Unexpected product fields')
        for rule in item['fields'].values():
            if 'file' in rule:
                path = (CATALOG.parent / rule['file']).resolve(strict=True)
                require(path.parent == CATALOG.parent, 'Content path escaped catalog')
                rule['value'] = path.read_text(encoding='utf-8').strip()
    for post in data['posts']:
        path = (CATALOG.parent / post['file']).resolve(strict=True)
        require(path.parent == CATALOG.parent, 'Article path escaped catalog')
        post['content'] = path.read_text(encoding='utf-8').strip()
    return data


def encode(value, old=None):
    text = json.dumps(value, ensure_ascii=False, separators=(',', ':'))
    return text.encode('utf-8') if old is None or isinstance(old, bytes) else text


def replace_chinese(value, rule, context):
    require(isinstance(value, dict), 'Expected localized object: ' + context)
    current = value.get('zh-CN', '')
    require(current in rule['before'] + [rule['value']], 'Newer copy found: ' + context)
    require(isinstance(rule['value'], str) and rule['value'].strip(), 'Empty content: ' + context)
    value['zh-CN'] = rule['value']


def plan(db, catalog):
    changes = []
    for item in catalog['products']:
        rows = db.execute('SELECT * FROM products WHERE slug=? AND deleted_at IS NULL', (item['slug'],)).fetchall()
        require(len(rows) == 1, 'Product missing or ambiguous: ' + item['slug'])
        row = rows[0]
        require(row['is_active'] == 1 and row['fulfillment_type'] == item['fulfillment'], 'Product state changed: ' + item['slug'])
        require(json.loads(row['title_json']).get('zh-CN') == item['title'], 'Product identity changed: ' + item['slug'])
        for field in ['description', 'content']:
            column = field + '_json'
            old = row[column]
            value = json.loads(old or '{}')
            before = deepcopy(value)
            replace_chinese(value, item['fields'][field], item['slug'] + '/' + field)
            if value != before:
                changes.append(('products', 'id', row['id'], column, old, encode(value, old)))
        old = row['seo_meta_json']
        value = json.loads(old or '{}')
        before = deepcopy(value)
        for field in ['title', 'description', 'keywords']:
            current = value.setdefault(field, {})
            require(isinstance(current, dict), 'Unexpected SEO metadata shape: ' + item['slug'])
            replace_chinese(current, item['fields']['seo.' + field], item['slug'] + '/seo.' + field)
        if value != before:
            changes.append(('products', 'id', row['id'], 'seo_meta_json', old, encode(value, old)))

    row = db.execute("SELECT * FROM settings WHERE key='site_config'").fetchone()
    require(row is not None, 'Missing site configuration')
    old = row['value_json']
    value = json.loads(old)
    before = deepcopy(value)
    for key, rule in catalog['site_seo'].items():
        replace_chinese(value.setdefault('seo', {}).setdefault(key, {}), rule, 'site.seo.' + key)
    if value != before:
        changes.append(('settings', 'key', 'site_config', 'value_json', old, encode(value, old)))

    inserts = []
    for post in catalog['posts']:
        row = db.execute('SELECT * FROM posts WHERE slug=?', (post['slug'],)).fetchone()
        expected = {key + '_json': {'zh-CN': post[key], 'zh-TW': '', 'en-US': ''} for key in ['title', 'summary', 'content']}
        if row:
            require(row['type'] == 'blog' and row['is_published'] == 1 and row['deleted_at'] is None and row['category_id'] is None and not row['thumbnail'], 'Article slug already in use: ' + post['slug'])
            require(all(json.loads(row[key]) == value for key, value in expected.items()), 'Newer article found: ' + post['slug'])
        else:
            inserts.append({'slug': post['slug'], **expected})
    return changes, inserts


def table_rows(db, table):
    name = '"' + table.replace('"', '""') + '"'
    return [dict(r) for r in db.execute('SELECT * FROM ' + name + ' ORDER BY rowid')]


def apply_transaction(db, catalog):
    try:
        db.execute('BEGIN IMMEDIATE')
        changes, inserts = plan(db, catalog)
        tables = [r[0] for r in db.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")]
        expected = {table: table_rows(db, table) for table in tables}
        for table, pk, key, column, old, new in changes:
            count = db.execute(f'UPDATE {table} SET {column}=? WHERE {pk}=? AND {column} IS ?', (new, key, old)).rowcount
            require(count == 1, 'Optimistic update failed')
            next(row for row in expected[table] if row[pk] == key)[column] = new
        now = datetime.now(timezone.utc).isoformat()
        for item in inserts:
            values = {key: encode(value) if key.endswith('_json') else value for key, value in item.items()}
            values.update(type='blog', thumbnail='', category_id=None, is_published=1, published_at=now, created_at=now, deleted_at=None)
            columns = list(values)
            cursor = db.execute('INSERT INTO posts (' + ','.join(columns) + ') VALUES (' + ','.join('?' for _ in columns) + ')', [values[c] for c in columns])
            expected['posts'].append({'id': cursor.lastrowid, **values})
        for table in tables:
            require(table_rows(db, table) == expected[table], 'Protected data changed: ' + table)
        require(plan(db, catalog) == ([], []), 'Content batch is not idempotent')
        db.commit()
        return {'changed_fields': len(changes), 'created_articles': len(inserts), 'tables_checked': len(tables), 'all_unlisted_fields_preserved': True, 'idempotent': True}
    except BaseException:
        db.rollback()
        raise


def apply_local(database, backup_root, apply=False):
    path = Path(database).resolve(strict=True)
    require(path.is_relative_to(ROOT / 'work'), 'Only a local workspace work/ database is allowed')
    catalog = load_catalog()
    with closing(sqlite3.connect(path.as_uri() + ('?mode=rw' if apply else '?mode=ro'), uri=True, timeout=8)) as db:
        db.row_factory = sqlite3.Row
        changes, inserts = plan(db, catalog)
        if not apply or not (changes or inserts):
            return {'status': 'planned' if not apply else 'already_applied', 'planned_fields': len(changes), 'planned_articles': len(inserts)}
        backup = Path(backup_root).resolve() / ('before-seo-round1-' + datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S%fZ'))
        require(backup.is_relative_to(ROOT / 'work'), 'Backup must stay inside workspace work/')
        backup.mkdir(parents=True, exist_ok=False)
        deadline = time.monotonic() + 25
        def progress(*_):
            if time.monotonic() > deadline:
                raise TimeoutError('SQLite backup deadline exceeded')
        with closing(sqlite3.connect(backup / 'shop.db')) as saved:
            db.backup(saved, pages=128, progress=progress, sleep=.05)
            require(saved.execute('PRAGMA quick_check').fetchone()[0] == 'ok', 'Backup integrity check failed')
            with closing(sqlite3.connect(':memory:')) as rehearsal:
                saved.backup(rehearsal)
                rehearsal.row_factory = sqlite3.Row
                apply_transaction(rehearsal, catalog)
                require(apply_transaction(rehearsal, catalog)['changed_fields'] == 0, 'Rehearsal repeated update')
        result = apply_transaction(db, catalog)
        result.update(status='applied_local', database=str(path), backup=str(backup / 'shop.db'), manifest_sha256=hashlib.sha256(CATALOG.read_bytes()).hexdigest())
        (backup / 'report.json').write_text(json.dumps(result, ensure_ascii=False, indent=2), encoding='utf-8')
        return result


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--database', default=str(ROOT / 'work/codex-runtime/live/shop.db'))
    parser.add_argument('--backup-root', default=str(ROOT / 'work/codex-logs/seo-round1-20261109'))
    parser.add_argument('--apply-local', action='store_true')
    args = parser.parse_args()
    print(json.dumps(apply_local(args.database, args.backup_root, args.apply_local), ensure_ascii=False, indent=2))
