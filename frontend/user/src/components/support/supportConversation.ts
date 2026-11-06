export type SupportFamily = 'gpt' | 'codex' | 'gemini'
export type SupportProduct = { slug: string; title: string; family: SupportFamily; delivery: 'auto' | 'manual' | 'unknown' }
export type SupportLink = { label: string; to: string }
type Choice = { label: string; value: string; kind: 'family' | 'goal' | 'product' | 'step' }
export type SupportConversation = { family?: SupportFamily; product?: SupportProduct; goal?: 'buy' | 'use'; variant?: 'ios'; step?: string; choices?: Choice[] }
export type SupportReply = { text: string; state: SupportConversation; links: SupportLink[]; handoff?: boolean }

const cn = {
  subtitle: '站内助手 · 选购与使用指引', footer: '试运行 · 具体规则以商品及订单说明为准',
  start: '可以，我陪你一步步来。你想了解哪一种？',
  families: ['ChatGPT 订阅', 'Codex 点数', 'Gemini 商品', '商城钱包充值'],
  goal: '你现在是想选购商品，还是已经购买、需要充值或使用帮助？', goals: ['还没买，帮我选', '已经购买，教我使用'],
  choose: '回复序号或商品名即可。价格、库存和资格请以商品详情为准。', catalog: '这是当前公开目录里匹配的商品：', more: '其余选项可以在商品中心查看。',
  unavailable: '这次没能读取商品目录，暂时不能确认具体在售选项。你可以稍后回复“重新推荐”，或打开商品中心查看；我仍然可以讲解充值与使用步骤。',
  empty: '当前读取的目录里没有匹配商品，不能据此保证有货。请查看商品中心或联系人工确认。',
  purpose: {
    gpt: '想给自己的 ChatGPT 账号开通或续费，请看 ChatGPT 商品。不同套餐、渠道和开通/续费条件需要分别核对。',
    codex: 'Codex 点数商品是额度补充，不等于 Plus / Pro 订阅，也不延长订阅时长。购买前需要已有有效订阅并符合该商品的点数购买资格。',
    gemini: 'Gemini 商品按各自说明交付和兑换，账号条件、活动使用记录与保障范围可能不同，不能套用 GPT 的兑换步骤。',
  },
  progress: '你现在到哪一步了？', steps: ['还没购买', '已付款，还没拿到交付内容', '已拿到卡密或兑换链接', '兑换报错 / 结果不对'],
  selected: '你选的是：', selectedHelp: '先打开这款商品的详情，核对规格、适用账号、交付方式和保障说明。选购不是资格审核，客服小窗不能保证账号一定可以开通。',
  rules: {
    activation: '这款属于新开通类：账号从未开通过会员，或上次订阅已到期 / 掉订至少 10 分钟；最终资格以兑换预检为准。',
    renewal: '这款属于 Pro 20x 续费：要求账号当前仍为 Pro 20x，绑卡后还需按商品说明联系人工确认。',
    credits: '这款只充值 Codex 点数：须有有效订阅并符合购买资格，不包含订阅，也不延长订阅时长。',
    ios: 'iOS 渠道请使用订单内专属说明。该类充值覆盖当前套餐，不叠加账号原有剩余时长；不要照普通卡密流程操作。',
    gemini: 'Gemini 需核对是否使用过对应活动、账号注册时间及本款保障范围；请以这一款的详情和交付说明为准。',
  },
  buy: '选购步骤：\n1. 在商品详情确认套餐/规格、账号条件、交付方式与保障说明。\n2. 选好数量进入结算，核对商品金额、手续费和实付合计。\n3. 按付款页当前提供的渠道付款；确认到账后，再去对应订单查看交付内容。\n\n拿到卡密不等于账号已经开通。你完成后可以继续问我“怎么兑换”。',
  delivery: '先找到你下单时对应的订单：\n• 登录购买：个人中心 → 我的订单。\n• 游客购买：使用下单邮箱和查询密码进入游客订单查询。\n\n核对订单实际付款状态和交付内容；自动发货、人工处理或预售的安排以该商品说明为准。已经付款但仍无交付内容，请联系人工提供订单号，不要重复付款。这里没有读取你的真实订单状态。',
  redeem: '已拿到交付内容后：\n1. 打开这笔已付款订单的交付说明，使用其中对应商品的兑换入口。\n2. 核对充值目标账号、商品条件，按页面完成资格预检。\n3. 按交付说明提交一次，再查询该次处理结果；不要反复提交。\n4. 返回对应服务，用刚充值的同一个账号核对结果。\n\n具体入口只在付款后的订单交付说明里提供。',
  geminiUse: 'Gemini 请打开这笔订单附带的兑换链接和说明，核对账号及活动条件后按提示完成兑换，再用同一个账号检查订阅。不要使用其他商品的卡密入口。',
  iosUnknown: '如果买的是 iOS 渠道，请先告诉我完整商品名；它与普通卡密流程不同。',
  next: '你可以继续告诉我“预检不通过”“卡密无效”或“显示成功但没到账”，我会接着排查。',
  error: '先分清问题出现在哪一步：\n• 预检不通过：先核对这一款商品的账号条件，不要强行提交。\n• 卡密无效或提示已使用：核对订单中的商品与卡密，查看是否已有处理记录。\n• 处理中或结果不确定：查询原提交记录，暂时不要重复提交或重新购买。\n\n请告诉我具体报错文字及商品名，不用发送完整卡密、密码或账号令牌。涉及重发、退款和人工补单，需要人工核实。',
  result: '“订单已付款”“卡密已发货”和“兑换成功”是不同状态。\n\n先检查兑换页面的处理结果，再确认登录的是充值时使用的同一个账号。订阅商品查看对应订阅状态；Codex 点数查看点数余额，不要用套餐名称是否变化来判断到账。仍不一致时，把商品名和不含敏感内容的报错交给人工核实。这里不能确认你的实际到账情况。',
  use: '兑换成功后，用充值时的同一个账号登录对应服务，再核对这款商品带来的订阅或点数。卡密只用于兑换，不是登录密码；不同服务的账号和入口不能混用。\n\n如果你卡在登录、资格检查或权益没有变化，告诉我具体是哪一步。',
  wallet: '商城钱包余额与 GPT / Gemini 订阅、Codex 点数不同。\n\n充值商城余额请进入个人中心 → 我的钱包，填写充值金额并选择页面当前支持的付款方式。余额到账后用于商城结算，不会自动给外部账号开通订阅。',
  context: '我还记得你正在咨询{product}。你是想看购买步骤、兑换步骤，还是处理报错？', contextSteps: ['购买步骤', '兑换步骤', '处理报错'],
  unknown: '这版先帮助你选购商品、领取交付内容、充值和排查使用步骤。你可以说“我想买 GPT”或“拿到 Gemini 链接后怎么用”；其他问题也可以联系人工。',
  thanks: '不客气。后面卡在哪一步，直接接着告诉我就行。', human: '可以，下面是人工客服联系方式。请说明商品名和遇到的问题；订单问题可提供订单号。',
  catalogLink: '打开商品中心', productLink: '查看这款商品详情', orderLink: '我的订单', guestLink: '游客订单查询', walletLink: '打开我的钱包',
}

