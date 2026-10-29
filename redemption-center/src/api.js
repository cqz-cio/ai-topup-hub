export async function api(path, body) {
  const response = await fetch(`/api/${path}`, {
    method: body === undefined ? 'GET' : 'POST', credentials: 'same-origin', cache: 'no-store',
    headers: body === undefined ? {} : {'Content-Type':'application/json','X-Pai-Request':'1'},
    body: body === undefined ? undefined : JSON.stringify(body), signal: AbortSignal.timeout(175000),
  });
  let result; try { result = await response.json(); } catch { throw new Error('服务暂时不可用，请稍后查询原记录。'); }
  if(!response.ok || !result.ok) { const error=new Error(result.error?.message||'请求未完成，请稍后再试。');error.code=result.error?.code;throw error; }
  return result.data;
}
export const active = state => ['submitting','queued','processing','uncertain'].includes(state);
export const terminal = state => ['completed','declined','failed_precharge','cancelled','failed','revoked','invalid'].includes(state);
export const statusLabel = state => ({pending:'待核验',error:'核验失败',ready:'可使用',unused:'未使用',used:'源头已使用',preparing:'验证中',preview:'待核对账号',preview_failed:'验证未完成',checking:'核验中',checked:'待确认',blocked:'需人工处理',submitting:'正在提交',queued:'排队中',processing:'处理中',uncertain:'结果待确认',completed:'已完成',declined:'未通过',failed_precharge:'充值未完成',cancelled:'已取消',failed:'失败',disabled:'已停用',revoked:'已销毁',revoking:'销毁核验中',invalid:'卡密已失效',unknown:'未找到',unavailable:'查询暂不可用'})[state]||'状态待确认';
export const productLabel = (product,plan) => `${({gpt:'ChatGPT',claude:'Claude',grok:'Grok',x:'X'})[product]||'待识别商品'}${plan?' · '+plan:''}`;
export function extractCard(text) {
  const trimmed=String(text||'').trim().toUpperCase();
  const codes=trimmed.match(/PAIPLUS-[A-HJ-NP-Z2-9]{10}-[A-HJ-NP-Z2-9]{10}/g)||[];
  if(codes.length!==1)throw new Error(codes.length?'检测到多张卡密，请使用批量功能。':'请粘贴完整的 PAIPLUS 卡密。');
  return codes[0];
}
export const formatDate = value => { if(value===null || value===undefined || value==='')return '暂未提供'; const date=new Date(typeof value==='number' && value<1e12?value*1000:value); return Number.isNaN(date.valueOf())?'暂未提供':date.toLocaleString('zh-CN'); };

