"""Maintain the explicitly reviewed manual supplier category assignments.

The first migration groups eight goods; split_claude_category subsequently moves
only #56/#58 to Claude. Run against each environment's own database; never copy
the local database over production. No order or upstream API is called.
"""
from contextlib import closing
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import sqlite3


GROUPS = (
    ('telegram-premium-links', 'Telegram会员链接', 18,
     {60: '86gamestore-10004', 61: '86gamestore-10005', 62: '86gamestore-10006'}),
    ('gpt-overwrite-topup', 'GPT可覆盖充值（ios谷歌支付）', 17,
     {55: '86gamestore-83', 56: '86gamestore-84', 57: '86gamestore-88',
      58: '86gamestore-10007', 59: '86gamestore-10010'}),
)
RETIRED = ((18, '86gamestore-category-10'), (17, '86gamestore-category-6'),
           (16, '86gamestore'), (1, 'zxa-category-1'))


def require(condition, message):
    if not condition:
        raise ValueError(message)


def rows(db, table):
    return {r['id']: dict(r) for r in db.execute('SELECT * FROM ' + table + ' ORDER BY id')}


def protected_hashes(db):
    result = {}
    for (table,) in db.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name"):
        if table not in ('categories', 'products'):
            quoted = '"' + table.replace('"', '""') + '"'
            values = [tuple(r) for r in db.execute('SELECT * FROM ' + quoted + ' ORDER BY rowid')]
            result[table] = hashlib.sha256(repr(values).encode()).hexdigest()
    return result


def public_ids(db):
    return [r[0] for r in db.execute('''SELECT p.id FROM products p JOIN categories c ON c.id=p.category_id
        WHERE p.deleted_at IS NULL AND p.is_active=1 AND c.deleted_at IS NULL AND c.is_active=1 ORDER BY p.id''')]


