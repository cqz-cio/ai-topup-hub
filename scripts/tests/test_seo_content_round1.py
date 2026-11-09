import importlib.util
import json
from pathlib import Path
import sqlite3
import unittest

SCRIPT = Path(__file__).resolve().parents[1] / 'seo_content_round1.py'
spec = importlib.util.spec_from_file_location('seo_content_round1', SCRIPT)
batch = importlib.util.module_from_spec(spec)
spec.loader.exec_module(batch)


class ContentBatchTests(unittest.TestCase):
    def setUp(self):
        self.catalog = batch.load_catalog()
        self.db = sqlite3.connect(':memory:')
        self.db.row_factory = sqlite3.Row
        self.db.executescript('''
          CREATE TABLE products (id INTEGER PRIMARY KEY, slug TEXT, title_json BLOB, description_json BLOB, content_json BLOB, seo_meta_json BLOB, is_active INTEGER, fulfillment_type TEXT, deleted_at TEXT, price TEXT, instructions_json BLOB);
          CREATE TABLE settings (key TEXT PRIMARY KEY, value_json BLOB);
          CREATE TABLE posts (id INTEGER PRIMARY KEY, slug TEXT UNIQUE, type TEXT, title_json BLOB, summary_json BLOB, content_json BLOB, thumbnail TEXT, category_id INTEGER, is_published INTEGER, published_at TEXT, created_at TEXT, deleted_at TEXT);
          CREATE TABLE orders (id INTEGER PRIMARY KEY, private_delivery TEXT);
          INSERT INTO orders VALUES (1, 'private-test-value');
        ''')
        for pid, item in enumerate(self.catalog['products'], 100):
            fields = item['fields']
            localized = lambda text: {'zh-CN': text, 'en-US': 'preserved translation'}
            seo = {key: localized(fields['seo.' + key]['before'][0]) for key in ['title', 'description', 'keywords']}
            self.db.execute('INSERT INTO products VALUES (?,?,?,?,?,?,?,?,?,?,?)', (pid, item['slug'], batch.encode(localized(item['title'])), batch.encode(localized(fields['description']['before'][0])), batch.encode(localized(fields['content']['before'][0])), batch.encode(seo), 1, item['fulfillment'], None, '123.45', batch.encode({'zh-CN': 'private instructions'})))
        self.db.execute('INSERT INTO settings VALUES (?,?)', ('site_config', batch.encode({'brand': {'site_name': 'π星卡社', 'site_logo': '/existing.png'}, 'seo': {key: {'zh-CN': rule['before'][0], 'en-US': 'preserve'} for key, rule in self.catalog['site_seo'].items()}, 'payments': {'keep': True}})))
        self.db.commit()

    def tearDown(self):
        self.db.close()

    def test_changes_are_limited_and_second_apply_is_empty(self):
        before = [dict(row) for row in self.db.execute('SELECT * FROM products')]
        result = batch.apply_transaction(self.db, self.catalog)
        self.assertEqual(result['created_articles'], 3)
        self.assertEqual(result['changed_fields'], 7)
        for original in before:
            row = self.db.execute('SELECT * FROM products WHERE id=?', (original['id'],)).fetchone()
            for field in original:
                if field not in {'description_json', 'content_json', 'seo_meta_json'}:
                    self.assertEqual(row[field], original[field])
            self.assertEqual(json.loads(row['content_json'])['en-US'], 'preserved translation')
        self.assertEqual(self.db.execute('SELECT private_delivery FROM orders').fetchone()[0], 'private-test-value')
        self.assertEqual(batch.apply_transaction(self.db, self.catalog)['changed_fields'], 0)
        self.assertEqual(self.db.execute('SELECT count(*) FROM posts').fetchone()[0], 3)

    def test_newer_product_copy_aborts_whole_batch(self):
        self.db.execute('UPDATE products SET content_json=? WHERE id=101', (batch.encode({'zh-CN': 'newer operator edit'}),))
        self.db.commit()
        before = [dict(row) for row in self.db.execute('SELECT * FROM products')]
        with self.assertRaisesRegex(ValueError, 'Newer copy'):
            batch.apply_transaction(self.db, self.catalog)
        self.assertEqual([dict(row) for row in self.db.execute('SELECT * FROM products')], before)
        self.assertEqual(self.db.execute('SELECT count(*) FROM posts').fetchone()[0], 0)

    def test_existing_article_slug_is_not_overwritten(self):
        self.db.execute('INSERT INTO posts (slug,type,is_published,title_json) VALUES (?,?,?,?)', (self.catalog['posts'][0]['slug'], 'notice', 0, batch.encode({'zh-CN': 'existing draft'})))
        self.db.commit()
        with self.assertRaisesRegex(ValueError, 'slug already in use'):
            batch.apply_transaction(self.db, self.catalog)
        self.assertEqual(self.db.execute('SELECT count(*) FROM posts').fetchone()[0], 1)

    def test_sql_failure_rolls_back_prior_product_updates(self):
        self.db.execute("CREATE TRIGGER reject_article BEFORE INSERT ON posts BEGIN SELECT RAISE(ABORT, 'simulated failure'); END")
        before = [dict(row) for row in self.db.execute('SELECT * FROM products')]
        with self.assertRaises(sqlite3.DatabaseError):
            batch.apply_transaction(self.db, self.catalog)
        self.assertEqual([dict(row) for row in self.db.execute('SELECT * FROM products')], before)
        self.assertEqual(self.db.execute('SELECT count(*) FROM posts').fetchone()[0], 0)


if __name__ == '__main__':
    unittest.main()
