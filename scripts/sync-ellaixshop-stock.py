"""Refresh merchant 1039 stock through admin sync endpoints, without purchasing.

The backend keeps the supplier secret encrypted. This helper only needs a local
admin login and never calls order, payment, or procurement endpoints.
"""
import argparse
from concurrent.futures import ThreadPoolExecutor
from datetime import datetime, timezone
from decimal import Decimal
import json
from pathlib import Path
import re
import sqlite3
import sys
import time
import urllib.parse
import urllib.request


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, request, fp, code, message, response_headers, new_url):
        return None


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", default="http://127.0.0.1:8080")
    parser.add_argument("--credentials-file", type=Path, required=True)
    parser.add_argument("--report-file", type=Path, required=True)
    parser.add_argument("--live-db", type=Path)
    parser.add_argument("--mirror-db", type=Path)
    parser.add_argument("--watch", action="store_true")
    parser.add_argument("--interval", type=int, default=300)
    args = parser.parse_args()
    base = urllib.parse.urlsplit(args.base_url)
    if base.username or base.password or base.query or base.fragment:
        raise ValueError("Use a base URL without credentials, query or fragment")
    if base.scheme != "https" and not (base.scheme == "http" and base.hostname in {"localhost", "127.0.0.1", "::1"}):
        raise ValueError("Admin calls require HTTPS or a loopback address")
    if args.interval < 60 or bool(args.live_db) != bool(args.mirror_db):
        raise ValueError("Use an interval >= 60 seconds and provide both database paths for mirroring")
    headers = {"Content-Type": "application/json; charset=utf-8"}

    def api(method, path, body=None):
        allowed = (
            (method == "POST" and path == "/admin/login") or
            (method == "GET" and path.startswith(("/admin/site-connections?", "/admin/product-mappings?", "/public/products?"))) or
            (method == "POST" and re.fullmatch(r"/admin/(?:product-mappings/\d+/sync|site-connections/\d+/ping)", path))
        )
        if not allowed:
            raise ValueError("Endpoint is outside the stock-only allowlist")
        request_headers = {k: v for k, v in headers.items() if k != "Authorization" or not path.startswith("/public/")}
        request = urllib.request.Request(args.base_url.rstrip("/") + "/api/v1" + path, data=None if body is None else json.dumps(body).encode(), headers=request_headers, method=method)
        try:
            with urllib.request.build_opener(NoRedirect()).open(request, timeout=45) as response:
                payload = json.load(response)
        except OSError as error:
            raise RuntimeError(f"Stock API failed: {method} {path}, {type(error).__name__}") from None
        if payload.get("status_code") != 0:
            raise RuntimeError(f"Stock API rejected {method} {path}")
        return payload

    def pages(path):
        rows, page = [], 1
        while True:
            payload = api("GET", f"{path}{'&' if '?' in path else '?'}page={page}&page_size=100")
            rows.extend(payload["data"])
            if page >= payload.get("pagination", {}).get("total_page", 1):
                return rows
            page += 1

    def mirror(connection_id, local_ids):
        if not args.live_db:
            return
        if args.live_db.resolve() == args.mirror_db.resolve():
            raise ValueError("Live and mirror databases must differ")
        source = sqlite3.connect(args.live_db.resolve().as_uri() + "?mode=ro", uri=True)
        source.row_factory = sqlite3.Row
        placeholders = ",".join("?" for _ in local_ids)
        groups = [
            ("site_connections", "id=?", [connection_id], ["base_url", "api_key", "protocol"]),
            ("products", f"id IN ({placeholders})", local_ids, ["slug"]),
            ("product_skus", f"product_id IN ({placeholders})", local_ids, ["product_id", "sku_code"]),
            ("product_mappings", "connection_id=? AND deleted_at IS NULL", [connection_id], ["connection_id", "local_product_id", "upstream_product_id"]),
            ("sku_mappings", "product_mapping_id IN (SELECT id FROM product_mappings WHERE connection_id=? AND deleted_at IS NULL) AND deleted_at IS NULL", [connection_id], ["product_mapping_id", "local_sku_id", "upstream_sku_id"]),
        ]
        target = sqlite3.connect(args.mirror_db)
        target.row_factory = sqlite3.Row
        target.execute("PRAGMA foreign_keys=ON")
        with target:
            for table, condition, values, identity in groups:
                for row in source.execute(f"SELECT * FROM {table} WHERE {condition}", values):
                    old = target.execute(f"SELECT * FROM {table} WHERE id=?", [row["id"]]).fetchone()
                    if old is not None and any(old[k] != row[k] for k in identity):
                        raise ValueError("Database identity conflict in " + table)
                    columns = row.keys()
                    marks = ",".join("?" for _ in columns)
                    updates = ",".join(f"{k}=excluded.{k}" for k in columns if k != "id")
                    target.execute(f"INSERT INTO {table} ({','.join(columns)}) VALUES ({marks}) ON CONFLICT(id) DO UPDATE SET {updates}", tuple(row))
            if target.execute("PRAGMA foreign_key_check").fetchone() is not None:
                raise ValueError("Database foreign key verification failed")
        source.close()
        target.close()

    def cycle():
        credentials = json.loads(args.credentials_file.read_text(encoding="utf-8-sig"))
        login = api("POST", "/admin/login", {k: credentials[k] for k in ("username", "password")})
        headers["Authorization"] = "Bearer " + login["data"]["token"]
        del credentials, login
        connections = pages("/admin/site-connections")
        found = [c for c in connections if c["base_url"] == "https://ellaixshop.com" and c["api_key"] == "1039" and c["protocol"] == "acg-faka"]
        if len(found) != 1 or found[0]["auto_sync_price"] or found[0]["status"] != "active":
            raise ValueError("An active merchant 1039 connection with auto_sync_price disabled is required")
        connection_id = found[0]["id"]
        mappings = [m for m in pages(f"/admin/product-mappings?connection_id={connection_id}") if m["is_active"]]
        if not mappings:
            raise ValueError("No active merchant 1039 mappings")
        before = {p["id"]: p for p in pages("/public/products")}
        balance_before = api("POST", f"/admin/site-connections/{connection_id}/ping")["data"]["balance"]
        with ThreadPoolExecutor(max_workers=2) as pool:
            list(pool.map(lambda m: api("POST", f"/admin/product-mappings/{m['id']}/sync"), mappings))
        after = {p["id"]: p for p in pages("/public/products")}
        for product_id, item in before.items():
            current = after.get(product_id)
            if current is None:
                continue  # The native sync may remove an unavailable supplier item.
            if Decimal(item["price_amount"]) != Decimal(current["price_amount"]):
                raise ValueError("Stock synchronization changed a retail price")
            if {s["id"]: s["price_amount"] for s in item["skus"]} != {s["id"]: s["price_amount"] for s in current["skus"]}:
                raise ValueError("Stock synchronization changed a SKU retail price")
        balance_after = api("POST", f"/admin/site-connections/{connection_id}/ping")["data"]["balance"]
        local_ids = [m["local_product_id"] for m in mappings]
        mirror(connection_id, local_ids)
        inventories = [{"slug": p["slug"], "price": p["price_amount"], "stock": p["auto_stock_available"], "stock_status": p["stock_status"], "quantity_hidden": p["stock_quantity_hidden"]} for pid, p in after.items() if pid in local_ids]
        report = {"at": datetime.now(timezone.utc).isoformat(), "merchant_id": "1039", "connection_id": connection_id, "mapped_count": len(mappings), "inventory": inventories, "balance_before": balance_before, "balance_after": balance_after, "balance_unchanged": Decimal(balance_before) == Decimal(balance_after), "prices_unchanged": True, "supplier_purchases": 0, "purchase_endpoint_called": False, "interval_seconds": args.interval if args.watch else None}
        args.report_file.parent.mkdir(parents=True, exist_ok=True)
        temporary = args.report_file.with_suffix(".tmp")
        temporary.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
        temporary.replace(args.report_file)
        print(json.dumps({k: report[k] for k in ["at", "mapped_count", "balance_before", "balance_after", "balance_unchanged", "prices_unchanged", "supplier_purchases"]}), flush=True)

    while True:
        try:
            cycle()
        except Exception as error:
            print(json.dumps({"at": datetime.now(timezone.utc).isoformat(), "error_type": type(error).__name__, "error": str(error)[:200]}), flush=True)
            if not args.watch:
                raise SystemExit(1) from None
        if not args.watch:
            break
        time.sleep(args.interval)


if __name__ == "__main__":
    sys.stdout.reconfigure(encoding="utf-8")
    main()
