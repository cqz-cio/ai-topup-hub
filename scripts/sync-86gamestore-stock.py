"""Mirror 86GameStore stock for manual goods; never enable or place purchases.

The existing backend decrypts the merchant credential and reads the supplier.
Only stock fields are written in a short transaction after the full fetch.
"""
import argparse
from contextlib import closing
from datetime import datetime, timezone
import json
from pathlib import Path
import re
import sqlite3
import sys
import time
import urllib.parse
import urllib.request

SUPPLIER_URL = "https://faka.redeemgpt.com"
MERCHANT_ID = "26113"
PREFIX = "86gamestore-"


def require(condition, message):
    if not condition:
        raise ValueError(message)


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, *args, **kwargs):
        return None


class AdminAPI:
    def __init__(self, base_url, credentials_file):
        base = urllib.parse.urlsplit(base_url)
        require(base.scheme == "http" and base.hostname in {"127.0.0.1", "localhost", "::1"}
                and base.path in {"", "/"} and not (base.username or base.password or base.query or base.fragment),
                "Use a loopback backend URL")
        self.base_url = base_url.rstrip("/") + "/api/v1"
        self.opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
        self.token = None
        credentials = json.loads(Path(credentials_file).read_text(encoding="utf-8-sig"))
        self.token = self.call("/admin/login", {k: credentials[k] for k in ("username", "password")})["token"]
        credentials.clear()

    def call(self, path, body=None):
        allowed = (path == "/admin/login" and body is not None) or (
            body is None and re.fullmatch(r"/admin/upstream-products\?connection_id=\d+&page=\d+&page_size=100", path))
        require(allowed, "Endpoint is outside the read-only inventory allowlist")
        headers = {"Content-Type": "application/json"}
        if self.token:
            headers["Authorization"] = "Bearer " + self.token
        request = urllib.request.Request(self.base_url + path, headers=headers,
                                         data=None if body is None else json.dumps(body).encode())
        with self.opener.open(request, timeout=45) as response:
            raw = response.read(16 * 1024 * 1024 + 1)
        require(len(raw) <= 16 * 1024 * 1024, "Inventory response is too large")
        payload = json.loads(raw)
        require(payload.get("status_code") == 0, "Inventory API rejected request")
        return payload["data"]


def fetch_inventory(api, connection_id):
    items, total = [], None
    for page in range(1, 101):
        result = api.call(f"/admin/upstream-products?connection_id={connection_id}&page={page}&page_size=100")
        current_total = result.get("total")
        require(type(current_total) is int and 0 <= current_total <= 10000, "Invalid upstream total")
        require(total is None or total == current_total, "Catalog changed during pagination; retry next cycle")
        total = current_total
        batch = result.get("items")
        require(isinstance(batch, list) and len(batch) <= 100, "Invalid upstream page")
        items.extend(batch)
        require(len(items) <= total, "Upstream page exceeds total")
        if len(items) == total:
            return normalize_inventory(items)
        require(len(batch) == 100, "Incomplete upstream catalog; keep previous inventory")
    raise ValueError("Upstream pagination limit exceeded")


def normalize_inventory(items):
    result = {}
    for product in items:
        pid = product.get("id")
        require(type(pid) is int and pid > 0 and pid not in result, "Duplicate or invalid upstream product")
        require(type(product.get("is_active")) is bool and isinstance(product.get("skus"), list), "Invalid upstream product state")
        skus = {}
        for sku in product["skus"]:
            sid, quantity = sku.get("id"), sku.get("stock_quantity")
            require(type(sid) is int and sid > 0 and sid not in skus, "Duplicate or invalid upstream SKU")
            require(type(quantity) is int and 0 <= quantity <= 2147483647, "Invalid ACG stock quantity")
            require(type(sku.get("is_active")) is bool, "Invalid upstream SKU state")
            active = product["is_active"] and sku["is_active"]
            skus[sid] = {"quantity": quantity if active else 0, "active": active}
        result[pid] = {"active": product["is_active"], "skus": skus}
    return result


def database_connection(db):
    rows = db.execute("SELECT id,protocol,status,auto_sync_price FROM site_connections "
                      "WHERE base_url=? AND api_key=? AND deleted_at IS NULL", (SUPPLIER_URL, MERCHANT_ID)).fetchall()
    require(len(rows) == 1 and tuple(rows[0])[1:] == ("acg-faka", "disabled", 0),
            "Expected one disabled merchant 26113 connection without automatic pricing")
    return rows[0][0]


def stock_audit(db, targets):
    """For first application/tests, prove every non-stock field stayed identical."""
    result = {}
    for (table,) in db.execute("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%' ORDER BY name"):
        rows = []
        for raw in db.execute('SELECT rowid AS audit_rowid,* FROM "' + table.replace('"', '""') + '" ORDER BY rowid'):
            row = dict(raw)
            ids, columns = targets.get(table, (set(), set()))
            if row.get("id") in ids:
                for column in columns:
                    row.pop(column, None)
            rows.append(row)
        result[table] = rows
    return result


