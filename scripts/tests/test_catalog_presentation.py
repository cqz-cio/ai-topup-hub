from contextlib import closing
import json
from pathlib import Path
import sqlite3
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from catalog_presentation import apply_database, public_copy, catalog_fields

class CatalogPresentationTest(unittest.TestCase):
    def fixture(self, path):
        with closing(sqlite3.connect(path)) as db, db:
            db.execute('CREATE TABLE products(id INTEGER PRIMARY KEY,slug TEXT,title_json BLOB,description_json BLOB,content_json BLOB,images BLOB,instructions_json BLOB,price_amount TEXT,is_active INTEGER)')
            def encoded(value): return json.dumps(value, ensure_ascii=False).encode()
            for product_id, slug, title in [(1, 'qzweb-55', 'ChatGPT Pro 20新老号(到期)开通 质保一个月'), (2, 'qzweb-62', 'ChatGPT Pro 20x｜iOS 渠道')]:
                db.execute('INSERT INTO products VALUES(?,?,?,?,?,?,?,?,?)', (product_id, slug, encoded({'zh-CN': title}), encoded({'zh-CN': 'Pro 20x'}), encoded({'zh-CN': '<p>Pro 20x · 渠道一（菲区）</p>'}), encoded(['/catalog-products/qzweb/pro20x-direct-20261001.svg']), b'private delivery 20x', '1160', 1))
            db.execute('CREATE TABLE product_skus(id INTEGER PRIMARY KEY,product_id INTEGER,spec_values_json BLOB,price_amount TEXT)')
            db.execute('INSERT INTO product_skus VALUES(1,1,?,?)', (encoded({'套餐': 'ChatGPT Pro 20x'}), '1160'))
            db.execute('CREATE TABLE settings(id INTEGER PRIMARY KEY,key TEXT,value_json BLOB)')
            db.execute('INSERT INTO settings VALUES(1,?,?)', ('about_page_config', encoded({'text': '20x 续费绑卡'})))
            db.execute('CREATE TABLE orders(id INTEGER PRIMARY KEY,title TEXT)')
            db.execute("INSERT INTO orders VALUES(1,'Historical Pro 20x 渠道一')")
            db.execute('CREATE TABLE product_mappings(id INTEGER PRIMARY KEY,upstream_id TEXT)')
            db.execute("INSERT INTO product_mappings VALUES(1,'ellaix_zovocard_pro_20x_direct_v1')")
            db.execute('CREATE TABLE payment_channels(id INTEGER PRIMARY KEY,config TEXT)')
            db.execute("INSERT INTO payment_channels VALUES(1,'preserve')")

    def test_region_and_quota_copy_is_idempotent_and_does_not_duplicate_regions(self):
        value = {'zh-CN': '渠道一（菲区）Pro 20x，渠道二 · 智利 Pro 50x', 'zh-TW': '渠道一（菲區）20x', 'en-US': 'Channel 1 (PH) Pro 20x; Channel 2 (Chile) Pro 50x'}
        revised = public_copy(value, tiers=True)
        self.assertEqual(revised['zh-CN'], '菲区Pro 10x，智利 Pro 25x')
        self.assertEqual(revised['zh-TW'], '菲區10x')
        self.assertEqual(revised['en-US'], 'PH Pro 10x; Chile Pro 25x')
        self.assertEqual(public_copy(revised, tiers=True), revised)

    def test_only_approved_fields_change_and_scanners_keep_blob_values(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / 'shop.db'
            self.fixture(path)
            result = apply_database(path, apply=True, backup_dir=Path(temp) / 'backups')
            self.assertTrue(result['all_other_fields_unchanged'])
            self.assertTrue(Path(result['backup']).exists())
            with closing(sqlite3.connect(path)) as db:
                title, images, instructions, price, active = db.execute('SELECT title_json,images,instructions_json,price_amount,is_active FROM products WHERE id=1').fetchone()
                self.assertIsInstance(title, bytes)
                self.assertIn('10x', json.loads(title)['zh-CN'])
                self.assertIn('菲区', json.loads(title)['zh-CN'])
                self.assertEqual(json.loads(images), ['/catalog-products/qzweb/pro10x-direct-ph-20261016.svg'])
                self.assertEqual((instructions, price, active), (b'private delivery 20x', '1160', 1))
                self.assertIn('20x', json.loads(db.execute('SELECT title_json FROM products WHERE id=2').fetchone()[0])['zh-CN'])
                self.assertEqual(db.execute('SELECT title FROM orders').fetchone()[0], 'Historical Pro 20x 渠道一')
                self.assertEqual(db.execute('SELECT upstream_id FROM product_mappings').fetchone()[0], 'ellaix_zovocard_pro_20x_direct_v1')
                self.assertEqual(db.execute('SELECT config FROM payment_channels').fetchone()[0], 'preserve')
            self.assertEqual(apply_database(path, apply=True, backup_dir=Path(temp) / 'backups')['changed_fields'], 0)

    def test_failed_write_rolls_back_every_catalog_field(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / 'shop.db'
            self.fixture(path)
            with closing(sqlite3.connect(path)) as db, db:
                before = db.execute('SELECT * FROM products').fetchall()
                db.execute("CREATE TRIGGER reject_sku BEFORE UPDATE ON product_skus BEGIN SELECT RAISE(ABORT,'reject'); END")
            with self.assertRaises(sqlite3.IntegrityError):
                apply_database(path, apply=True, backup_dir=Path(temp) / 'backups')
            with closing(sqlite3.connect(path)) as db:
                self.assertEqual(db.execute('SELECT * FROM products').fetchall(), before)

    def test_dry_run_and_unrelated_ios_product_are_unchanged(self):
        with tempfile.TemporaryDirectory() as temp:
            path = Path(temp) / 'shop.db'
            self.fixture(path)
            before = path.read_bytes()
            self.assertGreater(apply_database(path)['planned_fields'], 0)
            self.assertEqual(path.read_bytes(), before)
        values = ({'zh-CN': 'Pro 20x iOS'}, {}, {}, ['/custom.svg'])
        self.assertEqual(catalog_fields('qzweb-62', *values), values)

if __name__ == '__main__':
    unittest.main()
