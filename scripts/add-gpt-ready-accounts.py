"""Create the reviewed manual-delivery catalog without modifying existing data.

Default is a transaction rolled back after validation. --apply first takes a
consistent private backup, then inserts one category, two products and two SKUs.
No orders, payments, supplier calls, credentials or frontend changes are involved.
"""
import argparse
from datetime import datetime, timezone
from decimal import Decimal
import hashlib
import json
import os
from pathlib import Path
import sqlite3
import time


def encode(value):
    return json.dumps(value, ensure_ascii=False, separators=(",", ":")).encode("utf-8")


def snapshot(db):
    result = {}
    for (name,) in db.execute(
        "SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'"
    ).fetchall():
        quoted = '"' + name.replace('"', '""') + '"'
        result[name] = list(db.execute(f"SELECT * FROM {quoted} ORDER BY rowid"))
    return result


def insert(db, table, values):
    columns = ",".join(values)
    markers = ",".join("?" for _ in values)
    return db.execute(
        f"INSERT INTO {table} ({columns}) VALUES ({markers})", list(values.values())
    ).lastrowid


def create_catalog(db_path, catalog_path, backup_dir=None, apply=False):
    db_path = Path(db_path).resolve(strict=True)
    catalog_path = Path(catalog_path).resolve(strict=True)
    config = json.loads(catalog_path.read_text(encoding="utf-8"))
    assert config["version"] == 1
    assert [p["slug"] for p in config["products"]] == [
        "gpt-plus-brazil-account-24h", "gpt-plus-vietnam-account-30d"
    ]
    assert [Decimal(p["price_amount"]) for p in config["products"]] == [
        Decimal("90.00"), Decimal("115.00")
    ]
    defaults = config["creation_defaults"]
    assert defaults["fulfillment_type"] == "manual"
    assert defaults["manual_stock_total"] == 0
    contents = {}
    for product in config["products"]:
        path = (catalog_path.parent / product["content_file"]).resolve(strict=True)
        assert path.parent == catalog_path.parent
        contents[product["slug"]] = path.read_text(encoding="utf-8").strip()
        assert "Cqz0924ttxk" in contents[product["slug"]]
        assert "先联系客服" in contents[product["slug"]]
    now = datetime.now(timezone.utc)
    backup_path = None
    if apply:
        assert backup_dir, "A private backup directory is required"
        directory = Path(backup_dir) / now.strftime("before-gpt-ready-accounts-%Y%m%dT%H%M%S%fZ")
        directory.mkdir(parents=True, mode=0o700)
        backup_path = directory / "shop.db"
        deadline = time.monotonic() + 20
        def progress(status, remaining, total):
            if time.monotonic() > deadline:
                raise TimeoutError("Database backup exceeded 20 seconds")
        with sqlite3.connect(db_path.as_uri() + "?mode=ro", uri=True, timeout=5) as source:
            with sqlite3.connect(backup_path) as target:
                source.backup(target, pages=512, progress=progress, sleep=0.05)
                assert target.execute("PRAGMA quick_check").fetchone()[0] == "ok"
        if os.name != "nt":
            backup_path.chmod(0o600)
    created = {"categories": [], "products": [], "product_skus": []}
    ids = {}
    with sqlite3.connect(db_path.as_uri() + "?mode=rw", uri=True, timeout=5) as db:
        deadline = time.monotonic() + 30
        db.set_progress_handler(lambda: int(time.monotonic() > deadline), 10000)
        db.execute("PRAGMA foreign_keys=ON")
        db.execute("BEGIN IMMEDIATE")
        try:
            before = snapshot(db)
            fk_before = db.execute("PRAGMA foreign_key_check").fetchall()
            category = config["category"]
            row = db.execute("SELECT id,name_json,deleted_at FROM categories WHERE slug=?",
                             (category["slug"],)).fetchone()
            if row:
                assert row[2] is None and json.loads(row[1]) == category["name"], "Category conflict"
                category_id = row[0]
            else:
                category_id = insert(db, "categories", {
                    "parent_id": 0, "slug": category["slug"], "name_json": encode(category["name"]),
                    "icon": category["icon"], "sort_order": category["sort_order"],
                    "is_active": 1, "created_at": now.isoformat(),
                })
                created["categories"].append(category_id)
            for product in config["products"]:
                slug = product["slug"]
                values = {
                    "category_id": category_id, "slug": slug,
                    "title_json": encode({"zh-CN": product["title"]}),
                    "description_json": encode({"zh-CN": product["description"]}),
                    "content_json": encode({"zh-CN": contents[slug]}),
                    "instructions_json": encode({"zh-CN": product["instructions"]}),
                    "seo_meta_json": encode({}), "price_amount": product["price_amount"],
                    "cost_price_amount": 0, "wholesale_prices": encode([]),
                    "images": encode(defaults["images"]), "tags": encode(product["tags"]),
                    "purchase_type": defaults["purchase_type"],
                    "stock_display_mode": defaults["stock_display_mode"],
                    "fulfillment_type": "manual", "manual_stock_total": 0,
                    "manual_form_schema_json": encode({}), "payment_channel_ids": "[]",
                    "is_affiliate_enabled": 0, "is_mapped": 0, "is_active": 1,
                    "sort_order": product["sort_order"], "created_at": now.isoformat(),
                    "updated_at": now.isoformat(),
                }
                existing = db.execute(
                    "SELECT id,category_id,title_json,price_amount,fulfillment_type,is_mapped,deleted_at "
                    "FROM products WHERE slug=?", (slug,)
                ).fetchone()
                if existing:
                    assert existing[1] == category_id and json.loads(existing[2]) == {"zh-CN": product["title"]}
                    assert Decimal(str(existing[3])) == Decimal(product["price_amount"])
                    assert existing[4:] == ("manual", 0, None), "Product conflict"
                    product_id = existing[0]
                    stored_content = db.execute(
                        "SELECT content_json,description_json,instructions_json FROM products WHERE id=?",
                        (product_id,)
                    ).fetchone()
                    assert all(json.loads(raw) == json.loads(values[key]) for raw, key in zip(
                        stored_content, ("content_json", "description_json", "instructions_json")
                    )), "Existing product copy differs; review before updating"
                    assert db.execute("SELECT count(*) FROM product_skus WHERE product_id=? AND deleted_at IS NULL",
                                      (product_id,)).fetchone()[0] == 1
                else:
                    product_id = insert(db, "products", values)
                    created["products"].append(product_id)
                    sku_id = insert(db, "product_skus", {
                        "product_id": product_id, "sku_code": "DEFAULT",
                        "spec_values_json": encode({}), "price_amount": product["price_amount"],
                        "cost_price_amount": 0, "manual_stock_total": 0,
                        "manual_stock_locked": 0, "manual_stock_sold": 0,
                        "is_active": 1, "sort_order": 0,
                        "created_at": now.isoformat(), "updated_at": now.isoformat(),
                    })
                    created["product_skus"].append(sku_id)
                ids[slug] = product_id
            after = snapshot(db)
            for table, rows in before.items():
                if table in created:
                    # The ID column is first in these three verified model tables.
                    assert [r for r in after[table] if r[0] not in created[table]] == rows, table
                    assert len(after[table]) - len(rows) == len(created[table]), table
                else:
                    assert after[table] == rows, "Unrelated table changed: " + table
            assert db.execute("PRAGMA foreign_key_check").fetchall() == fk_before
            if apply:
                db.commit()
            else:
                db.rollback()
        except BaseException:
            db.rollback()
            raise
    report = {
        "passed": True, "applied": apply, "category_id": category_id, "product_ids": ids,
        "created": created, "backup": str(backup_path) if backup_path else None,
        "existing_rows_unchanged": True, "protected_tables": len(before) - 3,
        "manual_stock_total": 0, "purchase_flow": config["purchase_flow"],
        "business_api_calls": 0,
        "catalog_sha256": hashlib.sha256(catalog_path.read_bytes()).hexdigest(),
    }
    if backup_path:
        (backup_path.parent / "report.json").write_text(
            json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8"
        )
    return report


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--db", required=True, type=Path)
    parser.add_argument("--catalog", required=True, type=Path)
    parser.add_argument("--backup-dir", type=Path)
    parser.add_argument("--apply", action="store_true")
    args = parser.parse_args()
    print(json.dumps(create_catalog(args.db, args.catalog, args.backup_dir, args.apply),
                     ensure_ascii=False, indent=2))
