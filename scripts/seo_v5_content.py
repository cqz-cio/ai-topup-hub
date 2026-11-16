"""SEO v5 reviewed copy/media migration. Default is a read-only plan.

Production --apply requires an operator file bound to the exact catalog, HTML,
baseline and image digest. --apply-local accepts drafts only inside work/.
Never changes prices, inventory, credentials, orders, fulfillment or mappings.
"""
import argparse
from copy import deepcopy
from datetime import datetime, timezone
import hashlib
import importlib.util
import json
from pathlib import Path
import sqlite3

ROOT=Path(__file__).resolve().parents[1]
CATALOG=ROOT/'docs/catalog/seo-v5/copy.json'
spec=importlib.util.spec_from_file_location('reviewed_v4',ROOT/'scripts/seo_grok_v4_content.py')
reviewed=importlib.util.module_from_spec(spec);spec.loader.exec_module(reviewed)
require,encode,localized,approved=reviewed.require,reviewed.encode,reviewed.localized,reviewed.approved
ALLOWED={60:'telegram-premium-3m',61:'telegram-premium-6m',62:'telegram-premium-12m',45:'chatgpt-pro-100-ph',31:'chatgpt-pro-200-ph',33:'chatgpt-pro-500-ph',37:'chatgpt-pro-100-egypt',35:'chatgpt-pro-500-egypt',29:'chatgpt-go-code-egypt',32:'chatgpt-pro-200-renew-ph',54:'chatgpt-pro-200-manual',55:'chatgpt-pro-100-manual',59:'chatgpt-go-manual'}
MEDIA={('products',40):'gemini-pro-18m-warranty',('products',41):'gemini-pro-18m-no-warranty',('categories',11):'gemini'}
IMAGE='/media/seo-share/catalog-d7d4c32af9652177616a-1200x630.jpg'

def load_catalog(path=CATALOG):
    path=Path(path).resolve(strict=True)
    data=json.loads(path.read_text('utf-8'))
    require(len(data['products'])==13 and {p['id']:p['slug'] for p in data['products']}==ALLOWED,'Unexpected copy scope')
    require(len(data['media'])==3 and {(p['table'],p['id']):p['slug'] for p in data['media']}==MEDIA,'Unexpected media scope')
    digest=hashlib.sha256(path.read_bytes())
    digest.update((path.parent/'before.json').read_bytes())
    for product in data['products']:
        source=(path.parent/product['file']).resolve(strict=True)
        require(source.parent==path.parent and source.suffix=='.html','Content escaped reviewed folder')
        raw=source.read_bytes();digest.update(raw);product['content']=raw.decode('utf-8').strip()
        require(len(product['seo_description'])>=70,'Description too short')
    require(all(p['image']==IMAGE for p in data['media']),'Unreviewed image path')
    digest.update((ROOT/'frontend/user/public'/IMAGE.lstrip('/')).read_bytes())
    return data,digest.hexdigest()

def change(values,row,table,column,target):
    if row[column]!=target:values.append({'table':table,'id':row['id'],'column':column,'before':row[column],'after':target})

def identity(db,table,product):
    row=db.execute('SELECT * FROM '+table+' WHERE id=?',(product['id'],)).fetchone()
    require(row is not None and row['slug']==product['slug'] and row['is_active']==1 and row['deleted_at'] is None,'Product/category identity changed: '+product['slug'])
    return row

def plan(db,catalog,baseline,now):
    changes=[]
    for p in catalog['products']:
        row=identity(db,'products',p);before=baseline[p['slug']]
        require(row['category_id']==p['category_id'] and json.loads(row['title_json']).get('zh-CN')==p['title'],'Product category/title changed')
        for name in ['description','content']:
            column=name+'_json'
            value=localized(row[column],before[name].get('zh-CN',''),p[name],p['slug']+'/'+name)
            change(changes,row,'products',column,value)
        old=row['seo_meta_json'];seo=json.loads(old or '{}') or {};updated=deepcopy(seo)
        for field,target in [('title',p['seo_title']),('description',p['seo_description'])]:
            current=updated.setdefault(field,{})
            require(isinstance(current,dict),'Unexpected SEO metadata shape')
            expected=(before.get('seo_meta') or {}).get(field,{}) or {}
            require(isinstance(expected,dict) and current.get('zh-CN','') in (expected.get('zh-CN',''),target),'Newer SEO copy found: '+p['slug'])
            current['zh-CN']=target
        if updated!=seo:change(changes,row,'products','seo_meta_json',encode(updated,old))
    for p in catalog['media']:
        table=p['table'];row=identity(db,table,p)
        before=baseline[('category:' if table=='categories' else '')+p['slug']]
        if table=='products':
            require(row['category_id']==before['category_id'] and json.loads(row['title_json'])==before['title'],'Image product identity changed')
            old=json.loads(row['images'] or '[]')
            target=[p['image'] if value=='/catalog-products/qzweb/aa16fb0f3bb8f6933985a4c8df660946.jpg' else value for value in before['images']]
            require(p['image'] in target and old in (before['images'],target),'Newer product images found')
            if old!=target:change(changes,row,table,'images',encode(target,row['images']))
        else:
            require(row['icon'] in (before['icon'],p['image']),'Newer category icon found')
            change(changes,row,table,'icon',p['image'])
        old=row['seo_meta_json'];seo=json.loads(old or '{}') or {};updated=deepcopy(seo)
        for field,target in [('og_image',p['image']),('og_image_width',1200),('og_image_height',630)]:
            require(seo.get(field) in ((before.get('seo_meta') or {}).get(field),target),'Newer image metadata found')
            updated[field]=target
        if updated!=seo:change(changes,row,table,'seo_meta_json',encode(updated,old))
    for id in sorted({c['id'] for c in changes if c['table']=='products'}):
        row=db.execute('SELECT * FROM products WHERE id=?',(id,)).fetchone()
        change(changes,row,'products','updated_at',now)
    return changes

