"""Add the store's approved QQ and Telegram contacts without changing order history."""
import argparse
from contextlib import closing
from datetime import datetime, timezone
import json
from pathlib import Path
import re
import sqlite3
import time

WECHAT = 'Cqz0924ttxk'
QQ = '1669268759'
TELEGRAM = '@czq_piestar'
TELEGRAM_URL = 'https://t.me/czq_piestar'
IDENTITY_KEYS = {'supportWechat', 'wechat', 'qq', 'telegram'}

def enrich(value, key=''):
    if isinstance(value, dict): return {k: enrich(v, k) for k, v in value.items()}
    if isinstance(value, list): return [enrich(v, key) for v in value]
    if not isinstance(value, str) or key in IDENTITY_KEYS: return value
    value = value.replace('有问题，微信联系就好', '有问题，随时联系我们').replace('微信支持留言咨询。', '微信、QQ 和 Telegram 均支持留言咨询。')
    if WECHAT not in value: return value
    additions = []
    if QQ not in value: additions.append('QQ: ' + QQ)
    if TELEGRAM not in value:
        additions.append('Telegram: ' + ('<a href="' + TELEGRAM_URL + '" target="_blank" rel="noopener noreferrer">' + TELEGRAM + '</a>' if '<' in value else TELEGRAM))
    if not additions: return value
    return re.sub(re.escape(WECHAT) + r'(?:</strong>)?', lambda m: m[0] + ' · ' + ' · '.join(additions), value)

SCOPES = {'products': ['description_json', 'content_json', 'instructions_json', 'seo_meta_json'], 'posts': ['summary_json', 'content_json', 'seo_meta_json'], 'settings': ['value_json']}
SETTING_KEYS = {'home_announcement', 'about_page_config', 'site_config'}

def plan(db):
    updates = []
    for table, fields in SCOPES.items():
        columns = [r[1] for r in db.execute('PRAGMA table_info(' + table + ')')]
        if not columns: continue
        for row in db.execute('SELECT rowid,* FROM ' + table):
            item = dict(zip(['rowid'] + columns, row))
            if table == 'settings' and item['key'] not in SETTING_KEYS: continue
            for field in fields:
                original = item.get(field)
                if original is None: continue
                value = json.loads(original)
                if table == 'settings' and item['key'] == 'site_config':
                    revised = dict(value)
                    revised['contact'] = {**value.get('contact', {}), 'telegram': TELEGRAM_URL}
                    for name in ['about', 'legal']:
                        if name in value: revised[name] = enrich(value[name])
                else: revised = enrich(value)
                if revised != value:
                    encoded = json.dumps(revised, ensure_ascii=False, separators=(',', ':'))
                    if isinstance(original, bytes): encoded = encoded.encode()
                    updates.append((table, item['rowid'], field, original, encoded))
    return updates

def snapshot(db):
    return {name: list(db.execute('SELECT rowid,* FROM "' + name.replace('"', '""') + '" ORDER BY rowid')) for name, in db.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")}

def apply_database(path, *, apply=False, backup_dir=None):
    path = Path(path).resolve(strict=True)
    if apply and backup_dir is None: raise ValueError('A backup directory is required')
    with closing(sqlite3.connect(path.as_uri() + ('?mode=rw' if apply else '?mode=ro'), uri=True, timeout=5)) as db:
        updates = plan(db); backup_path = None; protected = False
        if apply and updates:
            backup_dir = Path(backup_dir); backup_dir.mkdir(parents=True, exist_ok=True)
            backup_path = backup_dir / ('before-support-contacts-' + datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S%fZ') + '.db')
            backup_path.touch(exist_ok=False); backup_path.chmod(0o600)
            deadline = time.monotonic() + 30
            def progress(*_):
                if time.monotonic() > deadline: raise TimeoutError('Backup exceeded deadline')
            with closing(sqlite3.connect(backup_path, timeout=5)) as backup:
                db.backup(backup, pages=128, progress=progress, sleep=.05)
                if backup.execute('PRAGMA quick_check').fetchone() != ('ok',): raise RuntimeError('Backup integrity failed')
            db.execute('BEGIN IMMEDIATE'); updates = plan(db); before = snapshot(db)
            expected = {name: [list(row) for row in rows] for name, rows in before.items()}
            for table, rowid, field, old, new in updates:
                columns = [r[1] for r in db.execute('PRAGMA table_info(' + table + ')')]
                changed = db.execute('UPDATE ' + table + ' SET ' + field + '=? WHERE rowid=? AND ' + field + '=?', (new, rowid, old)).rowcount
                if changed != 1: raise RuntimeError('Content changed during update')
                for row in expected[table]:
                    if row[0] == rowid: row[columns.index(field) + 1] = new
            if snapshot(db) != {k: [tuple(r) for r in rows] for k, rows in expected.items()} or plan(db): raise RuntimeError('Unexpected database changes')
            db.commit(); protected = True
        return {'mode': 'apply' if apply else 'plan', 'changed_fields': len(updates) if apply else 0, 'planned_fields': len(updates), 'fields': [{'table': t, 'rowid': i, 'field': f} for t, i, f, _, _ in updates], 'backup': str(backup_path) if backup_path else None, 'all_other_fields_unchanged': protected or not updates}

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--database', required=True); parser.add_argument('--apply', action='store_true'); parser.add_argument('--backup-dir')
    args = parser.parse_args()
    print(json.dumps(apply_database(args.database, apply=args.apply, backup_dir=args.backup_dir), ensure_ascii=True))
