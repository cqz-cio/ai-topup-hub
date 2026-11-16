import test from 'node:test'
import assert from 'node:assert/strict'
import {readFileSync} from 'node:fs'
import {fileURLToPath} from 'node:url'
import catalog from '../../../docs/catalog/seo-v5/copy.json' with {type:'json'}
import baseline from '../../../docs/catalog/seo-v5/before.json' with {type:'json'}
import clusters from '../../../internal/platform/publiccontent/product-clusters.json' with {type:'json'}
import {normalizeProductContent,productContentSimilarity} from '../src/utils/productContentSimilarity.ts'
const copy=new Map(catalog.products.map(product=>[product.slug,readFileSync(fileURLToPath(new URL('../../../docs/catalog/seo-v5/'+product.file,import.meta.url)),'utf8')]))

test('v5 treats tier, region, period and price substitutions as the same body',()=>{
  const left='<p>ChatGPT Pro 5x 菲区 100美金 3个月 礼品</p>'
  const right='<p>ChatGPT Go 埃及 500美金 12个月 礼品</p>'
  assert.equal(normalizeProductContent(left),normalizeProductContent(right))
})
test('all three v5 draft clusters have independent normalized text and unique visible FAQs',()=>{
  const questions=new Set<string>()
  for(const product of catalog.products){
    const body=copy.get(product.slug)!
    const faqs=[...body.matchAll(/<p><strong>([^<]+)<\/strong>/g)].map(match=>match[1]!)
    assert.ok(faqs.length>=3,product.slug)
    for(const question of faqs){assert.equal(questions.has(question),false,question);questions.add(question)}
    assert.ok(product.seo_description.length>=70,product.slug)
    assert.notEqual(product.seo_description,product.title)
    const original=(baseline as any)[product.slug].content['zh-CN'].replace(/^<h3>商品与交付<\/h3>[\s\S]*?(?=<h3>)/,'')
    assert.ok(body.includes(original),'original operative terms preserved: '+product.slug)
  }
  for(const cluster of clusters)for(let i=0;i<cluster.slugs.length;i++)for(const right of cluster.slugs.slice(i+1)){
    const left=cluster.slugs[i]!,a=copy.get(left)!,b=copy.get(right)!
    const score=productContentSimilarity(a,b)
    assert.ok(score<.85,`${left} / ${right}: ${score}`)
    assert.notEqual(normalizeProductContent(a),normalizeProductContent(b))
  }
})
test('baseline still detects real near-duplicate copy in each v5 cluster',()=>{
  for(const cluster of clusters){
    const [a,b]=cluster.slugs
    assert.ok(productContentSimilarity((baseline as any)[a!].content['zh-CN'],(baseline as any)[b!].content['zh-CN'])>.85,cluster.key)
  }
})
