#!/usr/bin/env python3
"""Anonymous GET/HEAD evidence only. No credentials, DB access or mutations.

Run before/after stage one with --expect-cart blocked. After an approved release,
use --expect-cart allowed. Output contains only selected public catalog fields.
"""
import argparse
import hashlib
import json
import re
from datetime import datetime, timezone
from html.parser import HTMLParser
from pathlib import Path
from urllib.error import HTTPError
from urllib.request import HTTPRedirectHandler, Request, build_opener
from xml.etree import ElementTree


class NoRedirect(HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def fetch(base, path, method="GET"):
    request = Request(base + path, method=method, headers={"User-Agent": "Qzweb-SEO-v6-readonly/1.0"})
    try:
        response = build_opener(NoRedirect).open(request, timeout=40)
    except HTTPError as error:
        response = error
    with response:
        body = response.read()
        record = {"path": path, "method": method, "status": response.status,
                  "headers": {k.lower(): v for k, v in response.headers.items()
                              if k.lower() in {"content-type", "cache-control", "x-robots-tag", "location", "vary"}},
                  "bytes": len(body), "sha256": hashlib.sha256(body).hexdigest()}
        return record, body.decode("utf-8")


class Page(HTMLParser):
    def __init__(self, body):
        super().__init__()
        self.meta, self.canonical, self.jsonld, self.text = {}, [], [], []
        self.script_type = None
        self.script_text = ""
        self.hidden = 0
        self.feed(body)

    def handle_starttag(self, tag, attrs):
        attrs = dict(attrs)
        if tag == "meta" and (attrs.get("name") or attrs.get("property")):
            self.meta[attrs.get("name", attrs.get("property", ""))] = attrs.get("content", "")
        if tag == "link" and attrs.get("rel") == "canonical":
            self.canonical.append(attrs.get("href"))
        if tag in {"script", "style"}:
            self.hidden += 1
        if tag == "script":
            self.script_type, self.script_text = attrs.get("type"), ""

    def handle_endtag(self, tag):
        if tag == "script":
            if self.script_type == "application/ld+json":
                self.jsonld.append(json.loads(self.script_text))
            self.script_type = None
        if tag in {"script", "style"}:
            self.hidden = max(0, self.hidden - 1)

    def handle_data(self, value):
        if self.script_type == "application/ld+json":
            self.script_text += value
        if not self.hidden and value.strip():
            self.text.append(value.strip())


def allowed(robots, target):
    """Single wildcard user-agent group; longest matching rule, Allow wins ties.

    Only the site's ASCII paths are evaluated here. '?' is literal; '*' and '$'
    follow Google's REP semantics, unlike urllib.robotparser's prefix-only rules.
    """
    best, result = -1, True
    for line in robots.splitlines():
        name, sep, pattern = line.partition(":")
        pattern = pattern.strip()
        if not sep or name.lower() not in {"allow", "disallow"} or not pattern:
            continue
        terminal = pattern.endswith("$")
        core = pattern[:-1] if terminal else pattern
        regex = "^" + re.escape(core).replace(r"\*", ".*") + ("$" if terminal else "")
        size = len(core.replace("*", ""))
        if re.search(regex, target) and (size > best or size == best and name.lower() == "allow"):
            best, result = size, name.lower() == "allow"
    return result


def collect(base, expect_cart):
    report = {"timestamp_utc": datetime.now(timezone.utc).isoformat(), "base": base,
              "mode": "anonymous-read-only", "expected_cart_crawl": expect_cart,
              "requests": [], "pages": {}, "errors": []}

    def get(path, method="GET"):
        record, body = fetch(base, path, method)
        report["requests"].append(record)
        return record, body

    def check(condition, message):
        if not condition:
            report["errors"].append(message)

    _, robots = get("/robots.txt")
    report["robots"] = robots
    cart_paths = ["/cart", "/cart/", "/cart?utm=test", "/cart/?utm=test", "/cart?next=/checkout"]
    for path in cart_paths:
        headers = []
        for method in ["GET", "HEAD"]:
            record, _ = get(path, method)
            headers.append(record["headers"])
            check(record["status"] == 200, f"{method} {path}: expected public shell 200")
            check(record["headers"].get("x-robots-tag") == "noindex, nofollow", f"{method} {path}: noindex header")
            cache = record["headers"].get("cache-control", "")
            check("no-cache" in cache or "no-store" in cache, f"{method} {path}: cache policy")
            check("public" not in cache and "s-maxage" not in cache, f"{method} {path}: shared cache")
            if method == "HEAD":
                check(record["bytes"] == 0, f"HEAD {path}: body")
        check(headers[0] == headers[1], f"{path}: GET/HEAD header mismatch")
        check(allowed(robots, path) == (expect_cart == "allowed"), f"{path}: unexpected robots match")
    report["robots_matches"] = {path: allowed(robots, path) for path in cart_paths + [
        "/cart/checkout", "/cart/api", "/cart/extra", "/cartoon", "/cart-old",
        "/checkout", "/pay", "/orders/1", "/recharge-orders/1", "/admin/", "/auth/login", "/me/", "/guest/orders",
        "/api/private", "/api/v1/public/products", "/api/v1/public/products?search=Plus", "/sitemap.xml",
        "/me", "/guest", "/admin", "/auth", "/orders", "/recharge-orders", "/payment-guide"]}
    for path in ["/cart/checkout", "/cart/api", "/cart/extra"]:
        get(path)
        check(not allowed(robots, path), f"{path}: subpath crawl opened")
    for path in ["/", "/products", "/blog", "/products/chatgpt-plus-code", "/categories/gpt-recharge",
                 "/blog/chatgpt-ph-recharge-guide", "/blog/chatgpt-plus-code-guide", "/blog/chatgpt-code-region-check",
                 "/blog/chatgpt-payment-methods", "/sitemap.xml", "/robots.txt"]:
        record, body = get(path)
        check(record["status"] == 200, f"{path}: status {record['status']}")
        if path == "/sitemap.xml":
            urls = [el.text for el in ElementTree.fromstring(body).iter() if el.tag.endswith("}loc")]
            report["sitemap_urls"] = urls
            check(not any(re.search(r"/cart(?:[/?]|$)", url) for url in urls), "cart in sitemap")
        elif path != "/robots.txt":
            page = Page(body)
            report["pages"][path] = {"canonical": page.canonical, "meta": page.meta,
                                      "jsonld": page.jsonld, "text": "\n".join(page.text)}
    products = []
    page_number = 1
    while True:
        record, body = get(f"/api/v1/public/products?page={page_number}&page_size=100")
        check(record["status"] == 200, "catalog status")
        payload = json.loads(body)
        products.extend(payload["data"])
        if page_number >= payload["pagination"]["total_page"]:
            break
        page_number += 1
    fields = ("id", "slug", "title", "tags", "price_amount", "effective_price_amount", "stock_status", "stock_quantity_hidden", "is_sold_out", "stock_display", "skus")
    report["catalog"] = sorted([{k: p[k] for k in fields if k in p} for p in products], key=lambda p: p["id"])
    report["tagged_products"] = [p for p in report["catalog"] if any("菲" in tag for tag in p.get("tags", []))]
    report["plus"] = json.loads(get("/api/v1/public/products/chatgpt-plus-code")[1])["data"]
    # The endpoint is public, but retain only content/identity fields needed for review.
    report["plus"] = {k: report["plus"][k] for k in ("id", "slug", "title", "tags", "description", "content", "seo_meta", "category")}
    report["status"] = "passed" if not report["errors"] else "failed"
    return report


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--base", default="https://www.qzweb.cloud")
    parser.add_argument("--expect-cart", choices=["blocked", "allowed"], required=True)
    parser.add_argument("--output", type=Path, required=True)
    parser.add_argument("--compare", type=Path)
    args = parser.parse_args()
    report = collect(args.base.rstrip("/"), args.expect_cart)
    if args.compare:
        before = {p["id"]: p for p in json.loads(args.compare.read_text(encoding="utf-8"))["catalog"]}
        after = {p["id"]: p for p in report["catalog"]}
        report["catalog_changes"] = [{"id": key, "before": before.get(key), "after": after.get(key)}
                                     for key in sorted(before.keys() | after.keys()) if before.get(key) != after.get(key)]
    args.output.parent.mkdir(parents=True, exist_ok=True)
    args.output.write_text(json.dumps(report, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    print(json.dumps({k: report[k] for k in ("status", "timestamp_utc", "errors")}, ensure_ascii=False))
    print(f"requests={len(report['requests'])} products={len(report['catalog'])} sitemap={len(report['sitemap_urls'])}")
    raise SystemExit(bool(report["errors"]))


if __name__ == "__main__":
    main()
