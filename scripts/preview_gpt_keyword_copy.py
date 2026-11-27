"""Apply the reviewed GPT copy to a new offline SQLite preview. The source is read-only."""
import argparse
from contextlib import closing
import json
import os
from pathlib import Path
import sqlite3
from tempfile import TemporaryDirectory
from time import monotonic

from datetime import datetime, timezone
import hashlib


def sha(text):
    return hashlib.sha256(text.encode('utf-8')).hexdigest()


def timestamp(value):
    result = datetime.fromisoformat(value.replace('Z', '+00:00'))
    if result.tzinfo is None:
        raise ValueError('Expected an explicit timezone for the original publication date')
    return result.astimezone(timezone.utc)


def encode(value, original):
    encoded = json.dumps(value, ensure_ascii=False, separators=(',', ':'))
    return encoded.encode('utf-8') if isinstance(original, bytes) else encoded

ROOT = Path(__file__).resolve().parents[1]
MANIFEST = ROOT / "docs/catalog/gpt-keywords-20261127/copy.json"
PRODUCTS = {30: "chatgpt-plus-code", 31: "chatgpt-pro-200-ph", 32: "chatgpt-pro-200-renew-ph"}
POSTS = {1: "purchase-and-agent-guide", 5: "chatgpt-plus-code-guide", 6: "chatgpt-plus-renewal-rules",
         7: "chatgpt-membership-not-active", 8: "chatgpt-plus-vs-api-balance", 9: "chatgpt-code-region-check"}


def target(db, entry):
    table = entry["table"]
    if table == "products" and PRODUCTS.get(entry["id"]) == entry["slug"]:
        item = db.execute("SELECT slug,category_id,fulfillment_type FROM products "
                          "WHERE id=? AND is_active=1 AND deleted_at IS NULL", (entry["id"],)).fetchone()
        if item and item[:2] == (entry["slug"], entry["category_id"]):
            mode = item[2]
            if mode == "upstream":
                modes = db.execute("SELECT upstream_fulfillment_type FROM product_mappings "
                                   "WHERE local_product_id=? AND is_active=1", (entry["id"],)).fetchall()
                if modes == [("auto",)]:
                    mode = "auto"
            if mode == "auto":
                return "id", entry["id"]
    elif table == "posts" and POSTS.get(entry["id"]) == entry["slug"]:
        item = db.execute("SELECT slug,type,published_at FROM posts "
                          "WHERE id=? AND is_published=1 AND deleted_at IS NULL", (entry["id"],)).fetchone()
        if item and item[:2] == (entry["slug"], entry["type"]) and timestamp(item[2]) == timestamp(entry["published_at"]):
            return "id", entry["id"]
    elif table == "settings" and entry["key"] == "about_page_config":
        return "key", entry["key"]
    raise ValueError(f"Source identity or sales status changed: {entry.get('slug', table)}")


def allowed(table, column, path):
    if table == "settings":
        return column == "value_json" and (
            (len(path) == 4 and path[0] == "services" and path[1] in (1, 2) and path[2:] == ["text", "zh-CN"])
            or (len(path) == 4 and path[0] == "recharge" and path[1] in (0, 1)
                and path[2] in ("title", "text") and path[3] == "zh-CN"))
    if table == "posts":
        return column in ("title_json", "summary_json", "content_json") and path == ["zh-CN"]
    return table == "products" and (
        (column in ("description_json", "content_json") and path == ["zh-CN"])
        or (column == "seo_meta_json" and path in (["title", "zh-CN"], ["description", "zh-CN"])))


def apply_copy(db, rows):
    changed = []
    db.execute("BEGIN IMMEDIATE")
    try:
        for entry in rows:
            table = entry["table"]
            key, identity = target(db, entry)
            for edit in entry["fields"]:
                column, path = edit["column"], edit["path"]
                if not allowed(table, column, path):
                    raise ValueError(f"Unreviewed field: {table}.{column} {path}")
                match = db.execute(f"SELECT {column} FROM {table} WHERE {key}=?", (identity,)).fetchone()
                if not match:
                    raise ValueError(f"Missing row: {table} {identity}")
                old = match[0]
                data = json.loads(old)
                parent = data
                for part in path[:-1]:
                    parent = parent[part]
                current, after = parent[path[-1]], edit["after"]
                if current == after:
                    continue
                if sha(current) not in edit["before_sha256"]:
                    raise ValueError(f"Copy changed since review: {table} {identity} {column} {path}")
                parent[path[-1]] = after
                count = db.execute(f"UPDATE {table} SET {column}=? WHERE {key}=? AND {column} IS ?",
                                   (encode(data, old), identity, old)).rowcount
                if count != 1:
                    raise ValueError(f"Concurrent change: {table} {identity} {column}")
                changed.append([table, identity, column, path])
        db.commit()
    except BaseException:
        db.rollback()
        raise
    return changed


def preview(source, output):
    source = source.resolve(strict=True)
    output = output.absolute()
    if output.exists():
        raise FileExistsError(f"Choose a new output database: {output}")
    rows = json.loads(MANIFEST.read_text(encoding="utf-8"))["rows"]
    with TemporaryDirectory(dir=output.parent) as staging:
        candidate = Path(staging) / "preview.db"
        with closing(sqlite3.connect(source.as_uri() + "?mode=ro", uri=True, timeout=5)) as original:
            with closing(sqlite3.connect(candidate, timeout=5)) as db:
                deadline = monotonic() + 30
                def progress(status, remaining, total):
                    if monotonic() > deadline:
                        raise TimeoutError("Preview source backup timed out")
                original.backup(db, pages=256, progress=progress)
                changes = apply_copy(db, rows)
                if apply_copy(db, rows) or db.execute("PRAGMA quick_check").fetchone() != ("ok",):
                    raise ValueError("Preview integrity or idempotency check failed")
        os.link(candidate, output)
    return changes


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--source-db", required=True, type=Path)
    parser.add_argument("--output-db", required=True, type=Path)
    args = parser.parse_args()
    print(json.dumps({"changed_fields": preview(args.source_db, args.output_db)}, ensure_ascii=False))
