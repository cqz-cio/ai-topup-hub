import { extractCard } from './api.js';

export const linkDone = state => ['succeeded','failed','cancelled','expired','card_unavailable'].includes(state);
export const linkLabel = state => ({link_pending:'链接准备中',awaiting_customer:'等待确认',queued:'排队中',running:'充值中',captcha_required:'需要验证',succeeded:'已完成',failed:'兑换失败',cancelled:'已取消',manual_review:'人工核实',uncertain:'结果待确认',expired:'链接已过期',card_unavailable:'卡密不可用',retryable_no_task:'可继续原订单',unverified:'尚无兑换记录'})[state];
export function parseLinkRows(input) {
  if(!Array.isArray(input))throw new Error('请导入卡密列表。');
  let rows=input.map((r,i)=>({row:i+1,values:r}));
  if(['卡密','兑换卡密','code'].includes(String(rows[0]?.values[0]||'').trim()))rows=rows.slice(1);
  while(rows.length&&rows.at(-1).values.every(v=>!String(v||'').trim()))rows.pop();
  if(!rows.length||rows.length>100)throw new Error('每批请填写 1–100 张卡密。');
  const seen=new Set();
  return rows.map(({row,values})=>{
    if(values.slice(1).some(v=>String(v||'').trim()))throw new Error(`第 ${row} 行只需填写卡密，请移除 Session 等其他内容。`);
    let code;try{code=extractCard(values[0]);}catch{throw new Error(`第 ${row} 行卡密不完整。`);}
    if(seen.has(code))throw new Error(`第 ${row} 行卡密重复。`);seen.add(code);
    return {row,code,state:'pending',result:null,error:''};
  });
}
// Poll responses deliberately omit source codes and private links. Keep an
// already displayed capability only for the same, still-open redemption.
export function mergeLinkResult(previous,next) {
  if(previous?.attempt_id!==next.attempt_id || !next.can_open)return next;
  return {...next,redeem_url:previous.redeem_url,redemption_code:previous.redemption_code};
}
