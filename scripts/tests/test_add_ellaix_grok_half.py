"""Offline additive-catalog, rollback and approval-boundary tests."""
from contextlib import closing
from datetime import datetime,timezone,timedelta
import importlib.util,json,sqlite3,unittest
from pathlib import Path
ROOT=Path(__file__).resolve().parents[2]

def load(name,path):
    spec=importlib.util.spec_from_file_location(name,path);m=importlib.util.module_from_spec(spec);spec.loader.exec_module(m);return m

class GrokHalfTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.m=load('grok_half',ROOT/'scripts/add-ellaix-grok-half.py')
        cls.fixture_class=load('fixture',ROOT/'scripts/tests/test_replace_gpt_ready_account.py').ReadyAccountMigrationTests
        cls.fixture_class.setUpClass()
        cls.catalog=ROOT/'docs/catalog/ellaix-grok-half/catalog.json';cls.config=json.loads(cls.catalog.read_text('utf8'))
    def setUp(self):
        self.fixture=self.fixture_class();self.fixture.setUp();self.db=self.fixture.db
        with closing(sqlite3.connect(self.db)) as db:
            cid=self.m.insert(db,'categories',{'slug':'grok','name_json':self.m.encode({'zh-CN':'Grok'}),'is_active':1})
            for p in self.config['items']:
                self.m.insert(db,'products',{'category_id':cid,'slug':p['cover_from_slug'],'price_amount':999,'images':self.m.encode(p['images']),'is_active':1})
            db.commit()
        self.source={'at':datetime.now(timezone.utc).isoformat(),'balance_unchanged':True,'purchase_endpoints_called':False,'items':[]}
        for p,cost in zip(self.config['items'],['108.80','345','700']):
            self.source['items'].append({'item':{'id':p['source_id'],'code':p['source_code'],'name':p['source_title'],'category_id':35,'status':1,'delivery_way':0,'widget':[],'config':[],'price':cost,'only_user':0,'minimum':1,'maximum':20},'quote':{'price':cost,'currency_code':'CNY'},'stock':{'stock':'10000'}})
    def tearDown(self):self.fixture.tearDown()
    def state(self):
        with closing(sqlite3.connect(self.db)) as db:return self.m.snapshot(db)
    def migrate(self,apply=False):return self.m.migrate(self.db,self.catalog,self.source,apply=apply,backup_root=Path(self.fixture.folder.name)/'backups')
    def test_dry_run_unchanged(self):
        before=self.state();self.migrate();self.assertEqual(before,self.state())
    def test_apply_backup_all_existing_rows_and_repeat(self):
        before=self.state();r=self.migrate(True)
        with closing(sqlite3.connect(r['backup'])) as db:self.assertEqual(before,self.m.snapshot(db))
        after=self.state()
        for table,rows in before.items():
            self.assertEqual(rows,[row for row in after[table] if row[0] not in r['created'].get(table,[])])
        repeat=self.migrate(True);self.assertEqual(after,self.state());self.assertTrue(all(not v for v in repeat['created'].values()))
    def test_late_failure_rolls_back(self):
        with closing(sqlite3.connect(self.db)) as db:
            db.execute("CREATE TRIGGER reject_last BEFORE INSERT ON product_skus WHEN NEW.sku_code='ELLAIX-118' BEGIN SELECT RAISE(ABORT,'fixture late failure'); END");db.commit()
        before=self.state()
        with self.assertRaises(sqlite3.IntegrityError):self.migrate(True)
        self.assertEqual(before,self.state())
    def test_9999_public_price_rejected(self):
        self.source['items'][0]['item']['price']='9999'
        with self.assertRaisesRegex(ValueError,'Placeholder'):self.migrate()
    def test_stale_quote_rejected(self):
        self.source['at']=(datetime.now(timezone.utc)-timedelta(hours=2)).isoformat()
        with self.assertRaisesRegex(ValueError,'Refresh'):self.migrate()
    def test_manual_or_form_rejected(self):
        self.source['items'][0]['item']['delivery_way']=1
        with self.assertRaises(ValueError):self.migrate()
        self.source['items'][0]['item']['delivery_way']=0;self.source['items'][0]['item']['widget']=[{'name':'account'}]
        with self.assertRaises(ValueError):self.migrate()
    def test_existing_repriced_product_not_overwritten(self):
        self.migrate(True)
        with closing(sqlite3.connect(self.db)) as db:db.execute("UPDATE products SET price_amount=139 WHERE slug='ellaix-grok-premium-monthly-half'");db.commit()
        before=self.state()
        with self.assertRaisesRegex(ValueError,'price differs'):self.migrate(True)
        self.assertEqual(before,self.state())

if __name__=='__main__':unittest.main()
