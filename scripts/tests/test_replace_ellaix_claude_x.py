"""Offline transactional tests; supplier calls and real purchases are forbidden."""
import copy
from datetime import datetime, timezone, timedelta
import importlib.util
import json
from pathlib import Path
import sqlite3
import unittest

ROOT=Path(__file__).resolve().parents[2]


def load(name,path):
    spec=importlib.util.spec_from_file_location(name,path)
    mod=importlib.util.module_from_spec(spec);spec.loader.exec_module(mod)
    return mod


class ClaudeXTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.m=load('claude_x',ROOT/'scripts/replace-ellaix-claude-x.py')
        cls.legacy=load('fixture',ROOT/'scripts/tests/test_replace_gpt_ready_account.py')
        cls.legacy.ReadyAccountMigrationTests.setUpClass()
        cls.catalog=ROOT/'docs/catalog/ellaix-claude-x/catalog.json'
        cls.config=json.loads(cls.catalog.read_text('utf8'))

    def setUp(self):
        self.fixture=self.legacy.ReadyAccountMigrationTests()
        self.fixture.setUp();self.db=self.fixture.db
        with sqlite3.connect(self.db) as db:
            cid=self.m.insert(db,'categories',{'slug':'claude','name_json':self.m.encode({'zh-CN':'Claude'}),'is_active':1})
            for slug in self.m.OLD:
                self.m.insert(db,'products',{'slug':slug,'category_id':cid,'fulfillment_type':'manual','is_mapped':0,'is_active':1,'content_json':self.m.encode({'zh-CN':'historical warranty preserved'})})
        self.source={'at':datetime.now(timezone.utc).isoformat(),'purchase_endpoints_called':False,'balance_unchanged':True,'items':[]}
        for p,cost in zip(self.config['items'],['140','680','1360','55','225','730']):
            self.source['items'].append({'item':{'id':p['source_id'],'code':p['source_code'],'name':p['source_title'],'category_id':p['source_category'],'status':1,'delivery_way':0,'config':[],'widget':[],'only_user':0,'price':cost,'minimum':1,'maximum':20},'quote':{'price':cost,'currency_code':'CNY'},'stock':{'stock':'9999'}})

    def tearDown(self):
        self.fixture.tearDown()

    def state(self):
        with sqlite3.connect(self.db) as db:return self.m.snapshot(db)

    def run_it(self,apply=False):
        return self.m.migrate(self.db,self.catalog,self.source,apply=apply,backup_root=Path(self.fixture.folder.name)/'backups')

    def test_dry_run_keeps_all_rows(self):
        before=self.state();r=self.run_it()
        self.assertEqual(before,self.state());self.assertEqual(len(r['items']),6)

    def test_apply_backup_prices_history_and_idempotency(self):
        before=self.state();r=self.run_it(True)
        with sqlite3.connect(r['backup']) as db:self.assertEqual(before,self.m.snapshot(db))
        after=self.state();again=self.run_it(True)
        self.assertEqual(after,self.state());self.assertTrue(all(not x for x in again['created'].values()))
        with sqlite3.connect(self.db) as db:
            for p in self.config['items']:
                row=db.execute('SELECT price_amount,typeof(content_json),typeof(images),fulfillment_type FROM products WHERE slug=?',(p['slug'],)).fetchone()
                self.assertEqual(float(row[0]),float(p['price']));self.assertEqual(row[1:],('blob','blob','upstream'))
            self.assertEqual(db.execute('SELECT count(*) FROM product_mappings WHERE upstream_product_id IN(121,123,125,126,127)').fetchone()[0],0)
        self.assertEqual(before['orders'],after['orders'])
        self.assertEqual(before['wallet_transactions'],after['wallet_transactions'])

    def test_late_failure_rolls_back_new_category_and_all_items(self):
        with sqlite3.connect(self.db) as db:db.execute("CREATE TRIGGER reject_last BEFORE INSERT ON product_skus WHEN NEW.sku_code='ELLAIX-124' BEGIN SELECT RAISE(ABORT,'late fixture failure'); END")
        before=self.state()
        with self.assertRaises(sqlite3.IntegrityError):self.run_it(True)
        self.assertEqual(before,self.state())

    def test_9999_public_price_rejected_even_with_low_merchant_quote(self):
        self.source['items'][0]['item']['price']='9999'
        before=self.state()
        with self.assertRaisesRegex(ValueError,'Placeholder'):self.run_it(True)
        self.assertEqual(before,self.state())

    def test_expired_quote_rejected(self):
        self.source['at']=(datetime.now(timezone.utc)-timedelta(hours=2)).isoformat()
        with self.assertRaisesRegex(ValueError,'Refresh'):self.run_it()

    def test_manual_delivery_or_extra_form_rejected(self):
        for key,value in [('delivery_way',1),('widget',[{'name':'password'}]),('only_user',1)]:
            with self.subTest(key=key):
                previous=copy.deepcopy(self.source['items'][0]['item'])
                self.source['items'][0]['item'][key]=value
                with self.assertRaises(ValueError):self.run_it()
                self.source['items'][0]['item']=previous

    def test_cost_over_sale_price_rejected(self):
        self.source['items'][0]['quote']['price']='169'
        with self.assertRaisesRegex(ValueError,'Invalid cost'):self.run_it()

    def test_existing_edited_sale_price_not_overwritten(self):
        self.run_it(True)
        with sqlite3.connect(self.db) as db:db.execute("UPDATE products SET price_amount=200 WHERE slug='ellaix-claude-pro'")
        before=self.state()
        with self.assertRaisesRegex(ValueError,'sale price differs'):self.run_it(True)
        self.assertEqual(before,self.state())

    def test_disabled_old_supplier_mappings_preserved(self):
        with sqlite3.connect(self.db) as db:
            cid=self.m.insert(db,'site_connections',{'base_url':'https://faka.redeemgpt.com','protocol':'acg-faka'})
            for slug,sid in zip(self.m.OLD,[84,10007]):
                pid=db.execute('SELECT id FROM products WHERE slug=?',(slug,)).fetchone()[0]
                db.execute('UPDATE products SET is_mapped=1 WHERE id=?',(pid,))
                self.m.insert(db,'product_mappings',{'connection_id':cid,'local_product_id':pid,'upstream_product_id':sid,'is_active':0})
        before=self.state()['product_mappings'];self.run_it(True)
        self.assertEqual(before,self.state()['product_mappings'][:len(before)])


if __name__=='__main__':unittest.main()
