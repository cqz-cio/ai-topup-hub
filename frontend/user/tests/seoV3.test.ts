import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync, statSync } from 'node:fs'
import titles from '../../../internal/platform/publiccontent/page-titles.json' with { type: 'json' }
import home from '../../../internal/platform/publiccontent/home-copy.json' with { type: 'json' }
import { productPurchaseGuide } from '../src/utils/productPurchaseGuide.ts'
import { codexCreditPoints } from '../src/utils/codexCredits.ts'
import { normalizeSupportProducts, respondToSupport } from '../src/components/support/supportConversation.ts'
const read = (path: string) => readFileSync(new URL(path, import.meta.url), 'utf8')

test('SSR copy and three storefront locales agree', () => {
  for (const locale of ['zh-CN','zh-TW','en-US'] as const) {
    const copy = JSON.parse(read('../src/i18n/locales/'+locale+'.json')).shopUi
    for (const key of ['productsTitle','blogTitle','noticeTitle','aboutTitle'] as const) assert.equal(copy[key], titles[locale][key])
    if (locale === 'zh-CN') assert.equal(copy.heroTitle + copy.heroAccent, home.heroTitle + home.heroAccent)
  }
})
test('renamed catalog items keep published eligibility guidance and credit identity', () => {
  assert.deepEqual(productPurchaseGuide('chatgpt-plus-code-2',['qzweb-5']), productPurchaseGuide('qzweb-5'))
  assert.deepEqual(productPurchaseGuide('later-manual-name',['qzweb-7']), productPurchaseGuide('qzweb-7'))
  assert.equal(codexCreditPoints({slug:'codex-250-dian-shu',slug_aliases:['qzweb-73']}),250)
  assert.equal(codexCreditPoints({slug:'fake-codex-250'}),undefined)
  const products = normalizeSupportProducts([{slug:'chatgpt-plus-code',slug_aliases:['qzweb-5'],title:{'zh-CN':'ChatGPT Plus'},fulfillment_type:'auto'}],'zh-CN')
  assert.deepEqual(products[0]?.slug_aliases,['qzweb-5'])
  const reply=respondToSupport('购买',{family:'gpt',product:products[0],step:'buy'},products,'zh-CN')
  assert.ok(reply)
})
test('wallpaper preload, CSS and renderer use new files while original media remain', () => {
  const css=read('../src/styles/commerce.css'), index=read('../index.html'), renderer=read('../src/utils/jellyfishing/renderer.js')
  for (const size of [1280,1672]) for (const ext of ['webp','avif']) {
    assert.ok(css.includes('/media/storefront-20261114/jellyfishing-'+size+'.'+ext))
    assert.ok(statSync(new URL('../public/media/storefront-20261114/jellyfishing-'+size+'.'+ext,import.meta.url)).size < 400_000)
  }
  assert.ok(index.includes("location.pathname === '/'"))
  assert.ok(index.includes('(max-width: 1280px)'))
  assert.ok(![css,index,renderer].some(text=>text.includes('/storefront-20261020/jellyfishing.webp')))
  for (const file of ['jellyfishing-1672.webp','clean-plate.webp','foreground.webp']) assert.ok(renderer.includes('/media/storefront-20261114/'+file))
  assert.ok(statSync(new URL('../public/media/storefront-20261020/jellyfishing.webp',import.meta.url)).size > 1_000_000)
})
