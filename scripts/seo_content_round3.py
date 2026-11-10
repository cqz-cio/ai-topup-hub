"""Review/apply the third SEO batch to an isolated work/ SQLite database only."""
import argparse
from contextlib import closing
from copy import deepcopy
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import sqlite3
import time

from seo_content_round1 import encode, require, table_rows

ROOT = Path(__file__).resolve().parents[1]
CATALOG = ROOT / 'docs/catalog/seo-round3/catalog.json'
SLUGS = {'chatgpt-membership-not-active', 'chatgpt-plus-vs-api-balance', 'chatgpt-code-region-check'}


def load_catalog():
    data = json.loads(CATALOG.read_text('utf-8'))
    require(len(data['posts']) == 3 and {p['slug'] for p in data['posts']} == SLUGS, 'Unexpected article scope')
    require({(r['section'], r['index']) for r in data['about']} == {('services', 2), ('recharge', 2)} and len(data['about']) == 2, 'Unexpected about scope')
    for post in data['posts']:
        path = (CATALOG.parent / post['file']).resolve(strict=True)
        require(path.parent == CATALOG.parent, 'Article path escaped catalog')
        post['content'] = path.read_text('utf-8').strip()
    return data


def plan(db, catalog):
    row = db.execute("SELECT * FROM settings WHERE key='about_page_config'").fetchone()
    require(row is not None, 'Missing saved about page; review before migration')
    old = row['value_json']; value = json.loads(old); before = deepcopy(value)
    require(value.get('version') == 1, 'Unexpected about page version')
    for rule in catalog['about']:
        item = value[rule['section']][rule['index']]
        require(item['title'].get('zh-CN') == rule['title'], 'About section identity changed')
        require(item['text'].get('zh-CN') in rule['before'] + [rule['value']], 'Newer about copy found')
        item['text']['zh-CN'] = rule['value']
    changes = [] if value == before else [(row['key'], old, encode(value, old))]
    inserts = []
    for post in catalog['posts']:
        expected = {key + '_json': {'zh-CN': post[key], 'zh-TW': '', 'en-US': ''} for key in ['title', 'summary', 'content']}
        row = db.execute('SELECT * FROM posts WHERE slug=?', (post['slug'],)).fetchone()
        if row:
            require(row['type'] == 'blog' and row['is_published'] == 1 and row['deleted_at'] is None and row['published_at'] and row['category_id'] is None and not row['thumbnail'], 'Article slug already in use')
            require(all(json.loads(row[key]) == value for key, value in expected.items()), 'Newer article found: ' + post['slug'])
        else:
            inserts.append({'slug': post['slug'], **expected})
    return changes, inserts


def apply_transaction(db, catalog):
    try:
        db.execute('BEGIN IMMEDIATE')
        changes, inserts = plan(db, catalog)
        tables = [row[0] for row in db.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")]
        expected = {table: table_rows(db, table) for table in tables}
        for key, old, value in changes:
            require(db.execute('UPDATE settings SET value_json=? WHERE key=? AND value_json IS ?', (value, key, old)).rowcount == 1, 'Concurrent about change')
            next(row for row in expected['settings'] if row['key'] == key)['value_json'] = value
        now = datetime.now(timezone.utc).isoformat()
        for item in inserts:
            values = {k: encode(v) if k.endswith('_json') else v for k, v in item.items()}
            values.update(type='blog', thumbnail='', category_id=None, is_published=1, published_at=now, created_at=now, deleted_at=None)
            columns = list(values)
            cursor = db.execute('INSERT INTO posts (' + ','.join(columns) + ') VALUES (' + ','.join('?' for _ in columns) + ')', list(values.values()))
            expected['posts'].append({'id': cursor.lastrowid, **values})
        for table in tables:
            require(table_rows(db, table) == expected[table], 'Protected data changed: ' + table)
        require(plan(db, catalog) == ([], []), 'Non-idempotent content batch')
        db.commit()
        return {'changed_about_records': len(changes), 'created_articles': len(inserts), 'tables_checked': len(tables), 'unlisted_fields_preserved': True}
    except BaseException:
        db.rollback()
        raise


def apply_local(database, apply=False):
    path = Path(database).resolve(strict=True)
    require(path.is_relative_to(ROOT / 'work'), 'Only a workspace work/ database is allowed')
    catalog = load_catalog()
    with closing(sqlite3.connect(path.as_uri() + ('?mode=rw' if apply else '?mode=ro'), uri=True, timeout=8)) as db:
        db.row_factory = sqlite3.Row
        changes, inserts = plan(db, catalog)
        if not apply or not (changes or inserts):
            return {'status': 'planned' if not apply else 'already_applied', 'about_records': len(changes), 'articles': len(inserts)}
        backup = path.parent / ('before-seo-round3-' + datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S%fZ') + '.db')
        deadline = time.monotonic() + 25
        def progress(*_):
            if time.monotonic() > deadline: raise TimeoutError('Backup exceeded 25s')
        with closing(sqlite3.connect(backup)) as saved:
            db.backup(saved, pages=128, progress=progress, sleep=.05)
            require(saved.execute('PRAGMA quick_check').fetchone()[0] == 'ok', 'Invalid backup')
        result = apply_transaction(db, catalog)
        result.update(status='applied_local', database=str(path), backup=str(backup), manifest_sha256=hashlib.sha256(CATALOG.read_bytes()).hexdigest())
        backup.with_suffix('.json').write_text(json.dumps(result, ensure_ascii=False, indent=2), 'utf-8')
        return result


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--database', required=True)
    parser.add_argument('--apply-local', action='store_true')
    args = parser.parse_args()
    print(json.dumps(apply_local(args.database, args.apply_local), ensure_ascii=False, indent=2))