def apply_inventory(database, inventory, connection_id, audit=False):
    stamp = datetime.now(timezone.utc).isoformat()
    db = sqlite3.connect(Path(database).resolve().as_uri() + "?mode=rw", uri=True, timeout=20)
    db.row_factory = sqlite3.Row
    db.execute("PRAGMA foreign_keys=ON")
    try:
        db.execute("BEGIN IMMEDIATE")
        require(database_connection(db) == connection_id, "Supplier connection changed")
        mappings = db.execute("SELECT * FROM product_mappings WHERE connection_id=? AND deleted_at IS NULL ORDER BY id", (connection_id,)).fetchall()
        require(bool(mappings), "No manual supplier mappings")
        products, sku_rows, sku_mappings = {}, {}, {}
        for mapping in mappings:
            require(not mapping["is_active"] and mapping["stock_limit"] >= 0, "Automatic mapping or invalid stock limit")
            product = db.execute("SELECT * FROM products WHERE id=? AND deleted_at IS NULL", (mapping["local_product_id"],)).fetchone()
            require(product is not None and product["fulfillment_type"] == "manual" and product["is_mapped"]
                    and product["slug"] == PREFIX + str(mapping["upstream_product_id"]), "Manual product identity changed")
            products[product["id"]] = product
            rows = db.execute("SELECT * FROM product_skus WHERE product_id=? AND deleted_at IS NULL", (product["id"],)).fetchall()
            links = db.execute("SELECT * FROM sku_mappings WHERE product_mapping_id=? AND deleted_at IS NULL", (mapping["id"],)).fetchall()
            require(rows and len(rows) == len(links) and {s["id"] for s in rows} == {s["local_sku_id"] for s in links},
                    "Incomplete SKU mapping; preserve existing inventory")
            require(all(s["manual_stock_locked"] >= 0 and s["manual_stock_sold"] >= 0 for s in rows), "Invalid local stock counters")
            sku_rows.update({s["id"]: s for s in rows})
            sku_mappings[mapping["id"]] = links
        targets = {
            "products": (set(products), {"manual_stock_total", "updated_at"}),
            "product_skus": (set(sku_rows), {"manual_stock_total", "updated_at"}),
            "product_mappings": ({m["id"] for m in mappings}, {"upstream_status", "last_synced_at", "updated_at"}),
            "sku_mappings": ({s["id"] for rows in sku_mappings.values() for s in rows},
                             {"upstream_stock", "upstream_is_active", "stock_synced_at", "updated_at"}),
        }
        before = stock_audit(db, targets) if audit else None
        report = []
        for mapping in mappings:
            upstream = inventory.get(mapping["upstream_product_id"])
            active = bool(upstream and upstream["active"])
            total = 0
            for link in sku_mappings[mapping["id"]]:
                sku = sku_rows[link["local_sku_id"]]
                source = upstream["skus"].get(link["upstream_sku_id"]) if active else None
                upstream_active = bool(source and source["active"])
                raw_stock = source["quantity"] if upstream_active else 0
                capped = min(raw_stock, mapping["stock_limit"]) if mapping["stock_limit"] else raw_stock
                # Reservations are local obligations; never restore these as sellable units.
                available = max(0, capped - sku["manual_stock_locked"])
                db.execute("UPDATE product_skus SET manual_stock_total=?,updated_at=? WHERE id=? AND manual_stock_total<>?",
                           (available, stamp, sku["id"], available))
                db.execute("UPDATE sku_mappings SET upstream_stock=?,upstream_is_active=?,stock_synced_at=?,updated_at=? WHERE id=?",
                           (raw_stock, int(upstream_active), stamp, stamp, link["id"]))
                if sku["is_active"]:
                    total += available
                report.append({"product_id": mapping["local_product_id"], "sku_id": sku["id"],
                               "upstream_id": mapping["upstream_product_id"], "upstream_stock": raw_stock,
                               "upstream_active": upstream_active, "local_available": available,
                               "locked": sku["manual_stock_locked"], "sold": sku["manual_stock_sold"]})
            db.execute("UPDATE products SET manual_stock_total=?,updated_at=? WHERE id=? AND manual_stock_total<>?",
                       (total, stamp, mapping["local_product_id"], total))
            db.execute("UPDATE product_mappings SET upstream_status=?,last_synced_at=?,updated_at=? WHERE id=?",
                       ("active" if active else "inactive", stamp, stamp, mapping["id"]))
        if audit:
            require(before == stock_audit(db, targets), "Unexpected non-stock database change; rolling back")
        db.commit()
        return {"at": stamp, "status": "synced", "connection_id": connection_id, "products": len(mappings),
                "inventory": report, "non_stock_fields_preserved": True, "audited_tables": len(before) if audit else None,
                "purchase_endpoint_called": False}
    except Exception:
        db.rollback()
        raise
    finally:
        db.close()


def cycle(args):
    with closing(sqlite3.connect(args.database.resolve().as_uri() + "?mode=ro", uri=True)) as db:
        connection_id = database_connection(db)
    inventory = fetch_inventory(AdminAPI(args.base_url, args.credentials_file), connection_id)
    # Fetch first; a timeout or incomplete response cannot clear existing stock.
    report = apply_inventory(args.database, inventory, connection_id, audit=args.audit)
    args.report_file.parent.mkdir(parents=True, exist_ok=True)
    temporary = args.report_file.with_suffix(".tmp")
    temporary.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    temporary.replace(args.report_file)
    return report


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", default="http://127.0.0.1:8080")
    parser.add_argument("--credentials-file", type=Path, required=True)
    parser.add_argument("--database", type=Path, required=True)
    parser.add_argument("--report-file", type=Path, required=True)
    parser.add_argument("--audit", action="store_true")
    parser.add_argument("--watch", action="store_true")
    parser.add_argument("--interval", type=int, default=300)
    args = parser.parse_args()
    require(args.interval >= 60, "Use an interval of at least 60 seconds")
    while True:
        try:
            report = cycle(args)
            print(json.dumps({k: v for k, v in report.items() if k != "inventory"}), flush=True)
        except Exception as error:
            # Do not log API responses, headers, credentials or traceback locals.
            print(json.dumps({"at": datetime.now(timezone.utc).isoformat(), "status": "failed",
                              "error_type": type(error).__name__}), flush=True)
            if not args.watch:
                raise SystemExit(1) from None
        if not args.watch:
            return
        time.sleep(args.interval)


if __name__ == "__main__":
    main()
