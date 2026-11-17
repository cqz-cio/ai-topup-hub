"""Publish the approved PH guide, four product relations and two backlinks.

Default is a read-only plan. Apply requires the exact content digest, a consistent
backup, a rehearsal and a transaction that checks every other database value.
No program deployment, payment, order, catalog or credential updates are made.
"""
import argparse
import copy
from datetime import datetime, timezone
import hashlib
import json
from pathlib import Path
import sqlite3
import time

SLUG = 'chatgpt-ph-recharge-guide'
PRODUCTS = {45: 'chatgpt-pro-100-ph', 31: 'chatgpt-pro-200-ph',
            33: 'chatgpt-pro-500-ph', 32: 'chatgpt-pro-200-renew-ph'}
BACKLINKS = {'chatgpt-code-region-check', 'chatgpt-payment-methods'}
TITLE = '菲区 ChatGPT 充值怎么选、怎么兑'


def require(ok, message):
    if not ok:
        raise ValueError(message)


def digest(value):
    return hashlib.sha256(value.encode('utf-8')).hexdigest()


def encoded(value, original=None):
    text = json.dumps(value, ensure_ascii=False, separators=(',', ':'))
    return text.encode() if original is None or isinstance(original, bytes) else text


def load(folder):
    folder = Path(folder).resolve()
    catalog = json.loads((folder / 'catalog.json').read_text('utf-8'))
    require(catalog['publication_approved'] is True, 'Owner publication approval required')
    require(len(catalog['owner_approval']['answers']) == 4, 'Missing owner answers')
    require(len(catalog['posts']) == 1, 'Unexpected article scope')
    post = catalog['posts'][0]
    require(post['slug'] == SLUG and post['type'] == 'blog' and post['title']['zh-CN'] == TITLE, 'Article identity changed')
    require({p['id_observed']: p['slug'] for p in catalog['approved_related_products']} == PRODUCTS, 'Unexpected product scope')
    body_path = (folder / post['content_files']['zh-CN']).resolve(strict=True)
    require(body_path.parent == folder, 'Content path escaped catalog')
    body = body_path.read_text('utf-8').strip()
    require(body.count('<strong>你们卖 ChatGPT 成品号吗？</strong>') == 1, 'Approved sixth FAQ missing')
    require(not any(s in body for s in ['需店主确认', '须店主确认', 'TODO', '待填写']), 'Internal marker in article')
    backlinks = json.loads((folder / 'backlinks.json').read_text('utf-8'))
    require(len(backlinks) == 2 and {p['slug'] for p in backlinks} == BACKLINKS, 'Unexpected backlink scope')
    suffix = '<p>延伸阅读：<a href="/blog/' + SLUG + '">' + TITLE + '</a>。</p>'
    for item in backlinks:
        require(item['append_html'] == suffix and len(item['before_sha256']) == 64, 'Backlink content changed')
    payload = {'slug': SLUG, 'title': post['title'], 'summary': post['summary'],
               'content': {'zh-CN': body, 'zh-TW': '', 'en-US': ''},
               'products': list(PRODUCTS.items()), 'backlinks': backlinks}
    return payload, digest(json.dumps(payload, ensure_ascii=False, sort_keys=True))


def rows(db, table):
    name = '"' + table.replace('"', '""') + '"'
    return [dict(r) for r in db.execute('SELECT * FROM ' + name + ' ORDER BY rowid')]


