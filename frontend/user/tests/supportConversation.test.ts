import assert from 'node:assert/strict'
import test from 'node:test'
import { getConversationCopy, normalizeSupportProducts, respondToSupport, type SupportConversation, type SupportProduct } from '../src/components/support/supportConversation.ts'

const catalog: SupportProduct[] = [
  { slug: 'qzweb-5', title: 'ChatGPT Plus 新开通', family: 'gpt', delivery: 'auto' },
  { slug: 'qzweb-7', title: 'ChatGPT Pro 20x 续费', family: 'gpt', delivery: 'auto' },
  { slug: 'qzweb-ios', title: 'ChatGPT iOS 渠道', family: 'gpt', delivery: 'auto' },
  { slug: 'qzweb-73', title: 'Codex 250 点数充值', family: 'codex', delivery: 'auto' },
  { slug: 'qzweb-50', title: 'Gemini 18 个月', family: 'gemini', delivery: 'manual' },
]
function session(locale = 'zh-CN', products = catalog) {
  let state: SupportConversation = {}
  return (question: string, available = true) => {
    const result = respondToSupport(question, state, products, locale, available)
    assert.ok(result, 'Expected a guide reply for: ' + question)
    state = result.state
    return result
  }
}
test('product choice continues through purchase, delivery, redemption and result checks', () => {
  const ask = session()
  const list = ask('我想买 GPT')
  assert.deepEqual(list.state.choices?.map(item => item.value), ['qzweb-5', 'qzweb-7', 'qzweb-ios'])
  const product = ask('1')
  assert.equal(product.state.product?.slug, 'qzweb-5')
  assert.match(product.text, /至少 10 分钟/)
  assert.equal(product.links[0]?.to, '/products/qzweb-5')
  assert.match(ask('1').text, /实付合计/)
  const delivered = ask('我已付款，但还没拿到卡密')
  assert.match(delivered.text, /没有读取你的真实订单状态/)
  assert.deepEqual(delivered.links.map(link => link.to), ['/me/orders','/guest/orders'])
  assert.match(ask('拿到了').text, /提交一次/)
  const result = ask('显示成功但没到账')
  assert.match(result.text, /不同状态/)
  assert.equal(result.state.product?.slug, 'qzweb-5')
})
test('unknown family is clarified and the pending redemption step resumes', () => {
  const ask = session()
  assert.equal(ask('我拿到了卡密').state.step, 'redeem')
  const gemini = ask('3')
  assert.equal(gemini.state.family, 'gemini')
  assert.match(gemini.text, /Gemini 请打开/)
  assert.doesNotMatch(gemini.text, /再按.*GPT/)
})
test('iOS channel survives a short follow-up without reverting to ordinary code steps', () => {
  const ask = session()
  assert.equal(ask('买了 iOS 怎么充值').state.variant, 'ios')
  const reply = ask('拿到了')
  assert.match(reply.text, /不叠加/)
  assert.doesNotMatch(reply.text, /完成资格预检/)
})
test('choosing a new product clears the previous iOS channel context', () => {
  const ask = session()
  ask('买了 iOS 怎么充值')
  ask('我想买 GPT')
  ask('1')
  const reply = ask('拿到了')
  assert.match(reply.text, /完成资格预检/)
  assert.doesNotMatch(reply.text, /覆盖当前套餐/)
})
test('credits are not presented as a subscription or an extension', () => {
  const ask = session()
  assert.match(ask('我想买 Codex 点数').text, /不等于 Plus \/ Pro 订阅/)
  assert.match(ask('1').text, /不延长订阅/)
  assert.match(ask('拿到了').text, /只充值 Codex 点数/)
})
test('wallet recharge is distinguished from external subscriptions', () => {
  const ask = session()
  const start = ask('怎么充值')
  assert.equal(start.state.choices?.length, 4)
  const wallet = ask('4')
  assert.match(wallet.text, /不会自动.*开通订阅/)
  assert.equal(wallet.links[0]?.to, '/me/wallet')
  assert.equal(wallet.state.family, undefined)
})
test('not bought is not interpreted as paid or ready to redeem', () => {
  const ask = session()
  ask('买了 GPT 怎么充值')
  const reply = ask('其实还没买')
  assert.equal(reply.state.goal, 'buy')
  assert.match(reply.text, /公开目录/)
})
test('the not-yet-purchased numbered answer opens the real product choices', () => {
  const ask = session()
  ask('买了 GPT 怎么充值')
  const reply = ask('1')
  assert.equal(reply.state.choices?.[0]?.kind, 'product')
  assert.match(reply.text, /公开目录/)
})
test('short replies distinguish having a code from not having one', () => {
  const ask = session()
  ask('买了 GPT 怎么充值')
  assert.equal(ask('我没有卡密').state.step, 'delivery')
  assert.equal(ask('我有卡密').state.step, 'redeem')
  assert.equal(ask('其实我没有买').state.goal, 'buy')
})
test('missing codes take priority over having received a code', () => {
  const reply = session()('GPT 付款了但是没收到卡密')
  assert.equal(reply.state.step, 'delivery')
  assert.doesNotMatch(reply.text, /已拿到交付内容后/)
})
test('invalid or already-used codes never trigger repeated submission advice', () => {
  const ask = session()
  ask('买了 GPT 怎么充值')
  for (const input of ['卡密无效', '提示已经使用过了', '预检不通过']) {
    const reply = ask(input)
    assert.equal(reply.state.step, 'error')
    assert.match(reply.text, /不要重复提交/)
  }
})
test('explicit product name or slug preserves the correct product guide', () => {
  for (const input of ['qzweb-7', '我想了解 ChatGPT Pro 20x 续费']) {
    const reply = session()(input)
    assert.equal(reply.state.product?.slug, 'qzweb-7')
    assert.match(reply.text, /当前仍为 Pro 20x/)
  }
})
test('unmapped products do not inherit known account eligibility', () => {
  const unknown: SupportProduct = { slug: 'another-gpt', title: 'Another GPT 商品', family: 'gpt', delivery: 'unknown' }
  const reply = session('zh-CN', [unknown])('another-gpt')
  assert.doesNotMatch(reply.text, /至少 10 分钟|当前仍为 Pro 20x/)
  assert.match(reply.text, /不能保证/)
})
test('catalog failure does not invent products and does not block usage guidance', () => {
  const ask = session('zh-CN', [])
  const first = ask('我想买 GPT', false)
  assert.match(first.text, /没能读取商品目录/)
  assert.equal(first.state.choices, undefined)
  assert.match(ask('我拿到了卡密', false).text, /订单的交付说明/)
})
test('empty catalog and unavailable catalog have different responses', () => {
  assert.match(session('zh-CN', [])('我想买 GPT').text, /没有匹配商品/)
})
test('a removed catalog choice is not reused as a product recommendation', () => {
  const first = respondToSupport('我想买 GPT', {}, catalog, 'zh-CN')!
  const reply = respondToSupport('1', first.state, [], 'zh-CN')!
  assert.match(reply.text, /没有匹配商品/)
  assert.equal(reply.state.product, undefined)
})
test('numeric selection is only accepted for the latest active question', () => {
  const ask = session()
  ask('我想买 GPT'); ask('帮我写首诗')
  const reply = ask('1')
  assert.equal(reply.state.product, undefined)
  assert.equal(reply.state.choices?.[0]?.kind, 'family')
})
test('reset and family switch clear the previous selected product', () => {
  const ask = session()
  ask('qzweb-7')
  assert.equal(ask('我想买 Gemini').state.product, undefined)
  assert.equal(ask('重新开始').state.family, undefined)
})
test('next step after purchase is conditional order guidance, not a payment assertion', () => {
  const ask = session()
  ask('qzweb-5'); ask('1')
  assert.match(ask('下一步').text, /核对订单实际付款状态/)
})
test('human contact is an explicit handoff request, not an external message', () => {
  assert.equal(session()('我要联系人工').handoff, true)
})
test('other support topics stay with the existing independent replies', () => {
  for (const text of ['有哪些支付方式', 'GPT 退款', '返利规则', '我想开分店', '订单状态']) assert.equal(respondToSupport(text, {}, catalog, 'zh-CN'), null)
})
test('traditional Chinese and English support the same multi-turn guidance', () => {
  const tw = session('zh-TW')
  assert.match(tw('買了 GPT 怎麼儲值').text, /你現在到哪一步/)
  assert.match(tw('3').text, /付款後的訂單/)
  const en = session('en-US')
  assert.match(en('I bought GPT, how do I redeem?').text, /paid order/)
  assert.match(en('success but not credited').text, /separate states/)
})
test('all guide copy avoids presale URLs and fabricated live rates or balances', () => {
  for (const locale of ['zh-CN','zh-TW','en-US']) {
    const copy = JSON.stringify(getConversationCopy(locale))
    assert.doesNotMatch(copy, /https?:\/\/|\d+(\.\d+)?%|\bcdk\b/i)
  }
})
test('normalization keeps only public allowlisted fields', () => {
  const rows = [{ slug: 'qzweb-5', title: { 'zh-CN': 'GPT Plus', 'en-US': 'GPT Plus English' }, fulfillment_type: 'auto', instructions: 'PRIVATE', price: '9.99', password: 'PRIVATE', stock: 10 }]
  const result = normalizeSupportProducts(rows, 'en-US')
  assert.deepEqual(result, [{ slug: 'qzweb-5', title: 'GPT Plus English', family: 'gpt', delivery: 'auto' }])
})
test('unsafe names, paths, inactive rows and duplicate products are excluded', () => {
  const result = normalizeSupportProducts([
    { slug: '../secret', title: 'GPT' }, { slug: 'unsafe', title: '<b>GPT</b>' },
    { slug: 'link', title: 'GPT https://example.com/secret' }, { slug: 'bare', title: 'GPT example.com' },
    { slug: 'disabled', title: 'GPT', is_active: false }, { slug: 'disabled-zero', title: 'GPT', is_active: 0 },
    { slug: 'good', title: 'ＧＰＴ Plus' }, { slug: 'good', title: 'GPT duplicate' },
    { slug: 'unknown', title: 'Unrelated product' },
  ], 'zh-CN')
  assert.deepEqual(result.map(item => item.slug), ['good'])
})
