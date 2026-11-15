import importlib.util,json,sqlite3,tempfile,unittest
from pathlib import Path

ROOT=Path(__file__).resolve().parents[2]
spec=importlib.util.spec_from_file_location('grok_copy',ROOT/'scripts/seo_grok_v4_content.py')
module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)

class GrokCopyTests(unittest.TestCase):
    def setUp(self):
        self.temp=tempfile.TemporaryDirectory();self.path=Path(self.temp.name)/'shop.db'
        self.db=sqlite3.connect(self.path);self.db.row_factory=sqlite3.Row
        self.catalog,self.digest=module.load_catalog()
        self.baseline=json.loads((ROOT/'docs/catalog/seo-v4/before.json').read_text('utf-8'))
        self.db.executescript('CREATE TABLE products(id INTEGER PRIMARY KEY,slug TEXT,category_id INTEGER,is_active INTEGER,deleted_at TEXT,title_json BLOB,description_json BLOB,content_json BLOB,seo_meta_json BLOB,updated_at TEXT,price TEXT,stock INTEGER); CREATE TABLE orders(id INTEGER PRIMARY KEY,snapshot BLOB); INSERT INTO orders VALUES(1,x\'010203\'); CREATE TABLE mappings(id INTEGER,product_id INTEGER); INSERT INTO mappings VALUES(1,66);')
        for p in self.catalog['products']:
            before=self.baseline[p['slug']]
            enc=lambda value:json.dumps(value,ensure_ascii=False).encode()
            self.db.execute('INSERT INTO products VALUES(?,?,?,?,?,?,?,?,?,?,?,?)',(p['id'],p['slug'],23,1,None,enc({'zh-CN':p['title']}),enc({'zh-CN':before['description'],'en-US':'keep description'}),enc({'zh-CN':before['content'],'en-US':'keep content'}),enc({'og_image':'/uploads/preserved.jpg'}),'2026-11-05','88.00',0 if p['id']==66 else 10))
        self.db.commit()
    def tearDown(self):self.db.close();self.temp.cleanup()
    def test_copy_only_audit_timestamp_and_idempotence(self):
        before=module.protected_fingerprints(self.db)
        result=module.apply_transaction(self.db,self.catalog,self.baseline,'2026-11-15T15:00:00+00:00')
        self.assertEqual(result['changed_fields'],32)
        self.assertEqual(result['products_changed'],8)
        self.assertEqual(module.protected_fingerprints(self.db),before)
        for row in self.db.execute('SELECT * FROM products'):
            self.assertEqual(json.loads(row['content_json'])['en-US'],'keep content')
            self.assertEqual(json.loads(row['seo_meta_json'])['og_image'],'/uploads/preserved.jpg')
            self.assertEqual(row['updated_at'],'2026-11-15T15:00:00+00:00')
        self.assertEqual(module.apply_transaction(self.db,self.catalog,self.baseline,'2026-11-16T15:00:00+00:00')['changed_fields'],0)
        self.assertEqual(self.db.execute('SELECT stock FROM products WHERE id=66').fetchone()[0],0)
    def test_newer_operator_copy_rejects_entire_batch(self):
        self.db.execute('UPDATE products SET content_json=? WHERE id=73',(json.dumps({'zh-CN':'new operator copy'}).encode(),));self.db.commit()
        before=self.path.read_bytes()
        with self.assertRaisesRegex(ValueError,'Newer copy'):module.apply_transaction(self.db,self.catalog,self.baseline,'2026-11-15')
        self.assertEqual(self.path.read_bytes(),before)
    def test_changed_identity_rejects_without_writes(self):
        self.db.execute('UPDATE products SET slug=? WHERE id=66',('different-product',));self.db.commit();before=self.path.read_bytes()
        with self.assertRaisesRegex(ValueError,'identity'):module.apply_transaction(self.db,self.catalog,self.baseline,'2026-11-15')
        self.assertEqual(self.path.read_bytes(),before)
    def test_unconfirmed_or_changed_copy_cannot_be_published(self):
        with self.assertRaisesRegex(ValueError,'operator approval'):module.approved(None,self.digest)
        approval=Path(self.temp.name)/'approval.json'
        approval.write_text(json.dumps({'status':'approved','copy_sha256':'wrong','confirmed_by':'operator'}))
        with self.assertRaisesRegex(ValueError,'exact copy'):module.approved(approval,self.digest)
        approval.write_text(json.dumps({'status':'approved','copy_sha256':self.digest,'confirmed_by':'operator'}))
        module.approved(approval,self.digest)

if __name__=='__main__':unittest.main()
