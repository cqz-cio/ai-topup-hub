"""Publish just the approved ready-account article. Default CLI mode is rehearsal."""
import hashlib
import json
import sqlite3
from contextlib import closing
from datetime import datetime, timezone
from pathlib import Path
from time import monotonic

SLUG = "chatgpt-ready-account-guide"
TITLE = "ChatGPT Plus 成品号｜一个月订阅"
SUMMARY = "ChatGPT Plus 成品号每个 115 元，含一个月订阅。了解自动发货、账号使用和订阅质保期内的售后处理。"
BODY_SHA = "6ed64d0bfad94c2452fb9059f3600971f41b369eba621a512ce5ee00c4764263"
DATABASE = Path("/srv/zhixiang/shared/db/shop.db")
BACKUPS = Path("/srv/zhixiang/backups")


def require(condition, message):
    if not condition:
        raise ValueError(message)


def localized(value):
    return json.dumps({"zh-CN": value, "zh-TW": "", "en-US": ""}, ensure_ascii=False, separators=(",", ":")).encode()


def snapshot(db, target):
    deadline = monotonic() + 20

    def progress(*_):
        require(monotonic() < deadline, "SQLite backup deadline exceeded")

    db.backup(target, pages=256, progress=progress, sleep=.05)
    require(target.execute("PRAGMA quick_check").fetchone()[0] == "ok", "Invalid backup")


def transact(db, fields, now):
    with db:
        db.execute("BEGIN IMMEDIATE")
        require(not db.execute("SELECT 1 FROM sqlite_master WHERE type='trigger' AND tbl_name='posts'").fetchone(),
                "Unexpected posts trigger")
        previous = db.execute("SELECT * FROM posts WHERE slug=?", (SLUG,)).fetchall()
        if previous:
            require(len(previous) == 1 and all(previous[0][k] == v for k, v in fields.items()),
                    "Existing article differs from approved copy")
            return previous[0]["id"], False
        values = {**fields, "created_at": now, "published_at": now}
        start = db.total_changes
        cursor = db.execute(f"INSERT INTO posts ({','.join(values)}) VALUES ({','.join('?' for _ in values)})",
                            tuple(values.values()))
        ident = cursor.lastrowid
        stored = dict(db.execute("SELECT * FROM posts WHERE id=?", (ident,)).fetchone())
        require(db.total_changes == start + 1 and stored == {"id": ident, **values}, "Unexpected inserted values")
        require(not db.execute("SELECT 1 FROM post_products WHERE post_id=?", (ident,)).fetchone(),
                "Unexpected related products")
        return ident, True


def rollback(db, fields, now, ident):
    with db:
        db.execute("BEGIN IMMEDIATE")
        row = db.execute("SELECT * FROM posts WHERE id=?", (ident,)).fetchone()
        require(row is not None and dict(row) == {"id": ident, **fields, "published_at": now, "created_at": now},
                "Article changed since publication")
        require(not db.execute("SELECT 1 FROM post_products WHERE post_id=?", (ident,)).fetchone(),
                "Article acquired related products")
        require(not db.execute("SELECT 1 FROM sqlite_master WHERE type='trigger' AND tbl_name='posts'").fetchone(),
                "Unexpected posts trigger")
        start = db.total_changes
        require(db.execute("DELETE FROM posts WHERE id=? AND slug=?", (ident, SLUG)).rowcount == 1
                and db.total_changes == start + 1, "Unexpected rollback scope")


def publish(mode, body, receipt=None):
    require(mode in {"rehearse", "apply", "rollback"}, "Unknown publication mode")
    require(hashlib.sha256(body.encode()).hexdigest() == BODY_SHA, "Article digest mismatch")
    fields = dict(slug=SLUG, type="blog", title_json=localized(TITLE), summary_json=localized(SUMMARY),
                  content_json=localized(body), thumbnail="", category_id=None, is_published=1, deleted_at=None)
    now = datetime.now(timezone.utc).isoformat()
    with closing(sqlite3.connect(DATABASE.resolve(strict=True).as_uri() + ("?mode=ro" if mode == "rehearse" else "?mode=rw"),
                                 uri=True, timeout=6)) as db:
        db.row_factory = sqlite3.Row
        db.execute("PRAGMA foreign_keys=ON")
        if mode == "rollback":
            record = json.loads(Path(receipt).read_text("utf-8"))
            require(record["created"] and record["sha256"] == BODY_SHA, "Invalid rollback receipt")
            rollback(db, fields, record["timestamp"], record["id"])
            return {"rolled_back": record["id"]}
        with closing(sqlite3.connect(":memory:")) as trial:
            trial.row_factory = sqlite3.Row
            snapshot(db, trial)
            trial.execute("PRAGMA foreign_keys=ON")
            before = trial.execute("SELECT COUNT(*) FROM posts").fetchone()[0]
            ident, created = transact(trial, fields, now)
            require(transact(trial, fields, now) == (ident, False), "Repeat publication changed the article")
            if created:
                rollback(trial, fields, now, ident)
            require(trial.execute("SELECT COUNT(*) FROM posts").fetchone()[0] == before, "Rehearsal rollback failed")
        if mode == "rehearse" or not created:
            return {"status": "rehearsed" if created else "already_published", "would_insert": int(created), "sha256": BODY_SHA}
        directory = BACKUPS / ("before-ready-account-guide-" + datetime.now(timezone.utc).strftime("%Y%m%dT%H%M%S%fZ"))
        directory.mkdir(mode=0o700, exist_ok=False)
        with closing(sqlite3.connect(directory / "before.db")) as saved:
            snapshot(db, saved)
        ident, created = transact(db, fields, now)
        record = {"created": created, "id": ident, "slug": SLUG, "sha256": BODY_SHA, "timestamp": now,
                  "backup": str(directory)}
        (directory / "receipt.json").write_text(json.dumps(record), encoding="utf-8")
        return record


if __name__ == "__main__":
    import argparse
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("content", type=Path)
    parser.add_argument("--mode", choices=("rehearse", "apply", "rollback"), default="rehearse")
    parser.add_argument("--receipt", type=Path)
    args = parser.parse_args()
    print(json.dumps(publish(args.mode, args.content.read_text("utf-8").strip(), args.receipt)))
