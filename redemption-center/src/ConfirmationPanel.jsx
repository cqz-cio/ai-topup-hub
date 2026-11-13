import React, { useEffect, useState } from 'react';
import { AlertCircle, ArrowLeft, ArrowRight, Info, ShieldCheck, UserRound } from 'lucide-react';
import { productLabel } from './api.js';
import './confirmation.css';

const beijing = new Intl.DateTimeFormat('sv-SE', {
  timeZone:'Asia/Shanghai', year:'numeric', month:'2-digit', day:'2-digit',
  hour:'2-digit', minute:'2-digit', second:'2-digit', hourCycle:'h23',
});
const currencyNames = typeof Intl.DisplayNames === 'function' ? new Intl.DisplayNames(['zh-CN'],{type:'currency',fallback:'code'}) : null;
const gptPlans = {
  free:'免费版', plus:'Plus', pro:'Pro', team:'Team',
  pro5x:'Pro 5x', prolite:'Pro 5x', pro20x:'Pro 20x', pro50x:'Pro 50x', promax:'Pro 50x',
  pro5xrenew:'Pro 5x 续费', pro20xrenew:'Pro 20x 续费',
};
function planLabel(value, product) {
  if (!value || product !== 'gpt') return value;
  const key = value.trim().toLowerCase().replace(/^chatgpt/,'').replace(/[\s_-]+/g,'');
  return Object.hasOwn(gptPlans,key) ? gptPlans[key] : value;
}
function currencyLabel(code) {
  if (!/^[A-Z]{3}$/.test(code || '')) return null;
  const name = currencyNames?.of(code);
  return name && name !== code ? code + ' · ' + name : code;
}
function remaining(until, now) {
  if (until <= now) return '已到所示截止时间';
  const minutes = Math.floor((until - now) / 60000);
  if (!minutes) return '不足 1 分钟';
  const hours = Math.floor(minutes / 60);
  if (hours >= 24) return Math.floor(hours / 24) + ' 天 ' + hours % 24 + ' 小时';
  return hours ? hours + ' 小时 ' + minutes % 60 + ' 分钟' : minutes + ' 分钟';
}
function Detail({label,value,missing}) {
  return <div><dt>{label}</dt><dd className={value == null ? 'pf-missing' : undefined}>{value ?? missing}</dd></div>;
}

export default function ConfirmationPanel({attempt,confirmed,setConfirmed,busy,onBack,onSubmit,onRefresh,error}) {
  const details = attempt.preflight_details;
  const active = details?.active ?? attempt.has_active_subscription;
  const until = details?.active_until;
  const [now,setNow] = useState(Date.now);
  useEffect(() => {
    if (until == null) return;
    const timer = setInterval(() => setNow(Date.now()), 60000);
    return () => clearInterval(timer);
  }, [until]);

  const missing = details ? '上游未提供' : '需重新核验';
  const blocked = attempt.state === 'blocked' || attempt.blocked === true;
  const subscriptionOnly = attempt.subscription_only === true;
  const unavailable = blocked || subscriptionOnly;
  const planMismatch = unavailable && attempt.plan_mismatch === true;
  const needsRecheck = subscriptionOnly && !planMismatch;
  const currentPlan = planLabel(details?.current_plan ?? attempt.current_plan,attempt.product);
  const target = planLabel(attempt.plan, attempt.product);
  const targetLabel = attempt.product === 'gpt' && target ? 'ChatGPT ' + target : productLabel(attempt.product,target);
  const stateLabel = active === true ? '订阅有效' : active === false ? '无有效订阅' : '订阅状态未知';
  let noticeTitle = '本次暂不可充值';
  let noticeText = attempt.blocking_reason || '请更换账号或重新核验。';
  if (planMismatch) {
    noticeTitle = '当前账号不符合这张卡的充值条件';
    noticeText = `当前账号的 ${currentPlan} 订阅仍在有效期内。请核对账号，换用可开通 ${targetLabel} 的账号后再充值。`;
  } else if (needsRecheck) {
    noticeTitle = '暂时无法确认充值资格';
    noticeText = '请核对充值账号，稍后重新核验。';
  }
  return <>
    <div className="kicker"><ShieldCheck size={16}/>{subscriptionOnly ? '账号核验' : '最后确认'}</div>
    <h2>{subscriptionOnly ? '账号订阅信息' : '请确认本次充值信息'}</h2>
    <p className="intro pf-intro">{subscriptionOnly ? '以下是本次账单查询返回的资料。' : '核对目标账号、当前订阅与账单信息后，再决定是否充值。'}</p>
    <div className="pf-account">
      <span className="pf-avatar"><UserRound size={20}/></span>
      <div className="pf-account-name"><span>充值账号</span><strong>{attempt.email || '未识别'}</strong></div>
      <span className={'pf-state' + (active === true ? ' pf-active' : '')}>{stateLabel}</span>
    </div>
    <div className="pf-plans">
      <div><span>充值套餐</span><strong>{targetLabel}</strong></div>
      <div><span>当前套餐</span><strong>{currentPlan || missing}</strong></div>
    </div>
    <section className="pf-block">
      <div className="pf-block-head"><h3>订阅详情</h3><span>北京时间 · UTC+8</span></div>
      <dl className="pf-data">
        <Detail label="套餐到期" missing={missing} value={until == null ? null : beijing.format(until)}/>
        <Detail label="剩余时长" missing={missing} value={until == null ? null : remaining(until,now)}/>
        <Detail label="自动续费" missing={missing} value={details?.will_renew === true ? '已开启' : details?.will_renew === false ? '已关闭' : null}/>
      </dl>
    </section>
    <section className="pf-block">
      <div className="pf-block-head"><h3>账单信息</h3></div>
      <dl className="pf-data">
        <Detail label="最近付款时间" missing={missing} value={details?.last_paid_at == null ? null : beijing.format(details.last_paid_at)}/>
        <Detail label="结算币种" missing={missing} value={currencyLabel(details?.settlement_currency)}/>
        <Detail label="支付方式" missing={missing} value={details?.payment_method}/>
      </dl>
    </section>
    <div className="pf-card-line"><span>兑换卡密</span><strong>{attempt.code}</strong></div>
    {unavailable ? <div className="pf-notice pf-blocked" role="status"><AlertCircle size={17}/><div>
      <strong>{noticeTitle}</strong><p>{noticeText}</p>
    </div></div> : active === true && <div className="pf-notice" role="note"><Info size={17}/><div>
      <strong>当前账号已有有效订阅</strong><p>请核对上方的当前套餐与充值套餐，再确认是否继续。</p>
    </div></div>}
    {error && <p role="alert" className="error"><AlertCircle size={16}/>{error}</p>}
    {!unavailable && <label className="check-label"><input type="checkbox" checked={confirmed} onChange={e=>setConfirmed(e.target.checked)} disabled={busy}/>我已核对以上账号与套餐，并确认充值</label>}
    <div className="flow-actions pf-actions">
      <button className="text-button" disabled={busy} onClick={onBack}><ArrowLeft size={16}/>{needsRecheck ? '重新核验账号' : blocked && !details ? '重新核验订阅信息' : '更换账号'}</button>
      <button className="primary" disabled={unavailable || !confirmed || busy || !attempt.confirmation} onClick={onSubmit}>
        {needsRecheck ? '待重新核验' : unavailable ? '本次不可充值' : busy ? '正在提交…' : '确认充值'}{!unavailable && <ArrowRight size={17}/>}
      </button>
    </div>
    {error && <button className="text-button" onClick={onRefresh}>查询这笔兑换的状态</button>}
  </>;
}
