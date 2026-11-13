const boolean = value => typeof value === 'boolean' ? value : null;
const currency = value => typeof value === 'string' && /^[A-Z]{3}$/i.test(value.trim()) ? value.trim().toUpperCase() : null;
const plan = value => typeof value === 'string' && value.trim() && value.length <= 80 ? value.trim() : null;
function planKey(value) {
  const key = (value || '').trim().toLowerCase().replace(/^chatgpt/,'').replace(/[\s_-]/g,'');
  return key === 'prolite' ? 'pro5x' : key === 'promax' ? 'pro50x' : key;
}
function compatiblePlans(first, second) {
  const a = planKey(first), b = planKey(second);
  return !a || !b || a === b || (a === 'pro' && /^pro(?:5|20|50)x$/.test(b)) || (b === 'pro' && /^pro(?:5|20|50)x$/.test(a));
}

function instant(value) {
  if (value === null || value === undefined) return null;
  const text = String(value).trim();
  if (/^\d{10}$|^\d{13}$/.test(text)) return Number(text) * (text.length === 10 ? 1000 : 1);
  if (!/^\d{4}-\d\d-\d\d[T ]\d\d:\d\d(?::\d\d(?:\.\d+)?)?(?:Z|[+-]\d\d:?\d\d)$/i.test(text)) return null;
  const ms = Date.parse(text.replace(' ', 'T'));
  return Number.isFinite(ms) ? ms : null;
}

function paymentDetails(payment) {
  if (!payment || typeof payment !== 'object') return {};
  const status = String(payment.status || '').trim().toLowerCase();
  if (status && !['paid','succeeded','success','completed','settled'].includes(status)) return {};
  return {paid_at:instant(payment.paidAt ?? payment.paid_at ?? payment.date), currency:currency(payment.currency), method:paymentMethod(payment.payment_method)};
}

// Only return the recognizable payment method and, when present, the last four digits.
const paymentNames = new Map(Object.entries({
  visa:'Visa', mastercard:'Mastercard', amex:'American Express', american_express:'American Express',
  unionpay:'银联', discover:'Discover', diners:'Diners Club', jcb:'JCB', card:'银行卡', credit_card:'信用卡',
  paypal:'PayPal', apple_pay:'Apple Pay', google_pay:'Google Pay', alipay:'支付宝', wechat_pay:'微信支付', link:'Link',
}));
function paymentMethod(value) {
  const name = [value?.brand, value?.type, value].filter(v => typeof v === 'string')
    .map(v => paymentNames.get(v.trim().toLowerCase().replace(/[ -]/g,'_'))).find(Boolean);
  if (!name) return null;
  const last = String(value?.last4 ?? value?.last_4 ?? '');
  return /^\d{4}$/.test(last) ? name + ' · ' + last : name;
}

export function preflightDetails(check, billing = {}) {
  const checkedPlan = plan(check.currentPlan) || plan(check.current_plan) || plan(check.plan);
  const planType = plan(billing.summary?.plan_type), subscriptionPlan = plan(billing.summary?.subscription_plan);
  const billedPlan = planKey(planType) === 'pro' && compatiblePlans(planType, subscriptionPlan) ? subscriptionPlan || planType : planType || subscriptionPlan;
  // A single login may have several workspaces. Do not mix a different plan's bill into this check.
  if (!compatiblePlans(checkedPlan,billedPlan)) billing = {};
  const summary = billing.summary || {};
  const checkedPayment = paymentDetails(check.last_payment);
  const billedPayment = paymentDetails(billing.last_payment);
  const precisePlan = planKey(checkedPlan) === 'pro' && /^pro(?:5|20|50)x$/.test(planKey(billedPlan));
  return {
    current_plan:precisePlan ? billedPlan : checkedPlan || billedPlan,
    active:boolean(check.subscription_has_active) ?? boolean(check.has_active_subscription) ?? boolean(summary.has_active_subscription),
    active_until:instant(check.subscription_active_until) ?? instant(summary.active_until) ?? instant(summary.expires_at),
    will_renew:boolean(check.subscription_will_renew) ?? boolean(summary.will_renew),
    last_paid_at:checkedPayment.paid_at ?? billedPayment.paid_at ?? null,
    settlement_currency:currency(check.settlement_currency) ?? currency(summary.settlement_currency) ??
      currency(summary.billing_currency) ?? checkedPayment.currency ?? billedPayment.currency ?? null,
    payment_method:paymentMethod(check.payment_method) ?? checkedPayment.method ?? paymentMethod(summary.payment_method) ??
      paymentMethod(billing.payment_method) ?? billedPayment.method ?? null,
  };
}

// Already-open clients from before the details panel render only blocking_reason.
// Build their read-only summary for this response only, never for history or lookup.
const beijing = new Intl.DateTimeFormat('sv-SE',{timeZone:'Asia/Shanghai',year:'numeric',month:'2-digit',
  day:'2-digit',hour:'2-digit',minute:'2-digit',second:'2-digit',hourCycle:'h23'});
export function legacyBlockedSummary(reason, account, details, now = Date.now()) {
  const missing='上游未提供';
  const date=value=>value==null?missing:beijing.format(value);
  const hours=details.active_until==null?null:Math.floor((details.active_until-now)/3600000);
  const left=hours===null?missing:details.active_until<=now?'已到期':hours<1?'不足 1 小时':Math.floor(hours/24)+' 天 '+hours%24+' 小时';
  return [reason,'账号 '+(account||missing),'当前套餐 '+(details.current_plan?.replace(/^pro[_ -]?(\d+)x$/i,'Pro $1x')||missing),
    '订阅状态 '+(details.active===null?missing:details.active?'有效':'未生效'),
    '套餐到期 '+date(details.active_until)+'（北京时间）','剩余时长 '+left,
    '自动续费 '+(details.will_renew===null?missing:details.will_renew?'已开启':'已关闭'),
    '最近付款 '+date(details.last_paid_at),'结算币种 '+(details.settlement_currency||missing),
    '支付方式 '+(details.payment_method||missing),'刷新页面后重新核验，可查看完整详情卡片。'].join(' · ');
}

// These are the supplier's ordinary GPT opening tiers. Its bare "pro" uses the
// 20x tier for purchase eligibility; the UI still displays the name it received.
const openingTiers = new Map([
  ['plus', 1], ['plushalf', 1], ['plusplan', 1], ['pro5x', 2],
  ['pro', 3], ['pro20x', 3], ['pro50x', 4],
]);

export function preflightRestriction(check, {product, plan:target, plan_flow}, details) {
  if (check.subscription_recovery_required === true || (product === 'gpt' && check.subscription_is_delinquent === true)) return 'subscription';
  if (check.can_redeem === false || (typeof check.blocking_reason === 'string' ? check.blocking_reason.trim() : check.blocking_reason)) return 'unavailable';
  if (product !== 'gpt' || details.active === false) return null;

  const key = planKey(target);
  const flow = (plan_flow || '').trim().toLowerCase();
  const renewal = key === 'pro5xrenew' || key === 'pro20xrenew';
  if (flow === 'card_attach' || (!flow && renewal)) return null;
  const targetTier = openingTiers.get(renewal ? key.replace(/renew$/,'') : key);
  if (!targetTier) return null;
  if (!details.current_plan) return 'unverified_plan';
  const currentTier = openingTiers.get(planKey(details.current_plan)) ?? 0;
  if (currentTier >= targetTier) return 'plan_tier';
  return null;
}
