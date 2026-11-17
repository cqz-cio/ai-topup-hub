"""Offline review guardrails plus an independent crawl matcher fixture."""
import importlib.util
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
spec = importlib.util.spec_from_file_location("v6_public", ROOT / "scripts/verify_seo_v6_public.py")
public = importlib.util.module_from_spec(spec)
spec.loader.exec_module(public)


class CartMatcherTests(unittest.TestCase):
    def test_longest_match_and_allow_tie(self):
        body = "User-agent: *\nDisallow: /cart\nAllow: /cart$\nAllow: /cart?\nAllow: /cart/$\nAllow: /cart/?\n"
        for path in ["/cart", "/cart?", "/cart?next=/checkout", "/cart/", "/cart/?x=1"]:
            self.assertTrue(public.allowed(body, path), path)
        for path in ["/cart/checkout", "/cart/api?x=1", "/cartoon", "/cart-old", "/cart//"]:
            self.assertFalse(public.allowed(body, path), path)
        self.assertTrue(public.allowed("Allow: /cart\nDisallow: /cart\n", "/cart"))

    def test_google_wildcards_and_query_are_not_prefix_only(self):
        # Official REP example shape: '$' anchors the entire URL, including query.
        self.assertFalse(public.allowed("Disallow: /*.php$", "/example.php"))
        self.assertTrue(public.allowed("Disallow: /*.php$", "/example.php?x=1"))
        self.assertFalse(public.allowed("Disallow: /cart?", "/cart?x=1"))
        self.assertTrue(public.allowed("Disallow: /cart?", "/cartX"))


if __name__ == "__main__":
    unittest.main()
