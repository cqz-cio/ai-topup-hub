"""Apply the reviewed public QZWEB catalog through the shop's admin API.

Credentials are read locally and never written into the catalog or output.
This tool does not create orders, supplier connections, or card inventory.
"""

import argparse
from datetime import datetime
from decimal import Decimal
import html
import json
from pathlib import Path
import re
import sys
import urllib.error
import urllib.parse
import urllib.request

from redemption_guide import catalog_fields, load_guide
from catalog_presentation import catalog_fields as presentation_fields, public_copy as presentation_copy


def main():
    repo = Path(__file__).resolve().parents[1]
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base-url", default="http://127.0.0.1:8080")
    parser.add_argument("--credentials-file", type=Path, required=True)
    parser.add_argument("--manifest", type=Path, default=repo / "docs/catalog/qzweb-all.json")
    parser.add_argument("--backup-dir", type=Path, required=True)
    args = parser.parse_args()
    parsed = urllib.parse.urlsplit(args.base_url)
    if parsed.username or parsed.password or parsed.query or parsed.fragment:
        raise ValueError("Use a base URL without credentials, query or fragment")
    if parsed.scheme != "https" and not (
        parsed.scheme == "http" and parsed.hostname in {"127.0.0.1", "localhost", "::1"}
    ):
        raise ValueError("The admin API must use HTTPS or a loopback address")
    base = args.base_url.rstrip("/") + "/api/v1"
    headers = {"Content-Type": "application/json; charset=utf-8"}

    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, request, fp, code, message, response_headers, new_url):
            return None

    opener = urllib.request.build_opener(NoRedirect())

    def api(method, path, body=None):
        data = None if body is None else json.dumps(body, ensure_ascii=False).encode("utf-8")
        request_headers = {
            key: value for key, value in headers.items()
            if key != "Authorization" or not path.startswith("/public/")
        }
        request = urllib.request.Request(base + path, data=data, headers=request_headers, method=method)
        try:
            with opener.open(request, timeout=30) as response:
                payload = json.load(response)
        except urllib.error.HTTPError as error:
            raise RuntimeError(f"Admin API failed: {method} {path}, HTTP {error.code}") from None
        if payload.get("status_code") != 0:
            raise RuntimeError(f"Admin API rejected: {method} {path}, code {payload.get('status_code')}")
        return payload

    manifest = json.loads(args.manifest.read_text(encoding="utf-8-sig"))
    redemption_guide = load_guide()
    groups = manifest.get("categories") or [{**manifest["category"], "products": manifest["products"]}]
    items = [item for group in groups for item in group["products"]]
    assert len({group["id"] for group in groups}) == len(groups)
    assert len({item["id"] for item in items}) == len(items) and items
    price_overrides = json.loads((repo / "docs/catalog/qzweb-price-overrides.json").read_text(encoding="utf-8-sig"))
    effective_prices = {
        item["id"]: Decimal(str(price_overrides.get(str(item["id"]), {}).get("price", item["price"])))
        for item in items
    }
    for item in items:
        assert item["status"] == 1 and item["delivery_way"] in {0, 1}
        assert not item["config"] and not item["widget"]
        assert Decimal(str(item["price"])) > 0
        assert effective_prices[item["id"]].is_finite() and effective_prices[item["id"]] > 0
        assert not re.search(r"<script|\bon\w+\s*=|javascript:", item["content_html"], re.I)

    credentials = json.loads(args.credentials_file.read_text(encoding="utf-8-sig"))
    login = api("POST", "/admin/login", {key: credentials[key] for key in ("username", "password")})
    headers["Authorization"] = "Bearer " + login["data"]["token"]
    del credentials, login
    products = []
    page = 1
    while True:
        response = api("GET", f"/admin/products?page={page}&page_size=100")
        products.extend(response["data"])
        if page >= response["pagination"]["total_page"]:
            break
        page += 1
    categories = api("GET", "/admin/categories")["data"]
    args.backup_dir.mkdir(parents=True, exist_ok=True)
    stamp = datetime.now().strftime("%Y%m%d-%H%M%S-%f")
    backup = args.backup_dir / f"catalog-before-qzweb-{stamp}.json"
    backup.write_text(json.dumps({"products": products, "categories": categories}, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")

    # The source snapshot keeps its historic channel labels. Storefront groups
    # follow the user's current taxonomy, so later imports cannot undo it.
    category_layout = json.loads((repo / "docs/catalog/catalog-categories.json").read_text(encoding="utf-8"))
    category_definitions = {group["key"]: group for group in category_layout["categories"]}
    category_map = {}
    saved_categories = {}
    for index, group in enumerate(groups):
        category_body = {
            "slug": f"qzweb-category-{group['id']}",
            "name": {"zh-CN": group["name"]},
            "icon": group.get("local_icon", ""),
            "parent_id": 0,
            "sort_order": 1000 - index,
        }
        override_key = category_layout["source_category_overrides"].get(str(group["id"]))
        if override_key:
            definition = category_definitions[override_key]
            category_body.update({key: definition[key] for key in ("slug", "name", "icon", "sort_order")})
        if category_body["slug"] in saved_categories:
            category_map[group["id"]] = saved_categories[category_body["slug"]]
            continue
        prior_category = next((row for row in categories if row["slug"] == category_body["slug"]), None)
        category = api("PUT", f"/admin/categories/{prior_category['id']}", category_body)["data"] if prior_category else api("POST", "/admin/categories", category_body)["data"]
        category_map[group["id"]] = category
        saved_categories[category_body["slug"]] = category
    source_categories = {item["id"]: group for group in groups for item in group["products"]}
    existing = {row["slug"]: row for row in products}
    updated = []
    for index, item in enumerate(items):
        price = effective_prices[item["id"]]
        source_category = source_categories[item["id"]]
        category = category_map[source_category["id"]]
        slug = f"qzweb-{item['id']}"
        prior = existing.get(slug)
        content, instructions = catalog_fields(
            slug, {"zh-CN": item["content_html"]}, {"zh-CN": item["content_html"]}, redemption_guide,
        )
        intro = re.search(r"<p(?:\s[^>]*)?>(.*?)</p>", item["content_html"], re.S | re.I)
        summary = html.unescape(re.sub(r"<[^>]*>", "", intro.group(1))).strip() if intro else item["name"]
        title, description, content, images = presentation_fields(slug, {"zh-CN": item["name"]}, {"zh-CN": summary}, content, [item["local_cover"]])
        sku = {
            "sku_code": f"QZWEB-{item['id']}",
            "spec_values": {"\u5957\u9910": title["zh-CN"]},
            "price_amount": float(price),
            "cost_price_amount": 0,
            "manual_stock_total": 0,
            "is_active": True,
            "sort_order": 0,
        }
        if prior and prior.get("skus"):
            assert len(prior["skus"]) == 1, "Review multi-SKU products before syncing"
            sku["id"] = prior["skus"][0]["id"]
            sku["cost_price_amount"] = float(prior["skus"][0].get("cost_price_amount") or 0)
        body = {
            "category_id": category["id"], "slug": slug,
            "title": title,
            "description": description,
            "content": content,
            "instructions": instructions,
            "price_amount": float(price),
            "cost_price_amount": float(prior.get("cost_price_amount") or 0) if prior else 0,
            "images": images,
            "tags": presentation_copy([source_category["name"]]),
            "purchase_type": "member" if item["only_user"] else "guest",
            "min_purchase_quantity": item["minimum"],
            "max_purchase_quantity": item["maximum"],
            "stock_display_mode": (prior.get("stock_display_mode") or "status") if prior else "status",
            "fulfillment_type": (prior["fulfillment_type"] if prior else "auto") if item["delivery_way"] == 0 else "manual",
            "manual_stock_total": 0,
            "manual_form_schema": {"fields": []},
            "skus": [sku], "is_active": True, "sort_order": 10000 - index,
        }
        if prior:
            channels = prior.get("payment_channel_ids") or []
            body["payment_channel_ids"] = json.loads(channels) if isinstance(channels, str) else channels
            body["is_affiliate_enabled"] = prior.get("is_affiliate_enabled", False)
        product = api("PUT", f"/admin/products/{prior['id']}", body)["data"] if prior else api("POST", "/admin/products", body)["data"]
        public = api("GET", f"/public/products/{slug}")["data"]
        assert public["title"] == title
        assert Decimal(public["price_amount"]) == price
        assert public["content"] == content
        assert product["instructions"] == instructions
        assert public["images"] == images
        assert Decimal(public["skus"][0]["price_amount"]) == price
        assert public["category_id"] == category["id"]
        assert public["fulfillment_type"] == body["fulfillment_type"]
        updated.append({"id": product["id"], "slug": slug, "source_id": item["id"], "price": public["price_amount"]})

    retired = [row["id"] for row in products if row["slug"] in manifest.get("retire_slugs", []) and row["is_active"]]
    for product_id in retired:
        api("PATCH", f"/admin/products/{product_id}", {"is_active": False})
    retired_categories = [row["id"] for row in categories if row["slug"] in manifest.get("retire_category_slugs", []) and row["is_active"]]
    for category_id in retired_categories:
        api("PATCH", f"/admin/categories/{category_id}/active", {"is_active": False})
    report = {"source_url": manifest["source_url"], "category_ids": {key: value["id"] for key, value in category_map.items()}, "updated": updated, "retired_product_ids": retired, "retired_category_ids": retired_categories, "backup": str(backup), "orders_created": 0, "supplier_purchases": 0}
    (args.backup_dir / f"qzweb-catalog-sync-{stamp}.json").write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps(report, ensure_ascii=False))


if __name__ == "__main__":
    sys.stdout.reconfigure(encoding="utf-8")
    main()
