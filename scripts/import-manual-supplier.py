"""Import a reviewed ACG catalog as unpublished manual goods, without purchases.

Called by the maintenance runner with an already encrypted merchant secret.
The snapshot and credentials are private inputs, never embedded in this script.
Existing rows are immutable; conflicting or partial imports roll back in full.
"""
import datetime
from decimal import Decimal
import hashlib
import json
from pathlib import Path
import sqlite3


def blob(value):
    # Go jsonmap/jsonslice Scan expects SQLite BLOB, not SQLite TEXT.
    return json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode()


def rows(db):
    result = {}
    for (table,) in db.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name"):
        result[table] = {row[0]: tuple(row[1:]) for row in db.execute('SELECT rowid,* FROM "' + table.replace('"', '""') + '" ORDER BY rowid')}
    return result


def digest(value):
    return hashlib.sha256(repr(value).encode()).hexdigest()


def apply_catalog(database, catalog, encrypted_secret, backup_directory):
    assert catalog['base_url'].startswith('https://')
    assert str(catalog['merchant_id']).isdigit()
    products = catalog['products']
    assert products and len({p['id'] for p in products}) == len(products)
    assert encrypted_secret and len(encrypted_secret) >= 56
    prefix = catalog['slug_prefix']
    assert prefix and all(c in 'abcdefghijklmnopqrstuvwxyz0123456789-' for c in prefix)
    category_ids = {c['id'] for c in catalog['categories']}
    for p in products:
        assert p['category_id'] in category_ids
        assert Decimal(p['price']) > 0 and p['name']
        assert p['upstream_supported'] is True
        assert p['stock'] >= 0 and p['id'] > 0
    db = sqlite3.connect(database, timeout=20)
    db.execute('PRAGMA foreign_keys=ON')
    backup = Path(backup_directory)
    backup.mkdir(parents=True, exist_ok=False)
    with sqlite3.connect(backup / 'shop.db') as saved:
        db.backup(saved)
    stamp = datetime.datetime.now(datetime.timezone.utc).isoformat()
    result = {'products': [], 'purchase_endpoint_called': False, 'mode': 'manual', 'published': False,
              'backup': str(backup / 'shop.db')}
    try:
        db.execute('BEGIN IMMEDIATE')
        before = rows(db)
        public_before = db.execute('SELECT id FROM products WHERE is_active=1 AND deleted_at IS NULL ORDER BY id').fetchall()
        matches = db.execute('SELECT id,status,auto_sync_price FROM site_connections WHERE base_url=? AND deleted_at IS NULL', (catalog['base_url'],)).fetchall()
        if matches:
            assert len(matches) == 1 and matches[0][1:] == ('disabled', 0), 'Existing connection requires review'
            connection_id = matches[0][0]
            existing = db.execute('SELECT p.id,m.upstream_product_id,p.fulfillment_type,p.is_active,p.is_mapped,m.is_active FROM product_mappings m JOIN products p ON p.id=m.local_product_id WHERE m.connection_id=? AND m.deleted_at IS NULL', (connection_id,)).fetchall()
            assert {r[1] for r in existing} == {p['id'] for p in products}, 'Partial import requires review'
            assert all(r[2:] == ('manual', 0, 1, 0) for r in existing), 'Existing import has been edited; preserve it'
            result.update(connection_id=connection_id, already_imported=True, products=[{'local_id': r[0], 'upstream_id': r[1]} for r in existing], original_rows_preserved=True, tables_checked=len(before))
            db.rollback()
            return result

        def insert(table, fields):
            return db.execute('INSERT INTO "'+table+'" ('+','.join('"'+k+'"' for k in fields)+') VALUES ('+','.join('?' for _ in fields)+')', list(fields.values())).lastrowid

        connection_id = insert('site_connections', {
            'name': catalog['name'] + ' 商户' + str(catalog['merchant_id']) + '（人工发货）',
            'base_url': catalog['base_url'], 'api_key': str(catalog['merchant_id']), 'api_secret': encrypted_secret,
            'protocol': 'acg-faka', 'callback_url': '', 'status': 'disabled', 'last_ping_at': catalog['checked_at'],
            'last_ping_ok': 1, 'retry_max': 5, 'retry_intervals': '[30,60,300]', 'exchange_rate': 1,
            'price_markup_percent': 0, 'price_rounding_mode': 'none', 'auto_sync_price': 0,
            'created_at': stamp, 'updated_at': stamp})
        root_category = insert('categories', {'parent_id': 0, 'slug': prefix, 'name_json': blob({'zh-CN': catalog['name']}),
            'icon': '', 'sort_order': 0, 'is_active': 0, 'created_at': stamp})
        categories = {}
        for category in catalog['categories']:
            categories[category['id']] = insert('categories', {'parent_id': root_category, 'slug': prefix+'-category-'+str(category['id']),
                'name_json': blob({'zh-CN': category['name']}), 'icon': '', 'sort_order': 0, 'is_active': 0, 'created_at': stamp})
        for p in products:
            price = str(Decimal(p['price']).quantize(Decimal('.01')))
            product_id = insert('products', {
                'category_id': categories[p['category_id']], 'slug': prefix+'-'+str(p['id']),
                'seo_meta_json': blob({}), 'title_json': blob({'zh-CN': p['name']}),
                'description_json': blob({'zh-CN': p['summary']}), 'content_json': blob({'zh-CN': p['content']}),
                'instructions_json': blob({'zh-CN': p['instructions']}), 'price_amount': price, 'cost_price_amount': price,
                'wholesale_prices': blob([]), 'images': blob(p['images']), 'tags': blob([catalog['name']]),
                'purchase_type': 'member', 'min_purchase_quantity': p['minimum'], 'max_purchase_quantity': p['maximum'],
                'stock_display_mode': 'exact', 'fulfillment_type': 'manual', 'manual_form_schema_json': blob({}),
                'manual_stock_total': 0, 'manual_stock_locked': 0, 'manual_stock_sold': 0, 'payment_channel_ids': '',
                'is_affiliate_enabled': 0, 'is_mapped': 1, 'is_active': 0, 'sort_order': 0,
                'created_at': stamp, 'updated_at': stamp})
            sku_id = insert('product_skus', {'product_id': product_id, 'sku_code': prefix+'-'+str(p['id']),
                'spec_values_json': blob({}), 'price_amount': price, 'cost_price_amount': price, 'manual_stock_total': 0,
                'manual_stock_locked': 0, 'manual_stock_sold': 0, 'is_active': 1, 'sort_order': 0,
                'created_at': stamp, 'updated_at': stamp})
            mapping_id = insert('product_mappings', {'connection_id': connection_id, 'local_product_id': product_id,
                'upstream_product_id': p['id'], 'upstream_fulfillment_type': 'auto', 'upstream_status': 'active',
                'is_active': 0, 'stock_limit': 0, 'last_synced_at': catalog['checked_at'], 'created_at': stamp, 'updated_at': stamp})
            insert('sku_mappings', {'product_mapping_id': mapping_id, 'local_sku_id': sku_id, 'upstream_sku_id': p['id'],
                'upstream_price': price, 'upstream_stock': p['stock'], 'upstream_is_active': 1,
                'stock_synced_at': catalog['checked_at'], 'created_at': stamp, 'updated_at': stamp})
            result['products'].append({'local_id': product_id, 'sku_id': sku_id, 'mapping_id': mapping_id, 'upstream_id': p['id']})
        after = rows(db)
        allowed = {'site_connections': 1, 'categories': len(categories)+1, 'products': len(products),
                   'product_skus': len(products), 'product_mappings': len(products), 'sku_mappings': len(products)}
        for table, old in before.items():
            assert all(after[table].get(key) == row for key, row in old.items()), 'Original rows changed: '+table
            assert len(after[table])-len(old) == allowed.get(table, 0), 'Unexpected row count: '+table
        assert db.execute('SELECT id FROM products WHERE is_active=1 AND deleted_at IS NULL ORDER BY id').fetchall() == public_before
        assert db.execute('PRAGMA quick_check').fetchone()[0] == 'ok'
        assert db.execute('PRAGMA foreign_key_check').fetchone() is None
        db.commit()
        result.update(connection_id=connection_id, category_ids=[root_category,*categories.values()], already_imported=False,
                      original_rows_preserved=True, tables_checked=len(before), public_product_count=len(public_before),
                      added_rows=allowed, original_table_hashes={t:digest(v) for t,v in before.items()})
        return result
    except Exception:
        db.rollback()
        raise
    finally:
        db.close()
