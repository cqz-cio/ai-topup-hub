import importlib.util,json,sqlite3,tempfile,unittest
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]
spec=importlib.util.spec_from_file_location('v5',ROOT/'scripts/seo_v5_content.py')
batch=importlib.util.module_from_spec(spec);spec.loader.exec_module(batch)

class V5ContentTests(unittest.TestCase):
    def setUp(self):
        self.db=sqlite3.connect(':memory:');self.db.row_factory=sqlite3.Row
        self.catalog,self.digest=batch.load_catalog()
        self.before=json.loads((ROOT/'docs/catalog/seo-v5/before.json').read_text('utf-8'))
        self.db.executescript('''CREATE TABLE products(id INTEGER PRIMARY KEY,slug TEXT,category_id INTEGER,is_active INTEGER,deleted_at TEXT,title_json BLOB,description_json BLOB,content_json BLOB,seo_meta_json BLOB,updated_at TEXT,images BLOB,price TEXT,stock INTEGER);
          CREATE TABLE categories(id INTEGER PRIMARY KEY,slug TEXT,is_active INTEGER,deleted_at TEXT,icon TEXT,seo_meta_json BLOB);
          CREATE TABLE orders(id INTEGER,snapshot BLOB);INSERT INTO orders VALUES(1,x'0102');
          CREATE TABLE card_secrets(id INTEGER,secret TEXT);INSERT INTO card_secrets VALUES(1,'test-only-do-not-touch');''')
        enc=lambda value:json.dumps(value,ensure_ascii=False).encode()
        for slug,p in self.before.items():
            if slug.startswith('category:'):
                self.db.execute('INSERT INTO categories VALUES(?,?,?,?,?,?)',(p['id'],p['slug'],1,None,p['icon'],enc(p['seo_meta'])));continue
            desc=p.get('description',{}).copy();desc['en-US']='keep English'
            content=p.get('content',{}).copy();content['en-US']='keep original translation'
            self.db.execute('INSERT INTO products VALUES(?,?,?,?,?,?,?,?,?,?,?,?,?)',(p['id'],p['slug'],p['category_id'],1,None,enc(p['title']),enc(desc),enc(content),enc(p['seo_meta']),'2026-11-05',enc(p['images']),'123.00',5))
        self.db.commit()
    def tearDown(self):self.db.close()
    def test_copy_and_images_change_only_allowed_fields_and_are_idempotent(self):
        before=batch.protected_fingerprints(self.db)
        result=batch.apply_transaction(self.db,self.catalog,self.before,'2026-11-16T08:00:00+00:00')
        self.assertEqual(result['rows_changed'],16)
        self.assertEqual(batch.protected_fingerprints(self.db),before)
        self.assertEqual(batch.apply_transaction(self.db,self.catalog,self.before,'2026-12-01')['changed_fields'],0)
        for row in self.db.execute('SELECT * FROM products'):
            self.assertEqual(json.loads(row['content_json'])['en-US'],'keep original translation')
            self.assertEqual(json.loads(row['description_json'])['en-US'],'keep English')
            self.assertEqual(row['updated_at'],'2026-11-16T08:00:00+00:00')
    def test_newer_text_rolls_back_whole_batch(self):
        self.db.execute('UPDATE products SET content_json=? WHERE id=59',(json.dumps({'zh-CN':'operator edit'}),));self.db.commit()
        before=list(self.db.iterdump())
        with self.assertRaisesRegex(ValueError,'Newer copy'):batch.apply_transaction(self.db,self.catalog,self.before,'now')
        self.assertEqual(list(self.db.iterdump()),before)
    def test_newer_category_image_is_not_overwritten(self):
        self.db.execute("UPDATE categories SET icon='/new-operator.jpg'");self.db.commit();before=list(self.db.iterdump())
        with self.assertRaisesRegex(ValueError,'Newer category'):batch.apply_transaction(self.db,self.catalog,self.before,'now')
        self.assertEqual(list(self.db.iterdump()),before)
    def test_missing_and_mismatched_approval_refused(self):
        with self.assertRaisesRegex(ValueError,'operator approval'):batch.approved(None,self.digest)
        with tempfile.TemporaryDirectory() as directory:
            path=Path(directory)/'approval.json';path.write_text(json.dumps({'status':'approved','confirmed_by':'test','copy_sha256':'different'}))
            with self.assertRaisesRegex(ValueError,'exact copy'):batch.approved(path,self.digest)
    def test_reassigned_product_category_refused(self):
        self.db.execute('UPDATE products SET category_id=99 WHERE id=60');self.db.commit()
        with self.assertRaisesRegex(ValueError,'category/title'):batch.apply_transaction(self.db,self.catalog,self.before,'now')

if __name__=='__main__':unittest.main()