type GuideCopy = typeof cn
const tw: GuideCopy = {
  subtitle: '站內助手 · 選購與使用指引', footer: '試運行 · 具體規則以商品及訂單說明為準',
  start: '可以，我陪你一步步來。你想了解哪一種？', families: ['ChatGPT 訂閱', 'Codex 點數', 'Gemini 商品', '商城錢包儲值'],
  goal: '你現在是想選購商品，還是已經購買、需要儲值或使用協助？', goals: ['還沒買，幫我選', '已經購買，教我使用'],
  choose: '回覆序號或商品名即可。價格、庫存和資格請以商品詳情為準。', catalog: '這是目前公開目錄裡符合的商品：', more: '其餘選項可以在商品中心查看。',
  unavailable: '這次無法讀取商品目錄，暫時不能確認具體在售選項。可以稍後回覆「重新推薦」，或開啟商品中心；我仍可說明儲值與使用步驟。', empty: '目前讀取的目錄裡沒有符合的商品，不能據此保證有貨。請查看商品中心或聯絡人工確認。',
  purpose: { gpt: '為自己的 ChatGPT 帳號開通或續費，請看 ChatGPT 商品。不同方案、管道與開通/續費條件需分別核對。', codex: 'Codex 點數是額度補充，不等於 Plus / Pro 訂閱，也不延長訂閱時長。購買前須有有效訂閱並符合本商品的資格。', gemini: 'Gemini 商品按各自說明交付及兌換，帳號條件、活動紀錄和保障範圍可能不同，不能套用 GPT 步驟。' },
  progress: '你現在到哪一步了？', steps: ['還沒購買', '已付款，還沒拿到交付內容', '已拿到卡密或兌換連結', '兌換報錯 / 結果不對'],
  selected: '你選的是：', selectedHelp: '先開啟這款商品詳情，核對規格、適用帳號、交付方式和保障說明。選購不是資格審核，客服小窗不能保證帳號一定能開通。',
  rules: { activation: '這款屬於新開通類：帳號從未開通會員，或上次訂閱已到期 / 掉訂至少 10 分鐘；最終資格以兌換預檢為準。', renewal: '這款屬於 Pro 20x 續費：帳號目前須仍為 Pro 20x，綁卡後還需依商品說明聯絡人工確認。', credits: '這款只儲值 Codex 點數：須有有效訂閱並符合購買資格，不包含訂閱，也不延長訂閱時長。', ios: 'iOS 管道請使用訂單內專屬說明。此類儲值覆蓋目前方案，不累加原有剩餘時長；不要套用一般卡密流程。', gemini: 'Gemini 須核對是否使用過對應活動、帳號註冊時間和本款保障範圍；以這一款的詳情和交付說明為準。' },
  buy: '選購步驟：\n1. 在商品詳情確認方案/規格、帳號條件、交付方式及保障說明。\n2. 選好數量進入結帳，核對商品金額、手續費與實付合計。\n3. 按付款頁目前提供的管道付款；確認到帳後，去對應訂單查看交付內容。\n\n拿到卡密不等於帳號已開通。完成後可以接著問「怎麼兌換」。',
  delivery: '先找到下單時對應的訂單：\n• 登入購買：個人中心 → 我的訂單。\n• 訪客購買：使用下單信箱和查詢密碼進入訪客訂單查詢。\n\n核對實際付款狀態和交付內容；自動發貨、人工處理或預售以商品說明為準。已付款仍無交付內容，請聯絡人工提供訂單號，不要重複付款。這裡沒有讀取你的真實訂單狀態。',
  redeem: '拿到交付內容後：\n1. 開啟這筆已付款訂單的交付說明，使用其中對應商品的兌換入口。\n2. 核對目標帳號、商品條件，按頁面完成資格預檢。\n3. 依說明提交一次，再查詢這次處理結果；不要反覆提交。\n4. 回到對應服務，用剛儲值的同一帳號核對結果。\n\n具體入口只在付款後的訂單交付說明提供。',
  geminiUse: 'Gemini 請開啟這筆訂單附帶的兌換連結和說明，核對帳號及活動條件後完成兌換，再用同一帳號檢查訂閱。不要使用其他商品的卡密入口。', iosUnknown: '如果買的是 iOS 管道，請先告訴我完整商品名；它與一般卡密流程不同。',
  next: '可以接著告訴我「預檢不通過」「卡密無效」或「顯示成功但沒到帳」，我會繼續排查。',
  error: '先分清問題出現在哪一步：\n• 預檢不通過：核對本款商品的帳號條件，不要強行提交。\n• 卡密無效或已使用：核對訂單商品與卡密，查看是否已有處理紀錄。\n• 處理中或結果不確定：查詢原提交紀錄，暫時不要重複提交或購買。\n\n請提供具體報錯文字及商品名，不用傳送完整卡密、密碼或帳號令牌。重發、退款與人工補單需由人工核實。',
  result: '「訂單已付款」「卡密已發貨」和「兌換成功」是不同狀態。\n\n先檢查兌換處理結果，再確認登入的是儲值時的同一帳號。訂閱商品查看訂閱狀態；Codex 點數查看點數餘額，不要以方案名稱是否變化判斷到帳。仍不一致時，將商品名和不含敏感內容的報錯交給人工核實。這裡不能確認實際到帳情況。',
  use: '兌換成功後，用儲值時的同一帳號登入對應服務，核對這款商品帶來的訂閱或點數。卡密只用於兌換，不是登入密碼；不同服務的帳號和入口不能混用。\n\n如果卡在登入、資格檢查或權益沒有變化，告訴我具體步驟。',
  wallet: '商城錢包餘額與 GPT / Gemini 訂閱、Codex 點數不同。\n\n儲值商城餘額請進入個人中心 → 我的錢包，填寫金額並選擇目前支援的付款方式。餘額到帳後用於商城結帳，不會自動開通外部帳號的訂閱。',
  context: '我還記得你正在詢問{product}。你是想看購買步驟、兌換步驟，還是處理報錯？', contextSteps: ['購買步驟', '兌換步驟', '處理報錯'],
  unknown: '這版先協助選購商品、領取交付內容、儲值與使用步驟。可以說「我想買 GPT」或「拿到 Gemini 連結後怎麼用」；其他問題可聯絡人工。', thanks: '不客氣。後面卡在哪一步，直接接著告訴我即可。', human: '可以，下面是人工客服聯絡方式。請說明商品名和遇到的問題；訂單問題可提供訂單號。',
  catalogLink: '開啟商品中心', productLink: '查看商品詳情', orderLink: '我的訂單', guestLink: '訪客訂單查詢', walletLink: '開啟我的錢包',
}
const en: GuideCopy = {
  subtitle: 'Store assistant · Shopping & use', footer: 'Local trial · Product and order terms apply',
  start: 'I can walk you through it. Which do you need?', families: ['ChatGPT subscription', 'Codex credits', 'Gemini products', 'Store wallet top-up'],
  goal: 'Are you choosing a product, or do you already have a purchase to redeem or use?', goals: ['Help me choose', 'Help with my purchase'],
  choose: 'Reply with a number or product name. Check the product page for price, stock and eligibility.', catalog: 'These products match in the public catalog:', more: 'More options may be available in the product catalog.',
  unavailable: 'I could not load the product catalog, so I cannot confirm the available options. Try “recommend again” later or open the catalog. I can still explain redemption and use.', empty: 'No matching products were found in the catalog I loaded. This does not confirm availability; check the catalog or contact support.',
  purpose: { gpt: 'Choose a ChatGPT product to activate or renew your own account. Check each plan, channel and activation or renewal requirement separately.', codex: 'Codex credits add usage credits, not a Plus / Pro subscription or more subscription time. An eligible active subscription is required.', gemini: 'Gemini products have their own delivery, account, promotion and warranty requirements. Do not apply the GPT redemption steps.' },
  progress: 'Which stage are you at?', steps: ['Not purchased yet', 'Paid, but no delivery content', 'Have the code or redemption link', 'Redemption error / unexpected result'],
  selected: 'You selected:', selectedHelp: 'Open this product page and check the variant, eligible account, delivery and warranty terms. Choosing a product does not verify that your account can activate it.',
  rules: { activation: 'New activation: no previous membership, or a subscription that expired / ended at least 10 minutes ago. Eligibility is decided by the redemption check.', renewal: 'Pro 20x renewal: your account must currently have Pro 20x. Follow the product instructions and contact support after linking the card.', credits: 'Codex credits only: requires an eligible active subscription; no subscription or subscription extension is included.', ios: 'For the iOS channel, use the specific instructions in your order. It replaces the current plan and does not add remaining time. Do not use the ordinary code workflow.', gemini: 'Check prior promotion use, account age and this product’s warranty on its own product page and delivery instructions.' },
  buy: 'Buying steps:\n1. Check the plan/variant, account requirements, delivery and warranty on the product page.\n2. Choose a quantity and check the item amount, payment fee and final total at checkout.\n3. Pay using an available checkout method. After payment is confirmed, find the delivery content in that order.\n\nReceiving a code does not activate your account. Ask “how do I redeem it?” when ready.',
  delivery: 'Find the order made at checkout:\n• Signed-in purchase: Account → My orders.\n• Guest purchase: use the checkout email and lookup password in guest order lookup.\n\nCheck the actual payment and delivery status. Automatic, manual and presale delivery follow the product terms. If paid but still undelivered, contact support with the order number; do not pay again. I have not read your actual order status.',
  redeem: 'Once you have the delivery content:\n1. Open the paid order’s delivery instructions and use its product-specific redemption link.\n2. Check the target account and requirements, then complete the eligibility check.\n3. Submit once as instructed and check that submission’s result; avoid repeated submissions.\n4. Return to the service with the same account and verify the result.\n\nThe actual redemption link is supplied only in the paid order.',
  geminiUse: 'For Gemini, open the link and instructions supplied with this order. Check the account and promotion requirements, redeem as instructed, then check the subscription on the same account. Do not use another product’s code portal.', iosUnknown: 'If you bought the iOS channel, tell me the full product name first; its process differs from ordinary codes.',
  next: 'Tell me if eligibility failed, the code is invalid, or redemption succeeded but the balance/subscription has not changed.',
  error: 'Where did it fail?\n• Eligibility check: review this product’s account requirements before submitting.\n• Invalid or already-used code: check the order’s product/code and existing processing records.\n• Pending or uncertain result: check the original submission; avoid resubmitting or buying again.\n\nShare the error wording and product name, without the full code, password or account token. Reissue, refunds and manual fixes require human review.',
  result: '“Order paid”, “code delivered” and “redemption successful” are separate states.\n\nCheck the redemption result and confirm you are using the same account. For subscriptions, check the subscription; for Codex credits, check the credit balance rather than the plan name. If still inconsistent, contact support with the product and non-sensitive error details. I cannot confirm your actual credit or subscription status.',
  use: 'After successful redemption, sign in to the relevant service with the same account and check the subscription or credits from that product. A redemption code is not a login password; accounts and links for different services are not interchangeable.\n\nTell me whether you are stuck at login, eligibility or checking your benefits.',
  wallet: 'Store wallet balance is separate from GPT / Gemini subscriptions and Codex credits.\n\nTo add store balance, open Account → My wallet, enter an amount and choose an available payment method. The balance pays for store purchases; it does not activate an external subscription automatically.',
  context: 'We were discussing {product}. Do you need buying steps, redemption steps or help with an error?', contextSteps: ['Buying steps', 'Redemption steps', 'Help with an error'],
  unknown: 'This version helps with product choice, delivery, redemption and usage steps. Try “I want to buy GPT” or “How do I use my Gemini link?”. Contact human support for other questions.', thanks: 'You’re welcome. Tell me which step you get stuck on next.', human: 'Here are the human support contacts. Explain the product and problem; include the order number for an order issue.',
  catalogLink: 'Open product catalog', productLink: 'View this product', orderLink: 'My orders', guestLink: 'Guest order lookup', walletLink: 'Open my wallet',
}
export const getConversationCopy = (locale: string): GuideCopy => locale.toLowerCase().startsWith('en') ? en : /tw|hk|hant/i.test(locale) ? tw : cn
const normalize = (value: string) => value.normalize('NFKC').trim().toLowerCase()
const compact = (value: string) => normalize(value).replace(/[^\p{L}\p{N}]/gu, '')

