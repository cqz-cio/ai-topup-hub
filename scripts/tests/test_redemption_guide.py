from contextlib import closing
import json
from pathlib import Path
import sqlite3
import sys
import tempfile
import unittest


sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from redemption_guide import apply_to_database, catalog_fields, load_guide


class RedemptionGuideTest(unittest.TestCase):
    def setUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.addCleanup(self.temp.cleanup)
        self.database = Path(self.temp.name) / "shop.db"
        self.backups = Path(self.temp.name) / "backups"
        self.guide = load_guide()
        manifest = json.loads((Path(__file__).resolve().parents[2] / "docs/catalog/qzweb-all.json").read_text(encoding="utf-8-sig"))
        self.products = [item for group in manifest["categories"] for item in group["products"]]
        with closing(sqlite3.connect(self.database)) as db, db:
            db.execute("CREATE TABLE products (id INTEGER PRIMARY KEY, slug TEXT UNIQUE, content_json TEXT, instructions_json TEXT, price_amount TEXT, stock INTEGER, deleted_at TEXT)")
            db.execute("CREATE TABLE orders (id INTEGER PRIMARY KEY, status TEXT, payload TEXT)")
            db.execute("INSERT INTO orders VALUES (1, 'pending_payment', 'SIMULATED-ONLY')")
            for product in self.products:
                body = json.dumps({"zh-CN": product["content_html"], "en-US": "Existing product-specific terms"}, ensure_ascii=False).encode("utf-8")
                price = "1160.00" if product["id"] == 55 else str(product["price"])
                db.execute("INSERT INTO products VALUES (?, ?, ?, ?, ?, 17, NULL)", (product["id"], f"qzweb-{product['id']}", body, body, price))

    def snapshot(self, path=None):
        with closing(sqlite3.connect(path or self.database)) as db:
            return {
                "products": db.execute("SELECT * FROM products ORDER BY id").fetchall(),
                "orders": db.execute("SELECT * FROM orders ORDER BY id").fetchall(),
            }

    def test_plan_is_read_only(self):
        before = self.snapshot()
        report = apply_to_database(self.database, self.guide)
        self.assertEqual(report["planned_count"], 11)
        self.assertEqual(report["changed_count"], 0)
        self.assertIsNone(report["backup"])
        self.assertEqual(self.snapshot(), before)

    def test_apply_scopes_changes_and_preserves_backup(self):
        before = self.snapshot()
        report = apply_to_database(self.database, self.guide, apply=True, backup_dir=self.backups)
        self.assertEqual(report["changed_count"], 11)
        self.assertEqual(self.snapshot(report["backup"]), before)
        after = self.snapshot()
        self.assertEqual(after["orders"], before["orders"])
        with closing(sqlite3.connect(self.database)) as db:
            storage = db.execute("SELECT slug, typeof(content_json), typeof(instructions_json) FROM products").fetchall()
        for slug, content_type, instruction_type in storage:
            self.assertEqual((content_type, instruction_type), ("blob", "blob"), slug)
        for old, new in zip(before["products"], after["products"]):
            self.assertEqual(new[:2], old[:2])
            self.assertEqual(new[4:], old[4:])
            if new[1] not in self.guide["product_slugs"]:
                self.assertEqual(new, old)
                continue
            instructions = json.loads(new[3])
            self.assertIn('href="https://cdkai.kdns.fr/"', instructions["zh-CN"])
            self.assertIn('>https://cdkai.kdns.fr/</a>', instructions["zh-CN"])
            self.assertIn("确认充值账号和套餐无误", instructions["zh-CN"])
            self.assertEqual(instructions["en-US"], json.loads(old[3])["en-US"])
            if new[1] in self.guide["ios_product_slugs"]:
                content = json.loads(new[2])["zh-CN"]
                self.assertNotIn("请勿直接套用普通卡充渠道", content)
                self.assertIn("https://cdkai.kdns.fr/", content)
                self.assertIn("原有剩余时长不会叠加", content)
                self.assertIn("30 天权益", content)
            else:
                self.assertEqual(new[2], old[2])

    def test_repeat_is_idempotent(self):
        apply_to_database(self.database, self.guide, apply=True, backup_dir=self.backups)
        before = self.snapshot()
        report = apply_to_database(self.database, self.guide, apply=True, backup_dir=self.backups)
        self.assertEqual(report["changed_count"], 0)
        self.assertIsNone(report["backup"])
        self.assertEqual(self.snapshot(), before)

    def test_repairs_text_json_so_go_scanner_can_read_it(self):
        apply_to_database(self.database, self.guide, apply=True, backup_dir=self.backups)
        with closing(sqlite3.connect(self.database)) as db, db:
            db.execute("UPDATE products SET content_json = CAST(content_json AS TEXT), instructions_json = CAST(instructions_json AS TEXT) WHERE slug = 'qzweb-62'")
        report = apply_to_database(self.database, self.guide, apply=True, backup_dir=self.backups)
        self.assertEqual(report["product_slugs"], ["qzweb-62"])
        with closing(sqlite3.connect(self.database)) as db:
            row = db.execute("SELECT typeof(content_json), typeof(instructions_json), instructions_json FROM products WHERE slug = 'qzweb-62'").fetchone()
        self.assertEqual(row[:2], ("blob", "blob"))
        self.assertIn("https://cdkai.kdns.fr/", json.loads(row[2])["zh-CN"])

    def test_missing_product_stops_before_any_write(self):
        with closing(sqlite3.connect(self.database)) as db, db:
            db.execute("DELETE FROM products WHERE slug = 'qzweb-62'")
        before = self.snapshot()
        with self.assertRaisesRegex(ValueError, "qzweb-62"):
            apply_to_database(self.database, self.guide, apply=True, backup_dir=self.backups)
        self.assertEqual(self.snapshot(), before)
        self.assertFalse(self.backups.exists())

    def test_failed_update_rolls_back_all_products(self):
        with closing(sqlite3.connect(self.database)) as db, db:
            db.execute("CREATE TRIGGER reject_guide BEFORE UPDATE ON products WHEN NEW.slug = 'qzweb-62' BEGIN SELECT RAISE(ABORT, 'simulated failure'); END")
        before = self.snapshot()
        with self.assertRaises(sqlite3.IntegrityError):
            apply_to_database(self.database, self.guide, apply=True, backup_dir=self.backups)
        self.assertEqual(self.snapshot(), before)

    def test_requires_backup_and_existing_database(self):
        with self.assertRaises(ValueError):
            apply_to_database(self.database, self.guide, apply=True)
        with self.assertRaises(FileNotFoundError):
            apply_to_database(self.database.parent / "missing.db", self.guide)

    def test_catalog_import_and_database_update_use_same_fields(self):
        apply_to_database(self.database, self.guide, apply=True, backup_dir=self.backups)
        rows = {row[1]: row for row in self.snapshot()["products"]}
        for item in self.products:
            slug = f"qzweb-{item['id']}"
            source = {"zh-CN": item["content_html"], "en-US": "Existing product-specific terms"}
            content, instructions = catalog_fields(slug, source, source, self.guide)
            self.assertEqual(content, json.loads(rows[slug][2]))
            self.assertEqual(instructions, json.loads(rows[slug][3]))


if __name__ == "__main__":
    unittest.main()
