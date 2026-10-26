"""Apply the explicitly approved six-item catalog; dry-run by default, never purchase.

Requires recent signed supplier evidence. Preserves all historical rows and audits
every table within the write transaction. Prices are pinned to user approval.
"""
import argparse
from contextlib import closing
from datetime import datetime, timezone
from decimal import Decimal
import json
import os
from pathlib import Path
import sqlite3
import time

APPROVED = {112: '168.88', 113: '788.00', 114: '1588.00', 120: '68.00', 122: '268.00', 124: '818.00'}
IDENTITIES = {
    112: ('ellaix-claude-pro', 'ellaix_claude_pro_20261024'),
    113: ('ellaix-claude-max-5x', 'ellaix_claude_max_5x_20261024'),
    114: ('ellaix-claude-max-20x', 'ellaix_claude_max_20x_20261024'),
    120: ('ellaix-x-plus-monthly-ng', 'ellaix_x_plus_monthly_ng_20261025'),
    122: ('ellaix-x-premium-monthly-ng', 'ellaix_x_premium_monthly_ng_20261025'),
    124: ('ellaix-x-super-monthly-ng', 'ellaix_x_super_monthly_ng_20261025'),
}
OLD = ['86gamestore-84', '86gamestore-10007']
EXCLUDED = [121, 123, 125, 126, 127]


def require(condition, message):
    if not condition:
        raise ValueError(message)


def encode(value):
    return json.dumps(value, ensure_ascii=False, separators=(',', ':')).encode('utf8')