export function detectSupportFamily(text: string): SupportFamily | undefined {
  const value = normalize(text)
  if (/codex|点数|點數|credits?/.test(value)) return 'codex'
  if (/gemini/.test(value)) return 'gemini'
  if (/gpt|\bplus\b|\bpro\s*(5|20)\s*x|ios/.test(value)) return 'gpt'
}

// Only public identifiers/names are carried into the conversation. Never copy
// product HTML, private instructions, prices or delivery codes into its context.
export function normalizeSupportProducts(rows: unknown, locale: string): SupportProduct[] {
  if (!Array.isArray(rows)) return []
  const result: SupportProduct[] = [], seen = new Set<string>()
  for (const item of rows) {
    if (!item || typeof item !== 'object' || typeof item.slug !== 'string' || !/^[a-z0-9][a-z0-9_-]{0,119}$/i.test(item.slug) || seen.has(item.slug) || item.is_active === false || item.is_active === 0) continue
    const title = typeof item.title === 'string' ? item.title : item.title?.[locale] || item.title?.['zh-CN'] || item.title?.['en-US']
    if (typeof title !== 'string' || !title.trim() || /https?:|(?:[a-z0-9-]+\.)+[a-z]{2,}|<|>|[\u0000-\u001f]/i.test(title)) continue
    const family = detectSupportFamily(Object.values(typeof item.title === 'object' && item.title ? item.title : { title }).filter(value => typeof value === 'string').join(' '))
    if (!family) continue
    seen.add(item.slug)
    result.push({ slug: item.slug, title: title.trim().slice(0, 160), family, delivery: item.fulfillment_type === 'auto' ? 'auto' : item.fulfillment_type === 'manual' ? 'manual' : 'unknown' })
  }
  return result
}