def apply_categories(database, backup_root, allow_nonempty=()):
    """allow_nonempty explicitly preserves local-only drafts in old categories."""
    require(set(allow_nonempty) <= {1, 16, 17}, 'Unexpected nonempty-category exception')
    path = Path(database).resolve(strict=True)
    stamp = datetime.now(timezone.utc)
    backup = Path(backup_root) / ('before-manual-categories-' + stamp.strftime('%Y%m%dT%H%M%S%fZ'))
    backup.mkdir(parents=True, mode=0o700, exist_ok=False)
    with closing(sqlite3.connect(path.as_uri() + '?mode=ro', uri=True)) as source:
        with closing(sqlite3.connect(backup / 'shop.db')) as saved:
            source.backup(saved)
            require(saved.execute('PRAGMA quick_check').fetchone()[0] == 'ok', 'Backup integrity check failed')
    if os.name != 'nt':
        os.chmod(backup / 'shop.db', 0o600)

    with closing(sqlite3.connect(path.as_uri() + '?mode=rw', uri=True, timeout=20)) as db:
        db.row_factory = sqlite3.Row
        db.execute('PRAGMA foreign_keys=ON')
        db.execute('BEGIN IMMEDIATE')
        try:
            before_products = rows(db, 'products')
            before_categories = rows(db, 'categories')
            protected = protected_hashes(db)
            public_before = public_ids(db)
            by_slug = {c['slug']: c for c in before_categories.values()}
            for cid, slug in RETIRED:
                row = before_categories.get(cid)
                require(row is not None and row['slug'] == slug and not row['is_active'],
                        'Old category identity/status changed: ' + str(cid))

            targets, assignments, created, moved = {}, {}, [], []
            expected_categories = {cid: row.copy() for cid, row in before_categories.items()}
            for slug, name, old_id, products in GROUPS:
                current = by_slug.get(slug)
                if current is not None:
                    require(current['deleted_at'] is None and current['parent_id'] == 0
                            and not current['is_active'] and json.loads(current['name_json']) == {'zh-CN': name},
                            'New category conflicts with an existing category: ' + slug)
                    target_id = current['id']
                else:
                    # The Go JSON scanner requires SQLite BLOB, not TEXT.
                    encoded_name = json.dumps({'zh-CN': name}, ensure_ascii=False).encode('utf-8')
                    target_id = db.execute('''INSERT INTO categories
                        (parent_id,slug,name_json,icon,sort_order,is_active,created_at)
                        VALUES (0,?,?,?,0,0,?)''',
                        (slug, encoded_name, '', stamp.isoformat())).lastrowid
                    created.append(target_id)
                    expected_categories[target_id] = dict(db.execute('SELECT * FROM categories WHERE id=?', (target_id,)).fetchone())
                targets[name] = target_id
                for pid, product_slug in products.items():
                    product = before_products.get(pid)
                    require(product is not None and product['slug'] == product_slug and product['deleted_at'] is None,
                            'Reviewed product identity changed: ' + str(pid))
                    require(product['fulfillment_type'] == 'manual' and not product['is_active'],
                            'Reviewed product delivery/publication changed: ' + str(pid))
                    require(product['category_id'] in (old_id, target_id), 'Reviewed product was reclassified: ' + str(pid))
                    assignments[pid] = target_id
                    if product['category_id'] != target_id:
                        db.execute('UPDATE products SET category_id=? WHERE id=?', (target_id, pid))
                        moved.append(pid)

            removed, retained = [], {}
            for cid, _ in RETIRED:
                product_refs = [r[0] for r in db.execute('SELECT id FROM products WHERE category_id=? AND deleted_at IS NULL ORDER BY id', (cid,))]
                child_refs = [r[0] for r in db.execute('SELECT id FROM categories WHERE parent_id=? AND deleted_at IS NULL ORDER BY id', (cid,))]
                if product_refs or child_refs:
                    require(cid in allow_nonempty and before_categories[cid]['deleted_at'] is None,
                            'Old category still has live references: ' + str(cid))
                    retained[cid] = {'products': product_refs, 'children': child_refs}
                elif before_categories[cid]['deleted_at'] is None:
                    db.execute('UPDATE categories SET deleted_at=? WHERE id=?', (stamp.isoformat(), cid))
                    expected_categories[cid]['deleted_at'] = stamp.isoformat()
                    removed.append(cid)

            expected_products = {pid: row.copy() for pid, row in before_products.items()}
            for pid, cid in assignments.items():
                expected_products[pid]['category_id'] = cid
            require(rows(db, 'products') == expected_products, 'Unexpected product field change')
            require(rows(db, 'categories') == expected_categories, 'Unexpected category field change')
            require(protected_hashes(db) == protected, 'Unexpected change to another table')
            require(public_ids(db) == public_before, 'Public product set changed')
            require(db.execute('PRAGMA foreign_key_check').fetchone() is None, 'Foreign key check failed')
            db.commit()
        except BaseException:
            db.rollback()
            raise
    report = {
        'status': 'applied', 'backup': str(backup / 'shop.db'), 'at': stamp.isoformat(),
        'category_ids': targets, 'created_category_ids': created, 'product_assignments': assignments,
        'moved_product_ids': sorted(moved), 'removed_category_ids': sorted(removed),
        'retained_category_references': retained, 'tables_checked': len(protected) + 2,
        'protected_table_hashes': protected, 'other_product_fields_unchanged': True,
        'public_product_ids_unchanged': public_before,
    }
    (backup / 'report.json').write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding='utf-8')
    return report


