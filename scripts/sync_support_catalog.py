"""Synchronize public catalog only. Never opens a shop database or model API.

Run from the agent checkout: python scripts/sync_support_catalog.py
The generated snapshot is reviewed, tested, then embedded by rebuilding the agent.
"""
import datetime
import hashlib
import html
import json
import pathlib
import re
import urllib.request

SOURCE = "https://www.qzweb.cloud"
ROOT = pathlib.Path(__file__).resolve().parents[1]
TARGET = ROOT / "internal/modules/supportagent/catalog-snapshot.json"


def clean(value):
    value = re.sub(r"<(script|style)\b[^>]*>.*?</\1>", "", value, flags=re.I | re.S)
    value = re.sub(r"</(?:p|li|h[1-6])>|<br\s*/?>", "\n", value, flags=re.I)
    value = html.unescape(re.sub(r"<[^>]*>", "", value))
    return "\n".join(" ".join(line.split()) for line in value.splitlines() if line.strip())


def sections(content):
    matches = list(re.finditer(r"<h[1-6][^>]*>(.*?)</h[1-6]>", content, re.I | re.S))
    result = []
    for i, match in enumerate(matches):
        end = matches[i + 1].start() if i + 1 < len(matches) else len(content)
        result.append({"heading": clean(match[1]), "text": clean(content[match.end():end])})
    if not result:
        raise ValueError("Missing recognizable public policy sections; manual review required")
    return result


def project_product(p):
    # Explicit public customer-facing allowlist, including full descriptions and
    # specs. No upstream stock, costs, supplier identifiers or delivery secrets.
    result = {k: p[k] for k in ("id", "slug", "title", "description", "price_amount",
                               "is_sold_out", "fulfillment_type", "purchase_type",
                               "min_purchase_quantity", "max_purchase_quantity")}
    result["content"] = {locale: clean(value) for locale, value in p["content"].items()}
    result["category"] = p["category"]["name"]
    result["skus"] = [{k: sku[k] for k in ("spec_values", "price_amount", "is_active", "is_sold_out")}
                      for sku in p["skus"]]
    result["sections"] = sections(p["content"]["zh-CN"])
    if not re.fullmatch(r"[a-zA-Z0-9][a-zA-Z0-9_-]{0,119}", result["slug"]):
        raise ValueError("Invalid slug")
    if not result["title"].get("zh-CN") or not result["skus"]:
        raise ValueError("Incomplete product")
    if len(result["content"]["zh-CN"]) > 20000:
        raise ValueError("Oversized product content")
    return result


def product_policy(p):
    def pick(*headings):
        return "\n\n".join(s["text"] for s in p["sections"] if s["heading"] in headings)
    slug = p["slug"]
    eligibility = pick("适用账号", "购买前请确认", "覆盖充值规则", "账号验证提醒", "下单前必读", "使用说明")
    after_sales = pick("订阅质保", "售后规则", "激活与售后", "退款金额核算与材料")
    usage = pick("兑换说明", "如何兑换", "使用方式", "使用说明", "链接保管", "自动交付内容与使用步骤")
    if not usage and "订单交付指引" in p["content"]["zh-CN"]:
        usage = "请按付款后的订单交付指引操作，实际兑换或开通结果以处理记录为准。"
    delivery = pick("商品与交付", "商品介绍", "交付与帮助", "发货时效", "订阅期限", "兑换地区")
    unused = pick("未使用卡密退款")
    if slug.startswith("86gamestore-1000"):
        eligibility += "\n" + pick("激活与售后")
        unused = pick("激活与售后")
    refund_mode = "manual_review"
    if "实际订阅总时长" in after_sales and "实付金额" in after_sales:
        refund_mode = "proportional"
    if slug.startswith("ellaix-claude-"):
        if not all(s in eligibility for s in ("不做任何质保", "KYC")):
            raise ValueError("Claude policy changed; manual review required")
        after_sales = eligibility
        refund_mode = "no_warranty"
    elif slug == "ellaix-gpt-plus-account-30d":
        delivery = p["sections"][0]["text"]
        after_sales = ("订阅质保一个月，质保问题请提供订单号联系本站客服处理。\n" + eligibility
                       + "\n该成品号公开说明未给出按剩余时长退款的计算规则，不能套用其他充值商品的退款比例。")
    elif slug.startswith("ellaix-grok-") or slug.startswith("ellaix-x-"):
        after_sales = (eligibility + "\n" + pick("交付与帮助")
                       + "\n本商品没有已确认的退款比例或持续质保期限；异常由人工核实，不套用 GPT 的按剩余时长退款规则。")
    elif slug == "qzweb-52":
        refund_mode = "activation_month_full_review"
    elif slug == "qzweb-53" or slug.startswith("86gamestore-1000"):
        refund_mode = "activation_only"
    if not all((eligibility, after_sales, usage)):
        raise ValueError("Missing reviewed policy topics: " + slug)
    return {"eligibility": eligibility, "after_sales": after_sales,
            "usage": usage, "delivery": delivery or p["sections"][0]["text"],
            "unused": unused or "公开说明未确认本款未使用卡密或链接的退款条件，请提供订单号由人工核实，不承诺可退。",
            "refund_mode": refund_mode}


def fetch():
    class NoRedirect(urllib.request.HTTPRedirectHandler):
        def redirect_request(self, *args, **kwargs):
            raise ValueError("Unexpected catalog redirect")
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    all_rows = []
    total = None
    for page in range(1, 6):
        with opener.open(SOURCE + f"/api/v1/public/products?page={page}&page_size=100", timeout=10) as response:
            body = response.read(2 * 1024 * 1024 + 1)
        if len(body) > 2 * 1024 * 1024:
            raise ValueError("Oversized catalog")
        data = json.loads(body)
        if data["status_code"] != 0:
            raise ValueError("Catalog API failed")
        pagination = data["pagination"]
        if total is not None and total != pagination["total"]:
            raise ValueError("Catalog changed during pagination")
        total = pagination["total"]
        all_rows.extend(data["data"])
        if page >= pagination["total_page"]:
            break
    if not all_rows or len(all_rows) != total or len({p["slug"] for p in all_rows}) != total:
        raise ValueError("Incomplete/duplicate catalog; previous snapshot preserved")
    products = sorted((project_product(p) for p in all_rows), key=lambda p: p["id"])
    policies = {p["slug"]: product_policy(p) for p in products}
    snapshot = {"source": SOURCE + "/api/v1/public/products", "fetched_at": datetime.datetime.now(datetime.timezone.utc).isoformat(),
                "count": len(products), "products": products, "policies": policies}
    snapshot["content_sha256"] = hashlib.sha256(json.dumps(products, ensure_ascii=False, sort_keys=True).encode()).hexdigest()
    temporary = TARGET.with_suffix(".json.tmp")
    temporary.write_text(json.dumps(snapshot, ensure_ascii=False, indent=2) + "\n", encoding="utf-8")
    temporary.replace(TARGET)
    print(json.dumps({k: snapshot[k] for k in ("source", "fetched_at", "count", "content_sha256")}, ensure_ascii=False))


if __name__ == "__main__":
    fetch()