const activation = new Set(['qzweb-5', 'qzweb-6', 'qzweb-55', 'qzweb-56', 'qzweb-57', 'qzweb-58'])
const credits = new Set(['qzweb-73', 'qzweb-74', 'qzweb-75', 'qzweb-76', 'qzweb-77', 'qzweb-78'])
function productRule(product: SupportProduct, copy: GuideCopy): string {
  if (/\bios\b/i.test(product.title)) return copy.rules.ios
  if (activation.has(product.slug)) return copy.rules.activation
  if (product.slug === 'qzweb-7') return copy.rules.renewal
  if (credits.has(product.slug)) return copy.rules.credits
  if (['qzweb-50', 'qzweb-51', 'qzweb-52', 'qzweb-53'].includes(product.slug)) return copy.rules.gemini
  return ''
}

export function respondToSupport(input: string, previous: SupportConversation, products: SupportProduct[], locale: string, catalogAvailable = true): SupportReply | null {
  const copy = getConversationCopy(locale), question = normalize(input).slice(0, 500)
  let state: SupportConversation = { ...previous, choices: undefined }
  const catalogLink = { label: copy.catalogLink, to: '/products' }
  const productLinks = () => state.product ? [{ label: copy.productLink, to: `/products/${state.product.slug}` }] : [catalogLink]
  const reply = (text: string, links: SupportLink[] = [], choices?: Choice[]): SupportReply => ({ text: [text, choices?.map((choice, index) => `${index + 1}. ${choice.label}`).join('\n')].filter(Boolean).join('\n\n'), state: { ...state, choices }, links })
  const familyPrompt = () => reply(copy.start, [], copy.families.map((label, index) => ({ label, value: ['gpt','codex','gemini','wallet'][index]!, kind: 'family' })))
  const progress = (prefix = '') => reply([prefix, copy.progress].filter(Boolean).join('\n\n'), productLinks(), copy.steps.map((label, index) => ({ label, value: ['buy','delivery','redeem','error'][index]!, kind: 'step' })))
  const listProducts = (): SupportReply => {
    if (!state.family) return familyPrompt()
    if (!catalogAvailable) return reply(copy.unavailable, [catalogLink])
    const matches = products.filter(product => product.family === state.family)
    if (!matches.length) return reply(copy.empty, [catalogLink])
    return reply([copy.purpose[state.family], copy.catalog, copy.choose, matches.length > 8 ? copy.more : ''].filter(Boolean).join('\n\n'), [catalogLink], matches.slice(0, 8).map(product => ({ label: product.title, value: product.slug, kind: 'product' })))
  }
  const runStep = (step: string): SupportReply => {
    state.goal = step === 'buy' ? 'buy' : 'use'; state.step = step
    if (step === 'buy') return state.product ? reply([productRule(state.product, copy), copy.buy].filter(Boolean).join('\n\n'), productLinks()) : listProducts()
    if (step === 'delivery') return reply(copy.delivery, [{ label: copy.orderLink, to: '/me/orders' }, { label: copy.guestLink, to: '/guest/orders' }])
    if (step === 'error') return reply(copy.error, productLinks())
    if (step === 'result') return reply(copy.result, productLinks())
    if (step === 'use') return reply(copy.use, productLinks())
    if (!state.family) return familyPrompt()
    const ios = state.variant === 'ios' || /\bios\b/i.test(state.product?.title || question)
    return reply([ios ? copy.rules.ios : state.family === 'gemini' ? copy.geminiUse : copy.redeem, state.family === 'codex' ? copy.rules.credits : '', state.family === 'gpt' && !ios && !state.product ? copy.iosUnknown : '', copy.next].filter(Boolean).join('\n\n'), [{ label: copy.orderLink, to: '/me/orders' }, { label: copy.guestLink, to: '/guest/orders' }])
  }

  if (/联系人工|聯絡人工|转人工|轉人工|找人工|人工客服|^(人工|human)$|真人|human support|human agent|real person/.test(question)) return { ...reply(copy.human), handoff: true }
  // Other store topics continue to use their existing replies, with stale choices cleared.
  if (/返利|返傭|推广|推廣|佣金|referr|rewards?|开分店|開分店|分销|分銷|代理|resell|open.*store|退款|refund|查单|查單|订单.*状态|訂單.*狀態|order.*status|where.*order|怎么付款|怎麼付款|支付方式|付款方式|手续费|手續費|payment method|how.*pay|usdt|币安|幣安|欧易|歐易/.test(question)) return null
  if (/重新开始|重新開始|换个问题|換個問題|start over|new question/.test(question)) { state = {}; return familyPrompt() }
  if (/^(谢谢|謝謝|好的|好哒|明白了|thanks|thank you|ok|okay)[!！。. ]*$/.test(question)) return reply(copy.thanks)

  const number = question.match(/^(?:第)?([1-9]\d?)(?:个|個|项|項)?[.、\s]*$/)
  const choice = number ? previous.choices?.[Number(number[1]) - 1] : previous.choices?.find(item => compact(item.label) === compact(question))
  if (number && !choice) { state = {}; return familyPrompt() }
  if (choice?.kind === 'family') {
    if (choice.value === 'wallet') { state = {}; return reply(copy.wallet, [{ label: copy.walletLink, to: '/me/wallet' }]) }
    state.family = choice.value as SupportFamily; state.product = undefined; state.variant = undefined
    if (state.goal === 'use' && state.step) return runStep(state.step)
  } else if (choice?.kind === 'goal') state.goal = choice.value as 'buy' | 'use'
  else if (choice?.kind === 'step') return runStep(choice.value)
  else if (choice?.kind === 'product') {
    const product = products.find(item => item.slug === choice.value)
    if (!product) return reply(catalogAvailable ? copy.empty : copy.unavailable, [catalogLink])
    state.product = product; state.family = product.family; state.variant = /\bios\b/i.test(product.title) ? 'ios' : undefined
    return progress([`${copy.selected}${product.title}`, productRule(product, copy), copy.selectedHelp].filter(Boolean).join('\n\n'))
  }

  if (/钱包|錢包|商城余额|商城餘額|store balance|wallet/.test(question)) { state = {}; return reply(copy.wallet, [{ label: copy.walletLink, to: '/me/wallet' }]) }
  const explicitProduct = products.find(product => question === normalize(product.slug) || compact(question).includes(compact(product.title)))
  const family = explicitProduct?.family || detectSupportFamily(question)
  if (family && family !== state.family) { state = { family, goal: state.goal }; }
  if (explicitProduct) { state.product = explicitProduct; state.variant = /\bios\b/i.test(explicitProduct.title) ? 'ios' : undefined }
  else if (/\bios\b/.test(question)) state.variant = 'ios'
  else if (/普通|渠道一|渠道二|regular|channel\s*[12]/.test(question)) state.variant = undefined
  const notBought = /还没买|還沒買|没买|沒買|没有买|沒有買|未购买|未購買|没付款|沒付款|没有付款|沒有付款|not (bought|purchased)|haven.t (bought|paid)/.test(question)
  const buyIntent = notBought || /想买|想買|选购|選購|推荐|推薦|选哪|選哪|怎么买|怎麼買|购买步骤|購買步驟|购买建议|(?:有什么|有哪些|有啥).*(?:商品|套餐)|help.*choose|want.*buy|recommend|buying steps|how.*buy|what (products|plans)/.test(question)
  const useIntent = !notBought && /买了|買了|已购|已購|已经购买|已經購買|已付款|已经付款|已經付款|充值|儲值|兑换|兌換|卡密|拿到|收到|怎么用|怎麼用|使用|bought|purchased|redeem|top.?up|how.*use|my purchase|got.*code/.test(question)
  if (buyIntent) state.goal = 'buy'
  else if (useIntent) state.goal = 'use'

  if (/报错|報錯|无效|無效|已(?:经|經)?使用|用过了|用過了|不通过|不通過|不符合|失败|失敗|error|invalid|already.used|failed|ineligible/.test(question)) return runStep('error')
  if (/没到账|沒到帳|未到账|未到帳|权益.*没|權益.*沒|没变化|沒變化|not.*(credited|changed)|success.*but/.test(question)) return runStep('result')
  if (!notBought && /没收到|沒收到|没拿到|沒拿到|(?:没有|沒有).*(卡密|兑换码|兌換碼|交付)|未发货|未發貨|还没发|還沒發|no.*(code|delivery)|not.*deliver/.test(question)) return runStep('delivery')
  if (/拿到了|拿到.*(卡密|链接|連結)|收到.*(卡密|链接|連結)|有(?:了)?(?:卡密|兑换码|兌換碼)|怎么兑换|怎麼兌換|兑换步骤|兌換步驟|how.*redeem|redemption steps|have.*(code|link)|got.*(code|link)/.test(question)) return runStep('redeem')
  if (/兑换成功|兌換成功|成功了|已经开通|已經開通|怎么使用|怎麼使用|redemption succeeded|activated|how.*use/.test(question) && !/买了|買了|bought/.test(question)) return runStep('use')
  if (/^(下一步|接下来呢|接下來呢|然后呢|然後呢|继续|繼續|next|what next)[?？。 ]*$/.test(question)) return runStep(state.step === 'redeem' || state.step === 'error' ? 'result' : state.step === 'buy' ? 'delivery' : 'redeem')
  if (explicitProduct) return progress([`${copy.selected}${explicitProduct.title}`, productRule(explicitProduct, copy), copy.selectedHelp].filter(Boolean).join('\n\n'))
  if (choice || family || buyIntent || useIntent || /你好|您好|hello|help|开始|開始/.test(question)) {
    if (!state.family) return familyPrompt()
    if (state.goal === 'buy') return listProducts()
    if (state.goal === 'use') return progress(state.product ? `${copy.selected}${state.product.title}` : '')
    return reply(copy.goal, [], copy.goals.map((label, index) => ({ label, value: index === 0 ? 'buy' : 'use', kind: 'goal' })))
  }
  if (state.family && /这个|這個|它|资格|資格|条件|條件|续费|續費|時長|时长|多久|区别|區別|this|that|eligible|renew|duration|difference/.test(question)) {
    const label = state.product?.title || state.family.toUpperCase()
    return reply([state.product ? productRule(state.product, copy) : copy.purpose[state.family], copy.context.replace('{product}', label)].filter(Boolean).join('\n\n'), productLinks(), copy.contextSteps.map((text, index) => ({ label: text, value: ['buy','redeem','error'][index]!, kind: 'step' })))
  }
  state = {}
  return reply(copy.unknown, [catalogLink])
}
