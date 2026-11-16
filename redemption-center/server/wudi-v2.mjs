import { Problem } from './security.mjs';
import { WudiPartner, linkStates } from './wudi.mjs';

const operations=new Set(['preview','account-check','account-check/status','redeem','status','lookup','orders','subscriptions/query','subscriptions/status']);
const identifier=/^[A-Za-z0-9][A-Za-z0-9_.:-]{0,255}$/;
const text=(v,n=2000)=>typeof v==='string'?v.slice(0,n):null;
const bool=v=>typeof v==='boolean'?v:null;
const date=v=>typeof v==='string'&&Number.isFinite(Date.parse(v))?v:null;
const errorCode=v=>typeof v==='string'&&/^[A-Z][A-Z0-9_]{0,79}$/.test(v)?v:null;
const validID=v=>{if(typeof v!=='string'||!identifier.test(v))throw new Problem('UPSTREAM_RESPONSE','兑换服务返回异常，请继续查询原订单。',502);return v;};

export function sessionObject(value) {
  let object;try{object=typeof value==='string'?JSON.parse(value):structuredClone(value);}catch{throw new Problem('SESSION_FORMAT_INVALID','请粘贴完整 Session JSON 对象。');}
  if(!object||typeof object!=='object'||Array.isArray(object)||Buffer.byteLength(JSON.stringify(object))>190000)throw new Problem('SESSION_FORMAT_INVALID','请粘贴完整 Session JSON 对象，大小不超过 190 KB。');
  let fields=0;
  const count=(o,depth=0)=>{if(depth>64)throw new Problem('SESSION_FORMAT_INVALID','Session JSON 嵌套过深。');if(o&&typeof o==='object')for(const [key,v] of Object.entries(o)){if(++fields>8192||['__proto__','prototype','constructor'].includes(key))throw new Problem('SESSION_FORMAT_INVALID','Session JSON 字段不符合要求。');count(v,depth+1);}};
  count(object);return object;
}
export function subscription(value) {
  if(!value||typeof value!=='object')return null;
  return {email:text(value.email,254),userId:text(value.userId,256),queriedAt:date(value.queriedAt),accountCount:Number.isSafeInteger(value.accountCount)?value.accountCount:null,
    accounts:Array.isArray(value.accounts)?value.accounts.slice(0,512).map(a=>{
      const result={};for(const k of ['accountId','plan','planType','subscriptionPlan','subscriptionStatus','source','currency','campaignId','cancellation'])result[k]=text(a?.[k],300);
      for(const k of ['selected','active','willRenew','plusPromoEligible','pro20xEligible'])result[k]=bool(a?.[k]);
      for(const k of ['startedAt','expiresAt'])result[k]=date(a?.[k]);return result;
    }):[]};
}
function card(v) {return v&&typeof v==='object'?{plan_name:text(v.plan_name,200),region:text(v.region,40),region_name:text(v.region_name,100),require_plus_promo:bool(v.require_plus_promo)}:null;}
function order(v) {
  if(!v||typeof v!=='object'||typeof v.status!=='string')throw new Problem('UPSTREAM_RESPONSE','兑换服务返回异常，请继续查询原订单。',502);
  const result={id:validID(v.id),status:linkStates.has(v.status)?v.status:'uncertain',message:text(v.message),error_code:errorCode(v.error_code),preflight_error_code:errorCode(v.preflight_error_code),preflight_message:text(v.preflight_message),card_reenabled:v.status==='failed'&&v.card_reenabled===true};
  if(v.card)result.card=card(v.card);
  for(const k of ['created_at','updated_at','expires_at'])if(v[k])result[k]=date(v[k]);
  if(v.client_request_id)result.client_request_id=text(v.client_request_id,128);
  if(v.email)result.email=text(v.email,254);
  return result;
}
function preflight(v) {
  if(!['queued','running','ready','failed'].includes(v.status))throw new Problem('UPSTREAM_RESPONSE','账号预检响应异常，请重新预检。',502);
  return {id:validID(v.id),status:v.status,code:errorCode(v.code),message:text(v.message),expiresAt:date(v.expiresAt),currentPlan:text(v.currentPlan,120),subscription:subscription(v.subscription)};
}

