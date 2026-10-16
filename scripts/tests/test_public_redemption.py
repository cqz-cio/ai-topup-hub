from contextlib import closing
import json,sqlite3,tempfile,unittest
from pathlib import Path
import sys
sys.path.insert(0,str(Path(__file__).resolve().parents[1]))
from public_redemption import public_copy,apply_database,snapshot

class PublicRedemptionTest(unittest.TestCase):
    def setUp(self):
        self.tmp=tempfile.TemporaryDirectory();self.addCleanup(self.tmp.cleanup)
        self.db=Path(self.tmp.name)/'shop.db';self.backup=Path(self.tmp.name)/'backups'
        self.copy=json.dumps({'zh-CN':'质保一个月。<a href="https://cdk.bar/">自助兑换</a>','en-US':'Redeem at https://cdkai.kdns.fr/. Warranty unchanged.'}).encode()
        with closing(sqlite3.connect(self.db)) as db, db:
            db.executescript('CREATE TABLE products(id INTEGER PRIMARY KEY,content_json BLOB,instructions_json BLOB,price_amount TEXT); CREATE TABLE settings(key TEXT PRIMARY KEY,value_json BLOB); CREATE TABLE posts(id INTEGER PRIMARY KEY,content_json BLOB); CREATE TABLE orders(id INTEGER PRIMARY KEY,payload BLOB);')
            db.execute('INSERT INTO products VALUES(1,?,?,?)',(self.copy,self.copy,'130.00'))
            db.execute('INSERT INTO settings VALUES(?,?)',('home_announcement',self.copy))
            db.execute('INSERT INTO settings VALUES(?,?)',('private_config',self.copy))
            db.execute('INSERT INTO posts VALUES(1,?)',(self.copy,));db.execute('INSERT INTO orders VALUES(1,?)',(self.copy,))
    def rows(self,path=None):
        with closing(sqlite3.connect(path or self.db)) as db:return snapshot(db)
    def test_public_html_removes_anchor_and_raw_addresses_in_all_languages(self):
        changed=public_copy(json.loads(self.copy))
        self.assertNotIn('cdk',json.dumps(changed));self.assertNotIn('href',changed['zh-CN'])
        self.assertIn('质保一个月',changed['zh-CN']);self.assertIn('order delivery',changed['en-US'])
        self.assertEqual(public_copy(changed),changed)
        other='<a href="https://example.com/">Other service</a>'
        self.assertEqual(public_copy(other),other)
    def test_readonly_plan_backup_and_exact_protected_data(self):
        before=self.rows();report=apply_database(self.db)
        self.assertEqual(report['planned_fields'],3);self.assertEqual(before,self.rows())
        report=apply_database(self.db,apply=True,backup_dir=self.backup)
        self.assertTrue(report['all_other_fields_unchanged']);self.assertEqual(before,self.rows(report['backup']))
        after=self.rows();self.assertEqual(before['orders'],after['orders'])
        self.assertEqual(before['products'][0][3:],after['products'][0][3:])
        self.assertEqual(before['settings'][1],after['settings'][1])
        self.assertIsInstance(after['products'][0][2],bytes)
        self.assertEqual(apply_database(self.db,apply=True,backup_dir=self.backup)['changed_fields'],0)
    def test_failed_update_rolls_back_all_public_copy(self):
        with closing(sqlite3.connect(self.db)) as db, db:db.execute("CREATE TRIGGER reject_post BEFORE UPDATE ON posts BEGIN SELECT RAISE(ABORT,'test rollback'); END")
        before=self.rows()
        with self.assertRaises(sqlite3.IntegrityError):apply_database(self.db,apply=True,backup_dir=self.backup)
        self.assertEqual(before,self.rows())
    def test_refuses_write_without_backup(self):
        with self.assertRaises(ValueError):apply_database(self.db,apply=True)

if __name__=='__main__':unittest.main()
