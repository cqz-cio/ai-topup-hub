import test from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import catalog from '../../../docs/catalog/seo-v4/grok-copy.json' with {type:'json'}
import {normalizeProductContent,productContentSimilarity} from '../src/utils/productContentSimilarity.ts'
test('all eight reviewed drafts have independent bodies, descriptions and SEO descriptions',()=>{
  const rows=catalog.products.map(p=>({...p,content:readFileSync(new URL('../../../docs/catalog/seo-v4/'+p.file,import.meta.url),'utf8')}))
  assert.equal(rows.length,8)
  for(const p of rows){
    assert.ok(normalizeProductContent(p.content).length>500,p.slug)
    assert.ok(p.content.includes('常见问题'))
    assert.ok((p.content.match(/<strong>/g)||[]).length>=3)
    for(const q of rows.filter(q=>q.id!==p.id)){
      assert.ok(productContentSimilarity(p.content,q.content)<0.85,p.slug+' / '+q.slug)
      assert.notEqual(normalizeProductContent(p.description),normalizeProductContent(q.description))
      assert.notEqual(normalizeProductContent(p.seo_description),normalizeProductContent(q.seo_description))
    }
  }
})
