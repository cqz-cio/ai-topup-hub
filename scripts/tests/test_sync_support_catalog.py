import importlib.util,json,pathlib,tempfile,unittest
from unittest.mock import patch
ROOT=pathlib.Path(__file__).resolve().parents[2]
spec=importlib.util.spec_from_file_location('sync_catalog',ROOT/'scripts/sync_support_catalog.py')
sync=importlib.util.module_from_spec(spec);spec.loader.exec_module(sync)
class Response:
 def __init__(self,data):self.data=data
 def __enter__(self):return self
 def __exit__(self,*a):pass
 def read(self,*a):return json.dumps(self.data).encode()
class CatalogSafety(unittest.TestCase):
 def test_incomplete_or_duplicate_never_replaces_snapshot(self):
  for rows,total in [([],1),([{'slug':'same'},{'slug':'same'}],2)]:
   with self.subTest(rows=rows),tempfile.TemporaryDirectory() as tmp:
    target=pathlib.Path(tmp)/'snapshot.json';target.write_text('previous')
    opener=unittest.mock.Mock();opener.open.return_value=Response({'status_code':0,'data':rows,'pagination':{'total':total,'total_page':1}})
    with patch.object(sync,'TARGET',target),patch.object(sync.urllib.request,'build_opener',return_value=opener):
     with self.assertRaises(ValueError):sync.fetch()
    self.assertEqual(target.read_text(),'previous')
 def test_private_fields_and_markup_are_not_projected(self):
  p={'id':1,'slug':'fixture','title':{'zh-CN':'商品'},'description':{'zh-CN':'说明'},'price_amount':'1.00','is_sold_out':False,'fulfillment_type':'manual','purchase_type':'guest','min_purchase_quantity':1,'max_purchase_quantity':1,'content':{'zh-CN':'<script>secret()</script><h2>购买前请确认</h2><p>条件</p>'},'category':{'name':{'zh-CN':'测试'}},'skus':[{'spec_values':{},'price_amount':'1.00','is_active':True,'is_sold_out':False,'cost_price_amount':'PRIVATE'}],'api_key':'PRIVATE','delivery':'PRIVATE','upstream_cost':'PRIVATE'}
  projected=sync.project_product(p)
  text=json.dumps(projected)
  self.assertNotIn('PRIVATE',text);self.assertNotIn('secret()',text)
 def test_unknown_policy_structure_fails_closed(self):
  with self.assertRaises(ValueError):sync.sections('<p>无法识别的新格式</p>')
  with self.assertRaises(ValueError):sync.product_policy({'slug':'unknown','sections':[],'content':{'zh-CN':'不明确'}})
 def test_new_claude_never_uses_legacy_warranty(self):
  p={'slug':'ellaix-claude-pro','sections':[{'heading':'适用账号','text':'已通过 KYC，不做任何质保'},{'heading':'兑换说明','text':'按订单交付操作'}],'content':{'zh-CN':''}}
  result=sync.product_policy(p);self.assertEqual(result['refund_mode'],'no_warranty')
  p['sections'][0]['text']='旧质保';
  with self.assertRaises(ValueError):sync.product_policy(p)
if __name__=='__main__':unittest.main()
