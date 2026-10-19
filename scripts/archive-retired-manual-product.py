"""Archive a retired, unmapped manual product without deleting its history.

The normal admin delete endpoint deliberately rejects products with order
records. This maintenance operation only marks the selected product as deleted;
SKUs, orders, payments and fulfillment snapshots remain available to old orders.
"""
from contextlib import closing
from datetime import datetime, timezone
import hashlib
from pathlib import Path
import sqlite3


def require(value, message):
    if not value:
        raise ValueError(message)


def snapshot(db):
    result = {}
    for (table,) in db.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name"):
        result[table] = {row['audit_rowid']: dict(row) for row in db.execute(
            'SELECT rowid AS audit_rowid,* FROM "' + table.replace('"', '""') + '" ORDER BY rowid')}
    return result


def archive_product(database, product_id, expected_slug, backup_directory):
    require(type(product_id) is int and product_id > 0 and bool(expected_slug), "Explicit product identity required")
    db = sqlite3.connect(Path(database).resolve().as_uri() + "?mode=rw", uri=True, timeout=20)
    db.row_factory = sqlite3.Row
    db.execute("PRAGMA foreign_keys=ON")
    try:
        backup = Path(backup_directory)
        backup.mkdir(parents=True, exist_ok=False)
        with closing(sqlite3.connect(backup / 'shop.db')) as saved:
            db.backup(saved)
        db.execute("BEGIN IMMEDIATE")
        product = db.execute('SELECT * FROM products WHERE id=?', (product_id,)).fetchone()
        require(product is not None and product['slug'] == expected_slug, "Product identity changed")
        require(product['fulfillment_type'] == 'manual' and not product['is_active'] and not product['is_mapped'],
                "Only retired unmapped manual products can be archived")
        require(db.execute('SELECT count(*) FROM product_mappings WHERE local_product_id=? AND deleted_at IS NULL', (product_id,)).fetchone()[0] == 0,
                "Product still has an upstream mapping")
        if product['deleted_at'] is not None:
            db.rollback()
            return {'status': 'already_archived', 'product_id': product_id, 'backup': str(backup / 'shop.db')}
        require(product['manual_stock_locked'] == 0 and db.execute(
            'SELECT count(*) FROM product_skus WHERE product_id=? AND deleted_at IS NULL AND manual_stock_locked<>0', (product_id,)).fetchone()[0] == 0,
                "Product has locked manual inventory")
        require(db.execute("SELECT count(*) FROM card_secrets WHERE product_id=? AND deleted_at IS NULL AND status IN ('available','reserved')", (product_id,)).fetchone()[0] == 0,
                "Product has available or reserved card inventory")
        before = snapshot(db)
        stamp = datetime.now(timezone.utc).isoformat()
        changed = db.execute('UPDATE products SET deleted_at=?,updated_at=? WHERE id=? AND slug=? AND deleted_at IS NULL',
                             (stamp, stamp, product_id, expected_slug)).rowcount
        require(changed == 1, "Product archive conflict")
        after = snapshot(db)
        require(set(before) == set(after), "Unexpected table change")
        for table, rows in before.items():
            require(set(rows) == set(after[table]), "Unexpected row creation or deletion")
            for rowid, row in rows.items():
                current = after[table][rowid]
                allowed = {'deleted_at', 'updated_at'} if table == 'products' and row['id'] == product_id else set()
                require({k: v for k, v in row.items() if k not in allowed} == {k: v for k, v in current.items() if k not in allowed},
                        "Unexpected non-archive data change")
        require(db.execute('PRAGMA foreign_key_check').fetchone() is None, "Foreign key verification failed")
        db.commit()
        return {'status': 'archived', 'product_id': product_id, 'slug': expected_slug, 'at': stamp,
                'backup': str(backup / 'shop.db'), 'tables_checked': len(before),
                'only_changed_fields': ['products.deleted_at', 'products.updated_at'],
                'all_order_and_sku_records_preserved': True,
                'protected_table_hashes': {t: hashlib.sha256(repr(rows).encode()).hexdigest() for t, rows in before.items() if t != 'products'}}
    except Exception:
        db.rollback()
        raise
    finally:
        db.close()
