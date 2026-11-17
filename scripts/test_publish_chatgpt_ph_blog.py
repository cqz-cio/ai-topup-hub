import copy
import importlib.util
import json
from pathlib import Path
import sqlite3
import unittest

spec=importlib.util.spec_from_file_location('publish',Path(__file__).with_name('publish_chatgpt_ph_blog.py'))
m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m)
ROOT=Path(__file__).resolve().parents[1]


class PublicationTests(unittest.TestCase):
    def setUp(self):
        self.db=sqlite3.connect(':memory:');self.db.row_factory=sqlite3.Row
        self.db.executescript('''CREATE TABLE products(id INTEGER PRIMARY KEY,slug TEXT,title_json BLOB,is_active INTEGER,deleted_at TEXT,price_amount TEXT);
        CREATE TABLE posts(id INTEGER PRIMARY KEY,slug TEXT UNIQUE,type TEXT,title_json BLOB,summary_json BLOB,content_json BLOB,thumbnail TEXT,category_id INTEGER,is_published INTEGER,published_at TEXT,created_at TEXT,deleted_at TEXT);
        CREATE TABLE post_products(id INTEGER PRIMARY KEY,post_id INTEGER,product_id INTEGER,sort INTEGER,created_at TEXT,UNIQUE(post_id,product_id));
        CREATE TABLE protected_orders(id INTEGER PRIMARY KEY,value TEXT);INSERT INTO protected_orders VALUES(1,'keep');''')
        self.data,_=m.load(ROOT/'docs/catalog/blog-ph-chatgpt-20261117')
        for ident,slug in m.PRODUCTS.items():
            self.db.execute('INSERT INTO products VALUES(?,?,?,?,?,?)',(ident,slug,json.dumps({'zh-CN':'ChatGPT Pro 菲区'}).encode(),1,None,'unchanged'))
        for p in self.data['backlinks']:
            text='<p>Existing '+p['slug']+'</p>'
            p['before_sha256']=m.digest(text)
            values=(p['id'],p['slug'],'blog',b'{}',b'{}',json.dumps({'zh-CN':text,'en-US':'Preserve English','zh-TW':'保留繁中'}).encode(),'',None,1,'2026-11-09','2026-11-09',None)
            self.db.execute('INSERT INTO posts VALUES(?,?,?,?,?,?,?,?,?,?,?,?)',values)
        self.db.commit();self.before=m.snapshot(self.db)

    def tearDown(self):self.db.close()

    def test_publication_idempotence_and_exact_rollback(self):
        result=m.apply_transaction(self.db,self.data,'2026-11-17T08:00:00+00:00')
        self.assertEqual((result['articles_created'],result['relations_created'],result['backlinks_updated']),(1,4,2))
        self.assertEqual(m.apply_transaction(self.db,self.data,'later')['status'],'already_applied')
        m.rollback_transaction(self.db,result);self.assertEqual(m.snapshot(self.db),self.before)

    def test_wrong_product_identity_refused_before_any_write(self):
        self.db.execute("UPDATE products SET slug='not-ph' WHERE id=31");self.db.commit()
        before=m.snapshot(self.db)
        with self.assertRaisesRegex(ValueError,'identity'):m.apply_transaction(self.db,self.data,'now')
        self.assertEqual(m.snapshot(self.db),before)

    def test_non_ph_title_refused(self):
        self.db.execute('UPDATE products SET title_json=? WHERE id=33',(b'{"zh-CN":"Pro"}',));self.db.commit()
        with self.assertRaisesRegex(ValueError,'Not a PH'):m.apply_transaction(self.db,self.data,'now')

    def test_concurrent_editor_copy_is_preserved(self):
        self.db.execute('UPDATE posts SET content_json=? WHERE id=?',(b'{"zh-CN":"new edit"}',self.data['backlinks'][0]['id']));self.db.commit()
        before=m.snapshot(self.db)
        with self.assertRaisesRegex(ValueError,'Newer article'):m.apply_transaction(self.db,self.data,'now')
        self.assertEqual(m.snapshot(self.db),before)

    def test_other_languages_dates_and_prices_preserved(self):
        m.apply_transaction(self.db,self.data,'now')
        after=m.snapshot(self.db)
        self.assertEqual(after['products'],self.before['products'])
        for before in self.before['posts']:
            row=next(p for p in after['posts'] if p['id']==before['id'])
            self.assertEqual(row['published_at'],before['published_at'])
            self.assertEqual(type(row['content_json']),type(before['content_json']))
            for locale in ['en-US','zh-TW']:self.assertEqual(json.loads(row['content_json'])[locale],json.loads(before['content_json'])[locale])

    def test_existing_slug_with_different_copy_refused(self):
        m.apply_transaction(self.db,self.data,'now')
        self.db.execute('UPDATE posts SET summary_json=? WHERE slug=?',(b'{}',m.SLUG));self.db.commit()
        before=m.snapshot(self.db)
        with self.assertRaisesRegex(ValueError,'differs'):m.apply_transaction(self.db,self.data,'now')
        self.assertEqual(m.snapshot(self.db),before)

    def test_trigger_write_to_protected_table_rolls_back_everything(self):
        self.db.executescript("CREATE TRIGGER unexpected AFTER INSERT ON posts BEGIN UPDATE protected_orders SET value='bad'; END;")
        with self.assertRaisesRegex(ValueError,'Protected'):m.apply_transaction(self.db,self.data,'now')
        self.assertEqual(m.snapshot(self.db),self.before)

    def test_rollback_refuses_later_editor_changes(self):
        result=m.apply_transaction(self.db,self.data,'now')
        self.db.execute('UPDATE posts SET summary_json=? WHERE slug=?',(b'{}',m.SLUG));self.db.commit()
        before=m.snapshot(self.db)
        with self.assertRaisesRegex(ValueError,'changed'):m.rollback_transaction(self.db,result)
        self.assertEqual(m.snapshot(self.db),before)


if __name__=='__main__':unittest.main()
