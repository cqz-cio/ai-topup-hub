import copy
import json
from pathlib import Path
import sqlite3
import sys
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
import catalog_pro_usd as migration


class CatalogProUsdTest(unittest.TestCase):
    def setUp(self):
        self.db = sqlite3.connect(':memory:')
        self.db.row_factory = sqlite3.Row
        self.db.executescript('CREATE TABLE products (id INTEGER PRIMARY KEY, slug TEXT, deleted_at TEXT, is_active INTEGER, title_json BLOB, images TEXT, price_amount INTEGER); CREATE TABLE orders (id INTEGER, title_snapshot TEXT, amount INTEGER); INSERT INTO orders VALUES (1, "ChatGPT Pro 5x", 700);')
        self.review = []
        for pid, slug in migration.PRODUCTS.items():
            amount = migration.COVERS.get(pid, '200' if pid == 54 else '100').split('-')[0]
            tier = {'100': '5', '200': '10', '500': '25'}[amount]
            title = {'zh-CN': 'ChatGPT Pro '+tier+'x｜菲区｜自动发码'}
            images = ['/original.svg', '/second.png']
            self.db.execute('INSERT INTO products VALUES (?,?,NULL,1,?,?,700)', (pid, slug, json.dumps(title).encode(), json.dumps(images)))
            before = {'title_json': title}
            after = {'title_json': {k: migration.rename(v) for k, v in title.items()}}
            if pid in migration.COVERS:
                before['images'] = images
                after['images'] = ['/catalog-products/pro-usd-20261111/pro-'+migration.COVERS[pid]+'.svg', images[1]]
            self.review.append({'id': pid, 'slug': slug, 'before': before, 'after': after})
        self.db.commit()

    def tearDown(self):
        self.db.close()

    def test_exact_scope_history_price_and_idempotence(self):
        result = migration.apply_transaction(self.db, self.review)
        self.assertEqual(result['changed_fields'], 14)
        self.assertEqual(tuple(self.db.execute('SELECT * FROM orders').fetchone()), (1, 'ChatGPT Pro 5x', 700))
        self.assertEqual([r[0] for r in self.db.execute('SELECT DISTINCT price_amount FROM products')], [700])
        self.assertEqual(migration.apply_transaction(self.db, self.review)['changed_fields'], 0)
        self.assertIsInstance(self.db.execute('SELECT title_json FROM products LIMIT 1').fetchone()[0], bytes)

    def test_concurrent_edit_refuses_whole_batch(self):
        self.db.execute('UPDATE products SET title_json=? WHERE id=55', ('{"zh-CN":"newer title"}',))
        self.db.commit()
        with self.assertRaisesRegex(ValueError, 'Concurrent edit'):
            migration.apply_transaction(self.db, self.review)
        self.assertIn('10x', json.loads(self.db.execute('SELECT title_json FROM products WHERE id=31').fetchone()[0])['zh-CN'])

    def test_unapproved_price_or_cover_refused(self):
        reviewed = copy.deepcopy(self.review)
        reviewed[0]['before']['price_amount'] = 700
        reviewed[0]['after']['price_amount'] = 100
        with self.assertRaisesRegex(ValueError, 'Unexpected fields'):
            migration.apply_transaction(self.db, reviewed)

    def test_other_tiers_and_supplier_codes_unchanged(self):
        self.assertEqual(migration.rename('Pro 50x ellaix_pro_5x_v1'), 'Pro 50x ellaix_pro_5x_v1')


if __name__ == '__main__':
    unittest.main()
