from contextlib import closing
"""Remove redemption addresses from public copy; leave delivery and business data intact."""
import argparse
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import re
import sqlite3
import time

HOST = re.compile(r'(?:cdk\.bar|cdkai\.kdns\.fr)', re.I)
ADDRESS = re.compile(r'(?:(?:https?:)?//)?(?:www\.)?(?:cdk\.bar|cdkai\.kdns\.fr)(?:/[^\s<>"\'。，；]*)?', re.I)
ANCHOR = re.compile(r'<a\b[^>]*>.*?</a\s*>', re.I | re.S)

def public_copy(value, locale='zh-CN'):
    if isinstance(value, dict):
        return {k: public_copy(v, k if k in ['zh-CN','zh-TW','en-US'] else locale) for k,v in value.items()}
    if isinstance(value, list):
        return [public_copy(v,locale) for v in value]
    if not isinstance(value,str) or not HOST.search(value):
        return value
    label = {'zh-CN':'订单交付说明中的兑换入口','zh-TW':'訂單交付說明中的兌換入口','en-US':'the redemption link in your order delivery instructions'}[locale]
    value = ANCHOR.sub(lambda m: label if HOST.search(m[0]) else m[0], value)
    return ADDRESS.sub(label,value)

SCOPES = {'products':['title_json','description_json','content_json','seo_meta_json'], 'posts':['title_json','summary_json','content_json','seo_meta_json'], 'settings':['value_json']}
SETTING_KEYS = {'home_announcement','about_page_config'}

def plan(db):
    updates=[]
    for table,fields in SCOPES.items():
        columns=[r[1] for r in db.execute('PRAGMA table_info('+table+')')]
        if not columns: continue
        for row in db.execute('SELECT rowid,* FROM '+table):
            item=dict(zip(['rowid']+columns,row))
            if table=='settings' and item['key'] not in SETTING_KEYS: continue
            for field in fields:
                original=item.get(field)
                if original is None: continue
                value=json.loads(original)
                revised=public_copy(value)
                if revised!=value:
                    encoded=json.dumps(revised,ensure_ascii=False,separators=(',',':'))
                    if isinstance(original,bytes): encoded=encoded.encode()
                    updates.append((table,item['rowid'],field,original,encoded))
    return updates

def snapshot(db):
    return {name:list(db.execute('SELECT rowid,* FROM "'+name.replace('"','""')+'" ORDER BY rowid')) for name, in db.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")}

def apply_database(path, *, apply=False, backup_dir=None):
    path=Path(path).resolve(strict=True)
    if apply and backup_dir is None: raise ValueError('A backup directory is required')
    with closing(sqlite3.connect(path.as_uri()+('?mode=rw' if apply else '?mode=ro'),uri=True,timeout=5)) as db, db:
        updates=plan(db);backup_path=None;protected=False
        if apply and updates:
            backup_dir=Path(backup_dir);backup_dir.mkdir(parents=True,exist_ok=True)
            backup_path=backup_dir/('before-public-redemption-'+datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S%fZ')+'.db')
            backup_path.touch(exist_ok=False);backup_path.chmod(0o600)
            deadline=time.monotonic()+30
            def progress(*_):
                if time.monotonic()>deadline: raise TimeoutError('Backup exceeded deadline')
            with closing(sqlite3.connect(backup_path,timeout=5)) as backup:
                db.backup(backup,pages=128,progress=progress,sleep=.05)
                if backup.execute('PRAGMA quick_check').fetchone()!=('ok',): raise RuntimeError('Backup integrity failed')
            db.execute('BEGIN IMMEDIATE');updates=plan(db);before=snapshot(db)
            expected={name:[list(row) for row in rows] for name,rows in before.items()}
            for table,rowid,field,old,new in updates:
                columns=[r[1] for r in db.execute('PRAGMA table_info('+table+')')]
                changed=db.execute('UPDATE '+table+' SET '+field+'=? WHERE rowid=? AND '+field+'=?',(new,rowid,old)).rowcount
                if changed!=1: raise RuntimeError('Public copy changed during update')
                for row in expected[table]:
                    if row[0]==rowid: row[columns.index(field)+1]=new
            after=snapshot(db)
            if after!={k:[tuple(r) for r in rows] for k,rows in expected.items()} or plan(db): raise RuntimeError('Unexpected database changes')
            db.commit();protected=True
        return {'mode':'apply' if apply else 'plan','planned_fields':len(updates),'changed_fields':len(updates) if apply else 0,'fields':[{'table':t,'rowid':i,'field':f} for t,i,f,_,_ in updates],'backup':str(backup_path) if backup_path else None,'all_other_fields_unchanged':protected or not updates}

if __name__=='__main__':
    parser=argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--database',required=True);parser.add_argument('--apply',action='store_true');parser.add_argument('--backup-dir')
    args=parser.parse_args()
    print(json.dumps(apply_database(args.database,apply=args.apply,backup_dir=args.backup_dir),ensure_ascii=True))