def split_claude_category(database, backup_root):
    """Move only #56/#58; preserve current publication, price and fulfillment.

    A new category inherits the current source category's visibility. Existing
    categories are never changed, including the retired zxa-category-4 record.
    """
    targets = {56: '86gamestore-84', 58: '86gamestore-10007'}
    path = Path(database).resolve(strict=True)
    stamp = datetime.now(timezone.utc)
    backup = Path(backup_root) / ('before-claude-category-' + stamp.strftime('%Y%m%dT%H%M%S%fZ'))
    backup.mkdir(parents=True, mode=0o700, exist_ok=False)
    with closing(sqlite3.connect(path.as_uri() + '?mode=ro', uri=True)) as source:
        with closing(sqlite3.connect(backup / 'shop.db')) as saved:
            source.backup(saved)
            require(saved.execute('PRAGMA quick_check').fetchone()[0] == 'ok', 'Backup integrity check failed')
    if os.name != 'nt':
        os.chmod(backup / 'shop.db', 0o600)

    with closing(sqlite3.connect(path.as_uri() + '?mode=rw', uri=True, timeout=20)) as db:
        db.row_factory = sqlite3.Row
        db.execute('PRAGMA foreign_keys=ON')
        db.execute('BEGIN IMMEDIATE')
        try:
            before_products = rows(db, 'products')
            before_categories = rows(db, 'categories')
            protected = protected_hashes(db)
            public_before = public_ids(db)
            origin = before_categories.get(20)
            require(origin is not None and origin['slug'] == 'gpt-overwrite-topup'
                    and origin['parent_id'] == 0 and origin['deleted_at'] is None, 'Source category identity changed')
            current = next((c for c in before_categories.values() if c['slug'] == 'claude'), None)
            created = current is None
            if created:
                name = json.dumps({'zh-CN': 'Claude'}, ensure_ascii=False).encode('utf-8')
                cid = db.execute('''INSERT INTO categories
                    (parent_id,slug,name_json,icon,sort_order,is_active,created_at)
                    VALUES (0,'claude',?,'',0,?,?)''',
                    (name, origin['is_active'], stamp.isoformat())).lastrowid
                current = dict(db.execute('SELECT * FROM categories WHERE id=?', (cid,)).fetchone())
            else:
                require(current['deleted_at'] is None and current['parent_id'] == 0
                        and json.loads(current['name_json']) == {'zh-CN': 'Claude'}, 'Conflicting Claude category')
                cid = current['id']

            moved = []
            expected_products = {pid: row.copy() for pid, row in before_products.items()}
            for pid, slug in targets.items():
                product = before_products.get(pid)
                require(product is not None and product['slug'] == slug and product['deleted_at'] is None,
                        'Reviewed Claude product identity changed: ' + str(pid))
                require(product['category_id'] in (20, cid), 'Reviewed Claude product was reclassified: ' + str(pid))
                if product['category_id'] != cid:
                    db.execute('UPDATE products SET category_id=? WHERE id=?', (cid, pid))
                    moved.append(pid)
                expected_products[pid]['category_id'] = cid
            expected_categories = before_categories.copy()
            if created:
                expected_categories[cid] = current
            require(rows(db, 'products') == expected_products, 'Unexpected product field change')
            require(rows(db, 'categories') == expected_categories, 'Unexpected existing category change')
            require(protected_hashes(db) == protected, 'Unexpected change to another table')
            require(public_ids(db) == public_before, 'Public product set changed')
            require(db.execute('PRAGMA foreign_key_check').fetchone() is None, 'Foreign key check failed')
            db.commit()
        except BaseException:
            db.rollback()
            raise
    report = {
        'status': 'applied', 'backup': str(backup / 'shop.db'), 'at': stamp.isoformat(),
        'category_id': cid, 'category_is_active': bool(current['is_active']), 'created': created,
        'product_assignments': {pid: cid for pid in targets}, 'moved_product_ids': moved,
        'tables_checked': len(protected) + 2, 'protected_table_hashes': protected,
        'other_product_fields_unchanged': True, 'existing_categories_unchanged': True,
        'public_product_ids_unchanged': public_before,
    }
    (backup / 'report.json').write_text(json.dumps(report, ensure_ascii=False, indent=2), encoding='utf-8')
    return report
