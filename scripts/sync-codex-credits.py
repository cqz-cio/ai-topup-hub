"""Add only the six approved Codex credit products to an existing SQLite shop.

Run against the intended environment after its normal schema migration. The
catalog contains human-confirmed retail prices; source quotes are costs only.
This maintenance command makes no network, order, payment or purchase calls.
"""
from public_redemption import public_copy
import argparse
from datetime import datetime, timezone
from decimal import Decimal
import hashlib
import json
from pathlib import Path
import sqlite3


def encode(value):
    # The application's SQLite JSON scanners expect the same BLOB representation
    # written by GORM, rather than SQLite TEXT values.
    return json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode('utf-8')


def digest_existing(db):
    """Fingerprint existing rows in every table while holding the write lock."""
    result = {}
    for name, in db.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'"):
        quoted = '"' + name.replace('"', '""') + '"'
        rows = list(db.execute(f'SELECT rowid,* FROM {quoted} ORDER BY rowid'))
        result[name] = (len(rows), hashlib.sha256(repr(rows).encode()).hexdigest())
    return result


def insert(db, table, data):
    keys = list(data)
    sql = f"INSERT INTO {table} ({','.join(keys)}) VALUES ({','.join('?' for _ in keys)})"
    return db.execute(sql, [data[k] for k in keys]).lastrowid


