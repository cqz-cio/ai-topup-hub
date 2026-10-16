import gzip
from pathlib import Path
import sys
import tempfile
import unittest

sys.path.insert(0, str(Path(__file__).resolve().parents[1]))
from audit_public_redemption import contains_redemption_host, scan


class PublicAssetPrivacyTest(unittest.TestCase):
    def test_public_copy_without_a_destination_is_allowed(self):
        self.assertFalse(contains_redemption_host('付款后查看订单交付说明'.encode()))

    def test_domains_in_text_and_attributes_are_detected(self):
        for value in [b'CDK.BAR', b'<a href="https://cdkai.kdns.fr/">link</a>']:
            with self.subTest(value=value):
                self.assertTrue(contains_redemption_host(value))

    def test_encoded_html_url_and_javascript_are_detected(self):
        for value in [b'cdk&#46;bar', b'cdk%252ebar', br'cdk\u002ebar', br'cdk\x2ebar']:
            with self.subTest(value=value):
                self.assertTrue(contains_redemption_host(value))

    def test_scan_checks_nested_assets_and_compressed_copies(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            (root / 'assets').mkdir()
            (root / 'index.html').write_text('<main>Store</main>')
            (root / 'assets/old.js.gz').write_bytes(gzip.compress(b'"https://cdk.bar/"'))
            report = scan(root)
            self.assertFalse(report['passed'])
            self.assertEqual(report['files'], 2)
            self.assertEqual(report['hits'], ['assets/old.js.gz'])

    def test_missing_directory_and_corrupt_compression_fail_closed(self):
        with tempfile.TemporaryDirectory() as folder:
            root = Path(folder)
            with self.assertRaises(FileNotFoundError):
                scan(root / 'missing')
            (root / 'broken.js.gz').write_bytes(b'invalid gzip')
            with self.assertRaises(OSError):
                scan(root)


if __name__ == '__main__':
    unittest.main()
