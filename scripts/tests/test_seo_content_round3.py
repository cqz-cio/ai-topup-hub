import importlib.util
import json
from pathlib import Path
import sqlite3
import sys
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import seo_content_round3 as batch


class ThirdBatchTests(unittest.TestCase):
    def setUp(self):
        self.catalog = batch.load_catalog()
        self.db = sqlite3.connect(':memory:'); self.db.row_factory = sqlite3.Row
        self.db.executescript('''
          CREATE TABLE settings (key TEXT PRIMARY KEY, value_json BLOB);
          CREATE TABLE posts (id INTEGER PRIMARY KEY, slug TEXT UNIQUE, type TEXT, title_json BLOB, summary_json BLOB, content_json BLOB, thumbnail TEXT, category_id INTEGER, is_published INTEGER, published_at TEXT, created_at TEXT, deleted_at TEXT);
          CREATE TABLE products (id INTEGER PRIMARY KEY, price TEXT, private_delivery TEXT);
          INSERT INTO products VALUES (1, '123.45', 'test-private-value');
        ''')
        about = {'version': 1, 'supportWechat': 'unchanged', 'unrelated': 'keep'}
        for rule in self.catalog['about']:
            about[rule['section']] = [{}, {}, {'title': {'zh-CN': rule['title']}, 'text': {'zh-CN': rule['before'][0], 'en-US': 'preserve translation'}}]
        self.db.execute('INSERT INTO settings VALUES (?,?)', ('about_page_config', batch.encode(about)))
        self.db.execute('INSERT INTO settings VALUES (?,?)', ('site_config', b'{"payment":"untouched"}'))
        self.db.commit()

    def tearDown(self): self.db.close()

    def test_scope_and_repeat(self):
        result = batch.apply_transaction(self.db, self.catalog)
        self.assertEqual((result['created_articles'], result['changed_about_records']), (3, 1))
        self.assertEqual(tuple(self.db.execute('SELECT * FROM products').fetchone()), (1, '123.45', 'test-private-value'))
        about = json.loads(self.db.execute("SELECT value_json FROM settings WHERE key='about_page_config'").fetchone()[0])
        self.assertEqual(about['services'][2]['text']['en-US'], 'preserve translation')
        self.assertEqual(about['unrelated'], 'keep')
        self.assertEqual(batch.apply_transaction(self.db, self.catalog)['created_articles'], 0)

    def test_draft_collision_aborts(self):
        self.db.execute('INSERT INTO posts (slug,type,is_published) VALUES (?,?,?)', (self.catalog['posts'][0]['slug'], 'blog', 0)); self.db.commit()
        before = batch.table_rows(self.db, 'settings')
        with self.assertRaisesRegex(ValueError, 'slug already in use'): batch.apply_transaction(self.db, self.catalog)
        self.assertEqual(batch.table_rows(self.db, 'settings'), before)

    def test_newer_about_edit_is_preserved(self):
        row = self.db.execute("SELECT value_json FROM settings WHERE key='about_page_config'").fetchone()
        value = json.loads(row[0]); value['recharge'][2]['text']['zh-CN'] = 'new operator text'
        self.db.execute("UPDATE settings SET value_json=? WHERE key='about_page_config'", (batch.encode(value),)); self.db.commit()
        with self.assertRaisesRegex(ValueError, 'Newer about copy'): batch.apply_transaction(self.db, self.catalog)
        self.assertEqual(self.db.execute('SELECT count(*) FROM posts').fetchone()[0], 0)

    def test_insert_failure_rolls_back_about_edit(self):
        self.db.execute("CREATE TRIGGER reject_article BEFORE INSERT ON posts BEGIN SELECT RAISE(ABORT, 'test failure'); END")
        before = batch.table_rows(self.db, 'settings')
        with self.assertRaises(sqlite3.DatabaseError): batch.apply_transaction(self.db, self.catalog)
        self.assertEqual(batch.table_rows(self.db, 'settings'), before)

if __name__ == '__main__': unittest.main()
