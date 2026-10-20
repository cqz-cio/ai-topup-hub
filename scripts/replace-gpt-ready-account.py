"""Replace exactly two manual ready accounts with the reviewed supplier item 85.

Only catalog records change. Supplier calls, purchasing, service changes and
credentials are intentionally outside this migration. Apply requires a recent
read-only signed supplier quote and a consistent private database backup.
"""
import argparse
from contextlib import closing
from datetime import datetime, timezone
from decimal import Decimal
import json
import os
from pathlib import Path
import sqlite3
import time


OLD = ("gpt-plus-brazil-account-24h", "gpt-plus-vietnam-account-30d")
NEW = "ellaix-gpt-plus-account-30d"


def encode(value):
    # The Go JSON scanners expect []byte (SQLite BLOB), not TEXT.
    return json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode("utf-8")


def snapshot(db):
    return {name: list(db.execute('SELECT * FROM "' + name.replace('"', '""') + '" ORDER BY rowid'))
            for (name,) in db.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'").fetchall()}


def insert(db, table, values):
    return db.execute(f"INSERT INTO {table} ({','.join(values)}) VALUES ({','.join('?' for _ in values)})",
                      list(values.values())).lastrowid


def migrate(db_path, config, content, source, *, apply=False, backup_root=None):
    assert config["version"] == 1 and config["slug"] == NEW
    assert tuple(config["retire_slugs"]) == OLD
    assert config["category_slug"] == "gpt-ready-accounts"
    assert Decimal(config["price_amount"]) == Decimal("115.00")
    supplier = config["supplier"]
    assert (supplier["base_url"], supplier["merchant_id"], supplier["protocol"],
            supplier["product_id"], supplier["sku_id"], supplier["shared_code"]) == (
                "https://ellaixshop.com", "1039", "acg-faka", 85, 85, "D32C360755D69018")
    item = source["item"]
    assert item["id"] == 85 and item["code"] == supplier["shared_code"]
    assert item["name"] == supplier["source_title"] and item["status"] == 1 and item["delivery_way"] == 0
    assert item.get("config") in (None, "", [], {}) and item.get("widget") in (None, "", [], {})
    assert int(item.get("only_user", 0)) == 0
    assert source["purchase_endpoints_called"] is False and source["balance_unchanged"] is True
    age = (datetime.now(timezone.utc) - datetime.fromisoformat(source["at"])).total_seconds()
    assert -60 <= age <= 3600, "Refresh the signed supplier quote"
    assert source["quote"]["currency_code"] == "CNY"
    cost = Decimal(str(source["quote"]["price"]))
    stock = int(source["stock"]["stock"])
    assert 0 < cost < Decimal("115") and stock >= 0
    assert "自动" in content and "自行" in content and "2FA" in content and "Cqz0924ttxk" in content
    assert "https://cdk.bar/recharge" in config["instructions"]
    path = Path(db_path).resolve(strict=True)
    stamp = datetime.now(timezone.utc).isoformat()
    backup = None
    if apply:
        assert backup_root, "A private backup directory is required"
        directory = Path(backup_root) / ("before-ellaix-ready-" + datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%S%fZ"))
        directory.mkdir(parents=True, mode=0o700)
        backup = directory / "shop.db"
        deadline = time.monotonic() + 25
        def progress(*_):
            if time.monotonic() > deadline:
                raise TimeoutError("Database backup timed out")
        with closing(sqlite3.connect(path.as_uri() + "?mode=ro", uri=True)) as live, closing(sqlite3.connect(backup)) as dest:
            live.backup(dest, pages=512, progress=progress, sleep=0.05)
            assert dest.execute("PRAGMA quick_check").fetchone()[0] == "ok"
        if os.name != "nt":
            backup.chmod(0o600)
    created = {t: [] for t in ("products", "product_skus", "product_mappings", "sku_mappings")}
    with closing(sqlite3.connect(path.as_uri() + "?mode=rw", uri=True, timeout=10)) as db:
        db.execute("PRAGMA foreign_keys=ON")
        db.execute("BEGIN IMMEDIATE")
        deadline = time.monotonic() + 30
        db.set_progress_handler(lambda: int(time.monotonic() > deadline), 10000)
        try:
            before = snapshot(db)
            fk = db.execute("PRAGMA foreign_key_check").fetchall()
            category = db.execute("SELECT id,is_active FROM categories WHERE slug=? AND deleted_at IS NULL",
                                  (config["category_slug"],)).fetchone()
            assert category and category[1] == 1, "Existing active ready-account category required"
            category_id = category[0]
            connections = db.execute("SELECT id,status,auto_sync_price,exchange_rate FROM site_connections WHERE base_url=? AND api_key=? AND protocol=? AND deleted_at IS NULL",
                                     (supplier["base_url"], supplier["merchant_id"], supplier["protocol"])).fetchall()
            assert len(connections) == 1 and connections[0][1] in ("active", "disabled")
            connection_id, _, automatic_price, exchange = connections[0]
            assert not automatic_price and Decimal(str(exchange)) == 1
            old_ids, images = [], None
            for slug in OLD:
                old = db.execute("SELECT id,category_id,fulfillment_type,is_mapped,images FROM products WHERE slug=? AND deleted_at IS NULL", (slug,)).fetchone()
                assert old and old[1:4] == (category_id, "manual", 0), "Old product identity conflict"
                picture = json.loads(old[4])
                assert picture and (images is None or images == picture), "Old covers differ"
                images = picture
                old_ids.append(old[0])
            existing = db.execute("SELECT id,category_id,price_amount,cost_price_amount,fulfillment_type,is_mapped,is_active,images,content_json,instructions_json FROM products WHERE slug=? AND deleted_at IS NULL", (NEW,)).fetchone()
            if existing:
                product_id = existing[0]
                assert existing[1] == category_id and Decimal(str(existing[2])) == Decimal("115")
                assert existing[4:7] == ("upstream", 1, 1)
                assert json.loads(existing[7]) == images
                assert json.loads(existing[8]) == {"zh-CN": content}
                assert json.loads(existing[9]) == {"zh-CN": config["instructions"]}
                sku = db.execute("SELECT id,price_amount,cost_price_amount FROM product_skus WHERE product_id=? AND deleted_at IS NULL", (product_id,)).fetchall()
                assert len(sku) == 1 and Decimal(str(sku[0][1])) == Decimal("115")
                sku_id = sku[0][0]
                mapping = db.execute("SELECT id,connection_id,upstream_product_id,is_active FROM product_mappings WHERE local_product_id=? AND deleted_at IS NULL", (product_id,)).fetchall()
                assert len(mapping) == 1 and mapping[0][1:] == (connection_id, 85, 1)
                mapping_id = mapping[0][0]
                assert db.execute("SELECT local_sku_id,upstream_sku_id FROM sku_mappings WHERE product_mapping_id=? AND deleted_at IS NULL", (mapping_id,)).fetchall() == [(sku_id,85)]
            else:
                assert not db.execute("SELECT 1 FROM products WHERE slug=?", (NEW,)).fetchone(), "Archived slug conflict"
                assert not db.execute("SELECT 1 FROM product_mappings WHERE connection_id=? AND upstream_product_id=85 AND deleted_at IS NULL", (connection_id,)).fetchone(), "Supplier item already mapped"
                product_id = insert(db, "products", {
                    "category_id":category_id, "slug":NEW, "seo_meta_json":encode({}),
                    "title_json":encode({"zh-CN":config["title"]}), "description_json":encode({"zh-CN":config["description"]}),
                    "content_json":encode({"zh-CN":content}), "instructions_json":encode({"zh-CN":config["instructions"]}),
                    "price_amount":"115.00", "cost_price_amount":str(cost), "wholesale_prices":encode([]),
                    "images":encode(images), "tags":encode(config["tags"]), "purchase_type":"guest",
                    "min_purchase_quantity":int(item.get("minimum") or 0), "max_purchase_quantity":int(item.get("maximum") or 0),
                    "stock_display_mode":"exact", "fulfillment_type":"upstream", "manual_form_schema_json":encode({}),
                    "manual_stock_total":0, "manual_stock_locked":0, "manual_stock_sold":0, "payment_channel_ids":"[]",
                    "is_affiliate_enabled":0, "is_mapped":1, "is_active":1, "sort_order":20, "created_at":stamp, "updated_at":stamp,
                })
                created["products"].append(product_id)
                sku_id = insert(db, "product_skus", {
                    "product_id":product_id, "sku_code":"ELLAIX-85", "spec_values_json":encode({}),
                    "price_amount":"115.00", "cost_price_amount":str(cost), "manual_stock_total":0,
                    "manual_stock_locked":0, "manual_stock_sold":0, "is_active":1, "sort_order":0,
                    "created_at":stamp, "updated_at":stamp,
                })
                created["product_skus"].append(sku_id)
                mapping_id = insert(db, "product_mappings", {
                    "connection_id":connection_id,"local_product_id":product_id,"upstream_product_id":85,
                    "upstream_fulfillment_type":"auto","upstream_status":"active","is_active":1,"stock_limit":0,
                    "last_synced_at":stamp,"created_at":stamp,"updated_at":stamp,
                })
                created["product_mappings"].append(mapping_id)
                created["sku_mappings"].append(insert(db, "sku_mappings", {
                    "product_mapping_id":mapping_id,"local_sku_id":sku_id,"upstream_sku_id":85,
                    "upstream_price":str(cost),"upstream_stock":stock,"upstream_is_active":1,
                    "stock_synced_at":stamp,"created_at":stamp,"updated_at":stamp,
                }))
            for product in old_ids:
                db.execute("UPDATE products SET is_active=0,updated_at=? WHERE id=? AND is_active<>0", (stamp, product))
            after = snapshot(db)
            columns = [r[1] for r in db.execute("PRAGMA table_info(products)")]
            active_index, updated_index = columns.index("is_active"), columns.index("updated_at")
            for table, rows in before.items():
                kept = [r for r in after[table] if r[0] not in created.get(table, [])]
                expected = []
                for row in rows:
                    values = list(row)
                    if table == "products" and values[0] in old_ids and values[active_index]:
                        values[active_index], values[updated_index] = 0, stamp
                    expected.append(tuple(values))
                assert kept == expected, "Out-of-scope database change: " + table
                assert len(after[table]) - len(rows) == len(created.get(table, []))
            assert db.execute("PRAGMA foreign_key_check").fetchall() == fk
            assert [r[0] for r in db.execute("SELECT id FROM products WHERE category_id=? AND is_active=1 AND deleted_at IS NULL", (category_id,))] == [product_id]
            db.commit() if apply else db.rollback()
        except BaseException:
            db.rollback()
            raise
    report = {"passed":True,"applied":apply,"category_id":category_id,"product_id":product_id,"sku_id":sku_id,
              "mapping_id":mapping_id,"retired_product_ids":old_ids,"created":created,"price":"115.00","cost":str(cost),
              "source_stock":stock,"images":images,"backup":str(backup) if backup else None,
              "audited_tables":len(before),"unrelated_rows_unchanged":True,"purchase_endpoints_called":False}
    if backup:
        (backup.parent / "report.json").write_text(json.dumps(report,ensure_ascii=False,indent=2),encoding="utf-8")
    return report


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--db", required=True)
    parser.add_argument("--catalog", required=True, type=Path)
    parser.add_argument("--source", required=True, type=Path)
    parser.add_argument("--backup-root", type=Path)
    parser.add_argument("--apply", action="store_true")
    args = parser.parse_args()
    config = json.loads(args.catalog.read_text(encoding="utf-8"))
    content = (args.catalog.parent / config["content_file"]).read_text(encoding="utf-8").strip()
    source = json.loads(args.source.read_text(encoding="utf-8"))
    print(json.dumps(migrate(args.db, config, content, source, apply=args.apply, backup_root=args.backup_root),ensure_ascii=False))
