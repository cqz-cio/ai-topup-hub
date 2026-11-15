import test from 'node:test'
import assert from 'node:assert/strict'
import identities from '../../../internal/platform/publiccontent/grok-products.json' with {type:'json'}
import {otherGrokProducts} from '../src/utils/grokRelatedProducts.ts'
import {productSchemaImages} from '../src/utils/shareSeo.ts'
import {appendDeliveryPolicy} from '../src/utils/deliveryPolicy.ts'

test('SVG product covers use a raster share image in Product JSON-LD',()=>{
  assert.deepEqual(productSchemaImages('https://www.qzweb.cloud',['/uploads/grok.svg'],'','/media/seo-share/qzweb-og-1200x630.jpg'),['https://www.qzweb.cloud/media/seo-share/qzweb-og-1200x630.jpg'])
  assert.deepEqual(productSchemaImages('https://www.qzweb.cloud',['/uploads/grok.svg'],'/uploads/custom.jpg','/media/seo-share/qzweb-og-1200x630.jpg'),['https://www.qzweb.cloud/uploads/custom.jpg'])
})
test('Grok alternatives exclude self and unrelated offers, retain current canonical slugs after renaming',()=>{
  const rows=identities.map((slug,index)=>({id:66+index,slug,slug_aliases:[] as string[]}))
  for(const row of rows)assert.equal(otherGrokProducts(row,rows).length,7)
  const renamed={...rows[1]!,slug:'plus-year-current',slug_aliases:[rows[1]!.slug]}
  const result=otherGrokProducts(rows[0]!,[rows[0]!,renamed,{id:100,slug:'ellaix-grok-premium-monthly-half'}])
  assert.deepEqual(result,[renamed])
  assert.deepEqual(otherGrokProducts({id:9,slug:'unrelated'},rows),[])
})
test('one short policy link preserves the original body and an existing path is never appended again',()=>{
  const body='<h2>商品条件</h2><p>仅适用本商品的约定。</p>'
  const result=appendDeliveryPolicy(body,false)
  assert.equal(result,body+'<p>售后与退款流程见<a href="/blog/purchase-and-agent-guide">交付与售后说明</a>。</p>')
  assert.equal(appendDeliveryPolicy(result,false),result)
  const existing=body+'<a href="/blog/purchase-and-agent-guide#merchant-return-policy">处理流程</a>'
  assert.equal(appendDeliveryPolicy(existing,false),existing)
})
