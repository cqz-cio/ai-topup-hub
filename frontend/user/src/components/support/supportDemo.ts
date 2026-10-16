const zhCN = {
  name: '小π客服', subtitle: '演示助手 · 选购与使用指引', title: '遇到问题？小π在。', intro: '选购、充值、付款，陪你一步步来。',
  topics: ['不会充值', '怎么付款', '了解返利', '开分店'], hints: ['领取卡密与兑换', '渠道与操作指引', '申请与结算规则', '合作方式与咨询'],
  questions: ['买了 GPT，怎么充值？', '有哪些支付方式？', '返利规则在哪里看？', '我想了解开分店。'],
  answers: [
    '先在订单中领取卡密，再按该订单的交付说明打开兑换入口。\n\n不同商品的账号条件、充值和续费步骤可能不同，请先核对商品说明。具体入口以付款后的订单交付说明为准。',
    '付款页会列出当前可用渠道及实付金额。\n\n选择 U 支付时，请先核对网络，再按页面显示的精确金额操作；转账或提现手续费另付。微信支付请以付款页显示的手续费和合计为准。',
    '可从个人中心的推广返利入口查看规则与记录。\n\n返利比例、参加条件、确认时间和可结算金额，以页面当前规则及记录为准。演示助手不会计算或承诺返利。',
    '开分店可从个人中心了解分销申请，或先联系人工咨询。\n\n开通条件、费用、域名配置和维护方式，需要确认当前合作方案；提交申请后仍需审核。',
  ],
  unknown: '这个问题暂时不在演示问答范围内。你可以选择上方的充值、付款、返利或开分店问题，也可以联系人工确认。',
  order: '演示助手不能读取或处理你的订单。请通过商城的订单查询入口查看状态，或联系人工并提供订单号；不要发送账号密码。',
  placeholder: '问问小π…', send: '发送', thinking: '小π正在整理…', footer: '演示问答 · 具体规则以商品及订单说明为准', human: '联系人工',
  open: '打开小π客服', close: '收起客服小窗', greeting: '嗨！有问题找小π。', think: '让我想一想…', reply: '给你整理好啦。', idle: '随时叫我，小π在。',
  wechat: '微信', qq: 'QQ', telegram: 'Telegram', copy: '复制', copied: '已复制', copyFailed: '请选中联系方式复制', contactTitle: '人工客服', closeContacts: '关闭联系方式', newQuestion: '选择问题',
}
const zhTW: typeof zhCN = {
  name: '小π客服', subtitle: '示範助手 · 選購與使用指引', title: '遇到問題？小π在。', intro: '選購、儲值、付款，陪你一步步來。',
  topics: ['不會儲值', '怎麼付款', '了解返利', '開分店'], hints: ['領取卡密與兌換', '管道與操作指引', '申請與結算規則', '合作方式與諮詢'],
  questions: ['買了 GPT，怎麼儲值？', '有哪些付款方式？', '返利規則在哪裡看？', '我想了解開分店。'],
  answers: [
    '先在訂單中領取卡密，再依該筆訂單的交付說明開啟兌換入口。\n\n不同商品的帳號條件、儲值及續費步驟可能不同，請先核對商品說明。具體入口以付款後的訂單交付說明為準。',
    '付款頁會列出目前可用管道及實付金額。\n\n選擇 U 付款時，請先核對網路，再按頁面顯示的精確金額操作；轉帳或提領手續費另付。微信付款以付款頁顯示的手續費和合計為準。',
    '可從個人中心的推廣返利入口查看規則與紀錄。\n\n返利比例、參加條件、確認時間和可結算金額，以頁面目前規則及紀錄為準。示範助手不會計算或承諾返利。',
    '開分店可從個人中心了解分銷申請，或先聯絡人工諮詢。\n\n開通條件、費用、網域設定和維護方式，需確認目前合作方案；提交申請後仍需審核。',
  ],
  unknown: '這個問題暫時不在示範問答範圍內。可以選擇儲值、付款、返利或開分店問題，也可以聯絡人工確認。',
  order: '示範助手不能讀取或處理你的訂單。請透過商城的訂單查詢入口查看狀態，或聯絡人工並提供訂單號；不要傳送帳號密碼。',
  placeholder: '問問小π…', send: '傳送', thinking: '小π正在整理…', footer: '示範問答 · 具體規則以商品及訂單說明為準', human: '聯絡人工',
  open: '開啟小π客服', close: '收起客服小窗', greeting: '嗨！有問題找小π。', think: '讓我想一想…', reply: '幫你整理好了。', idle: '隨時叫我，小π在。',
  wechat: '微信', qq: 'QQ', telegram: 'Telegram', copy: '複製', copied: '已複製', copyFailed: '請選取聯絡方式複製', contactTitle: '人工客服', closeContacts: '關閉聯絡方式', newQuestion: '選擇問題',
}
const en: typeof zhCN = {
  name: 'Pi Support', subtitle: 'Demo assistant · Shopping & help', title: 'Need a hand? Pi is here.', intro: 'Guidance for shopping, redeeming and payments.',
  topics: ['Redeem a code', 'How to pay', 'Referral rewards', 'Open a store'], hints: ['Find your code and instructions', 'Methods and payment steps', 'Eligibility and settlement', 'Applications and support'],
  questions: ['How do I redeem my GPT purchase?', 'Which payment methods can I use?', 'Where can I find the referral rules?', 'How can I open a store?'],
  answers: [
    'Find your code in your order and follow that order’s delivery instructions.\n\nAccount eligibility and redemption or renewal steps vary by product. Read the product instructions first. The redemption link is provided with your paid order.',
    'The payment page lists the currently available methods and the final amount.\n\nFor U payments, check the network and send the exact amount shown; transfer or withdrawal fees are extra. For WeChat, use the fee and total shown at checkout.',
    'Open referral rewards in your account to review the rules and records.\n\nThe current page defines eligibility, rates, confirmation times and available rewards. This demo does not calculate or promise rewards.',
    'Check the reseller application in your account, or contact support about opening a store.\n\nEligibility, fees, domain setup and maintenance depend on the available arrangement. Applications still require review.',
  ],
  unknown: 'This question is outside the demo answers. Choose a topic about redemption, payments, rewards or opening a store, or contact a person for help.',
  order: 'This demo cannot read or process your order. Use the store’s order lookup, or contact support with the order number. Do not send account passwords.',
  placeholder: 'Ask Pi…', send: 'Send', thinking: 'Pi is putting this together…', footer: 'Demo answers · Product and order terms apply', human: 'Contact support',
  open: 'Open Pi support', close: 'Minimize support', greeting: 'Hi! Ask Pi for help.', think: 'Let me think…', reply: 'Here you go.', idle: 'Pi is here to help.',
  wechat: 'WeChat', qq: 'QQ', telegram: 'Telegram', copy: 'Copy', copied: 'Copied', copyFailed: 'Select the contact to copy it', contactTitle: 'Contact a person', closeContacts: 'Close contacts', newQuestion: 'Choose a topic',
}
export const supportContacts = { wechat: 'Cqz0924ttxk', qq: '1669268759', telegram: '@czq_piestar', telegramUrl: 'https://t.me/czq_piestar' } as const
export const getSupportCopy = (locale: string) => locale.toLowerCase().startsWith('en') ? en : /tw|hk|hant/i.test(locale) ? zhTW : zhCN

export type DemoIntent = 'redeem' | 'payment' | 'rewards' | 'store' | 'order' | 'unknown'
export function detectDemoIntent(input: string): DemoIntent {
  const question = input.normalize('NFKC').trim().slice(0, 500)
  if (/查单|查單|订单.*状态|訂單.*狀態|没收到|沒收到|退款|refund|order.*status|where.*order/i.test(question)) return 'order'
  if (/开分店|開分店|分销|分銷|代理|合作|resell|open.*store/i.test(question)) return 'store'
  if (/返利|推广|推廣|佣金|referr|reward/i.test(question)) return 'rewards'
  if (/付款|支付|手续费|手續費|usdt|币安|幣安|欧易|歐易|pay|fee/i.test(question)) return 'payment'
  if (/充值|儲值|兑换|兌換|卡密|续费|續費|gpt|gemini|codex|redeem|renew/i.test(question)) return 'redeem'
  return 'unknown'
}
export function getDemoAnswer(input: string, locale: string): string {
  const copy = getSupportCopy(locale), intent = detectDemoIntent(input)
  const index = ['redeem', 'payment', 'rewards', 'store'].indexOf(intent)
  return index >= 0 ? copy.answers[index]! : intent === 'order' ? copy.order : copy.unknown
}
