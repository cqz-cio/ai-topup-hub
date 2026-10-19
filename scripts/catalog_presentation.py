"""Apply the approved storefront quota and region labels to public catalog fields.

Supplier identifiers, private delivery instructions, prices and order snapshots
retain their original values. A consistent backup is required for writes.
"""
import argparse
from contextlib import closing
from datetime import datetime, timezone
import json
from pathlib import Path
import re
import sqlite3
import time

PRODUCT_SLUGS = {f'qzweb-{n}' for n in [4, 5, 6, 7, 55, 56, 57, 58, 63, 64, 73, 74, 75, 76, 77, 78]}
GPT_SLUGS = {f'qzweb-{n}' for n in [4, 5, 6, 7, 55, 56, 57, 58, 63, 64]}
TITLES = {
    'qzweb-55': 'ChatGPT Pro 10x 新老号（到期）开通 质保一个月｜菲区',
    'qzweb-7': 'ChatGPT Pro 10x 续费绑卡｜菲区｜自动发码',
    'qzweb-56': 'ChatGPT Pro 25x 新老号（到期）开通 质保一个月｜菲区',
    'qzweb-58': 'ChatGPT Pro 25x 新老号（到期）开通 质保一个月｜智利',
}
COVERS = {
    'qzweb-55': ('pro20x-direct-20261001.svg', 'pro10x-direct-ph-20261016.svg'),
    'qzweb-7': ('pro20x-renew-20261001.svg', 'pro10x-renew-ph-20261016.svg'),
    'qzweb-56': ('pro50x-ph-20261001.svg', 'pro25x-ph-20261016.svg'),
    'qzweb-58': ('pro50x-cl-20261001.svg', 'pro25x-cl-20261016.svg'),
    'qzweb-6': ('pro5x-ph-20261001.svg', 'pro5x-ph-20261016.svg'),
    'qzweb-57': ('pro5x-cl-20261001.svg', 'pro5x-cl-20261016.svg'),
    **{f'qzweb-{73+i}': (f'codex-{amount}-ph-20261015.svg', f'codex-{amount}-ph-20261016.svg') for i, amount in enumerate([250, 500, 1000, 2500, 5000, 25000])},
}
PUBLIC_FIELDS = ['title_json', 'description_json', 'content_json', 'seo_meta_json', 'tags', 'images']
LANGUAGES = {'zh-CN', 'zh-TW', 'en-US'}

def public_copy(value, *, tiers=False, locale='zh-CN'):
    if isinstance(value, dict):
        return {key: public_copy(item, tiers=tiers, locale=key if key in LANGUAGES else locale) for key, item in value.items()}
    if isinstance(value, list):
        return [public_copy(item, tiers=tiers, locale=locale) for item in value]
    if not isinstance(value, str):
        return value
    ph = 'PH' if locale == 'en-US' else '菲區' if locale == 'zh-TW' else '菲区'
    cl = 'Chile' if locale == 'en-US' else '智利'
    # Collapse combined labels before replacing the standalone labels.
    value = re.sub(r'渠道[一1]\s*(?:[（(]\s*菲[区區]\s*[）)]|[·/]\s*菲[区區])', ph, value)
    value = re.sub(r'渠道[二2]\s*(?:[（(]\s*智利\s*[）)]|[·/]\s*智利)', cl, value)
    value = re.sub(r'Channel\s*1\s*(?:\(\s*PH\s*\)|[·/]\s*PH)', 'PH', value, flags=re.I)
    value = re.sub(r'Channel\s*2\s*(?:\(\s*(?:CL|Chile)\s*\)|[·/]\s*(?:CL|Chile))', 'Chile', value, flags=re.I)
    value = re.sub(r'渠道[一1]|Channel\s*1\b', ph, value, flags=re.I)
    value = re.sub(r'渠道[二2]|Channel\s*2\b', cl, value, flags=re.I)
    if tiers:
        value = re.sub(r'(?<!\d)(20|50)\s*[xX×倍](?![a-zA-Z0-9])', lambda match: '10x' if match[1] == '20' else '25x', value)
        value = re.sub(r'\bPro\s*20(?=新老|\s|[（(])', 'Pro 10x', value, flags=re.I)
    return value

def title_copy(slug, title):
    revised = public_copy(title, tiers=slug in GPT_SLUGS)
    if slug in TITLES and isinstance(revised, dict) and revised.get('zh-CN'):
        revised = {**revised, 'zh-CN': TITLES[slug]}
    return revised

def image_copy(slug, images):
    if slug not in COVERS or not isinstance(images, list):
        return images
    old, new = COVERS[slug]
    return ['/catalog-products/qzweb/' + new if isinstance(item, str) and item.rsplit('/', 1)[-1] == old else item for item in images]

def catalog_fields(slug, title, description, content, images):
    if slug not in PRODUCT_SLUGS:
        return title, description, content, images
    return (title_copy(slug, title), public_copy(description, tiers=slug in GPT_SLUGS),
            public_copy(content, tiers=slug in GPT_SLUGS), image_copy(slug, images))

