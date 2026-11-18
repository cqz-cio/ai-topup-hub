"""Exact SEO v6 two-field transaction. No network, prices or order mutations."""
import base64
from datetime import datetime, timezone
import hashlib
import json
import sqlite3

OLD = '本站 Plus 兑换码没有标注菲区，不应混入这四款菲区 Pro 的选品表，也不能用 Pro 档位替代 Plus 套餐。'
NEW = 'Plus 与本节四款菲区 Pro 是不同套餐，请分别按各自商品说明选择，不能用 Pro 档位或资格规则替代 Plus。'
PRO = {45:'chatgpt-pro-100-ph',31:'chatgpt-pro-200-ph',33:'chatgpt-pro-500-ph',32:'chatgpt-pro-200-renew-ph'}

def require(ok, message):
    if not ok: raise ValueError(message)

def text(value):
    return value.decode('utf-8') if isinstance(value,bytes) else value

def typed(value, original):
    return value.encode('utf-8') if isinstance(original,bytes) else value

def pack(value):
    return {'bytes_base64':base64.b64encode(value).decode()} if isinstance(value,bytes) else value

def unpack(value):
    return base64.b64decode(value['bytes_base64']) if isinstance(value,dict) and 'bytes_base64' in value else value

def snapshot(db):
    result={}
    for table, in db.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name"):
        quoted='"'+table.replace('"','""')+'"'
        result[table]=[dict(row) for row in db.execute('SELECT * FROM '+quoted+' ORDER BY rowid')]
    return result

def fingerprints(snapshot_value):
    return {t:hashlib.sha256(json.dumps(rows,sort_keys=True,ensure_ascii=False,default=pack,separators=(',',':')).encode()).hexdigest() for t,rows in snapshot_value.items()}

def plan(db, expected_content):
    product=db.execute('SELECT * FROM products WHERE id=30').fetchone()
    require(product is not None and product['slug']=='chatgpt-plus-code' and product['category_id']==9 and product['is_active']==1 and product['deleted_at'] is None,'Plus identity/state changed')
    require(json.loads(product['title_json']).get('zh-CN')=='ChatGPT Plus 兑换码｜自动发码','Plus title changed')
    require(json.loads(product['tags'])==['ChatGpt 菲区'],'Plus tags differ from the approved old value')
    post=db.execute('SELECT * FROM posts WHERE id=10').fetchone()
    require(post is not None and post['slug']=='chatgpt-ph-recharge-guide' and post['type']=='blog' and post['is_published']==1 and post['deleted_at'] is None,'Article identity/state changed')
    raw=text(post['content_json'])
    require(raw==expected_content,'Article changed since the read-only preflight')
    content=json.loads(raw)
    require(content.get('zh-CN','').count(OLD)==1 and NEW not in content['zh-CN'],'Article does not contain the exact approved old sentence once')
    require(raw.count(OLD)==1,'Unexpected JSON encoding; refuse reformatting other languages')
    replacement=raw.replace(OLD,NEW,1)
    expected=dict(content);expected['zh-CN']=content['zh-CN'].replace(OLD,NEW,1)
    require(json.loads(replacement)==expected,'Unexpected language/content change')
    for ident,slug in PRO.items():
        row=db.execute('SELECT * FROM products WHERE id=?',(ident,)).fetchone()
        require(row is not None and row['slug']==slug and row['is_active']==1 and row['deleted_at'] is None and json.loads(row['tags'])==['ChatGpt 菲区'],'PH Pro identity/tag changed: '+str(ident))
    return product,post,typed(replacement,post['content_json'])

def apply(db,expected_content,now=None):
    now=now or datetime.now(timezone.utc).isoformat()
    try:
        db.execute('BEGIN IMMEDIATE')
        product,post,replacement=plan(db,expected_content)
        before=snapshot(db)
        changes=[
            {'table':'products','id':30,'column':'tags','before':product['tags'],'after':typed('[]',product['tags'])},
            {'table':'products','id':30,'column':'updated_at','before':product['updated_at'],'after':now},
            {'table':'posts','id':10,'column':'content_json','before':post['content_json'],'after':replacement}
        ]
        expected=snapshot(db)
        require(db.execute('UPDATE products SET tags=?,updated_at=? WHERE id=30 AND tags IS ? AND updated_at IS ?',(changes[0]['after'],now,product['tags'],product['updated_at'])).rowcount==1,'Concurrent Plus update')
        require(db.execute('UPDATE posts SET content_json=? WHERE id=10 AND content_json IS ?',(replacement,post['content_json'])).rowcount==1,'Concurrent article update')
        for c in changes:
            next(r for r in expected[c['table']] if r['id']==c['id'])[c['column']]=c['after']
        after=snapshot(db)
        require(after==expected,'Protected table/field changed')
        receipt={'status':'applied','time':now,'tables_checked':len(before),'non_target_values_preserved':True,
                 'before_fingerprints':fingerprints(before),'after_fingerprints':fingerprints(after),
                 'changes':[{**c,'before':pack(c['before']),'after':pack(c['after'])} for c in changes],
                 'post_has_no_updated_at_column':True,'post_published_at_preserved':True}
        db.commit()
        return receipt
    except BaseException:
        db.rollback()
        raise

def rollback(db,receipt):
    try:
        db.execute('BEGIN IMMEDIATE')
        before=snapshot(db);expected=snapshot(db)
        for c in receipt['changes']:
            require((c['table'],c['id'],c['column']) in [('products',30,'tags'),('products',30,'updated_at'),('posts',10,'content_json')],'Unexpected rollback target')
            row=db.execute('SELECT * FROM '+c['table']+' WHERE id=?',(c['id'],)).fetchone()
            require(row is not None and row[c['column']]==unpack(c['after']),'Later target edit; refuse rollback')
        require(db.execute('SELECT slug FROM products WHERE id=30').fetchone()[0]=='chatgpt-plus-code','Plus identity changed')
        require(db.execute('SELECT slug FROM posts WHERE id=10').fetchone()[0]=='chatgpt-ph-recharge-guide','Article identity changed')
        for c in receipt['changes']:
            require(db.execute('UPDATE '+c['table']+' SET '+c['column']+'=? WHERE id=? AND '+c['column']+' IS ?',(unpack(c['before']),c['id'],unpack(c['after']))).rowcount==1,'Rollback compare failed')
            next(r for r in expected[c['table']] if r['id']==c['id'])[c['column']]=unpack(c['before'])
        require(snapshot(db)==expected,'Rollback changed a protected field')
        db.commit()
        return {'status':'rolled_back_exact_fields','tables_checked':len(before),'non_target_values_preserved':True}
    except BaseException:
        db.rollback()
        raise
