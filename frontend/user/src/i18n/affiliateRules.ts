type Copy = {
  title: string; rate: string; currentRate: string; zeroRate: string; calculation: string; timing: string
  formula: (rate: string) => string; paidAfter: (days: number) => string
  noWait: string; credited: string; note: string; loadFailed: string; retry: string; closed: string
  conditionsTitle: string; eligibility: string; exclusions: string; settlement: string; withdrawal: string
}

export const affiliateRulesCopy: Record<string, Copy> = {
  'zh-CN': {
    title: '返利规则', rate: '返多少', currentRate: '当前返利比例', zeroRate: '当前比例为 0%，暂不产生返利',
    calculation: '怎么算', timing: '多久到账', formula: (rate) => `符合条件的商品本金 × ${rate}%`,
    paidAfter: (days) => `付款后 ${days} 天`, noWait: '付款后结算', credited: '确认后自动转入「我的钱包」',
    note: '按人民币计算，四舍五入保留两位小数；支付手续费不计入返利。',
    loadFailed: '暂时无法读取返利规则，请刷新后重试。', retry: '刷新规则', closed: '推广返利暂未开放，已有佣金以实际记录为准。',
    conditionsTitle: '参与条件与到账说明', eligibility: '仅已开启返利的商品，且整单按原价成交时参与。',
    exclusions: '享受个人专属价、会员价、活动价、批发价或优惠券的订单不参与。自购及分销站订单不返利；使用钱包付款不影响符合条件的原价订单。',
    settlement: '返利先记为待确认佣金，确认期结束后转入钱包；无确认等待期时，付款后结算。退款会相应扣减返利。',
    withdrawal: '钱包到账后可消费或申请提现，提现需人工审核和打款。',
  },
  'zh-TW': {
    title: '返利規則', rate: '返多少', currentRate: '目前返利比例', zeroRate: '目前比例為 0%，暫不產生返利',
    calculation: '怎麼算', timing: '多久到帳', formula: (rate) => `符合條件的商品本金 × ${rate}%`,
    paidAfter: (days) => `付款後 ${days} 天`, noWait: '付款後結算', credited: '確認後自動轉入「我的錢包」',
    note: '以人民幣計算，四捨五入保留兩位小數；支付手續費不計入返利。',
    loadFailed: '暫時無法讀取返利規則，請重新整理後重試。', retry: '重新整理規則', closed: '推廣返利暫未開放，已有佣金以實際紀錄為準。',
    conditionsTitle: '參與條件與到帳說明', eligibility: '僅已開啟返利的商品，且整筆訂單按原價成交時參與。',
    exclusions: '享有個人專屬價、會員價、活動價、批發價或優惠券的訂單不參與。自購及分銷站訂單不返利；使用錢包付款不影響符合條件的原價訂單。',
    settlement: '返利先記為待確認佣金，確認期結束後轉入錢包；無確認等待期時，付款後結算。退款會相應扣減返利。',
    withdrawal: '錢包到帳後可消費或申請提現，提現須人工審核和打款。',
  },
  'en-US': {
    title: 'Commission rules', rate: 'Commission rate', currentRate: 'Current commission rate', zeroRate: 'The current rate is 0%; no commission is earned',
    calculation: 'How it is calculated', timing: 'When it arrives', formula: (rate) => `Eligible product principal × ${rate}%`,
    paidAfter: (days) => `${days} days after payment`, noWait: 'Settled after payment', credited: 'Credited to My Wallet after confirmation',
    note: 'Calculated in CNY and rounded to two decimal places. Payment fees are excluded.',
    loadFailed: 'Commission rules could not be loaded. Please refresh and try again.', retry: 'Refresh rules', closed: 'Affiliate commissions are currently unavailable. Existing commissions remain in your records.',
    conditionsTitle: 'Eligibility and settlement', eligibility: 'Only enabled products in orders paid entirely at full price qualify.',
    exclusions: 'Personal, member, promotion, wholesale and coupon pricing disqualify the entire order. Self-purchases and reseller orders are excluded. Wallet payment does not disqualify an eligible full-price order.',
    settlement: 'Commission remains pending until the confirmation period ends, then moves to your wallet. With no waiting period, it is settled after payment. Refunds reduce the commission accordingly.',
    withdrawal: 'Wallet earnings can be spent or submitted for withdrawal. Withdrawals require manual review and payment.',
  },
}
