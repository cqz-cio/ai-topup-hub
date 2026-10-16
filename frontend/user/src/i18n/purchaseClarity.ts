import type { PurchaseRule } from '../utils/productPurchaseGuide'

type Copy = {
  title: string; help: string; codeDelivery: string; creditsDelivery: string; linkDelivery: string
  rules: Record<PurchaseRule, string>
  subtotal: string; fee: string; total: string; wallet: string; online: string
  choose: string; updating: string; channel: string; usdt: string; final: string
}

export const purchaseClarityCopy: Record<string, Copy> = {
  'zh-CN': {
    title: '购买前请确认', help: '不确定账号是否符合条件，请先联系微信 Cqz0924ttxk 核实再购买。',
    codeDelivery: '付款后在订单交付说明中领取兑换码和兑换入口，再按提示预检、提交并查询结果。收到卡密不代表账号已开通。',
    creditsDelivery: '付款后在订单交付说明中领取兑换码和兑换入口，再完成资格预检、提交并查询结果。收到兑换码不代表点数已到账。',
    linkDelivery: '付款后在订单中领取本商品的兑换链接，使用自己的账号兑换，无需提供密码。',
    rules: {
      activation: '适用账号：从未开通过会员，或上次订阅已到期 / 掉订至少 10 分钟。最终资格以兑换预检为准。',
      renewal: '续费条件：账号当前仍为 Pro 20x。请勿按新开通商品购买；绑卡后需联系客服确认。',
      go: '这是 ChatGPT Go 兑换码。账号资格、套餐权益与实际有效期以兑换预检和兑换结果为准。',
      credits: '须已有有效订阅，并符合点数购买资格。仅充值 Codex 点数，不包含 Plus / Pro 订阅，也不延长订阅时长。',
      jio: '已使用过 Jio 活动的账号无法再次使用，请勿购买。',
      accountAge: '注册不满 7 天的新账号请勿购买。',
      antigravity: 'Antigravity 能否使用取决于账号本身，购买订阅不保证解锁该功能。',
      subscriptionWarranty: '质保期间掉订：按剩余天数退款或按商品说明补开，具体处理请联系客服核实。',
      activationOnly: '仅保激活成功及激活时 18 个月时长；不承诺激活后的持续有效期。个人操作导致掉权不补不换。',
      geminiWarranty: '保激活成功及激活时 18 个月时长；正常使用一个月内掉订全额退款。共享、违规等个人操作导致掉权不补不换。',
      presale: '这是预售商品，购买后需联系客服安排发货，原商品说明为当天内可发。',
    },
    subtotal: '商品小计（优惠后）', fee: '支付手续费', total: '实付合计', wallet: '其中余额支付', online: '还需在线支付',
    choose: '选择支付方式后显示含手续费的合计', updating: '正在更新金额…', channel: '支付方式',
    usdt: '这里显示人民币订单金额。提交后请按付款页的网络及完整 USDT 数量转账，链上转账手续费另计。',
    final: '金额会随优惠和支付方式更新，请在付款页核对最终应付金额。',
  },
  'zh-TW': {
    title: '購買前請確認', help: '不確定帳號是否符合條件，請先聯絡微信 Cqz0924ttxk 核實再購買。',
    codeDelivery: '付款後在訂單交付說明中領取兌換碼和兌換入口，再依提示預檢、提交並查詢結果。收到卡密不代表帳號已開通。',
    creditsDelivery: '付款後在訂單交付說明中領取兌換碼和兌換入口，再完成資格預檢、提交並查詢結果。收到兌換碼不代表點數已到帳。',
    linkDelivery: '付款後在訂單中領取本商品的兌換連結，使用自己的帳號兌換，無需提供密碼。',
    rules: {
      activation: '適用帳號：從未開通會員，或上次訂閱已到期 / 掉訂至少 10 分鐘。最終資格以兌換預檢為準。',
      renewal: '續費條件：帳號目前仍為 Pro 20x。請勿按新開通商品購買；綁卡後需聯絡客服確認。',
      go: '這是 ChatGPT Go 兌換碼。帳號資格、方案權益與實際有效期以兌換預檢和兌換結果為準。',
      credits: '須已有有效訂閱，並符合點數購買資格。僅儲值 Codex 點數，不包含 Plus / Pro 訂閱，也不延長訂閱時長。',
      jio: '已使用過 Jio 活動的帳號無法再次使用，請勿購買。', accountAge: '註冊未滿 7 天的新帳號請勿購買。',
      antigravity: 'Antigravity 能否使用取決於帳號本身，購買訂閱不保證解鎖此功能。',
      subscriptionWarranty: '質保期間掉訂：按剩餘天數退款或按商品說明補開，具體處理請聯絡客服核實。',
      activationOnly: '僅保啟用成功及啟用時 18 個月時長；不承諾啟用後的持續有效期。個人操作導致掉權不補不換。',
      geminiWarranty: '保啟用成功及啟用時 18 個月時長；正常使用一個月內掉訂全額退款。共享、違規等個人操作導致掉權不補不換。',
      presale: '這是預售商品，購買後需聯絡客服安排發貨，原商品說明為當天內可發。',
    },
    subtotal: '商品小計（優惠後）', fee: '支付手續費', total: '實付合計', wallet: '其中餘額支付', online: '仍需線上支付',
    choose: '選擇支付方式後顯示含手續費的合計', updating: '正在更新金額…', channel: '支付方式',
    usdt: '此處顯示人民幣訂單金額。提交後請依付款頁的網路及完整 USDT 數量轉帳，鏈上轉帳手續費另計。',
    final: '金額會隨優惠和支付方式更新，請在付款頁核對最終應付金額。',
  },
  'en-US': {
    title: 'Check before buying', help: 'Unsure about eligibility? Check with support on WeChat Cqz0924ttxk before buying.',
    codeDelivery: 'Find the code and redemption link in your order delivery instructions after payment. Follow the eligibility check, submit and check the result on the redemption page. Code delivery does not activate your account.',
    creditsDelivery: 'Find the code and redemption link in your order delivery instructions after payment. Complete the eligibility check, submit and check the result on the redemption page. Code delivery does not mean credits have been added.',
    linkDelivery: 'Find this product’s redemption link in your order after payment. Redeem with your own account; no password is required.',
    rules: {
      activation: 'For accounts with no previous membership, or a subscription that expired / ended at least 10 minutes ago. Eligibility is confirmed during redemption.',
      renewal: 'Renewal requires an account currently on Pro 20x. This is not a new activation. Contact support after linking the card.',
      go: 'This is a ChatGPT Go redemption code. Eligibility, benefits and duration depend on the eligibility check and redemption result.',
      credits: 'Requires an active subscription and eligibility to purchase credits. Codex credits only; no Plus / Pro subscription or subscription extension.',
      jio: 'Accounts that previously used the Jio offer are ineligible. Do not purchase for these accounts.',
      accountAge: 'Do not purchase for accounts registered less than 7 days ago.',
      antigravity: 'Antigravity availability depends on your account. This subscription does not guarantee access.',
      subscriptionWarranty: 'If the subscription ends during its warranty, contact support for a remaining-days refund or reactivation under the product terms.',
      activationOnly: 'Covers successful activation with 18 months at activation; no ongoing validity guarantee. Loss caused by personal account actions is excluded.',
      geminiWarranty: 'Covers activation with 18 months at activation. Full refund if lost within one month of normal use; sharing, violations and other personal actions are excluded.',
      presale: 'Presale: contact support after purchase to arrange delivery. The product terms state delivery within the day.',
    },
    subtotal: 'Items after discounts', fee: 'Payment fee', total: 'Total payable', wallet: 'Paid from balance', online: 'Online payment due',
    choose: 'Choose a payment method to see the total including fees', updating: 'Updating amount…', channel: 'Payment method',
    usdt: 'This is the order value in CNY. After submitting, use the network and exact USDT amount on the payment page. Network transfer fees are separate.',
    final: 'Discounts and payment method can change the total. Confirm the final amount on the payment page.',
  },
}
