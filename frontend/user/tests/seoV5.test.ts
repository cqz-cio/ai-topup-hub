import test from 'node:test'
import assert from 'node:assert/strict'
import clusters from '../../../internal/platform/publiccontent/product-clusters.json' with {type:'json'}
import {otherRelatedProducts} from '../src/utils/grokRelatedProducts.ts'
import {pageDescription,blogDescription,catalogDescription} from '../src/utils/publicDescriptions.ts'
import {imageDimensions} from '../src/utils/imageDimensions.ts'

test('product clusters link current peer slugs and never unrelated products',()=>{
  for(const cluster of clusters){
    const rows=cluster.slugs.map((slug,id)=>({id,slug,slug_aliases:[] as string[]}))
    for(const row of rows)assert.equal(otherRelatedProducts(row,rows).length,rows.length-1)
    const renamed={...rows[1]!,slug:'current-name',slug_aliases:[rows[1]!.slug]}
    assert.deepEqual(otherRelatedProducts(rows[0]!,[rows[0]!,renamed,{id:99,slug:'not-in-cluster'}]),[renamed])
  }
})
test('public descriptions are substantive and image sizes are measured, not guessed',()=>{
  const texts=['terms','privacy','about','products','notice'].map(pageDescription).concat(blogDescription(['购买、交付与售后说明']),catalogDescription('Gemini',['18 个月方案']))
  for(const text of texts)assert.ok(Array.from(text).length>=70 && Array.from(text).length<=130,text)
  assert.equal(new Set(texts).size,texts.length)
  assert.deepEqual(imageDimensions('/media/seo-share/catalog-d7d4c32af9652177616a-1200x630.jpg'),{width:1200,height:630})
  assert.deepEqual(imageDimensions('/uploads/unknown.jpg'),{})
  assert.deepEqual(imageDimensions('https://other.invalid/media/seo-share/catalog-d7d4c32af9652177616a-1200x630.jpg'),{})
})