def snapshot(db):
    return {t: rows(db, t) for t, in db.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name")}


def plan(db, data):
    for ident, slug in PRODUCTS.items():
        p = db.execute('SELECT * FROM products WHERE id=?', (ident,)).fetchone()
        require(p is not None and p['slug'] == slug and p['is_active'] == 1 and p['deleted_at'] is None, 'PH product identity/state changed: ' + str(ident))
        title = json.loads(p['title_json']).get('zh-CN', '')
        require('菲区' in title and 'Pro' in title, 'Not a PH Pro product: ' + str(ident))
    changes = []
    for item in data['backlinks']:
        p = db.execute('SELECT * FROM posts WHERE slug=?', (item['slug'],)).fetchone()
        require(p is not None and p['id'] == item['id'] and p['type'] == 'blog' and p['is_published'] == 1 and p['deleted_at'] is None, 'Backlink article identity changed')
        value = json.loads(p['content_json']); before = value['zh-CN']
        suffix = '\n' + item['append_html']
        if digest(before) == item['before_sha256']:
            require('/blog/' + SLUG not in before, 'Backlink already present in different form')
            value['zh-CN'] = before + suffix
            changes.append({'id': p['id'], 'slug': p['slug'], 'before': p['content_json'], 'after': encoded(value, p['content_json'])})
        else:
            require(before.endswith(suffix) and digest(before[:-len(suffix)]) == item['before_sha256'], 'Newer article content found: ' + item['slug'])
    post = db.execute('SELECT * FROM posts WHERE slug=?', (SLUG,)).fetchone()
    if post is not None:
        require(post['type'] == 'blog' and post['is_published'] == 1 and post['deleted_at'] is None and post['published_at'] and post['category_id'] is None and not post['thumbnail'], 'Article slug is already in use')
        require(all(json.loads(post[k + '_json']) == data[k] for k in ['title', 'summary', 'content']), 'Existing article differs from approved content')
        relations = db.execute('SELECT product_id,sort FROM post_products WHERE post_id=? ORDER BY sort,id', (post['id'],)).fetchall()
        require([(r['product_id'], r['sort']) for r in relations] == [(p, i) for i, p in enumerate(PRODUCTS)], 'Existing product relations differ')
    return changes, post is None


def apply_transaction(db, data, now):
    try:
        db.execute('BEGIN IMMEDIATE')
        changes, insert = plan(db, data)
        expected = snapshot(db)
        for c in changes:
            require(db.execute('UPDATE posts SET content_json=? WHERE id=? AND content_json IS ?', (c['after'], c['id'], c['before'])).rowcount == 1, 'Concurrent article update')
            next(r for r in expected['posts'] if r['id'] == c['id'])['content_json'] = c['after']
        added_post = None; added_relations = []
        if insert:
            values = {k + '_json': encoded(data[k]) for k in ['title', 'summary', 'content']}
            values.update(slug=SLUG, type='blog', thumbnail='', category_id=None, is_published=1,
                          published_at=now, created_at=now, deleted_at=None)
            cols = list(values)
            cursor = db.execute('INSERT INTO posts (' + ','.join(cols) + ') VALUES (' + ','.join('?' for _ in cols) + ')', [values[k] for k in cols])
            added_post = {'id': cursor.lastrowid, **values}
            expected['posts'].append(added_post)
            for i, ident in enumerate(PRODUCTS):
                values = {'post_id': added_post['id'], 'product_id': ident, 'sort': i, 'created_at': now}
                cursor = db.execute('INSERT INTO post_products (post_id,product_id,sort,created_at) VALUES (?,?,?,?)', tuple(values.values()))
                relation = {'id': cursor.lastrowid, **values}; added_relations.append(relation)
                expected['post_products'].append(relation)
        require(snapshot(db) == expected, 'Protected table or field changed')
        require(plan(db, data) == ([], False), 'Publication is not idempotent')
        current = db.execute('SELECT id,published_at FROM posts WHERE slug=?', (SLUG,)).fetchone()
        db.commit()
        return {'status': 'applied' if insert or changes else 'already_applied', 'article_id': current['id'],
                'published_at': current['published_at'], 'articles_created': int(insert),
                'relations_created': len(added_relations), 'backlinks_updated': len(changes),
                'tables_checked': len(expected), 'all_other_values_preserved': True,
                'changes': changes, 'added_post': added_post, 'added_relations': added_relations}
    except BaseException:
        db.rollback(); raise


def rollback_transaction(db, report):
    """Remove only this exact insertion and reverse the two unchanged backlinks."""
    try:
        db.execute('BEGIN IMMEDIATE'); expected = snapshot(db)
        for c in report['changes']:
            require(db.execute('UPDATE posts SET content_json=? WHERE id=? AND content_json IS ?', (c['before'], c['id'], c['after'])).rowcount == 1, 'Backlink changed after publication; refuse rollback')
            next(r for r in expected['posts'] if r['id'] == c['id'])['content_json'] = c['before']
        post = report['added_post']
        if post:
            stored = db.execute('SELECT * FROM posts WHERE id=?', (post['id'],)).fetchone()
            require(stored is not None and dict(stored) == post, 'New article changed; refuse rollback')
            relations = [dict(r) for r in db.execute('SELECT * FROM post_products WHERE post_id=? ORDER BY id', (post['id'],))]
            require(relations == report['added_relations'], 'Relations changed; refuse rollback')
            db.execute('DELETE FROM post_products WHERE post_id=?', (post['id'],))
            db.execute('DELETE FROM posts WHERE id=?', (post['id'],))
            expected['posts'] = [r for r in expected['posts'] if r['id'] != post['id']]
            expected['post_products'] = [r for r in expected['post_products'] if r['post_id'] != post['id']]
        require(snapshot(db) == expected, 'Rollback affected protected data')
        db.commit()
    except BaseException:
        db.rollback(); raise


def backup_database(db, path):
    deadline = time.monotonic() + 25
    def progress(*_):
        if time.monotonic() > deadline:
            raise TimeoutError('Database backup deadline exceeded')
    with sqlite3.connect(path) as target:
        db.backup(target, pages=128, progress=progress, sleep=.05)
        require(target.execute('PRAGMA quick_check').fetchone()[0] == 'ok', 'Backup integrity failed')


def public_result(report):
    return {k: v for k, v in report.items() if k not in {'changes', 'added_post', 'added_relations'}}


def main():
    p = argparse.ArgumentParser(description=__doc__)
    p.add_argument('--database', required=True); p.add_argument('--catalog-dir', required=True)
    p.add_argument('--apply', action='store_true'); p.add_argument('--approved-sha256'); p.add_argument('--backup-root')
    args = p.parse_args(); data, sha = load(args.catalog_dir)
    database = Path(args.database).resolve(strict=True)
    with sqlite3.connect(database.as_uri() + ('?mode=rw' if args.apply else '?mode=ro'), uri=True, timeout=8) as db:
        db.row_factory = sqlite3.Row
        changes, insert = plan(db, data)
        if not args.apply:
            print(json.dumps({'status': 'planned', 'content_sha256': sha, 'articles': int(insert), 'relations': 4 if insert else 0, 'backlinks': len(changes)})); return
        require(args.approved_sha256 == sha and args.backup_root, 'Exact approval digest and backup are required')
        now = datetime.now(timezone.utc).isoformat()
        backup = Path(args.backup_root) / ('ph-blog-' + datetime.now(timezone.utc).strftime('%Y%m%dT%H%M%S%fZ'))
        backup.mkdir(parents=True, exist_ok=False, mode=0o700)
        backup_database(db, backup / 'before.db')
        with sqlite3.connect(backup / 'before.db') as saved, sqlite3.connect(':memory:') as rehearsal:
            saved.backup(rehearsal); rehearsal.row_factory = sqlite3.Row
            before = snapshot(rehearsal); trial = apply_transaction(rehearsal, data, now)
            again = apply_transaction(rehearsal, data, now)
            require(again['status'] == 'already_applied', 'Rehearsal repeated writes')
            rollback_transaction(rehearsal, trial)
            require(snapshot(rehearsal) == before, 'Targeted rollback rehearsal failed')
        report = apply_transaction(db, data, now)
        report.update(content_sha256=sha, backup=str(backup), rehearsal_and_idempotence_and_rollback_passed=True)
        (backup / 'report.json').write_text(json.dumps(report, ensure_ascii=False, indent=2, default=lambda b: {'blob': b.hex()}), encoding='utf-8')
        print(json.dumps(public_result(report), ensure_ascii=False))


if __name__ == '__main__':
    main()
