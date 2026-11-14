"""Export versioned SEO raster assets without rewriting any published source.

Requires Pillow with WebP/AVIF support. Run from any directory; --share-source
accepts the approved generated brand card. Catalog covers are generated offline,
never during a public request. The manifest is shared by Go and Vue.
"""
from __future__ import annotations

import argparse
import hashlib
import json
import io
from pathlib import Path

from PIL import Image, ImageOps, features

ROOT = Path(__file__).resolve().parents[1]
PUBLIC = ROOT / "frontend/user/public"
SHARE = PUBLIC / "media/seo-share"
SCENE = PUBLIC / "media/storefront-20261114"


def letterbox(source: Image.Image) -> Image.Image:
    image = ImageOps.exif_transpose(source).convert("RGBA")
    image.thumbnail((1080, 550), Image.Resampling.LANCZOS)
    canvas = Image.new("RGB", (1200, 630), "#e7f7ef")
    canvas.paste(image, ((1200 - image.width) // 2, (630 - image.height) // 2), image)
    return canvas


def write_new(image: Image.Image, path: Path, **options: object) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    encoded = io.BytesIO()
    image.save(encoded, format=Image.registered_extensions()[path.suffix.lower()], **options)
    data = encoded.getvalue()
    if path.exists():
        if path.read_bytes() != data:
            raise RuntimeError(f"Refusing to replace immutable media: {path}; choose a new version/name")
        return
    path.write_bytes(data)


def generate(share_source: Path | None, upload_covers: Path | None = None) -> None:
    if share_source:
        with Image.open(share_source) as image:
            card = ImageOps.fit(image.convert("RGB"), (1200, 630), Image.Resampling.LANCZOS)
            write_new(card, SHARE / "qzweb-og-1200x630.jpg", quality=86, optimize=True, progressive=True)
    manifest_path = ROOT / "internal/platform/publiccontent/seo-share-images.json"
    manifest: dict[str, str] = json.loads(manifest_path.read_text(encoding="utf-8")) if manifest_path.exists() else {}
    sources: list[tuple[str, Path]] = []
    for directory in (PUBLIC / "products", PUBLIC / "catalog-products"):
        for path in sorted(directory.rglob("*")):
            if path.suffix.lower() not in (".png", ".jpg", ".jpeg", ".webp"):
                continue
            sources.append(("/" + path.relative_to(PUBLIC).as_posix(), path))
    if upload_covers:
        # Explicitly listed public covers only; never enumerate private uploads.
        for url in json.loads(upload_covers.read_text(encoding="utf-8")):
            source = (ROOT / str(url).lstrip("/")).resolve()
            if not str(url).startswith("/uploads/") or not source.is_relative_to(ROOT / "uploads"):
                raise ValueError("Expected a public /uploads/ cover path")
            if source.suffix.lower() not in (".png", ".jpg", ".jpeg", ".webp"):
                raise ValueError("Expected a raster cover")
            sources.append((url, source))
    for url, path in sources:
        try:
            digest = hashlib.sha256(path.read_bytes()).hexdigest()[:20]
            target = SHARE / f"catalog-{digest}-1200x630.jpg"
            with Image.open(path) as image:
                write_new(letterbox(image), target, quality=84, optimize=True, progressive=True)
            manifest[url] = "/" + target.relative_to(PUBLIC).as_posix()
        except (OSError, ValueError, Image.DecompressionBombError):
            continue
    manifest_path.write_text(
        json.dumps(dict(sorted(manifest.items())), indent=2, ensure_ascii=False) + "\n", encoding="utf-8"
    )
    old = PUBLIC / "media/storefront-20261020"
    with Image.open(old / "jellyfishing.webp") as image:
        for width in (1280, 1672):
            resized = image.resize((width, round(image.height * width / image.width)), Image.Resampling.LANCZOS)
            write_new(resized, SCENE / f"jellyfishing-{width}.webp", quality=80, method=6)
            if features.check("avif"):
                write_new(resized, SCENE / f"jellyfishing-{width}.avif", quality=60, speed=5)
    for name in ("clean-plate", "foreground"):
        with Image.open(old / f"{name}.webp") as image:
            # Keep the exact art dimensions and lossless alpha for WebGL coordinates.
            write_new(image, SCENE / f"{name}.webp", quality=82, method=6, alpha_quality=100, exact=True)
    outputs = [p for folder in (SHARE, SCENE) for p in sorted(folder.glob("*"))]
    print(json.dumps({"catalog_covers": len(manifest), "assets": [
        {"path": "/" + p.relative_to(PUBLIC).as_posix(), "bytes": p.stat().st_size,
         "size": list(Image.open(p).size)} for p in outputs
    ]}, ensure_ascii=False, indent=2))


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--share-source", type=Path)
    parser.add_argument("--upload-covers", type=Path, help="JSON list of verified public /uploads/ raster URLs already copied locally")
    args = parser.parse_args()
    generate(args.share_source, args.upload_covers)
