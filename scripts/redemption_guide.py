"""Build the approved ChatGPT delivery guide and apply only its catalog fields.

Without --apply, this command only reports a plan. Applying to SQLite requires
a consistent backup and never changes prices, stock, orders, or card contents.
"""

from public_redemption import public_copy
import argparse
from datetime import datetime, timezone
import html
import json
from pathlib import Path
import re
import sqlite3
import time
from urllib.parse import urlsplit


GUIDE_START = "<!-- pi-star-redemption-guide:start -->"
GUIDE_END = "<!-- pi-star-redemption-guide:end -->"
GUIDE_PATTERN = re.compile(re.escape(GUIDE_START) + r".*?" + re.escape(GUIDE_END) + r"\s*", re.S)


def replace_redemption_url(content, guide):
    """Migrate the previous approved host while preserving other product terms."""
    return content.replace("https://cdkai.kdns.fr/", guide["redemption_url"]).replace(
        "https://cdkai.kdns.fr", guide["redemption_url"].rstrip("/"),
    )


def load_guide(path=None):
    path = path or Path(__file__).resolve().parents[1] / "docs/catalog/qzweb-redemption-guide.json"
    guide = json.loads(Path(path).read_text(encoding="utf-8-sig"))
    parsed = urlsplit(guide["redemption_url"])
    if parsed.scheme != "https" or not parsed.hostname or parsed.username or parsed.password or parsed.query or parsed.fragment:
        raise ValueError("The redemption URL must be a public HTTPS address without embedded data")
    slugs = guide["product_slugs"]
    if not slugs or len(set(slugs)) != len(slugs) or not set(guide["ios_product_slugs"]).issubset(slugs):
        raise ValueError("Invalid redemption product mapping")
    return guide


def update_ios_content(slug, content, guide):
    if slug not in guide["ios_product_slugs"]:
        return content
    return content.replace(
        "按照卡密附带的 iOS 渠道说明完成充值。",
        "通过统一兑换地址完成自助充值：" + html.escape(guide["redemption_url"]) + "。",
    ).replace(
        "iOS 渠道与普通卡充渠道不同，请勿直接套用普通卡充渠道的兑换流程。",
        "iOS 渠道也使用上述兑换网站；系统会根据卡密识别套餐，请按页面提示核对账号后充值。",
    )


def delivery_instructions(slug, current, guide):
    if slug not in guide["product_slugs"]:
        return current
    original = replace_redemption_url(GUIDE_PATTERN.sub("", current).strip(), guide)
    original = update_ios_content(slug, original, guide)
    url = html.escape(guide["redemption_url"], quote=True)
    steps = "".join("<li>" + html.escape(step) + "</li>" for step in guide["steps"])
    block = (
        GUIDE_START + "<h3>" + html.escape(guide["title"]) + "</h3>"
        '<p>兑换地址：<a href="' + url + '" target="_blank" rel="noopener noreferrer">'
        + url + "</a></p><ol>" + steps + "</ol><p>" + html.escape(guide["notice"]) + "</p>" + GUIDE_END
    )
    return block + ("\n" + original if original else "")


def catalog_fields(slug, content, instructions, guide):
    if slug not in guide["product_slugs"]:
        return content, instructions
    content = {locale: replace_redemption_url(body, guide) for locale, body in content.items()}
    instructions = {locale: replace_redemption_url(body, guide) for locale, body in instructions.items()}
    content["zh-CN"] = update_ios_content(slug, content.get("zh-CN", ""), guide)
    instructions["zh-CN"] = delivery_instructions(
        slug, instructions.get("zh-CN") or content["zh-CN"], guide,
    )
    return public_copy(content), instructions


def plan_updates(connection, guide):
    updates = []
    for slug in guide["product_slugs"]:
        row = connection.execute(
            "SELECT id, content_json, instructions_json FROM products WHERE slug = ? AND deleted_at IS NULL",
            (slug,),
        ).fetchone()
        if row is None:
            raise ValueError("Missing expected product: " + slug)
        product_id, raw_content, raw_instructions = row
        content = json.loads(raw_content or "{}")
        instructions = json.loads(raw_instructions or "{}")
        next_content, next_instructions = catalog_fields(slug, content, instructions, guide)
        # jsonmap.JSON.Value() writes []byte and the current Go scanner only
        # accepts []byte. SQLite TEXT would silently hide the JSON in the app.
        if (content != next_content or instructions != next_instructions
                or not isinstance(raw_content, bytes) or not isinstance(raw_instructions, bytes)):
            updates.append((
                product_id, slug,
                json.dumps(next_content, ensure_ascii=False).encode("utf-8"),
                json.dumps(next_instructions, ensure_ascii=False).encode("utf-8"),
            ))
    return updates


def apply_to_database(database, guide, *, apply=False, backup_dir=None):
    database = Path(database).resolve(strict=True)
    if apply and backup_dir is None:
        raise ValueError("--backup-dir is required with --apply")
    uri = database.as_uri() + ("?mode=rw" if apply else "?mode=ro")
    connection = sqlite3.connect(uri, uri=True, timeout=5)
    backup_path = None
    try:
        updates = plan_updates(connection, guide)
        if apply and updates:
            backup_dir = Path(backup_dir)
            backup_dir.mkdir(parents=True, exist_ok=True)
            stamp = datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%S%fZ")
            backup_path = backup_dir / ("before-redemption-guide-" + stamp + ".db")
            # Create privately before SQLite opens it. No backup data is printed.
            descriptor = backup_path.open("xb")
            descriptor.close()
            backup_path.chmod(0o600)
            deadline = time.monotonic() + 30

            def check_deadline(_status, _remaining, _total):
                if time.monotonic() > deadline:
                    raise TimeoutError("Database backup exceeded 30 seconds; no update applied")

            backup = sqlite3.connect(backup_path, timeout=5)
            try:
                connection.backup(backup, pages=128, progress=check_deadline, sleep=0.05)
                if backup.execute("PRAGMA quick_check").fetchone() != ("ok",):
                    raise RuntimeError("Backup integrity check failed; no update applied")
            finally:
                backup.close()
            # Re-read under the write lock so concurrent catalog edits are preserved.
            connection.execute("BEGIN IMMEDIATE")
            updates = plan_updates(connection, guide)
            for product_id, slug, content, instructions in updates:
                changed = connection.execute(
                    "UPDATE products SET content_json = ?, instructions_json = ? WHERE id = ? AND slug = ? AND deleted_at IS NULL",
                    (content, instructions, product_id, slug),
                ).rowcount
                if changed != 1:
                    raise RuntimeError("Product changed during update: " + slug)
            if plan_updates(connection, guide):
                raise RuntimeError("Redemption guide verification failed")
            connection.commit()
        return {
            "mode": "apply" if apply else "plan",
            "redemption_url": guide["redemption_url"],
            "product_slugs": [row[1] for row in updates],
            "changed_count": len(updates) if apply else 0,
            "planned_count": len(updates),
            "backup": str(backup_path) if backup_path else None,
        }
    finally:
        connection.close()


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--database", type=Path, required=True)
    parser.add_argument("--guide", type=Path)
    parser.add_argument("--apply", action="store_true")
    parser.add_argument("--backup-dir", type=Path)
    args = parser.parse_args()
    report = apply_to_database(args.database, load_guide(args.guide), apply=args.apply, backup_dir=args.backup_dir)
    print(json.dumps(report, ensure_ascii=True))


if __name__ == "__main__":
    main()