def allowed_columns(table,id):
    if table=='products' and id in ALLOWED:return {'description_json','content_json','seo_meta_json','updated_at'}
    if (table,id) in MEDIA:return {'images','seo_meta_json','updated_at'} if table=='products' else {'icon','seo_meta_json'}
    return set()

def protected_fingerprints(db):
    result={}
    for table, in db.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name"):
        records=[]
        for row in db.execute('SELECT * FROM "'+table.replace('"','""')+'"'):
            values=dict(row)
            for key in allowed_columns(table,values.get('id')):values.pop(key,None)
            records.append(json.dumps(values,sort_keys=True,ensure_ascii=True,default=lambda value:{'blob':value.hex()}))
        result[table]={'rows':len(records),'sha256':hashlib.sha256('\n'.join(sorted(records)).encode()).hexdigest()}
    return result

def apply_transaction(db,catalog,baseline,now):
    try:
        db.execute('BEGIN IMMEDIATE');before=protected_fingerprints(db)
        changes=plan(db,catalog,baseline,now)
        for c in changes:
            require(c['column'] in allowed_columns(c['table'],c['id']),'Unexpected write scope')
            count=db.execute('UPDATE '+c['table']+' SET '+c['column']+'=? WHERE id=? AND '+c['column']+' IS ?',(c['after'],c['id'],c['before'])).rowcount
            require(count==1,'Concurrent copy change')
        require(protected_fingerprints(db)==before,'Non-copy data changed')
        require(plan(db,catalog,baseline,now)==[],'Update is not idempotent')
        db.commit()
        return {'changed_fields':len(changes),'rows_changed':len({(c['table'],c['id']) for c in changes}),'tables_checked':len(before),'non_copy_fields_preserved':True,'idempotent':True,'changes':changes}
    except BaseException:
        db.rollback();raise

def main():
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--database',required=True);parser.add_argument('--catalog',default=str(CATALOG))
    parser.add_argument('--approval-file');parser.add_argument('--backup-root')
    modes=parser.add_mutually_exclusive_group();modes.add_argument('--apply',action='store_true');modes.add_argument('--apply-local',action='store_true')
    args=parser.parse_args();database=Path(args.database).resolve(strict=True)
    catalog,digest=load_catalog(args.catalog)
    baseline=json.loads((Path(args.catalog).parent/'before.json').read_text('utf-8'))
    if args.apply:approved(args.approval_file,digest)
    if args.apply_local:require(database.is_relative_to((ROOT/'work').resolve()),'Draft application is restricted to this checkout work/ database')
    now=datetime.now(timezone.utc).isoformat()
    with sqlite3.connect('file:'+database.as_posix()+'?mode='+('rw' if args.apply or args.apply_local else 'ro'),uri=True) as db:
        db.row_factory=sqlite3.Row
        if not args.apply and not args.apply_local:
            print(json.dumps({'mode':'plan','copy_sha256':digest,'status':catalog['status'],'changed_fields':len(plan(db,catalog,baseline,now))}));return
        require(args.backup_root,'An explicit backup directory is required')
        backup=Path(args.backup_root).resolve()/('seo-v5-'+datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S%fZ'))
        backup.mkdir(parents=True,exist_ok=False)
        with sqlite3.connect(backup/'shop.db') as dst:db.backup(dst)
        result=apply_transaction(db,catalog,baseline,now)
        result.update(copy_sha256=digest,mode='local-draft' if args.apply_local else 'approved-publication',backup=str(backup),time=now)
        (backup/'report.json').write_text(json.dumps(result,ensure_ascii=False,indent=2,default=lambda value:{'blob':value.hex()}),encoding='utf-8')
        print(json.dumps({key:value for key,value in result.items() if key!='changes'},ensure_ascii=False))

if __name__=='__main__':main()
