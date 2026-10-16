"""Fail a release when public static files disclose a private redemption host."""
import argparse
import gzip
import html
import json
from pathlib import Path
import re
from urllib.parse import unquote

HOST = re.compile(r'cdk\s*\.\s*bar|cdkai\s*\.\s*kdns\s*\.\s*fr', re.I)
ESCAPE = re.compile(r'\\(?:u00|x)([0-9a-f]{2})', re.I)


def contains_redemption_host(raw):
    text = raw.decode('utf-8', errors='replace')
    for _ in range(3):
        text = html.unescape(unquote(text))
        text = ESCAPE.sub(lambda m: chr(int(m[1], 16)), text)
    return bool(HOST.search(text))


def scan(root):
    root = Path(root).resolve(strict=True)
    if not root.is_dir():
        raise ValueError('Expected a public asset directory')
    files = 0
    hits = []
    for path in sorted(root.rglob('*')):
        if not path.is_file():
            continue
        files += 1
        raw = path.read_bytes()
        if path.suffix == '.gz':
            raw = gzip.decompress(raw)
        elif path.suffix == '.br':
            # Fail closed if the optional decoder is absent; do not skip files.
            import brotli
            raw = brotli.decompress(raw)
        if contains_redemption_host(raw):
            hits.append(path.relative_to(root).as_posix())
    return {'root': str(root), 'files': files, 'hits': hits, 'passed': not hits}


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('directories', nargs='+')
    args = parser.parse_args()
    reports = [scan(path) for path in args.directories]
    print(json.dumps(reports, ensure_ascii=True, indent=2))
    raise SystemExit(0 if all(r['passed'] for r in reports) else 1)