export class WudiV2 extends WudiPartner {
  constructor(options) {super(options);this.protocol=2;this.windows=new Map();this.checksInFlight=0;}
  reserve(operation) {
    const now=Date.now(),limits=[['all',110]];
    if(operation==='account-check')limits.push(['checks',4]);
    if(operation==='account-check/status')limits.push(['check-polls',35]);
    if(operation==='subscriptions/status')limits.push(['subscription-polls',35]);
    if(operation==='redeem')limits.push(['confirmations',9]);
    for(const [name,max] of limits){const times=(this.windows.get(name)||[]).filter(t=>now-t<60000);this.windows.set(name,times);if(times.length>=max)throw new Problem('RATE_LIMITED','请求较频繁，请稍后继续原订单。',429);}
    if(operation==='account-check'&&this.checksInFlight>=2)throw new Problem('RATE_LIMITED','已有账号正在预检，请稍后再试。',429);
    for(const [name] of limits)this.windows.get(name).push(now);
  }
  async request(operation,body) {
    if(!operations.has(operation))throw new Error('Unsupported Wudi v2 operation');
    const encoded=JSON.stringify(body);
    if(Buffer.byteLength(encoded)>(['preview','lookup','orders','status'].includes(operation)?4096:192*1024))throw new Problem('BODY_LIMIT','请求内容超出接口限制。',413);
    this.reserve(operation);if(operation==='account-check')this.checksInFlight++;
    try {
      const response=await this.fetchImpl(`https://wudi.cdk.bar/api/partner/v1/${operation}`,{method:'POST',redirect:'error',cache:'no-store',signal:AbortSignal.timeout(this.timeout),headers:{Authorization:`Bearer ${this.key}`,'Content-Type':'application/json'},body:encoded});
      let raw=await response.text();if(raw.length>1048576)throw new Error('Response too large');
      // Messages are plain text, but must never echo credentials supplied in this request.
      const secrets=[this.key,body.code];
      const collect=o=>{if(o&&typeof o==='object')for(const [k,v] of Object.entries(o)){if(/token|secret|password|cookie/i.test(k)&&typeof v==='string')secrets.push(v);else if(typeof v==='object')collect(v);}};collect(body.session);
      for(const value of secrets.filter(v=>typeof v==='string'&&v.length>=8))raw=raw.split(JSON.stringify(value).slice(1,-1)).join('[已隐藏]');
      const data=JSON.parse(raw);
      if(!response.ok||data?.ok!==true)throw new Problem(errorCode(data?.code)||'SERVICE_UNAVAILABLE',text(data?.message)||'兑换服务暂不可用，请稍后查询原记录。',response.status===429?429:response.status>=500?502:400);
      if(['preview','redeem','status'].includes(operation)) {
        const result=order(data);
        if(operation==='preview'&&(result.client_request_id!==body.client_request_id||!result.expires_at||!result.card))throw new Error('Preview mismatch');
        if(body.id&&result.id!==body.id)throw new Error('Order mismatch');
        return result;
      }
      if(operation==='account-check'||operation==='account-check/status'){const result=preflight(data);if(body.preflight_id&&result.id!==body.preflight_id)throw new Error('Preflight mismatch');return result;}
      if(operation==='lookup'){
        if(!Object.hasOwn(data,'order') || (data.order!==null && (typeof data.order!=='object'||Array.isArray(data.order))))throw new Error('Invalid lookup response');
        return {order:data.order===null?null:order(data.order)};
      }
      if(operation==='orders'){if(!Array.isArray(data.orders)||data.orders.length>50)throw new Error('Invalid orders');return {orders:data.orders.map(order),next_before_id:text(data.next_before_id,512)};}
      if(!['queued','running','success','failed'].includes(data.status))throw new Error('Invalid subscription query');
      if(operation==='subscriptions/status'&&data.id!==undefined&&data.id!==body.id)throw new Error('Subscription query mismatch');
      return {id:validID(data.id||body.id),status:data.status,message:text(data.message),code:errorCode(data.code),result:subscription(data.result)};
    } catch(error) {if(error instanceof Problem)throw error;throw new Problem('SERVICE_UNAVAILABLE','请求中断，请继续查询原订单，勿重复兑换。',502);}
    finally {if(operation==='account-check')this.checksInFlight--;}
  }
  create(code,requestID) {
    if(typeof code!=='string'||!/^[A-Za-z0-9_-]{8,128}$/.test(code)||!/^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/.test(requestID))throw new Problem('INVALID_REQUEST','原卡密或兑换编号不符合新版接口要求。');
    return this.request('preview',{code,client_request_id:requestID}).then(result=>({...result,protocol:2}));
  }
  accountCheck(id,code,session) {return this.request('account-check',{id:validID(id),code,session:sessionObject(session)});}
  accountStatus(id,preflightID) {return this.request('account-check/status',{id:validID(id),preflight_id:validID(preflightID)});}
  redeem(id,code,session,preflightID) {return this.request('redeem',{id:validID(id),code,session:sessionObject(session),preflight_id:validID(preflightID),confirmed:true});}
  lookup(code) {return this.request('lookup',{code});}
  // Main-site result lookup is card-scoped, unlike the partner's own orders.
  // It requires the site's Origin header but no partner key or customer Session.
  async cardStatus(code) {
    if(typeof code!=='string'||!/^[A-Za-z0-9_-]{8,128}$/.test(code))throw new Problem('INVALID_REQUEST','原卡密格式不正确。');
    this.reserve('card-status');
    try {
      const response=await this.fetchImpl('https://wudi.cdk.bar/api/status',{method:'POST',redirect:'error',cache:'no-store',signal:AbortSignal.timeout(this.timeout),
        headers:{Accept:'application/json','Content-Type':'application/json',Origin:'https://wudi.cdk.bar'},body:JSON.stringify({code})});
      const raw=await response.text();if(raw.length>65536)throw new Error('Response too large');
      const data=JSON.parse(raw),c=data?.card,r=data?.redemption;
      if(!response.ok||data?.ok!==true)throw new Problem(response.status===429?'RATE_LIMITED':'CHECK_UNAVAILABLE','上游卡密状态查询暂不可用，请稍后再查。',response.status===429?429:502);
      if(!c||Array.isArray(c)||!['unused','used','processing','review','disabled'].includes(c.status))throw new Error('Invalid card state');
      for(const field of ['code','cdk_code'])if(c[field]!==undefined&&c[field]!==code)throw new Error('Card mismatch');
      if(r!=null&&(typeof r!=='object'||Array.isArray(r)||!linkStates.has(r.status)))throw new Error('Invalid redemption state');
      if(c.status==='unused'&&r?.status==='succeeded')throw new Error('Conflicting card and redemption states');
      // Store status evidence only; discard the returned email, order ID and text.
      return {scope:'card',source:'wudi_main',status:c.status,used:c.status==='used'?true:c.status==='unused'?false:null,can_resubmit:false,
        redemption_status:r?.status||null,redemption_updated_at:date(r?.updatedAt),used_at:c.status==='used'?date(r?.updatedAt):null,
        message:'已查询上游卡密状态，包含在 wudi.cdk.bar 主站直接兑换的结果；退款前仍需人工核对。'};
    } catch(error) {if(error instanceof Problem)throw error;throw new Problem('CHECK_UNAVAILABLE','上游卡密状态响应无法确认，保留原状态，请稍后再查。',502);}
  }
  orders(code,beforeID) {if(beforeID!==undefined&&(typeof beforeID!=='string'||!beforeID||beforeID.length>512))throw new Problem('INVALID_REQUEST','分页编号不正确。');return this.request('orders',{code,...(beforeID?{before_id:beforeID}:{})});}
  querySubscription(session) {return this.request('subscriptions/query',{session:sessionObject(session)});}
  subscriptionStatus(id) {return this.request('subscriptions/status',{id:validID(id)});}
}
