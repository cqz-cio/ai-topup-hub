import { Problem } from './security.mjs';

const origin = 'https://wudi.cdk.bar';
const identifier = /^[A-Za-z0-9][A-Za-z0-9_.:-]{0,255}$/;
export const linkStates = new Set(['awaiting_customer','queued','running','captcha_required','succeeded','failed','cancelled','manual_review','uncertain','expired','card_unavailable','retryable_no_task']);
export const linkTerminal = s => ['succeeded','failed','cancelled','expired','card_unavailable'].includes(s);
export const reasonMessages = {
  PLUS_PROMO_INELIGIBLE:'当前账号没有半价资格，请核对所购套餐。',
  PLUS_PROMO_UNKNOWN:'暂时无法确认半价资格，请稍后在原链接重试。',
  SESSION_INVALID:'请在兑换站重新登录并获取新的 Session。',
  SESSION_FORMAT_INVALID:'请在兑换站粘贴完整的 Session JSON。',
  ACCOUNT_PLAN_INCOMPATIBLE:'当前订阅与卡密不兼容，请核对套餐及订阅来源。',
  ACCOUNT_CHECK_MISMATCH:'核验账号不一致，请重新获取账号凭证。',
  RETRY_ACCOUNT_MISMATCH:'继续原订单时，请使用原充值账号。',
  RETRY_PROVIDER_CHANGED:'原订单的服务连接已变更，请联系客服。',
  ACCOUNT_CHECK_UNAVAILABLE:'账号验证暂不可用，请稍后在原链接重试。',
  ACCOUNT_CHECK_TIMEOUT:'账号验证超时，请回到原链接重新核验。',
  ACTION_REQUIRED:'请回到原兑换链接完成进一步验证。',
  REDEMPTION_FAILED:'兑换失败，请核对原订单结果。',
  REDEMPTION_CANCELLED:'本次兑换已取消。',
  MANUAL_REVIEW:'需要人工核实，请保留原兑换记录并联系客服。',
  RESULT_UNCERTAIN:'结果尚未确定，正在查询原记录，请勿重复兑换。',
  INTENT_EXPIRED:'确认前链接已过期，可重新生成链接。',
  CARD_UNAVAILABLE:'本链接绑定的卡密暂时无法用于兑换，请联系客服。',
  NO_TASK_FOUND:'未查到上游任务，请通过原链接核验原账号并继续；链接失效时可在兑换首页使用原卡密继续。',
};
const errors = {
  INVALID_REQUEST:'兑换请求格式未被接受，请联系客服。', JSON_REQUIRED:'兑换请求格式未被接受，请联系客服。',
  PARTNER_UNAUTHORIZED:'兑换服务鉴权失败，请联系客服更新配置。', INTENT_NOT_FOUND:'暂未查到原兑换记录，请保留卡密并联系客服。',
  METHOD_NOT_ALLOWED:'兑换接口暂不可用，请联系客服。', REQUEST_CONFLICT:'兑换编号发生冲突，请联系客服核对原记录。',
  CARD_UNAVAILABLE:reasonMessages.CARD_UNAVAILABLE, RATE_LIMITED:'查询较频繁，请稍后再试。', SERVICE_UNAVAILABLE:'兑换服务暂不可用，请稍后查询原记录。',
};
export class WudiError extends Problem {
  constructor(code='SERVICE_UNAVAILABLE',status=502) {
    super(Object.hasOwn(errors,code)?code:'SERVICE_UNAVAILABLE',errors[code]||errors.SERVICE_UNAVAILABLE,status);
  }
}
export function redeemURL(value) {
  try {
    const u=new URL(value);
    if(typeof value==='string' && value.length<=4096 && u.origin===origin && !u.username && !u.password && u.pathname==='/' && !u.search && /^#partner=[A-Za-z0-9_.~-]{8,4000}$/.test(u.hash))return value;
  } catch {}
  throw new WudiError();
}
export class WudiPartner {
  constructor({key,fetchImpl=fetch,timeout=20000}) {
    if(typeof key!=='string' || !/^wpk_[A-Za-z0-9_-]{20,200}$/.test(key))throw new Error('WUDI_PARTNER_KEY is required for wudi provider');
    this.key=key;this.fetchImpl=fetchImpl;this.timeout=timeout;
  }
  async request(operation,body) {
    if(!['redemptions','status'].includes(operation))throw new Error('Unsupported Wudi operation');
    try {
      const response=await this.fetchImpl(`${origin}/api/partner/v1/${operation}`,{
        method:'POST',redirect:'error',cache:'no-store',signal:AbortSignal.timeout(this.timeout),
        headers:{Authorization:`Bearer ${this.key}`,'Content-Type':'application/json'},body:JSON.stringify(body),
      });
      const raw=await response.text();
      if(raw.length>65536)throw new WudiError();
      let data;try{data=JSON.parse(raw);}catch{throw new WudiError();}
      if(!response.ok || data?.ok!==true)throw new WudiError(data?.code,response.status===429?429:502);
      if(typeof data.status!=='string')throw new WudiError();
      const result={status:linkStates.has(data.status)?data.status:'uncertain',card_reenabled:data.status==='failed'&&data.card_reenabled===true};
      for(const field of ['error_code','preflight_error_code'])if(Object.hasOwn(reasonMessages,data[field]))result[field]=data[field];
      if(operation==='redemptions') {
        if(!identifier.test(data.id||'') || data.client_request_id!==body.client_request_id || !Number.isFinite(Date.parse(data.expires_at)))throw new WudiError();
        Object.assign(result,{id:data.id,client_request_id:data.client_request_id,redeem_url:redeemURL(data.redeem_url),expires_at:data.expires_at});
      } else if(data.id!==undefined && data.id!==body.id)throw new WudiError();
      return result;
    } catch(error) {if(error instanceof WudiError)throw error;throw new WudiError();}
  }
  create(code,requestID) {
    if(typeof code!=='string' || !/^[A-Za-z0-9_-]{8,256}$/.test(code) || !/^[A-Za-z0-9][A-Za-z0-9_.:-]{0,127}$/.test(requestID))throw new Error('Invalid Wudi create input');
    return this.request('redemptions',{code,client_request_id:requestID});
  }
  status(intentID) {if(!identifier.test(intentID||''))throw new Error('Invalid Wudi status input');return this.request('status',{id:intentID});}
}
