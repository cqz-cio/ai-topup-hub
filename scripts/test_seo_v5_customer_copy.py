import importlib.util
from contextlib import closing
import json
import os
from pathlib import Path
import sqlite3
import subprocess
import sys
import tempfile
import unittest

ROOT=Path(__file__).resolve().parents[1]
SCRIPT=ROOT/'scripts/seo_v5_customer_copy.py'
MANIFEST=ROOT/'docs/catalog/seo-v5/customer-copy-fix-20261117.json'
spec=importlib.util.spec_from_file_location('customer_copy',SCRIPT)
module=importlib.util.module_from_spec(spec);spec.loader.exec_module(module)

class CustomerCopyTest(unittest.TestCase):
    def setUp(self):
        self.data,self.digest=module.load(MANIFEST)
        self.db=sqlite3.connect(':memory:');self.db.row_factory=sqlite3.Row
        self.db.execute('CREATE TABLE products(id INTEGER PRIMARY KEY,slug TEXT,is_active INTEGER,deleted_at TEXT,content_json BLOB,description_json BLOB,seo_meta_json BLOB,updated_at TEXT,price_amount TEXT,manual_stock_total INTEGER)')
        self.db.execute('CREATE TABLE orders(id INTEGER PRIMARY KEY,note TEXT)')
        self.db.execute("INSERT INTO orders VALUES (1,'unchanged test order')")
        for p in self.data['products']:
            fields={column:{'zh-CN':'unchanged','en-US':'Keep this locale'} for column in ['content_json','description_json']}
            fields['seo_meta_json']={'description':{'zh-CN':'unchanged','en-US':'Keep this locale'},'title':{'zh-CN':'keep title'}}
            for column in {e['column'] for e in p['replacements']}:
                text='\n'.join(e['before'] for e in p['replacements'] if e['column']==column)
                target=fields[column]['description'] if column=='seo_meta_json' else fields[column]
                target['zh-CN']=text
            values=[json.dumps(fields[c],ensure_ascii=False).encode() for c in ['content_json','description_json','seo_meta_json']]
            self.db.execute('INSERT INTO products VALUES (?,?,1,NULL,?,?,?,?,?,?)',[p['id'],p['slug'],*values,'2026-11-17T01:42:00Z','700.00',123])
        self.db.commit()

    def tearDown(self):self.db.close()

    def test_preserves_sqlite_blob_storage_business_values_and_other_locales(self):
        protected=module.protected(self.db)
        report=module.apply(self.db,self.data,'2026-11-17T02:00:00Z')
        self.assertEqual((report['products_changed'],report['changed_fields']),(8,18))
        self.assertEqual(module.protected(self.db),protected)
        for c in report['changes']:
            stored=self.db.execute('SELECT '+c['column']+' FROM products WHERE id=?',(c['id'],)).fetchone()[0]
            self.assertIs(type(stored),type(c['before']))
            if c['column']!='updated_at':
                self.assertIsInstance(stored,bytes)
                decoded=json.loads(stored)
                value=decoded['description'] if c['column']=='seo_meta_json' else decoded
                self.assertEqual(value['en-US'],'Keep this locale')
                self.assertNotIn('需店主确认',value['zh-CN'])
        self.assertEqual(module.apply(self.db,self.data,'later')['changed_fields'],0)

    def test_concurrent_editor_text_is_never_overwritten(self):
        self.db.execute('UPDATE products SET content_json=? WHERE id=59',(b'{"zh-CN":"new operator text"}',));self.db.commit()
        before=[tuple(r) for r in self.db.execute('SELECT * FROM products ORDER BY id')]
        with self.assertRaises(AssertionError):module.apply(self.db,self.data,'later')
        self.assertEqual(before,[tuple(r) for r in self.db.execute('SELECT * FROM products ORDER BY id')])

    def test_cli_writes_reversible_blob_report(self):
        with tempfile.TemporaryDirectory() as directory:
            db_path=Path(directory)/'shop.db'
            with closing(sqlite3.connect(db_path)) as target:self.db.backup(target)
            result=subprocess.run([sys.executable,str(SCRIPT),'--database',str(db_path),'--manifest',str(MANIFEST),'--apply','--approved-sha256',self.digest,'--backup-root',directory],capture_output=True,text=True,encoding='utf-8',env={**os.environ,'PYTHONIOENCODING':'utf-8'},timeout=15)
            self.assertEqual(result.returncode,0,result.stderr)
            summary=json.loads(result.stdout);report=json.loads((Path(summary['backup'])/'report.json').read_text('utf-8'))
            with closing(sqlite3.connect(db_path)) as db:
                for c in report['changes']:
                    if c['column']=='updated_at':continue
                    self.assertEqual(set(c['before']),{'blob'});self.assertEqual(set(c['after']),{'blob'})
                    self.assertEqual(db.execute('SELECT '+c['column']+' FROM products WHERE id=?',(c['id'],)).fetchone()[0],bytes.fromhex(c['after']['blob']))

if __name__=='__main__':unittest.main()