def apply_catalog(db_path, catalog, backup, local_snapshot=False, publish=False):
    products = catalog['products']
    assert [p['upstream_product_id'] for p in products] == list(range(73,79)), 'Unexpected product scope'
    assert [p['points'] for p in products] == [250,500,1000,2500,5000,25000], 'Unexpected credit amounts'
    assert catalog['stock_limit'] == 9999 and catalog['auto_sync_price'] is False
    for p in products:
        assert p['slug'] == f"qzweb-{p['upstream_product_id']}"
        assert p['upstream_sku_id'] == p['upstream_product_id']
        price = Decimal(str(p['retail_price_cny']))
        assert price.is_finite() and price > 0 and price == price.quantize(Decimal('.01')), 'A confirmed two-decimal retail price is required'
        assert p['stock'] == 9999
        assert all('https://cdk.bar/' in value and 'Cqz0924ttxk' in value for value in p['instructions'].values())
    db = sqlite3.connect(db_path, timeout=10)
    db.execute('PRAGMA foreign_keys=ON')
    assert 'stock_limit' in {r[1] for r in db.execute('PRAGMA table_info(product_mappings)')}, 'Rebuild and start the updated backend before importing'
    category = db.execute('SELECT id FROM categories WHERE slug=? AND deleted_at IS NULL', [catalog['category_slug']]).fetchone()
    assert category, 'Configured Codex category is required; apply catalog-categories.json first'
    if local_snapshot:
        connections = db.execute("SELECT id,auto_sync_price FROM site_connections WHERE status != 'active' AND base_url LIKE 'http://127.0.0.1:%' AND deleted_at IS NULL").fetchall()
    else:
        connections = db.execute("SELECT id,auto_sync_price FROM site_connections WHERE base_url=? AND api_key=? AND protocol=? AND status='active' AND deleted_at IS NULL", [catalog['base_url'],catalog['merchant_id'],catalog['protocol']]).fetchall()
    assert len(connections) == 1 and not connections[0][1], 'Require one correct supplier connection with price sync disabled'
    connection_id = connections[0][0]
    slugs = [p['slug'] for p in products]
    marks = ','.join('?' for _ in slugs)
    assert db.execute(f'SELECT count(*) FROM products WHERE slug IN ({marks})', slugs).fetchone()[0] == 0, 'Target products already exist; inspect instead of overwriting'
    assert db.execute('SELECT count(*) FROM product_mappings WHERE connection_id=? AND upstream_product_id BETWEEN 73 AND 78', [connection_id]).fetchone()[0] == 0, 'Target mappings already exist'
    backup = Path(backup)
    assert not backup.exists(), 'Never overwrite a backup'
    backup.parent.mkdir(parents=True, exist_ok=True)
    with sqlite3.connect(backup) as target:
        db.backup(target)
    backup.chmod(0o600)
    created = []
    now = datetime.now(timezone.utc).strftime('%Y-%m-%d %H:%M:%S')
    try:
        db.execute('BEGIN IMMEDIATE')
        before = digest_existing(db)
        for p in products:
            product_id = insert(db, 'products', {
                'category_id':category[0], 'slug':p['slug'], 'title_json':encode(p['title']),
                'description_json':encode(p['description']), 'content_json':encode(public_copy(p['content'])),
                'instructions_json':encode(p['instructions']), 'seo_meta_json':encode({}),
                'price_amount':p['retail_price_cny'], 'cost_price_amount':p['supplier_quote_cny'],
                'wholesale_prices':encode([]), 'images':encode(p['images']), 'tags':encode(['Codex','点数充值']),
                'purchase_type':'guest', 'min_purchase_quantity':1, 'max_purchase_quantity':20,
                'stock_display_mode':'exact', 'fulfillment_type':'upstream', 'manual_form_schema_json':encode({}),
                'manual_stock_total':0, 'manual_stock_locked':0, 'manual_stock_sold':0,
                'payment_channel_ids':'[]', 'is_affiliate_enabled':False, 'is_mapped':True,
                'is_active':bool(publish), 'sort_order':p['sort_order'], 'created_at':now, 'updated_at':now,
            })
            sku_id = insert(db, 'product_skus', {
                'product_id':product_id, 'sku_code':p['shared_code'], 'spec_values_json':encode({'规格':f"{p['points']} 点数"}),
                'price_amount':p['retail_price_cny'], 'cost_price_amount':p['supplier_quote_cny'],
                'manual_stock_total':0, 'manual_stock_locked':0, 'manual_stock_sold':0,
                'is_active':True,'sort_order':0,'created_at':now,'updated_at':now,
            })
            mapping_id = insert(db, 'product_mappings', {
                'connection_id':connection_id,'local_product_id':product_id,'upstream_product_id':p['upstream_product_id'],
                'upstream_fulfillment_type':'auto','upstream_status':'active','is_active':True,
                'stock_limit':9999,'last_synced_at':now,'created_at':now,'updated_at':now,
            })
            insert(db, 'sku_mappings', {
                'product_mapping_id':mapping_id,'local_sku_id':sku_id,'upstream_sku_id':p['upstream_sku_id'],
                'upstream_price':p['supplier_quote_cny'],'upstream_stock':9999,'upstream_is_active':True,
                'stock_synced_at':now,'created_at':now,'updated_at':now,
            })
            created.append({'slug':p['slug'],'product_id':product_id,'sku_id':sku_id,'mapping_id':mapping_id,'price':p['retail_price_cny'],'stock':9999})
        for name,(count,digest) in before.items():
            quoted = '"'+name.replace('"','""')+'"'
            old_rows = list(db.execute(f'SELECT rowid,* FROM {quoted} ORDER BY rowid LIMIT ?', [count]))
            assert hashlib.sha256(repr(old_rows).encode()).hexdigest() == digest, f'Existing data changed: {name}'
        assert not db.execute('PRAGMA foreign_key_check').fetchone(), 'Foreign key check failed'
        db.commit()
    except BaseException:
        db.rollback()
        raise
    finally:
        db.close()
    return {'created':created,'published':publish,'local_snapshot':local_snapshot,'backup':str(backup),'existing_data_unchanged':True,'purchase_calls':0}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--db', type=Path, required=True)
    parser.add_argument('--catalog', type=Path, required=True)
    parser.add_argument('--backup', type=Path, required=True)
    parser.add_argument('--local-snapshot', action='store_true')
    parser.add_argument('--publish', action='store_true')
    args = parser.parse_args()
    data = json.loads(args.catalog.read_text(encoding='utf-8'))
    print(json.dumps(apply_catalog(args.db,data,args.backup,args.local_snapshot,args.publish),ensure_ascii=False,indent=2))
