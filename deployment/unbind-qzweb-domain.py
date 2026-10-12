#!/usr/bin/env python3
"""Safely detach qzweb.cloud and www.qzweb.cloud from the old catalog."""
import argparse
import datetime
import hashlib
import json
import os
import pathlib
import re
import shutil
import subprocess
import time
import uuid

DOMAINS = {"qzweb.cloud", "www.qzweb.cloud"}
CONF = pathlib.Path("/etc/nginx/conf.d/zhixiang.conf")
ROOT_PREFIX = "/var/www/zhixiang/"
BACKUP_BASE = pathlib.Path("/etc/nginx/domain-unbind-backups")


def run(args):
    result = subprocess.run(args, capture_output=True, text=True, timeout=30)
    if result.returncode:
        raise RuntimeError(
            "Command failed: " + " ".join(args) + "\n" +
            (result.stderr or result.stdout).strip()
        )
    return result.stdout


def active_sections():
    text = run(["nginx", "-T"])
    matches = list(re.finditer(r"(?m)^# configuration file (.+):\s*$", text))
    if not matches:
        raise RuntimeError("Cannot identify active Nginx configuration files.")
    sections = {}
    for i, match in enumerate(matches):
        end = matches[i + 1].start() if i + 1 < len(matches) else len(text)
        sections[match.group(1)] = text[match.end():end]
    return sections


def directives(text, name):
    clean = re.sub(r"(?m)#[^\n]*", "", text)
    return re.findall(r"(?:^|[;{}])\s*" + re.escape(name) + r"\s+([^;{}]+);", clean)


def hosts(text):
    return {
        name.rstrip(".").lower()
        for names in directives(text, "server_name")
        for name in names.split()
    }


def catalog_roots(text):
    return [
        root.strip().strip("'\"")
        for root in directives(text, "root")
        if root.strip().strip("'\"").startswith(ROOT_PREFIX)
    ]


def preflight():
    if os.geteuid() != 0:
        raise RuntimeError("Run this script with sudo.")
    run(["nginx", "-t"])
    run(["systemctl", "is-active", "nginx"])
    if not CONF.is_file() or CONF.is_symlink():
        raise RuntimeError("Expected a regular file at " + str(CONF))
    original = CONF.read_bytes()
    text = original.decode("utf-8")
    if hosts(text) != DOMAINS:
        raise RuntimeError("Site aliases changed; stopped without modifying them.")
    if directives(text, "include"):
        raise RuntimeError("Site has nested includes; manual inspection required.")
    roots = set(catalog_roots(text))
    if not roots:
        raise RuntimeError("Expected catalog root was not found.")
    index_hashes = set()
    for root in roots:
        index = pathlib.Path(root) / "index.html"
        if not index.is_file():
            raise RuntimeError("Catalog index missing: " + str(index))
        index_hashes.add(hashlib.sha256(index.read_bytes()).hexdigest())
    sections = active_sections()
    if str(CONF) not in sections:
        raise RuntimeError("Site file is not in the active Nginx configuration.")
    conflicts = [
        path for path, body in sections.items()
        if path != str(CONF) and (hosts(body) & DOMAINS or catalog_roots(body))
    ]
    if conflicts:
        raise RuntimeError("Other active bindings need inspection: " + ", ".join(conflicts))
    plan = {
        "domains": sorted(DOMAINS),
        "configuration": str(CONF),
        "configuration_sha256": hashlib.sha256(original).hexdigest(),
        "catalog_roots": sorted(roots),
        "action": "Back up and disable only this site's Nginx configuration; reload Nginx.",
        "preserved": ["website files", "TLS certificates", "DNS records", "other site configurations"],
    }
    return plan, index_hashes


def probe(index_hashes):
    results = []
    resolve = []
    for domain in sorted(DOMAINS):
        for port in (80, 443):
            resolve += ["--resolve", domain + ":" + str(port) + ":127.0.0.1"]
    for domain in sorted(DOMAINS):
        for scheme in ("http", "https"):
            url = scheme + "://" + domain + "/"
            command = [
                "curl", "--noproxy", "*", "-k", "-sS", "--connect-timeout", "4",
                "--max-time", "8", "--max-filesize", "2097152",
                "-w", "\n%{http_code}",
            ] + resolve + [url]
            result = subprocess.run(command, capture_output=True, timeout=12)
            body, separator, status = result.stdout.rpartition(b"\n")
            if result.returncode == 0:
                if not separator:
                    raise RuntimeError("Unexpected origin response: " + url)
                if hashlib.sha256(body).hexdigest() in index_hashes:
                    raise RuntimeError("Origin still serves the old catalog: " + url)
                results.append({"url": url, "http_status": status.decode(), "old_catalog": False})
            elif result.returncode == 7:
                results.append({"url": url, "result": "connection_refused", "old_catalog": False})
            else:
                raise RuntimeError(
                    "Could not verify the origin: " + url + ": " +
                    result.stderr.decode(errors="replace").strip()
                )
    return results


def apply(plan, index_hashes):
    stamp = datetime.datetime.now(datetime.timezone.utc).strftime("%Y%m%dT%H%M%SZ")
    backup_dir = BACKUP_BASE / (stamp + "-qzweb-" + uuid.uuid4().hex[:8])
    backup_dir.mkdir(parents=True, mode=0o700, exist_ok=False)
    backup = backup_dir / "zhixiang.conf"
    shutil.copy2(CONF, backup)
    metadata = dict(plan, saved_at_utc=stamp, restore_from=str(backup))
    (backup_dir / "plan.json").write_text(json.dumps(metadata, indent=2) + "\n")
    if hashlib.sha256(CONF.read_bytes()).hexdigest() != plan["configuration_sha256"]:
        raise RuntimeError("Configuration changed after preflight; nothing disabled.")
    disabled = False
    try:
        CONF.unlink()
        disabled = True
        run(["nginx", "-t"])
        sections = active_sections()
        remaining = [
            path for path, body in sections.items()
            if hosts(body) & DOMAINS or catalog_roots(body)
        ]
        if remaining:
            raise RuntimeError("Bindings remain active: " + ", ".join(remaining))
        run(["systemctl", "reload", "nginx"])
        run(["systemctl", "is-active", "nginx"])
        # A graceful reload may need a moment for old workers to retire.
        time.sleep(1)
        verification = probe(index_hashes)
    except Exception:
        if disabled:
            shutil.copy2(backup, CONF)
            run(["nginx", "-t"])
            run(["systemctl", "reload", "nginx"])
            print(json.dumps({"status": "rolled_back", "backup": str(backup)}), flush=True)
        raise
    print(json.dumps({
        "status": "detached",
        "domains": sorted(DOMAINS),
        "backup": str(backup),
        "origin_checks": verification,
        "dns_changed": False,
    }, indent=2), flush=True)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--apply", action="store_true", help="Execute after successful preflight.")
    args = parser.parse_args()
    plan, index_hashes = preflight()
    print(json.dumps({"status": "preflight_ok", "plan": plan}, indent=2), flush=True)
    if args.apply:
        apply(plan, index_hashes)


if __name__ == "__main__":
    main()
