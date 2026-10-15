"""Reclassify existing products without changing their sale or delivery data.

Only categories and products.category_id can change. The whole update is
transactional, checks every unrelated table, and takes a SQLite backup first.
No API, order, payment, supplier or inventory calls are made.
"""
import argparse
from datetime import datetime, timezone
import hashlib
import json
import os
from pathlib import Path
import sqlite3


def encode(value):
    # GORM's JSON scanner expects a BLOB in this shop's SQLite database.
    return json.dumps(value, ensure_ascii=False, separators=(',', ':')).encode('utf-8')


def table_rows(db, table):
    name = '"' + table.replace('"', '""') + '"'
    return [tuple(r) for r in db.execute('SELECT * FROM ' + name + ' ORDER BY rowid')]


def protected_hashes(db):
    return {name: hashlib.sha256(repr(table_rows(db, name)).encode()).hexdigest()
            for name, in db.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
            if name not in ('categories', 'products')}


def visible_slugs(db):
    return {r[0] for r in db.execute('''SELECT p.slug FROM products p JOIN categories c ON c.id=p.category_id
        WHERE p.is_active=1 AND p.deleted_at IS NULL AND c.is_active=1 AND c.deleted_at IS NULL''')}


def apply_categories(db_path, catalog, backup_root):
    assert catalog['version'] == 1
    groups = catalog['categories']
    assert [g['key'] for g in groups] == ['gpt', 'codex', 'gemini']
    target_slugs = {g['slug'] for g in groups}
    product_slugs = [slug for g in groups for slug in g['product_slugs']]
    assert len(set(product_slugs)) == len(product_slugs) == 22
    retired = catalog['retire_category_slugs']
    assert set(retired) == {'qzweb-category-9', 'qzweb-category-10'}
    db_path = Path(db_path).resolve(strict=True)
    backup = Path(backup_root) / ('before-product-categories-' + datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S%fZ'))
    backup.mkdir(parents=True, mode=0o700)
    with sqlite3.connect(db_path.as_uri() + '?mode=ro', uri=True) as source:
        with sqlite3.connect(backup / 'shop.db') as snapshot:
            source.backup(snapshot)
            assert snapshot.execute('PRAGMA quick_check').fetchone()[0] == 'ok'
    if os.name != 'nt':
        os.chmod(backup / 'shop.db', 0o600)
    with sqlite3.connect(db_path.as_uri() + '?mode=rw', uri=True, timeout=15) as db:
        db.row_factory = sqlite3.Row
        db.execute('PRAGMA foreign_keys=ON')
        db.execute('BEGIN IMMEDIATE')
        try:
            before = protected_hashes(db)
            old_products = {r['id']: dict(r) for r in db.execute('SELECT * FROM products')}
            old_categories = {r['id']: dict(r) for r in db.execute('SELECT * FROM categories')}
            categories_by_slug = {r['slug']: r for r in old_categories.values()}
            products_by_slug = {r['slug']: r for r in old_products.values()}
            public_before = visible_slugs(db)
            assert all(slug in products_by_slug and products_by_slug[slug]['deleted_at'] is None for slug in product_slugs), 'Expected catalog product missing'
            ids = {}
            changed = []
            for group in groups:
                row = categories_by_slug.get(group['slug'])
                if row:
                    assert row['deleted_at'] is None and row['parent_id'] == 0
                    if group['key'] == 'codex':
                        assert json.loads(row['name_json']) == group['name'], 'Conflicting Codex category'
                    category_id = row['id']
                    # Keep the existing ID, slug, icon and creation metadata.
                    db.execute('UPDATE categories SET name_json=?,sort_order=?,is_active=1 WHERE id=?',
                               (encode(group['name']), group['sort_order'], category_id))
                else:
                    assert group['key'] == 'codex', 'Existing GPT and Gemini categories required'
                    category_id = db.execute('''INSERT INTO categories
                        (parent_id,slug,name_json,icon,sort_order,is_active,created_at)
                        VALUES (0,?,?,?,?,1,?)''', (group['slug'], encode(group['name']), group['icon'], group['sort_order'],
                        datetime.now(timezone.utc).strftime('%Y-%m-%d %H:%M:%S'))).lastrowid
                ids[group['key']] = category_id
                for slug in group['product_slugs']:
                    product = products_by_slug[slug]
                    origin = old_categories[product['category_id']]['slug']
                    assert origin in group['source_category_slugs'] + [group['slug']], 'Product category changed since review: ' + slug
                    if product['category_id'] != category_id:
                        db.execute('UPDATE products SET category_id=? WHERE id=?', (category_id, product['id']))
                        changed.append(slug)
            for slug in retired:
                row = categories_by_slug.get(slug)
                if row:
                    assert not db.execute('SELECT 1 FROM products WHERE category_id=? AND deleted_at IS NULL', (row['id'],)).fetchone(), 'Retired channel category still contains products'
                    assert not db.execute('SELECT 1 FROM categories WHERE parent_id=? AND deleted_at IS NULL', (row['id'],)).fetchone()
                    db.execute('UPDATE categories SET is_active=0 WHERE id=?', (row['id'],))
            expected_ids = {slug: ids[g['key']] for g in groups for slug in g['product_slugs']}
            for row in db.execute('SELECT * FROM products'):
                current = dict(row)
                expected = old_products[current['id']].copy()
                if expected['slug'] in expected_ids:
                    expected['category_id'] = expected_ids[expected['slug']]
                assert current == expected, 'An unrelated product field changed'
            for row in db.execute('SELECT * FROM categories'):
                current = dict(row)
                if current['id'] not in old_categories:
                    assert current['slug'] == 'codex'
                    continue
                expected = old_categories[current['id']].copy()
                group = next((g for g in groups if g['slug'] == current['slug']), None)
                if group:
                    expected.update(name_json=encode(group['name']), sort_order=group['sort_order'], is_active=1)
                if current['slug'] in retired:
                    expected['is_active'] = 0
                assert current == expected, 'An unrelated category field changed'
            assert protected_hashes(db) == before, 'An unrelated table changed'
            assert visible_slugs(db) == public_before, 'Public product availability changed'
            assert not db.execute('PRAGMA foreign_key_check').fetchone()
            counts = {g['name']['zh-CN']: db.execute('SELECT count(*) FROM products WHERE category_id=? AND is_active=1 AND deleted_at IS NULL', (ids[g['key']],)).fetchone()[0] for g in groups}
            db.commit()
        except BaseException:
            db.rollback()
            raise
        assert db.execute('PRAGMA quick_check').fetchone()[0] == 'ok'
    result = {'passed': True, 'backup': str(backup), 'category_ids': ids, 'active_product_counts': counts,
              'changed_product_slugs': changed, 'public_product_count_unchanged': len(public_before),
              'protected_tables_unchanged': len(before), 'other_product_fields_unchanged': True,
              'order_payment_purchase_calls': 0}
    report = backup / 'category-report.json'
    report.write_text(json.dumps(result, ensure_ascii=False, indent=2) + '\n', encoding='utf-8')
    if os.name != 'nt':
        os.chmod(report, 0o600)
    return result


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--db', type=Path, required=True)
    parser.add_argument('--catalog', type=Path, required=True)
    parser.add_argument('--backup-root', type=Path, required=True)
    args = parser.parse_args()
    catalog = json.loads(args.catalog.read_text(encoding='utf-8'))
    print(json.dumps(apply_categories(args.db, catalog, args.backup_root), ensure_ascii=False, indent=2))
