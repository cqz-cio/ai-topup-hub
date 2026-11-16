import { WudiV2 } from './wudi-v2.mjs';

// This adapter has no network path. Demo sessions and orders stay in memory.
export class MockWudiV2 extends WudiV2 {
  constructor(store) {
    const orders=new Map(),checks=new Map(),queries=new Map();
    super({key:'wpk_local_mock_partner_not_a_real_secret',fetchImpl:async(url,options)=>{
      const path=new URL(url).pathname.slice('/api/partner/v1/'.length),input=JSON.parse(options.body);
      const summary=session=>({email:session.user?.email||session.email||'demo@example.invalid',accountCount:1,queriedAt:new Date().toISOString(),accounts:[{selected:true,plan:'Free',active:false,source:'模拟数据',willRenew:null,plusPromoEligible:true,pro20xEligible:false}]});
      const output=o=>({ok:true,id:o.id,client_request_id:o.request,status:o.submitted?(Date.now()-o.submitted>2000?'succeeded':'queued'):'awaiting_customer',message:o.submitted?'模拟兑换正在处理。':'模拟验卡成功，尚未提交。',expires_at:o.expires,card:{plan_name:'ChatGPT Plus',region:'PH',region_name:'菲律宾',require_plus_promo:false}});
      if(path==='preview') {let o=[...orders.values()].find(o=>o.request===input.client_request_id);if(!o){o={id:'mock-'+input.client_request_id,request:input.client_request_id,code:input.code,expires:new Date(Date.now()+1800000).toISOString()};orders.set(o.id,o);}return Response.json(output(o));}
      if(path==='lookup')return Response.json({ok:true,order:[...orders.values()].filter(o=>o.code===input.code).map(output).at(-1)||null});
      if(path==='orders')return Response.json({ok:true,orders:[...orders.values()].filter(o=>o.code===input.code).map(output).reverse().slice(0,50)});
      if(path==='subscriptions/query'){const id='mock-query-'+(queries.size+1);queries.set(id,summary(input.session));return Response.json({ok:true,id,status:'queued',message:'模拟查询已受理。'});}
      if(path==='subscriptions/status')return Response.json({ok:true,id:input.id,status:'success',result:queries.get(input.id),message:'模拟查询完成。'});
      const o=orders.get(input.id);if(!o)return Response.json({ok:false,code:'INTENT_NOT_FOUND',message:'模拟订单不存在，请重新验证。'},{status:404});
      if(path==='status')return Response.json(output(o));
      if(path==='account-check') {
        const blocked=store.get('SELECT scenario FROM mock_tasks WHERE code=?',o.code)?.scenario==='blocked';
        const result={ok:true,id:'mock-check-'+(checks.size+1),status:blocked?'failed':'ready',code:blocked?'ACCOUNT_PLAN_INCOMPATIBLE':undefined,message:blocked?'模拟：当前套餐不兼容，未提交充值。':'模拟预检通过，请核对后确认。',expiresAt:new Date(Date.now()+120000).toISOString(),currentPlan:'Free',subscription:summary(input.session)};
        checks.set(result.id,result);return Response.json(result);
      }
      if(path==='account-check/status')return Response.json(checks.get(input.preflight_id));
      if(path==='redeem'){if(!o.submitted)o.submitted=Date.now();return Response.json(output(o));}
      throw new Error('Unsupported mock operation');
    }});
  }
}
