import json
from contextlib import contextmanager
from pathlib import Path
import sqlite3
import sys
import tempfile
import unittest
sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from support_contacts import enrich, apply_database

@contextmanager
def connection(path):
    db=sqlite3.connect(path)
    try:
        with db:yield db
    finally:db.close()

class SupportContactsTest(unittest.TestCase):
    def test_html_and_plain_copy_are_idempotent(self):
        for raw in ['客服微信：Cqz0924ttxk', '<p>客服微信 <strong>Cqz0924ttxk</strong>，请提供订单号。</p>']:
            new=enrich(raw)
            self.assertIn('1669268759',new)
            self.assertIn('@czq_piestar',new)
            self.assertEqual(enrich(new),new)
            if '<p>' in raw:self.assertIn('href="https://t.me/czq_piestar"',new)
        self.assertEqual(enrich({'supportWechat':'Cqz0924ttxk'}),{'supportWechat':'Cqz0924ttxk'})

    def fixture(self,path):
        with connection(path) as db:
            db.execute('CREATE TABLE products (id INTEGER PRIMARY KEY,content_json TEXT,instructions_json TEXT,price_amount TEXT)')
            db.execute('INSERT INTO products VALUES(1,?,?,?)',(json.dumps({'zh-CN':'客服微信 Cqz0924ttxk'}),json.dumps({'zh-CN':'私有交付入口；微信 Cqz0924ttxk'}),'80.88'))
            db.execute('CREATE TABLE posts (id INTEGER PRIMARY KEY,content_json TEXT)')
            db.execute('INSERT INTO posts VALUES(1,?)',(json.dumps({'zh-CN':'微信 Cqz0924ttxk'}),))
            db.execute('CREATE TABLE settings (key TEXT,value_json TEXT)')
            db.execute('INSERT INTO settings VALUES(?,?)',('site_config',json.dumps({'contact':{'whatsapp':'keep'},'brand':{'site_name':'keep'}})))
            db.execute('CREATE TABLE orders (id INTEGER,content TEXT)')
            db.execute('INSERT INTO orders VALUES(1,?)',('Historical contact Cqz0924ttxk',))
            db.execute('CREATE TABLE payment_channels (id INTEGER,config TEXT)')
            db.execute("INSERT INTO payment_channels VALUES(1,'keep')")

    def test_only_contact_fields_change_and_second_run_is_empty(self):
        with tempfile.TemporaryDirectory() as d:
            path=Path(d)/'shop.db';self.fixture(path)
            result=apply_database(path,apply=True,backup_dir=Path(d)/'backups')
            self.assertTrue(result['all_other_fields_unchanged'])
            self.assertTrue(Path(result['backup']).exists())
            with connection(path) as db:
                self.assertEqual(db.execute('SELECT price_amount FROM products').fetchone()[0],'80.88')
                self.assertEqual(db.execute('SELECT content FROM orders').fetchone()[0],'Historical contact Cqz0924ttxk')
                self.assertEqual(db.execute('SELECT config FROM payment_channels').fetchone()[0],'keep')
                config=json.loads(db.execute('SELECT value_json FROM settings').fetchone()[0])
                self.assertEqual(config['contact']['whatsapp'],'keep')
                self.assertEqual(config['brand']['site_name'],'keep')
                self.assertEqual(config['contact']['telegram'],'https://t.me/czq_piestar')
            self.assertEqual(apply_database(path,apply=True,backup_dir=Path(d)/'backups')['changed_fields'],0)

    def test_failed_write_rolls_back_every_content_field(self):
        with tempfile.TemporaryDirectory() as d:
            path=Path(d)/'shop.db';self.fixture(path)
            with connection(path) as db:
                before=db.execute('SELECT * FROM products').fetchall()
                db.execute("CREATE TRIGGER reject_post BEFORE UPDATE ON posts BEGIN SELECT RAISE(ABORT,'reject'); END")
            with self.assertRaises(sqlite3.IntegrityError):apply_database(path,apply=True,backup_dir=Path(d)/'backups')
            with connection(path) as db:self.assertEqual(db.execute('SELECT * FROM products').fetchall(),before)

    def test_preview_does_not_write(self):
        with tempfile.TemporaryDirectory() as d:
            path=Path(d)/'shop.db';self.fixture(path);before=path.read_bytes()
            self.assertGreater(apply_database(path)['planned_fields'],0)
            self.assertEqual(path.read_bytes(),before)

if __name__=='__main__':unittest.main()
