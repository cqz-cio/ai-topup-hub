import hashlib
import json
from pathlib import Path
import sqlite3
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import seo_delivery_content as batch


class DeliveryContentTests(unittest.TestCase):
    def setUp(self):
        self.catalog = batch.load_catalog()
        self.db = sqlite3.connect(':memory:'); self.db.row_factory = sqlite3.Row
        self.db.executescript('''
          CREATE TABLE settings (key TEXT PRIMARY KEY, value_json BLOB);
          CREATE TABLE posts (id INTEGER PRIMARY KEY, slug TEXT, type TEXT, title_json BLOB, summary_json BLOB, content_json BLOB, is_published INTEGER, published_at TEXT, deleted_at TEXT);
          CREATE TABLE products (id INTEGER PRIMARY KEY, price TEXT, delivery TEXT);
          INSERT INTO products VALUES (1, '130', 'private-test-data');
        ''')
        fields = []
        for name in ['title', 'summary', 'content']:
            old = 'approved previous ' + name
            self.catalog['before_sha256'][name] = hashlib.sha256(old.encode()).hexdigest()
            fields.append(batch.encode({'zh-CN': old, 'en-US': 'keep translation'}))
        self.db.execute('INSERT INTO posts VALUES (1,?,?,?, ?,?,1,?,NULL)', (self.catalog['slug'], 'notice', *fields, '2026-10-13T02:43:53Z'))
        about = {'version': 1, 'services': [{}, {}, {}, {}], 'links': [{}], 'unrelated': 'keep'}
        for rule in self.catalog['about']:
            item = about[rule['section']][rule['index']]
            item[rule['field']] = {'zh-CN': rule['before'], 'en-US': 'keep translation'}
            if rule['section'] == 'links': item['to'] = rule['identity']
            else: item['title'] = {'zh-CN': rule['identity']}
        self.db.execute('INSERT INTO settings VALUES (?,?)', ('about_page_config', batch.encode(about)))
        self.db.execute('INSERT INTO settings VALUES (?,?)', ('site_config', b'{"payments":"preserved"}'))
        self.db.commit()

    def tearDown(self): self.db.close()

    def test_scope_translations_dates_and_repeat(self):
        self.assertEqual(batch.apply_transaction(self.db, self.catalog)['changed_fields'], 4)
        post = self.db.execute('SELECT * FROM posts').fetchone()
        self.assertEqual((post['type'], post['published_at']), ('notice', '2026-10-13T02:43:53Z'))
        self.assertEqual(json.loads(post['content_json'])['en-US'], 'keep translation')
        self.assertEqual(tuple(self.db.execute('SELECT * FROM products').fetchone()), (1, '130', 'private-test-data'))
        about = json.loads(self.db.execute("SELECT value_json FROM settings WHERE key='about_page_config'").fetchone()[0])
        self.assertEqual(about['unrelated'], 'keep')
        self.assertEqual(about['services'][1]['text']['en-US'], 'keep translation')
        self.assertEqual(batch.apply_transaction(self.db, self.catalog)['changed_fields'], 0)

    def test_newer_notice_aborts_without_overwriting(self):
        self.db.execute('UPDATE posts SET content_json=?', (batch.encode({'zh-CN': 'new operator change'}),)); self.db.commit()
        before = batch.table_rows(self.db, 'settings')
        with self.assertRaisesRegex(ValueError, 'Newer notice copy'): batch.apply_transaction(self.db, self.catalog)
        self.assertEqual(batch.table_rows(self.db, 'settings'), before)

    def test_changed_about_link_aborts_notice_changes(self):
        row = self.db.execute("SELECT value_json FROM settings WHERE key='about_page_config'").fetchone()
        value = json.loads(row[0]); value['links'][0]['to'] = '/new-policy'
        self.db.execute("UPDATE settings SET value_json=? WHERE key='about_page_config'", (batch.encode(value),)); self.db.commit()
        before = batch.table_rows(self.db, 'posts')
        with self.assertRaisesRegex(ValueError, 'identity changed'): batch.apply_transaction(self.db, self.catalog)
        self.assertEqual(batch.table_rows(self.db, 'posts'), before)

    def test_unexpected_database_side_effect_rolls_back(self):
        self.db.execute("CREATE TRIGGER bad_side_effect AFTER UPDATE ON posts BEGIN UPDATE products SET price='0'; END")
        before = batch.table_rows(self.db, 'posts')
        with self.assertRaisesRegex(ValueError, 'Protected data changed'): batch.apply_transaction(self.db, self.catalog)
        self.assertEqual(batch.table_rows(self.db, 'posts'), before)
        self.assertEqual(self.db.execute('SELECT price FROM products').fetchone()[0], '130')

    def test_local_apply_rejects_database_outside_workspace_work(self):
        with tempfile.NamedTemporaryFile(suffix='.db') as tmp:
            with self.assertRaisesRegex(ValueError, 'Only a workspace'): batch.apply_local(tmp.name, True)


if __name__ == '__main__': unittest.main()
