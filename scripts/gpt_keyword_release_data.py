"""Guarded Chinese-copy transaction for the approved 2026-11-27 storefront release."""
import hashlib
import json
import sqlite3

from preview_gpt_keyword_copy import allowed, target


def approved_state(db, m):
    values=[]
    for row in m['review']:
        key, ident=target(db,row)
        for edit in row['fields']:
            column,path=edit['column'],edit['path']
            if not allowed(row['table'],column,path):
                raise ValueError('Unexpected public field')
            raw=db.execute(f"SELECT {column} FROM {row['table']} WHERE {key}=?",(ident,)).fetchone()
            if raw is None:raise ValueError('Missing record')
            value=json.loads(raw[0])
            for part in path:value=value[part]
            values.append(value)
    return values

def approved_hashes(m):
    old=[r[3] for r in m['baseline_guards']]
    new=[digest_text(e['after']) for r in m['review'] for e in r['fields']]
    if len(old)!=25 or len(new)!=25:raise ValueError('Review scope changed')
    for row,check in zip([ (r,e) for r in m['review'] for e in r['fields']],m['baseline_guards']):
        r,e=row
        if check[:3]!=[r.get('slug',r.get('key')),e['column'],e['path']] or check[3] not in e['before_sha256']:
            raise ValueError('Unexpected base field')
    return old,new

def digest_text(v):
    return hashlib.sha256(v.encode('utf8')).hexdigest()

def verified_state(db,m,which):
    values=approved_state(db,m);want=approved_hashes(m)[which=='new']
    if [digest_text(v) for v in values]!=want:raise ValueError('Reviewed fields were edited concurrently')
    return values

def transition(db,m,old_values,direction,original_columns=None):
    old,new=approved_hashes(m)
    desired=([e['after'] for r in m['review'] for e in r['fields']] if direction=='new' else old_values)
    before_hash,after_hash=(old,new) if direction=='new' else (new,old)
    if [digest_text(v) for v in desired]!=after_hash:raise ValueError('Unverified desired values')
    controls={row['table']:{f['column'] for r in m['review'] if r['table']==row['table'] for f in r['fields']} for row in m['review']}
    def auth(code,table,col,schema,trigger):
        if code in (sqlite3.SQLITE_INSERT,sqlite3.SQLITE_DELETE) or code==sqlite3.SQLITE_UPDATE and (trigger or col not in controls.get(table,set())):return sqlite3.SQLITE_DENY
        return sqlite3.SQLITE_OK
    db.set_authorizer(auth)
    db.execute('BEGIN IMMEDIATE');counter=0
    try:
        current=approved_state(db,m)
        if [digest_text(v) for v in current]!=before_hash:raise ValueError('Reviewed fields changed before the transaction')
        for row in m['review']:
            key,ident=target(db,row)
            for edit in row['fields']:
                col,path=edit['column'],edit['path'];raw=db.execute(f"SELECT {col} FROM {row['table']} WHERE {key}=?",(ident,)).fetchone()[0]
                data=json.loads(raw);parent=data
                for part in path[:-1]:parent=parent[part]
                if parent[path[-1]]!=current[counter]:raise ValueError('Concurrent nested-field change')
                parent[path[-1]]=desired[counter]
                text=json.dumps(data,ensure_ascii=False,separators=(',',':'));output=text.encode() if isinstance(raw,bytes) else text
                if direction=='old' and original_columns is not None:
                    original=original_columns[(row['table'],ident,col)]
                    if json.loads(original)==data:output=original
                n=db.total_changes
                changed=db.execute(f"UPDATE {row['table']} SET {col}=? WHERE {key}=? AND {col} IS ?",(output,ident,raw))
                if changed.rowcount!=1 or db.total_changes!=n+1:raise ValueError('Unexpected number of writes')
                counter+=1
        if approved_state(db,m)!=desired:raise ValueError('Write verification failed')
        db.commit()
    except BaseException:
        db.rollback();raise
    finally:db.set_authorizer(None)
    return counter
