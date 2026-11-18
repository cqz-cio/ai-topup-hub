"""Check public sitemap membership, SSR, draft links and catalog drift."""
import argparse
import json
from concurrent.futures import ThreadPoolExecutor
from datetime import datetime, timezone
from html.parser import HTMLParser
from pathlib import Path
from urllib.parse import urljoin, urlsplit

from verify_seo_v6_public import Page, fetch

ORIGIN = "https://www.qzweb.cloud"


class Links(HTMLParser):
    def __init__(self, base=ORIGIN):
        super().__init__()
        self.base = base
        self.paths = set()

    def handle_starttag(self, tag, attrs):
        href = dict(attrs).get("href")
        if not href:
            return
        url = urlsplit(urljoin(self.base, href))
        if tag == "a" and url.hostname == "www.qzweb.cloud":
            self.paths.add(url.path)


def probe(path):
    rec, body = fetch(ORIGIN, path)
    result = {"status": rec["status"]}
    if rec["status"] == 200:
        page = Page(body)
        result.update(canonical=page.canonical, visible_chars=sum(map(len, page.text)))
        result["draft_markers"] = [s for s in ("需店主确认", "【发布前须店主确认】") if s in body]
        if path in {"/categories/gpt-recharge", "/products/chatgpt-plus-code", "/blog/chatgpt-ph-recharge-guide"}:
            links = Links(ORIGIN + path)
            links.feed(body)
            result["catalog_links"] = sorted(p for p in links.paths if p.startswith(("/products/", "/categories/")))
    elif "location" in rec["headers"]:
        result["location"] = rec["headers"]["location"]
    return path, result


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    for name in ("baseline", "current", "drafts", "output"):
        parser.add_argument("--" + name, type=Path, required=True)
    args = parser.parse_args()
    before, after = [json.loads(p.read_text(encoding="utf-8")) for p in (args.baseline, args.current)]
    old, new = [set(urlsplit(u).path for u in snap["sitemap_urls"]) for snap in (before, after)]
    links = Links()
    for name in ("category", "plus", "ph-guide", "ready-account"):
        links.feed((args.drafts / f"draft-{name}.html").read_text(encoding="utf-8"))
    alias = "/products/qzweb-5"
    missing = "/blog/chatgpt-ready-account-guide"
    with ThreadPoolExecutor(max_workers=4) as pool:
        pages = dict(pool.map(probe, sorted(old | new | links.paths | {alias, missing})))
    errors = []
    for path in sorted(new | links.paths):
        entry = pages[path]
        if (entry["status"] != 200 or entry.get("canonical") != [ORIGIN + path]
                or not entry.get("visible_chars") or entry.get("draft_markers")):
            errors.append(path)
    if pages[alias]["status"] != 301 or pages[alias].get("location") not in (
        "/products/chatgpt-plus-code", ORIGIN + "/products/chatgpt-plus-code"
    ):
        errors.append(alias)
    if pages[missing]["status"] != 404:
        errors.append(missing)
    result = {"checked_at": datetime.now(timezone.utc).isoformat(), "baseline_at": before["timestamp_utc"],
              "catalog_at": after["timestamp_utc"], "removed_from_sitemap": sorted(old - new),
              "added_to_sitemap": sorted(new - old), "draft_targets": sorted(links.paths),
              "pages": pages, "errors": errors}
    def by_id(items):
        return {item["id"]: item for item in items}

    old_items, new_items = [by_id(snap["catalog"]) for snap in (before, after)]
    changes = []
    for product_id in sorted(old_items.keys() & new_items.keys()):
        old_item, new_item = old_items[product_id], new_items[product_id]
        old_skus, new_skus = by_id(old_item["skus"]), by_id(new_item["skus"])
        for sku_id in sorted(old_skus.keys() | new_skus.keys()):
            old_sku, new_sku = old_skus.get(sku_id), new_skus.get(sku_id)
            if old_sku != new_sku:
                changes.append({"product_id": product_id, "sku_id": sku_id, "before": old_sku, "after": new_sku})
    result["catalog_comparison"] = {
        "before_count": len(old_items), "after_count": len(new_items),
        "removed_ids": sorted(old_items.keys() - new_items.keys()), "added_ids": sorted(new_items.keys() - old_items.keys()),
        "product_price_changes": [k for k in sorted(old_items.keys() & new_items.keys()) if any(
            old_items[k].get(f) != new_items[k].get(f) for f in ("price_amount", "effective_price_amount"))],
        "sku_changes": changes,
    }
    result["core_links_to_removed_pages"] = {
        p: sorted(set(entry["catalog_links"]) & (old - new)) for p, entry in pages.items() if "catalog_links" in entry}
    for path, targets in result["core_links_to_removed_pages"].items():
        errors.extend(f"{path} links to {target}" for target in targets if pages[target]["status"] == 404)
    result["visible_faq"] = {}
    for path, count in {"/products/chatgpt-plus-code": 3, "/blog/chatgpt-ph-recharge-guide": 6}.items():
        page = after["pages"][path]
        nodes = [node for block in page["jsonld"] for node in [block, *block.get("@graph", [])]]
        questions = [q["name"] for node in nodes if node.get("@type") == "FAQPage" for q in node["mainEntity"]]
        visible = all(q in page["text"] for q in questions)
        result["visible_faq"][path] = {"question_count": len(questions), "all_visible": visible}
        if len(questions) != count or not visible:
            errors.append(f"FAQ: {path}")
    args.output.write_bytes((json.dumps(result, indent=2, ensure_ascii=False) + "\n").encode("utf-8"))
    print(f"Checked {len(pages)} public pages. Errors: {errors}")
    return bool(errors)


if __name__ == "__main__":
    raise SystemExit(main())
