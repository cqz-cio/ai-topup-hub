"""Update delivery/after-sales copy in an isolated workspace work/ database only."""
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
CATALOG = ROOT / 'docs/catalog/seo-delivery/catalog.json'


def load_catalog():
    data = json.loads(CATALOG.read_text('utf-8'))
    require(data['slug'] == 'purchase-and-agent-guide', 'Unexpected article scope')
    path = (CATALOG.parent / data['file']).resolve(strict=True)
    require(path.parent == CATALOG.parent, 'Article path escaped catalog')
    data['content'] = path.read_text('utf-8').strip()
    defaults = json.loads((ROOT / 'internal/platform/publiccontent/about-defaults.json').read_text('utf-8'))
    require([(r['section'], r['index'], r['field']) for r in data['about']] == [('services', 1, 'text'), ('services', 3, 'text'), ('links', 0, 'title')], 'Unexpected about scope')
    for rule in data['about']:
        rule['value'] = defaults[rule['section']][rule['index']][rule['field']]
    return data


def plan(db, catalog):
    post = db.execute('SELECT * FROM posts WHERE slug=?', (catalog['slug'],)).fetchone()
    require(post is not None and post['type'] == 'notice' and post['is_published'] == 1 and post['deleted_at'] is None and post['published_at'], 'Missing or changed purchase notice')
    changes = []
    for field in ['title', 'summary', 'content']:
        column = field + '_json'
        old = post[column]; value = json.loads(old)
        text = value.get('zh-CN', '')
        require(text == catalog[field] or hashlib.sha256(text.encode()).hexdigest() == catalog['before_sha256'][field], 'Newer notice copy found: ' + field)
        if text != catalog[field]:
            value['zh-CN'] = catalog[field]
            changes.append(('posts', post['id'], column, old, encode(value, old)))
    about = db.execute("SELECT * FROM settings WHERE key='about_page_config'").fetchone()
    require(about is not None, 'Missing saved about page')
    old = about['value_json']; value = json.loads(old); before = deepcopy(value)
    require(value.get('version') == 1, 'Unexpected about page version')
    for rule in catalog['about']:
        item = value[rule['section']][rule['index']]
        identity = item['to'] if rule['section'] == 'links' else item['title'].get('zh-CN')
        require(identity == rule['identity'], 'About section identity changed')
        require(item[rule['field']].get('zh-CN') in [rule['before'], rule['value']], 'Newer about copy found')
        item[rule['field']]['zh-CN'] = rule['value']
    if before != value:
        changes.append(('settings', about['key'], 'value_json', old, encode(value, old)))
    return changes


def apply_transaction(db, catalog):
    try:
        db.execute('BEGIN IMMEDIATE')
        changes = plan(db, catalog)
        tables = [r[0] for r in db.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")]
        expected = {table: table_rows(db, table) for table in tables}
        for table, identity, column, old, value in changes:
            key = 'id' if table == 'posts' else 'key'
            # Table/column identifiers originate exclusively from the fixed plan above.
            count = db.execute(f'UPDATE {table} SET {column}=? WHERE {key}=? AND {column} IS ?', (value, identity, old)).rowcount
            require(count == 1, 'Concurrent content change')
            next(row for row in expected[table] if row[key] == identity)[column] = value
        for table in tables:
            require(table_rows(db, table) == expected[table], 'Protected data changed: ' + table)
        require(not plan(db, catalog), 'Non-idempotent content batch')
        db.commit()
        return {'changed_fields': len(changes), 'tables_checked': len(tables), 'unlisted_fields_preserved': True}
    except BaseException:
        db.rollback()
        raise


def apply_local(database, apply=False):
    path = Path(database).resolve(strict=True)
    require(path.is_relative_to(ROOT / 'work'), 'Only a workspace work/ database is allowed')
    catalog = load_catalog()
    with closing(sqlite3.connect(path.as_uri() + ('?mode=rw' if apply else '?mode=ro'), uri=True, timeout=8)) as db:
        db.row_factory = sqlite3.Row
        changes = plan(db, catalog)
        if not apply or not changes:
            return {'status': 'planned' if not apply else 'already_applied', 'changed_fields': len(changes)}
        backup = path.parent / ('before-seo-delivery-' + datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S%fZ') + '.db')
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
