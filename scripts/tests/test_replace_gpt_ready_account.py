"""Portable transaction and rollback tests; no network or business database."""
import copy
from datetime import datetime, timezone
import gc
import importlib.util
import json
from pathlib import Path
import sqlite3
import tempfile
import unittest

ROOT = Path(__file__).resolve().parents[2]


def module(name, filename):
    spec = importlib.util.spec_from_file_location(name, ROOT / "scripts" / filename)
    result = importlib.util.module_from_spec(spec)
    spec.loader.exec_module(result)
    return result


class ReadyAccountMigrationTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.new = module("replace_ready", "replace-gpt-ready-account.py")
        cls.old = module("create_ready", "add-gpt-ready-accounts.py")
        cls.config = json.loads((ROOT / "docs/catalog/gpt-ready-accounts/ellaix-plus-30d.json").read_text("utf-8"))
        cls.content = (ROOT / "docs/catalog/gpt-ready-accounts/ellaix-plus-30d.html").read_text("utf-8").strip()
        cls.source = {
            "at":datetime.now(timezone.utc).isoformat(),"purchase_endpoints_called":False,"balance_unchanged":True,
            "item":{"id":85,"code":"D32C360755D69018","name":cls.config["supplier"]["source_title"],
                    "status":1,"delivery_way":0,"config":[],"widget":[],"only_user":0},
            "quote":{"price":"61.50","currency_code":"CNY"},"stock":{"stock":"10"},
        }

    def setUp(self):
        self.folder = tempfile.TemporaryDirectory(prefix="ready-account-")
        self.db = Path(self.folder.name) / "shop.db"
        with sqlite3.connect(self.db) as db:
            db.executescript("""
                CREATE TABLE categories (id INTEGER PRIMARY KEY,parent_id,slug UNIQUE,name_json,icon,sort_order,is_active,created_at,deleted_at);
                CREATE TABLE products (id INTEGER PRIMARY KEY,category_id,slug UNIQUE,seo_meta_json,title_json,description_json,content_json,instructions_json,
                  price_amount NUMERIC,cost_price_amount NUMERIC,wholesale_prices,images,tags,purchase_type,min_purchase_quantity DEFAULT 0,max_purchase_quantity DEFAULT 0,
                  stock_display_mode,fulfillment_type,manual_form_schema_json,manual_stock_total DEFAULT 0,manual_stock_locked DEFAULT 0,manual_stock_sold DEFAULT 0,
                  payment_channel_ids,is_affiliate_enabled,is_mapped,is_active,sort_order,created_at,updated_at,deleted_at);
                CREATE TABLE product_skus (id INTEGER PRIMARY KEY,product_id,sku_code,spec_values_json,price_amount NUMERIC,cost_price_amount NUMERIC,
                  manual_stock_total,manual_stock_locked,manual_stock_sold,is_active,sort_order,created_at,updated_at,deleted_at);
                CREATE TABLE site_connections (id INTEGER PRIMARY KEY,base_url,api_key,protocol,status,auto_sync_price,exchange_rate,deleted_at);
                CREATE TABLE product_mappings (id INTEGER PRIMARY KEY,connection_id,local_product_id,upstream_product_id,upstream_fulfillment_type,
                  upstream_status,is_active,last_synced_at,created_at,updated_at,deleted_at,stock_limit DEFAULT 0);
                CREATE TABLE sku_mappings (id INTEGER PRIMARY KEY,product_mapping_id,local_sku_id,upstream_sku_id,upstream_price NUMERIC,upstream_stock,
                  upstream_is_active,stock_synced_at,created_at,updated_at,deleted_at);
                CREATE TABLE orders (id INTEGER PRIMARY KEY,status,delivery);
                CREATE TABLE wallet_transactions (id INTEGER PRIMARY KEY,amount NUMERIC);
                INSERT INTO site_connections VALUES (1,'https://ellaixshop.com','1039','acg-faka','active',0,1,NULL);
                INSERT INTO orders VALUES (1,'delivered','fixture historical delivery');
                INSERT INTO wallet_transactions VALUES (1,123);
            """)
        self.old.create_catalog(self.db, ROOT / "docs/catalog/gpt-ready-accounts/catalog.json",
                                Path(self.folder.name) / "backups", apply=True)

    def tearDown(self):
        gc.collect()  # The legacy fixture creator uses sqlite transaction contexts.
        target = Path(self.folder.name).resolve()
        assert target.parent == Path(tempfile.gettempdir()).resolve() and target.name.startswith("ready-account-")
        self.folder.cleanup()

    def run_migration(self, **kwargs):
        return self.new.migrate(self.db, self.config, self.content, kwargs.pop("source", self.source),
                                backup_root=Path(self.folder.name)/"backups", **kwargs)

    def state(self):
        with sqlite3.connect(self.db) as db:
            return self.new.snapshot(db)

    def test_dry_run_preserves_all_rows(self):
        before = self.state()
        result = self.run_migration()
        self.assertTrue(result["passed"])
        self.assertEqual(before, self.state())

    def test_apply_backup_retirement_and_idempotency(self):
        before = self.state()
        result = self.run_migration(apply=True)
        self.assertTrue(Path(result["backup"]).exists())
        with sqlite3.connect(result["backup"]) as backup:
            self.assertEqual(self.new.snapshot(backup), before)
        with sqlite3.connect(self.db) as db:
            self.assertEqual(db.execute("SELECT typeof(images),typeof(content_json),typeof(instructions_json) FROM products WHERE id=?",
                                         (result["product_id"],)).fetchone(), ("blob","blob","blob"))
            self.assertEqual(db.execute("SELECT slug,price_amount,fulfillment_type FROM products WHERE category_id=? AND is_active=1",
                                         (result["category_id"],)).fetchall(), [(self.new.NEW,115,"upstream")])
            self.assertEqual(db.execute("SELECT upstream_stock FROM sku_mappings WHERE product_mapping_id=?",
                                         (result["mapping_id"],)).fetchone()[0],int(self.source["stock"]["stock"]))
        after = self.state()
        repeated = self.run_migration(apply=True)
        self.assertTrue(all(not ids for ids in repeated["created"].values()))
        self.assertEqual(after,self.state())

    def test_late_write_failure_rolls_back_new_product_and_mapping(self):
        with sqlite3.connect(self.db) as db:
            db.execute("CREATE TRIGGER refuse_retire BEFORE UPDATE OF is_active ON products WHEN OLD.slug='gpt-plus-vietnam-account-30d' BEGIN SELECT RAISE(ABORT,'fixture late failure'); END")
        before = self.state()
        with self.assertRaises(sqlite3.IntegrityError):
            self.run_migration(apply=True)
        self.assertEqual(before,self.state())

    def test_conflicting_supplier_mapping_is_rejected(self):
        with sqlite3.connect(self.db) as db:
            db.execute("INSERT INTO product_mappings (connection_id,local_product_id,upstream_product_id,is_active) VALUES (1,1,85,1)")
        before = self.state()
        with self.assertRaisesRegex(AssertionError,"already mapped"):
            self.run_migration(apply=True)
        self.assertEqual(before,self.state())

    def test_stale_quote_and_unsupported_supplier_product(self):
        for change in ("stale","form","manual","too_expensive"):
            with self.subTest(change=change):
                source=copy.deepcopy(self.source)
                if change=="stale":
                    source["at"]="2020-01-01T00:00:00+00:00"
                elif change=="form":
                    source["item"]["widget"]=[{"name":"session"}]
                elif change=="manual":
                    source["item"]["delivery_way"]=1
                else:
                    source["quote"]["price"]="115.01"
                before=self.state()
                with self.assertRaises(AssertionError):
                    self.run_migration(source=source,apply=True)
                self.assertEqual(before,self.state())


if __name__=="__main__":
    unittest.main()
