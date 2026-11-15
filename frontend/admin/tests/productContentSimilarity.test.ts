import test from 'node:test'
import assert from 'node:assert/strict'
import fixture from './fixtures/grok-before-v4.json' with {type:'json'}
import {nearDuplicateProducts,normalizeProductContent,productContentSimilarity} from '../src/utils/productContentSimilarity.ts'

test('all eight existing Grok bodies are near duplicates after tier and period normalization',()=>{
  for(const product of fixture.products){
    const matches=nearDuplicateProducts(product.content,fixture.products,product.id)
    assert.equal(matches.length,7,product.slug)
    assert.ok(matches.every(match=>match.similarity>0.85))
  }
})
test('specific Plus redemption copy does not falsely match the Grok boilerplate',()=>{
  for(const product of fixture.products)assert.ok(productContentSimilarity(product.content,fixture.control.content)<0.85)
})
test('markup, numbers, price, stock and tier-only changes cannot hide duplication',()=>{
  assert.equal(normalizeProductContent('<h2>Ultra 年付</h2><p>套餐 ￥15348.00 CNY 有货</p><script>secret</script>'),normalizeProductContent('<b>Plus 月付</b><p>套餐 ￥88 CNY 暂时缺货</p>'))
  assert.deepEqual(nearDuplicateProducts('',fixture.products),[])
  assert.deepEqual(nearDuplicateProducts('太短',fixture.products),[])
})