def encode_like(original, value):
    encoded = json.dumps(value, ensure_ascii=False, separators=(',', ':'))
    return encoded.encode('utf-8') if isinstance(original, bytes) else encoded

def table_columns(db, table):
    return [row[1] for row in db.execute('PRAGMA table_info(' + table + ')')]

def records(db, table):
    columns = table_columns(db, table)
    return [dict(zip(['rowid'] + columns, row)) for row in db.execute('SELECT rowid,* FROM ' + table)] if columns else []

def plan(db):
    updates = []
    products = {}
    def append(table, row, field, revised):
        raw = row.get(field)
        if raw is not None and json.loads(raw) != revised:
            updates.append((table, row['rowid'], field, raw, encode_like(raw, revised)))
    for row in records(db, 'products'):
        slug = row['slug']
        if slug not in PRODUCT_SLUGS or row.get('deleted_at') is not None:
            continue
        products[row['id']] = slug
        for field in PUBLIC_FIELDS:
            raw = row.get(field)
            if raw is None:
                continue
            old = json.loads(raw)
            revised = title_copy(slug, old) if field == 'title_json' else image_copy(slug, old) if field == 'images' else public_copy(old, tiers=slug in GPT_SLUGS)
            append('products', row, field, revised)
    for row in records(db, 'product_skus'):
        slug = products.get(row.get('product_id'))
        if slug and row.get('deleted_at') is None and row.get('spec_values_json') is not None:
            append('product_skus', row, 'spec_values_json', public_copy(json.loads(row['spec_values_json']), tiers=slug in GPT_SLUGS))
    for row in records(db, 'posts'):
        if row.get('deleted_at') is not None:
            continue
        for field in ['title_json', 'summary_json', 'content_json', 'seo_meta_json']:
            if row.get(field) is not None:
                append('posts', row, field, public_copy(json.loads(row[field]), tiers=True))
    for row in records(db, 'settings'):
        if row['key'] not in {'home_announcement', 'about_page_config', 'site_config'} or row.get('value_json') is None:
            continue
        old = json.loads(row['value_json'])
        if row['key'] == 'site_config':
            revised = dict(old)
            for field in ['about', 'legal']:
                if field in old:
                    revised[field] = public_copy(old[field], tiers=True)
        else:
            revised = public_copy(old, tiers=True)
        append('settings', row, 'value_json', revised)
    return updates

def snapshot(db):
    return {name: list(db.execute('SELECT rowid,* FROM "' + name.replace('"', '""') + '" ORDER BY rowid')) for name, in db.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")}

def apply_database(path, *, apply=False, backup_dir=None):
    path = Path(path).resolve(strict=True)
    if apply and backup_dir is None:
        raise ValueError('A backup directory is required')
    with closing(sqlite3.connect(path.as_uri() + ('?mode=rw' if apply else '?mode=ro'), uri=True, timeout=5)) as db:
        updates = plan(db)
        backup_path = None
        if apply and updates:
            backup_dir = Path(backup_dir)
            backup_dir.mkdir(parents=True, exist_ok=True)
            backup_path = backup_dir / ('before-catalog-presentation-' + datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S%fZ') + '.db')
            backup_path.touch(exist_ok=False)
            backup_path.chmod(0o600)
            deadline = time.monotonic() + 25
            def progress(*_):
                if time.monotonic() > deadline:
                    raise TimeoutError('Catalog backup exceeded deadline')
            with closing(sqlite3.connect(backup_path, timeout=5)) as backup:
                db.backup(backup, pages=128, progress=progress, sleep=.05)
                if backup.execute('PRAGMA quick_check').fetchone() != ('ok',):
                    raise RuntimeError('Backup integrity failed')
            try:
                db.execute('BEGIN IMMEDIATE')
                updates = plan(db)
                expected = {name: [list(row) for row in rows] for name, rows in snapshot(db).items()}
                for table, rowid, field, old, new in updates:
                    changed = db.execute('UPDATE ' + table + ' SET ' + field + '=? WHERE rowid=? AND ' + field + '=?', (new, rowid, old)).rowcount
                    if changed != 1:
                        raise RuntimeError('Catalog changed during update')
                    index = table_columns(db, table).index(field) + 1
                    for row in expected[table]:
                        if row[0] == rowid:
                            row[index] = new
                if snapshot(db) != {name: [tuple(row) for row in rows] for name, rows in expected.items()} or plan(db):
                    raise RuntimeError('Unexpected database changes')
                db.commit()
            except BaseException:
                db.rollback()
                raise
        return {'mode': 'apply' if apply else 'plan', 'changed_fields': len(updates) if apply else 0, 'planned_fields': len(updates), 'fields': [{'table': table, 'rowid': rowid, 'field': field} for table, rowid, field, _, _ in updates], 'backup': str(backup_path) if backup_path else None, 'all_other_fields_unchanged': True}

if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--database', required=True)
    parser.add_argument('--apply', action='store_true')
    parser.add_argument('--backup-dir')
    args = parser.parse_args()
    print(json.dumps(apply_database(args.database, apply=args.apply, backup_dir=args.backup_dir), ensure_ascii=True))
