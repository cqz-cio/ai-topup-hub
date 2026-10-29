import { Problem } from './security.mjs';

const operations = new Set(['preview','preflight','redeem','result','result-by-code','lookup','lookup-batch','billing']);
export class PartnerError extends Problem {
  constructor(code,status=502,uncertain=false) { super(code,'源头服务暂未完成请求，请稍后查询原记录。',status); this.uncertain=uncertain; }
}
export class Partner {
  constructor({password,fetchImpl=fetch,timeout=125000,redeemTimeout=155000}) {
    if (!password) throw new Error('CDK_PARTNER_PASSWORD is required in live mode');
    this.authorization='Basic '+Buffer.from('partner:'+password).toString('base64');
    this.fetchImpl=fetchImpl; this.timeout=timeout; this.redeemTimeout=redeemTimeout;
  }
  async call(operation,body,device) {
    if (!operations.has(operation) || !/^[A-Za-z0-9_-]{16,128}$/.test(device)) throw new Error('Invalid partner operation or device');
    try {
      const response=await this.fetchImpl(`https://cdk.bar/api/partner/v1/${operation}`,{
        method:'POST',redirect:'error',cache:'no-store',signal:AbortSignal.timeout(operation==='redeem'?this.redeemTimeout:this.timeout),
        headers:{Authorization:this.authorization,'Content-Type':'application/json','X-Redemption-Device':device},body:JSON.stringify(body),
      });
      let envelope; try { envelope=await response.json(); } catch { throw new PartnerError('UPSTREAM_RESPONSE',502,operation==='redeem'); }
      if (!response.ok || envelope?.ok !== true || !envelope.data || typeof envelope.data !== 'object') {
        // Never forward arbitrary upstream messages, codes or bodies containing credentials.
        const status = response.status===429?429:response.status===404?404:response.status===422?422:502;
        throw new PartnerError(status===429?'UPSTREAM_RATE_LIMIT':status===404?'UPSTREAM_NOT_FOUND':status===422?'UPSTREAM_REJECTED':'UPSTREAM_ERROR',status,operation==='redeem');
      }
      return envelope.data;
    } catch(e) { if(e instanceof PartnerError)throw e; throw new PartnerError('UPSTREAM_NETWORK',502,operation==='redeem'); }
  }
}