def snapshot(db):
    names = [r[0] for r in db.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")]
    return {t: list(db.execute('SELECT * FROM "' + t.replace('"', '""') + '" ORDER BY rowid')) for t in names}


def insert(db, table, values):
    return db.execute(f"INSERT INTO {table} ({','.join(values)}) VALUES ({','.join('?' for _ in values)})", list(values.values())).lastrowid


def migrate(db_path, catalog_path, source, *, apply=False, backup_root=None):
    catalog_path = Path(catalog_path)
    config = json.loads(catalog_path.read_text('utf8'))
    require(config['version'] == 1 and config['retire_slugs'] == OLD and config['excluded_source_ids'] == EXCLUDED, 'Catalog scope changed')
    require([x['source_id'] for x in config['items']] == list(APPROVED), 'Unexpected product set')
    require(source['purchase_endpoints_called'] is False and source['balance_unchanged'] is True, 'Read-only supplier evidence required')
    age = (datetime.now(timezone.utc) - datetime.fromisoformat(source['at'])).total_seconds()
    require(-60 <= age <= 3600, 'Refresh signed supplier evidence')
    require(len(source['items']) == 6 and {x['item']['id'] for x in source['items']} == set(APPROVED), 'Incomplete supplier evidence')
    evidence = {x['item']['id']: x for x in source['items']}
    contents = {}
    for product in config['items']:
        sid = product['source_id']
        require((product['slug'], product['source_code']) == IDENTITIES[sid], 'Product identity changed')
        require(Decimal(product['price']) == Decimal(APPROVED[sid]), 'Unapproved sale price')
        expected_category = 'claude' if sid < 120 else 'x-subscriptions'
        require(product['category_slug'] == expected_category, 'Unexpected category')
        src = evidence[sid]
        item = src['item']
        require((item['code'], item['name'], item['category_id']) == (product['source_code'], product['source_title'], 34 if sid < 120 else 36), 'Supplier identity changed')
        require(item['status'] == 1 and item['delivery_way'] == 0 and not item.get('only_user'), 'Unsupported delivery or account requirement')
        require(item.get('config') in (None, '', [], {}) and item.get('widget') in (None, '', [], {}), 'Unsupported options/form')
        require(src['quote']['currency_code'] == 'CNY', 'Unexpected currency')
        cost = Decimal(str(src['quote']['price']))
        require(0 < cost < Decimal(product['price']) and int(src['stock']['stock']) >= 0, 'Invalid cost or stock')
        for field in ['price', 'user_price', 'factory_price']:
            if item.get(field) not in (None, ''):
                require(Decimal(str(item[field])) < 9999, 'Placeholder price rejected')
        content = (catalog_path.parent / product['content_file']).read_text('utf8').strip()
        require('自动' in content and 'Cqz0924ttxk' in content, 'Incomplete delivery/support text')
        require(all(url.startswith('/uploads/') for url in product['images']), 'Only reviewed local covers allowed')
        if sid < 120:
            require(all(text in content for text in ['KYC', '不保证成功率', '不做任何质保', '可能无法退款']), 'Missing approved Claude terms')
            require('应退金额＝' not in content and '质保一个月' not in content, 'Old warranty must not carry over')
        else:
            require('尼日利亚' in content and '不是成品账号' in content, 'Missing X eligibility terms')
        contents[sid] = content
    path = Path(db_path).resolve(strict=True)
    stamp = datetime.now(timezone.utc).isoformat()
    backup = None
    if apply:
        require(backup_root is not None, 'Backup directory required')
        folder = Path(backup_root) / ('before-ellaix-claude-x-' + datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S%fZ'))
        folder.mkdir(parents=True, mode=0o700)
        backup = folder / 'shop.db'
        deadline = time.monotonic() + 25
        def progress(*_):
            if time.monotonic() > deadline:
                raise TimeoutError('Backup deadline exceeded')
        with closing(sqlite3.connect(path.as_uri() + '?mode=ro', uri=True, timeout=5)) as live, closing(sqlite3.connect(backup)) as dest:
            live.backup(dest, pages=512, progress=progress, sleep=.05)
            require(dest.execute('PRAGMA quick_check').fetchone()[0] == 'ok', 'Invalid backup')
        if os.name != 'nt':
            backup.chmod(0o600)
    created = {t: [] for t in ['categories', 'products', 'product_skus', 'product_mappings', 'sku_mappings']}
    results = []
    with closing(sqlite3.connect(path.as_uri() + '?mode=rw', uri=True, timeout=10)) as db:
        db.execute('PRAGMA foreign_keys=ON')
        db.execute('BEGIN IMMEDIATE')
        deadline = time.monotonic() + 30
        db.set_progress_handler(lambda: int(time.monotonic() > deadline), 10000)
        try:
            before = snapshot(db)
            fk = db.execute('PRAGMA foreign_key_check').fetchall()
            conn = db.execute("SELECT id,status,auto_sync_price,exchange_rate FROM site_connections WHERE base_url='https://ellaixshop.com' AND api_key='1039' AND protocol='acg-faka' AND deleted_at IS NULL").fetchall()
            require(len(conn) == 1 and conn[0][1] == 'active' and not conn[0][2] and Decimal(str(conn[0][3])) == 1, 'Existing active fixed-price supplier connection required')
            connection = conn[0][0]
            category = db.execute("SELECT id FROM categories WHERE slug='claude' AND deleted_at IS NULL AND is_active=1").fetchone()
            require(category is not None, 'Existing active Claude category required')
            category_ids = {'claude': category[0]}
            old_ids = []
            for slug in OLD:
                old = db.execute('SELECT id,category_id,fulfillment_type,is_mapped FROM products WHERE slug=? AND deleted_at IS NULL', (slug,)).fetchone()
                require(old is not None and old[1:3] == (category[0], 'manual') and old[3] in (0, 1), 'Old Claude identity conflict')
                old_maps = db.execute('SELECT m.upstream_product_id,m.is_active,c.base_url,c.protocol FROM product_mappings m JOIN site_connections c ON c.id=m.connection_id WHERE m.local_product_id=? AND m.deleted_at IS NULL',(old[0],)).fetchall()
                expected_old_id = 84 if slug == OLD[0] else 10007
                require(old_maps == ([(expected_old_id, 0, 'https://faka.redeemgpt.com', 'acg-faka')] if old[3] else []), 'Old manual supplier mapping changed')
                old_ids.append(old[0])
            x = config['x_category']
            require(x == {'slug': 'x-subscriptions', 'name': 'X（推特）', 'sort_order': 80}, 'Unexpected X category')
            existing = db.execute('SELECT id,is_active,deleted_at,name_json FROM categories WHERE slug=?', (x['slug'],)).fetchone()
            if existing:
                require(existing[1] == 1 and existing[2] is None and json.loads(existing[3]) == {'zh-CN': x['name']}, 'X category conflict')
                category_ids[x['slug']] = existing[0]
            else:
                cid = insert(db, 'categories', {'parent_id': 0, 'slug': x['slug'], 'name_json': encode({'zh-CN': x['name']}), 'icon': '', 'sort_order': x['sort_order'], 'is_active': 1, 'created_at': stamp})
                created['categories'].append(cid)
                category_ids[x['slug']] = cid
            for position, product in enumerate(config['items']):
                sid = product['source_id']; src = evidence[sid]; cost = str(src['quote']['price']); stock = int(src['stock']['stock'])
                values = {
                    'category_id': category_ids[product['category_slug']], 'slug': product['slug'], 'seo_meta_json': encode({}),
                    'title_json': encode({'zh-CN': product['title']}), 'description_json': encode({'zh-CN': product['description']}),
                    'content_json': encode({'zh-CN': contents[sid]}), 'instructions_json': encode({'zh-CN': product['instructions']}),
                    'price_amount': product['price'], 'cost_price_amount': cost, 'wholesale_prices': encode([]), 'images': encode(product['images']),
                    'tags': encode(product['tags']), 'purchase_type': 'guest', 'min_purchase_quantity': int(src['item'].get('minimum') or 0),
                    'max_purchase_quantity': int(src['item'].get('maximum') or 0), 'stock_display_mode': 'status', 'fulfillment_type': 'upstream',
                    'manual_form_schema_json': encode({}), 'manual_stock_total': 0, 'manual_stock_locked': 0, 'manual_stock_sold': 0,
                    'payment_channel_ids': '[]', 'is_affiliate_enabled': 0, 'is_mapped': 1, 'is_active': 1, 'sort_order': position * 10,
                    'created_at': stamp, 'updated_at': stamp,
                }
                prior = db.execute('SELECT id,deleted_at FROM products WHERE slug=?', (product['slug'],)).fetchone()
                if prior:
                    require(prior[1] is None, 'Archived product conflict')
                    pid = prior[0]
                    cols = ['category_id','title_json','description_json','content_json','instructions_json','images','fulfillment_type','is_mapped','is_active']
                    require(db.execute('SELECT '+','.join(cols)+' FROM products WHERE id=?', (pid,)).fetchone() == tuple(values[c] for c in cols), 'Existing product differs; review manually')
                    require(Decimal(str(db.execute('SELECT price_amount FROM products WHERE id=?',(pid,)).fetchone()[0])) == Decimal(product['price']), 'Existing sale price differs')
                    skus = db.execute('SELECT id,price_amount FROM product_skus WHERE product_id=? AND deleted_at IS NULL',(pid,)).fetchall()
                    require(len(skus) == 1 and Decimal(str(skus[0][1])) == Decimal(product['price']), 'Existing SKU differs')
                    sku = skus[0][0]
                    mappings = db.execute('SELECT id,connection_id,upstream_product_id,is_active FROM product_mappings WHERE local_product_id=? AND deleted_at IS NULL',(pid,)).fetchall()
                    require(len(mappings) == 1 and mappings[0][1:] == (connection,sid,1), 'Existing mapping differs')
                    mid = mappings[0][0]
                    require(db.execute('SELECT local_sku_id,upstream_sku_id FROM sku_mappings WHERE product_mapping_id=? AND deleted_at IS NULL',(mid,)).fetchall() == [(sku,sid)], 'Existing SKU mapping differs')
                else:
                    require(not db.execute('SELECT 1 FROM product_mappings WHERE connection_id=? AND upstream_product_id=? AND deleted_at IS NULL',(connection,sid)).fetchone(), 'Duplicate supplier mapping')
                    pid = insert(db,'products',values);created['products'].append(pid)
                    sku = insert(db,'product_skus',{'product_id':pid,'sku_code':'ELLAIX-'+str(sid),'spec_values_json':encode({}),'price_amount':product['price'],'cost_price_amount':cost,'manual_stock_total':0,'manual_stock_locked':0,'manual_stock_sold':0,'is_active':1,'sort_order':0,'created_at':stamp,'updated_at':stamp});created['product_skus'].append(sku)
                    mid = insert(db,'product_mappings',{'connection_id':connection,'local_product_id':pid,'upstream_product_id':sid,'upstream_fulfillment_type':'auto','upstream_status':'active','is_active':1,'stock_limit':0,'last_synced_at':stamp,'created_at':stamp,'updated_at':stamp});created['product_mappings'].append(mid)
                    smid = insert(db,'sku_mappings',{'product_mapping_id':mid,'local_sku_id':sku,'upstream_sku_id':sid,'upstream_price':cost,'upstream_stock':stock,'upstream_is_active':1,'stock_synced_at':stamp,'created_at':stamp,'updated_at':stamp});created['sku_mappings'].append(smid)
                results.append({'product_id':pid,'sku_id':sku,'mapping_id':mid,'source_id':sid,'slug':product['slug'],'price':product['price'],'quoted_cost':cost,'source_stock':stock})
            for pid in old_ids:
                db.execute('UPDATE products SET is_active=0,updated_at=? WHERE id=? AND is_active<>0',(stamp,pid))
            after = snapshot(db)
            columns = [r[1] for r in db.execute('PRAGMA table_info(products)')]
            active, updated = columns.index('is_active'), columns.index('updated_at')
            for table, rows in before.items():
                expected = []
                for row in rows:
                    value = list(row)
                    if table == 'products' and value[0] in old_ids and value[active]:
                        value[active],value[updated] = 0,stamp
                    expected.append(tuple(value))
                require([r for r in after[table] if r[0] not in created.get(table,[])] == expected, 'Out-of-scope change: '+table)
                require(len(after[table])-len(rows) == len(created.get(table,[])), 'Unexpected row count: '+table)
            require(db.execute('PRAGMA foreign_key_check').fetchall() == fk,'New foreign key violation')
            for slug,cid in category_ids.items():
                actual = {r[0] for r in db.execute('SELECT slug FROM products WHERE category_id=? AND is_active=1 AND deleted_at IS NULL',(cid,))}
                require(actual == {p['slug'] for p in config['items'] if p['category_slug']==slug},'Unexpected active category contents')
            db.commit() if apply else db.rollback()
        except BaseException:
            db.rollback()
            raise
    report={'passed':True,'applied':apply,'items':results,'retired_product_ids':old_ids,'category_ids':category_ids,'created':created,'audited_tables':len(before),'unrelated_rows_unchanged':True,'purchase_endpoints_called':False,'backup':str(backup) if backup else None}
    if backup:
        (backup.parent/'report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2),encoding='utf8')
    return report


if __name__ == '__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--db',required=True)
    parser.add_argument('--catalog',required=True,type=Path)
    parser.add_argument('--source',required=True,type=Path)
    parser.add_argument('--backup-root',type=Path)
    parser.add_argument('--apply',action='store_true')
    args=parser.parse_args()
    print(json.dumps(migrate(args.db,args.catalog,json.loads(args.source.read_text('utf8')),apply=args.apply,backup_root=args.backup_root),ensure_ascii=False))
