"""Add only the three approved Grok half-price plans; never retire existing goods.

Dry-run by default. Recent signed read-only evidence and a backup are required
before applying. Uses the existing catalog migration's transaction utilities.
"""
import argparse
from contextlib import closing
from datetime import datetime, timezone
from decimal import Decimal
import json
import os
from pathlib import Path
import runpy
import sqlite3
import time

helpers=runpy.run_path(str(Path(__file__).with_name('replace-ellaix-claude-x.py')))
require,encode,snapshot,insert=(helpers[x] for x in ['require','encode','snapshot','insert'])
APPROVED={116:('premium','138.00'),117:('super','398.00'),118:('ultra','818.00')}


def migrate(db_path,catalog_path,source,*,apply=False,backup_root=None):
    catalog_path=Path(catalog_path)
    config=json.loads(catalog_path.read_text('utf8'))
    require(config['version']==1 and config['category_slug']=='grok','Unexpected catalog scope')
    require([p['source_id'] for p in config['items']]==list(APPROVED),'Unexpected product set')
    require(source['purchase_endpoints_called'] is False and source['balance_unchanged'] is True,'Read-only evidence required')
    age=(datetime.now(timezone.utc)-datetime.fromisoformat(source['at'])).total_seconds()
    require(-60<=age<=3600,'Refresh signed supplier evidence')
    require(len(source['items'])==3 and {s['item']['id'] for s in source['items']}==set(APPROVED),'Incomplete source evidence')
    evidence={s['item']['id']:s for s in source['items']};contents={}
    for p in config['items']:
        sid=p['source_id'];tier,price=APPROVED[sid];src=evidence[sid];item=src['item']
        require(p['slug']=='ellaix-grok-'+tier+'-monthly-half' and p['source_code']=='ellaix_grok_'+tier+'_monthly_half_20261025','Product identity changed')
        require(p['cover_from_slug']=='ellaix-grok-'+tier+'-monthly','Cover identity changed')
        require(Decimal(p['price'])==Decimal(price),'Unapproved sale price')
        require((item['code'],item['name'],item['category_id'])==(p['source_code'],p['source_title'],35),'Supplier identity changed')
        require(item['status']==1 and item['delivery_way']==0 and not item.get('only_user'),'Unsupported delivery or account requirement')
        require(item.get('config') in (None,'',[],{}) and item.get('widget') in (None,'',[],{}),'Unsupported options/form')
        require(src['quote']['currency_code']=='CNY','Unexpected currency')
        require(0<Decimal(str(src['quote']['price']))<Decimal(price) and int(src['stock']['stock'])>=0,'Invalid cost or stock')
        for field in ['price','user_price','factory_price']:
            if item.get(field) not in (None,''):require(Decimal(str(item[field]))<9999,'Placeholder price rejected')
        content=(catalog_path.parent/p['content_file']).read_text('utf8').strip()
        require(all(x in content for x in ['半价','优惠资格','普通全价套餐','自动','Cqz0924ttxk']),'Missing eligibility or support information')
        require('半价' in p['title'] and '优惠资格' in p['description'],'Title and summary must distinguish the offer')
        require(p['images'] and all(x.startswith('/uploads/') for x in p['images']),'Existing uploaded cover required')
        contents[sid]=content
    path=Path(db_path).resolve(strict=True);stamp=datetime.now(timezone.utc).isoformat();backup=None
    if apply:
        require(backup_root is not None,'Backup directory required')
        folder=Path(backup_root)/('before-grok-half-'+datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S%fZ'))
        folder.mkdir(parents=True,mode=0o700);backup=folder/'shop.db';deadline=time.monotonic()+25
        def progress(*_):
            if time.monotonic()>deadline:raise TimeoutError('Backup deadline exceeded')
        with closing(sqlite3.connect(path.as_uri()+'?mode=ro',uri=True,timeout=5)) as live, closing(sqlite3.connect(backup)) as dest:
            live.backup(dest,pages=512,progress=progress,sleep=.05)
            require(dest.execute('PRAGMA quick_check').fetchone()[0]=='ok','Invalid backup')
        if os.name!='nt':backup.chmod(0o600)
    created={t:[] for t in ['products','product_skus','product_mappings','sku_mappings']};results=[]
    with closing(sqlite3.connect(path.as_uri()+'?mode=rw',uri=True,timeout=10)) as db:
        db.execute('PRAGMA foreign_keys=ON');db.execute('BEGIN IMMEDIATE')
        deadline=time.monotonic()+30;db.set_progress_handler(lambda:int(time.monotonic()>deadline),10000)
        try:
            before=snapshot(db);fk=db.execute('PRAGMA foreign_key_check').fetchall()
            connections=db.execute("SELECT id,status,auto_sync_price,exchange_rate FROM site_connections WHERE base_url='https://ellaixshop.com' AND api_key='1039' AND protocol='acg-faka' AND deleted_at IS NULL").fetchall()
            require(len(connections)==1 and connections[0][1]=='active' and not connections[0][2] and Decimal(str(connections[0][3]))==1,'Existing fixed-price supplier connection required')
            connection=connections[0][0]
            category=db.execute("SELECT id FROM categories WHERE slug='grok' AND is_active=1 AND deleted_at IS NULL").fetchone()
            require(category is not None,'Existing active Grok category required');cid=category[0]
            for position,p in enumerate(config['items']):
                sid=p['source_id'];src=evidence[sid];cost=str(src['quote']['price']);stock=int(src['stock']['stock'])
                old=db.execute('SELECT images,category_id FROM products WHERE slug=? AND deleted_at IS NULL',(p['cover_from_slug'],)).fetchone()
                require(old is not None and old[1]==cid and json.loads(old[0])==p['images'],'Existing cover changed')
                values={'category_id':cid,'slug':p['slug'],'seo_meta_json':encode({}),'title_json':encode({'zh-CN':p['title']}),'description_json':encode({'zh-CN':p['description']}),'content_json':encode({'zh-CN':contents[sid]}),'instructions_json':encode({'zh-CN':p['instructions']}),'price_amount':p['price'],'cost_price_amount':cost,'wholesale_prices':encode([]),'images':encode(p['images']),'tags':encode(p['tags']),'purchase_type':'guest','min_purchase_quantity':int(src['item'].get('minimum') or 0),'max_purchase_quantity':int(src['item'].get('maximum') or 0),'stock_display_mode':'status','fulfillment_type':'upstream','manual_form_schema_json':encode({}),'manual_stock_total':0,'manual_stock_locked':0,'manual_stock_sold':0,'payment_channel_ids':'[]','is_affiliate_enabled':0,'is_mapped':1,'is_active':1,'sort_order':100+position*10,'created_at':stamp,'updated_at':stamp}
                prior=db.execute('SELECT id,deleted_at FROM products WHERE slug=?',(p['slug'],)).fetchone()
                if prior:
                    require(prior[1] is None,'Archived product conflict');pid=prior[0]
                    cols=['category_id','title_json','description_json','content_json','instructions_json','images','fulfillment_type','is_mapped','is_active']
                    require(db.execute('SELECT '+','.join(cols)+' FROM products WHERE id=?',(pid,)).fetchone()==tuple(values[c] for c in cols),'Existing product differs')
                    require(Decimal(str(db.execute('SELECT price_amount FROM products WHERE id=?',(pid,)).fetchone()[0]))==Decimal(p['price']),'Existing price differs')
                    skus=db.execute('SELECT id,price_amount FROM product_skus WHERE product_id=? AND deleted_at IS NULL',(pid,)).fetchall()
                    require(len(skus)==1 and Decimal(str(skus[0][1]))==Decimal(p['price']),'Existing SKU differs');sku=skus[0][0]
                    maps=db.execute('SELECT id,connection_id,upstream_product_id,is_active FROM product_mappings WHERE local_product_id=? AND deleted_at IS NULL',(pid,)).fetchall()
                    require(len(maps)==1 and maps[0][1:]==(connection,sid,1),'Existing mapping differs');mid=maps[0][0]
                    require(db.execute('SELECT local_sku_id,upstream_sku_id FROM sku_mappings WHERE product_mapping_id=? AND deleted_at IS NULL',(mid,)).fetchall()==[(sku,sid)],'Existing SKU mapping differs')
                else:
                    require(not db.execute('SELECT 1 FROM product_mappings WHERE connection_id=? AND upstream_product_id=? AND deleted_at IS NULL',(connection,sid)).fetchone(),'Duplicate supplier mapping')
                    pid=insert(db,'products',values);created['products'].append(pid)
                    sku=insert(db,'product_skus',{'product_id':pid,'sku_code':'ELLAIX-'+str(sid),'spec_values_json':encode({}),'price_amount':p['price'],'cost_price_amount':cost,'manual_stock_total':0,'manual_stock_locked':0,'manual_stock_sold':0,'is_active':1,'sort_order':0,'created_at':stamp,'updated_at':stamp});created['product_skus'].append(sku)
                    mid=insert(db,'product_mappings',{'connection_id':connection,'local_product_id':pid,'upstream_product_id':sid,'upstream_fulfillment_type':'auto','upstream_status':'active','is_active':1,'stock_limit':0,'last_synced_at':stamp,'created_at':stamp,'updated_at':stamp});created['product_mappings'].append(mid)
                    smid=insert(db,'sku_mappings',{'product_mapping_id':mid,'local_sku_id':sku,'upstream_sku_id':sid,'upstream_price':cost,'upstream_stock':stock,'upstream_is_active':1,'stock_synced_at':stamp,'created_at':stamp,'updated_at':stamp});created['sku_mappings'].append(smid)
                results.append({'product_id':pid,'sku_id':sku,'mapping_id':mid,'source_id':sid,'slug':p['slug'],'price':p['price'],'quoted_cost':cost,'source_stock':stock})
            after=snapshot(db)
            for table,rows in before.items():
                require([r for r in after[table] if r[0] not in created.get(table,[])]==rows,'Existing rows changed: '+table)
                require(len(after[table])-len(rows)==len(created.get(table,[])),'Unexpected row count: '+table)
            require(db.execute('PRAGMA foreign_key_check').fetchall()==fk,'New foreign key violation')
            db.commit() if apply else db.rollback()
        except BaseException:
            db.rollback();raise
    report={'passed':True,'applied':apply,'category_id':cid,'items':results,'created':created,'audited_tables':len(before),'all_existing_rows_unchanged':True,'purchase_endpoints_called':False,'backup':str(backup) if backup else None}
    if backup:(backup.parent/'report.json').write_text(json.dumps(report,ensure_ascii=False,indent=2),encoding='utf8')
    return report


if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__)
    p.add_argument('--db',required=True);p.add_argument('--catalog',required=True,type=Path);p.add_argument('--source',required=True,type=Path)
    p.add_argument('--backup-root',type=Path);p.add_argument('--apply',action='store_true');a=p.parse_args()
    print(json.dumps(migrate(a.db,a.catalog,json.loads(a.source.read_text('utf8')),apply=a.apply,backup_root=a.backup_root),ensure_ascii=False))
