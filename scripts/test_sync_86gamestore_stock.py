import copy
from contextlib import closing
import importlib.util
from pathlib import Path
import sqlite3
import tempfile
import unittest

spec = importlib.util.spec_from_file_location("supplier_stock", Path(__file__).with_name("sync-86gamestore-stock.py"))
sync = importlib.util.module_from_spec(spec)
spec.loader.exec_module(sync)


def upstream(pid=66, stock=12, active=True):
    return {"id": pid, "is_active": active,
            "skus": [{"id": pid, "stock_quantity": stock, "is_active": active}]}


class StockTests(unittest.TestCase):
    def setUp(self):
        self.tmp = tempfile.TemporaryDirectory()
        self.addCleanup(self.tmp.cleanup)
        self.path = Path(self.tmp.name) / "shop.db"
        with closing(sqlite3.connect(self.path)) as db, db:
            db.executescript("""
            CREATE TABLE site_connections(id INTEGER PRIMARY KEY,base_url TEXT,api_key TEXT,protocol TEXT,status TEXT,auto_sync_price INTEGER,deleted_at TEXT,api_secret TEXT);
            CREATE TABLE products(id INTEGER PRIMARY KEY,slug TEXT,fulfillment_type TEXT,is_mapped INTEGER,is_active INTEGER,manual_stock_total INTEGER,manual_stock_locked INTEGER,manual_stock_sold INTEGER,price_amount TEXT,updated_at TEXT,deleted_at TEXT);
            CREATE TABLE product_skus(id INTEGER PRIMARY KEY,product_id INTEGER,is_active INTEGER,manual_stock_total INTEGER,manual_stock_locked INTEGER,manual_stock_sold INTEGER,price_amount TEXT,updated_at TEXT,deleted_at TEXT);
            CREATE TABLE product_mappings(id INTEGER PRIMARY KEY,connection_id INTEGER,local_product_id INTEGER,upstream_product_id INTEGER,is_active INTEGER,stock_limit INTEGER,upstream_status TEXT,last_synced_at TEXT,updated_at TEXT,deleted_at TEXT);
            CREATE TABLE sku_mappings(id INTEGER PRIMARY KEY,product_mapping_id INTEGER,local_sku_id INTEGER,upstream_sku_id INTEGER,upstream_stock INTEGER,upstream_is_active INTEGER,stock_synced_at TEXT,updated_at TEXT,deleted_at TEXT,upstream_price TEXT);
            CREATE TABLE orders(id INTEGER PRIMARY KEY,status TEXT,amount TEXT);
            INSERT INTO site_connections VALUES(1,'https://ellaixshop.com','1039','acg-faka','active',0,NULL,'unchanged-1');
            INSERT INTO site_connections VALUES(2,'https://faka.redeemgpt.com','26113','acg-faka','disabled',0,NULL,'unchanged-2');
            INSERT INTO products VALUES(1,'existing','upstream',1,1,99,2,3,'100','old',NULL);
            INSERT INTO products VALUES(52,'86gamestore-66','manual',1,0,0,0,0,'15','old',NULL);
            INSERT INTO products VALUES(53,'86gamestore-77','manual',1,1,0,0,0,'50','old',NULL);
            INSERT INTO product_skus VALUES(52,52,1,0,3,7,'15','old',NULL);
            INSERT INTO product_skus VALUES(53,53,1,0,0,0,'50','old',NULL);
            INSERT INTO product_mappings VALUES(20,2,52,66,0,0,'active','old','old',NULL);
            INSERT INTO product_mappings VALUES(21,2,53,77,0,0,'active','old','old',NULL);
            INSERT INTO sku_mappings VALUES(20,20,52,66,0,1,'old','old',NULL,'15');
            INSERT INTO sku_mappings VALUES(21,21,53,77,0,1,'old','old',NULL,'50');
            INSERT INTO orders VALUES(1,'paid','15');
            """)

    def execute(self, sql):
        with closing(sqlite3.connect(self.path)) as db, db:
            return db.execute(sql).fetchall()

    def apply(self, items=None):
        return sync.apply_inventory(self.path, sync.normalize_inventory(items or [upstream(), upstream(77, 5)]), 2, audit=True)

    def snapshot(self):
        with closing(sqlite3.connect(self.path)) as db:
            db.row_factory = sqlite3.Row
            return sync.stock_audit(db, {})

    def test_stock_follows_supplier_without_changing_operations(self):
        result = self.apply()
        self.assertEqual(result["audited_tables"], 6)
        self.assertEqual(self.execute("SELECT manual_stock_total,manual_stock_locked,manual_stock_sold FROM product_skus WHERE id=52"), [(9, 3, 7)])
        self.assertEqual(self.execute("SELECT manual_stock_total,is_active,fulfillment_type,price_amount FROM products WHERE id=52"), [(9, 0, 'manual', '15')])
        self.assertEqual(self.execute("SELECT is_active FROM products WHERE id=53"), [(1,)])
        self.assertEqual(self.execute("SELECT manual_stock_total FROM products WHERE id=1"), [(99,)])
        self.assertEqual(self.execute("SELECT status FROM site_connections ORDER BY id"), [('active',), ('disabled',)])
        self.assertFalse(result["purchase_endpoint_called"])

    def test_repeated_sync_and_restock_do_not_accumulate_or_erase_sales(self):
        self.apply()
        self.apply()
        self.assertEqual(self.execute("SELECT manual_stock_total FROM product_skus WHERE id=52"), [(9,)])
        self.apply([upstream(stock=1), upstream(77, 0)])
        self.assertEqual(self.execute("SELECT manual_stock_total FROM product_skus ORDER BY id"), [(0,), (0,)])
        self.apply([upstream(stock=20), upstream(77, 8)])
        self.assertEqual(self.execute("SELECT manual_stock_total,manual_stock_locked,manual_stock_sold FROM product_skus WHERE id=52"), [(17, 3, 7)])

    def test_unavailable_and_missing_products_become_zero(self):
        self.apply([upstream(active=False)])
        self.assertEqual(self.execute("SELECT upstream_stock,upstream_is_active FROM sku_mappings ORDER BY id"), [(0, 0), (0, 0)])
        self.assertEqual(self.execute("SELECT upstream_status FROM product_mappings ORDER BY id"), [('inactive',), ('inactive',)])

    def test_stock_limit_and_local_sku_disable_are_preserved(self):
        self.execute("UPDATE product_mappings SET stock_limit=5 WHERE id=20")
        self.execute("UPDATE product_skus SET is_active=0 WHERE id=53")
        self.apply()
        self.assertEqual(self.execute("SELECT manual_stock_total FROM products ORDER BY id"), [(99,), (2,), (0,)])
        self.assertEqual(self.execute("SELECT upstream_stock FROM sku_mappings WHERE id=20"), [(12,)])

    def test_changed_mapping_or_automatic_connection_is_rejected(self):
        for sql in ["UPDATE products SET fulfillment_type='upstream' WHERE id=53",
                    "UPDATE site_connections SET status='active' WHERE id=2",
                    "UPDATE product_mappings SET is_active=1 WHERE id=21",
                    "DELETE FROM sku_mappings WHERE id=21"]:
            with self.subTest(sql=sql):
                saved = self.snapshot()
                self.execute(sql)
                changed = self.snapshot()
                with self.assertRaises(ValueError):
                    self.apply()
                self.assertEqual(self.snapshot(), changed)
                # Restore this small test DB for the next independent condition.
                with closing(sqlite3.connect(self.path)) as db, db:
                    for table, rows in saved.items():
                        db.execute('DELETE FROM "' + table + '"')
                        for row in rows:
                            data = {k: v for k, v in row.items() if k != 'audit_rowid'}
                            db.execute('INSERT INTO "' + table + '" (' + ','.join(data) + ') VALUES (' + ','.join('?' for _ in data) + ')', list(data.values()))

    def test_late_database_error_rolls_back_earlier_products(self):
        self.execute("CREATE TRIGGER abort_last BEFORE UPDATE ON product_skus WHEN OLD.id=53 BEGIN SELECT RAISE(ABORT,'test'); END")
        before = self.snapshot()
        with self.assertRaises(sqlite3.IntegrityError):
            self.apply()
        self.assertEqual(self.snapshot(), before)

    def test_invalid_inventory_is_rejected(self):
        invalid = [[upstream(), upstream()], [upstream(stock=-1)], [upstream(stock=True)]]
        duplicate_sku = upstream()
        duplicate_sku['skus'].append(copy.deepcopy(duplicate_sku['skus'][0]))
        invalid.append([duplicate_sku])
        for items in invalid:
            with self.subTest(items=items), self.assertRaises(ValueError):
                sync.normalize_inventory(items)

    def test_incomplete_or_failed_fetch_cannot_apply_inventory(self):
        class FakeAPI:
            def call(self, path):
                return {"total": 11, "items": [upstream()]}
        before = self.snapshot()
        with self.assertRaises(ValueError):
            sync.fetch_inventory(FakeAPI(), 2)
        self.assertEqual(self.snapshot(), before)

    def test_empty_complete_catalog_is_valid(self):
        class FakeAPI:
            def call(self, path):
                return {"total": 0, "items": []}
        self.assertEqual(sync.fetch_inventory(FakeAPI(), 2), {})

    def test_purchase_and_configuration_endpoints_are_rejected(self):
        api = sync.AdminAPI.__new__(sync.AdminAPI)
        for path in ['/shared/commodity/trade', '/admin/orders', '/admin/product-mappings/20/sync', '/admin/site-connections/2']:
            with self.subTest(path=path), self.assertRaises(ValueError):
                api.call(path, {})


if __name__ == '__main__':
    unittest.main()
