import test from 'node:test';
import assert from 'node:assert/strict';
import { randomBytes } from 'node:crypto';
import { Store } from '../server/store.mjs';
import { vault } from '../server/security.mjs';
import { RedemptionService } from '../server/service.mjs';
import { MockPartner } from '../server/mock.mjs';
import { WudiV2 } from '../server/wudi-v2.mjs';
import { makeServer } from '../server/http.mjs';

const key='wpk_test_partner_key_not_a_real_credential',source='ELLPLUS-PRIVATE-V2-TEST';
const device='device-test-v2-browser',other='device-other-v2-browser';
const session={user:{id:'u1',email:'buyer@example.invalid'},account:{id:'a1'},accessToken:'private-access-value',sessionToken:'private-session-value',extra:{must:'preserve'}};
export function fixture(t) {
  const store=new Store(':memory:'),crypt=vault(randomBytes(32).toString('base64')),calls=[];
  const state={order:'awaiting_customer',preflight:'ready',losePreview:false,loseRedeem:false,reenabled:false,promo:false,preflightError:null};
  const order=()=>({ok:true,id:'order-v2',client_request_id:state.request,status:state.order,message:'上游实际处理提示',expires_at:new Date(Date.now()+1800000).toISOString(),card:{plan_name:'ChatGPT Plus',region:'PH',region_name:'菲律宾',require_plus_promo:state.promo},card_reenabled:state.reenabled});
  const preflight=()=>({ok:true,id:'preflight-v2',status:state.preflight,code:state.preflightError,message:state.preflightError?'网络超时，请重新充值。':'账号验证通过，请确认兑换。',expiresAt:new Date(Date.now()+120000).toISOString(),currentPlan:'Free',subscription:{email:session.user.email,accountCount:1,accounts:[{selected:true,accountId:'a1',plan:'Free',active:false,willRenew:null,plusPromoEligible:true,pro20xEligible:false}]}});
  const partner=new WudiV2({key,fetchImpl:async(url,options)=>{
    const path=new URL(url).pathname.replace('/api/partner/v1/',''),body=JSON.parse(options.body);calls.push({path,body,options});
    if(path==='preview'){state.request=body.client_request_id;if(state.losePreview)throw Error('lost');return Response.json(order());}
    if(path==='status')return Response.json(order());
    if(path==='account-check'||path==='account-check/status')return Response.json(preflight());
    if(path==='redeem'){state.order='queued';if(state.loseRedeem)throw Error('lost');return Response.json(order());}
    if(path==='lookup')return Response.json({ok:true,order:order()});
    if(path==='orders')return Response.json({ok:true,orders:[order()],...(!body.before_id?{next_before_id:'opaque/next=2'}:{})});
    if(path==='subscriptions/query')return Response.json({ok:true,id:'subscription-v2',status:'queued',message:'查询已受理，请稍候。'});
    if(path==='subscriptions/status')return Response.json({ok:true,id:'subscription-v2',status:'success',message:'查询完成',result:preflight().subscription});
    throw Error('Unexpected path');
  }});
  const legacy=new MockPartner(store),service=new RedemptionService({store,vault:crypt,partner:legacy,linkPartner:partner,publicOrigin:'https://redeem.example.invalid',mock:true});
  const card=service.issue([{reference:'v2-order',source_code:source,label:'Plus'}])[0];
  t.after(()=>store.close());
  return {store,crypt,partner,service,card,state,calls,legacy,due:()=>store.run('UPDATE partner_redemptions SET next_poll=0')};
}

test('v2 keeps card preview, async account check and explicit confirmation on our server',async t=>{
  const f=fixture(t),preview=await f.service.links.prepare(f.card.code,device);
  assert.equal(preview.protocol,2);assert.equal(preview.card.region_name,'菲律宾');assert.equal(preview.can_check,true);
  assert.equal(preview.redemption_code,undefined);assert.equal(preview.redeem_url,undefined);
  const checked=await f.service.links.check({code:f.card.code,attempt_id:preview.attempt_id,session,acknowledged:true},device);
  assert.equal(checked.preflight.status,'ready');assert.equal(checked.preflight.subscription.email,session.user.email);
  assert.equal(f.calls.filter(c=>c.path==='redeem').length,0);
  const result=await f.service.links.confirm({code:f.card.code,attempt_id:preview.attempt_id,confirmation:checked.confirmation,confirmed:true},device);
  assert.equal(result.state,'queued');assert.equal(f.store.get('SELECT status FROM cards').status,'processing');
  const submitted=f.calls.find(c=>c.path==='redeem');assert.deepEqual(submitted.body.session,session);assert.equal(submitted.body.preflight_id,'preflight-v2');assert.equal(submitted.body.confirmed,true);
  for(const c of f.calls){assert.deepEqual(Object.keys(c.options.headers).sort(),['Authorization','Content-Type']);assert.equal(c.options.redirect,'error');assert.equal(c.options.headers.Authorization,'Bearer '+key);}
  assert.ok(!JSON.stringify(f.store.all('SELECT * FROM partner_redemptions')).includes(session.accessToken));
  f.state.order='succeeded';f.due();assert.equal((await f.service.recover(f.card.code,device)).state,'succeeded');
  assert.equal(f.store.get('SELECT status FROM cards').status,'completed');assert.equal(f.legacy.calls.length,0);
});

test('preview response loss and process restart preserve the same upstream request',async t=>{
  const f=fixture(t);f.state.losePreview=true;
  const first=await f.service.links.prepare(f.card.code,device);assert.equal(first.state,'link_pending');
  f.state.losePreview=false;f.due();
  const restarted=new RedemptionService({store:f.store,vault:f.crypt,partner:f.legacy,linkPartner:f.partner,publicOrigin:'https://redeem.example.invalid',mock:true});
  const second=await restarted.links.prepare(f.card.code,other);assert.equal(second.attempt_id,first.attempt_id);
  assert.deepEqual(f.calls.filter(c=>c.path==='preview').map(c=>c.body.client_request_id),[f.state.request,f.state.request]);
  assert.equal(f.store.get('SELECT count(*) n FROM partner_redemptions').n,1);
});
test('only a ready, unexpired, same-device confirmation can redeem; Session is never persisted',async t=>{
  const f=fixture(t),p=await f.service.links.prepare(f.card.code,device),input={code:f.card.code,attempt_id:p.attempt_id,session,acknowledged:true};
  await assert.rejects(f.service.links.check({...input,acknowledged:false},device),e=>e.code==='ACKNOWLEDGEMENT_REQUIRED');
  f.state.preflight='running';const pending=await f.service.links.check(input,device);assert.equal(pending.confirmation,null);
  await assert.rejects(f.service.links.confirm({...input,confirmed:true},device),e=>e.code==='PREFLIGHT_EXPIRED');
  await assert.rejects(f.service.links.checkStatus(input,other),e=>e.code==='PREFLIGHT_EXPIRED');
  f.service.links.checks.get(p.attempt_id).nextPoll=0;f.state.preflight='ready';
  const ready=await f.service.links.checkStatus(input,device);
  await assert.rejects(f.service.links.confirm({...input,confirmation:ready.confirmation,confirmed:true},other));
  await assert.rejects(f.service.links.confirm({...input,confirmation:ready.confirmation,confirmed:false},device),e=>e.code==='CONFIRM_REQUIRED');
  for(const row of f.store.all('SELECT * FROM partner_redemptions')){const data=f.crypt.decrypt(row.data_enc);assert.ok(!data.includes(session.sessionToken));assert.ok(!data.includes(session.accessToken));assert.ok(!data.includes(session.user.email));}
  const restarted=new RedemptionService({store:f.store,vault:f.crypt,partner:f.legacy,linkPartner:f.partner,publicOrigin:'https://redeem.example.invalid',mock:true});
  await assert.rejects(restarted.links.confirm({...input,confirmation:ready.confirmation,confirmed:true},device),e=>e.code==='PREFLIGHT_EXPIRED');
  f.service.links.checks.get(p.attempt_id).expires=0;
  await assert.rejects(f.service.links.confirm({...input,confirmation:ready.confirmation,confirmed:true},device));
  assert.equal(f.calls.filter(c=>c.path==='redeem').length,0);
});
test('failed preflight retains exact business reason and subscription details without charging',async t=>{
  const f=fixture(t),p=await f.service.links.prepare(f.card.code,device);f.state.preflight='failed';f.state.preflightError='ACCOUNT_CHECK_TIMEOUT';
  const result=await f.service.links.check({code:f.card.code,attempt_id:p.attempt_id,session,acknowledged:true},device);
  assert.equal(result.preflight.code,'ACCOUNT_CHECK_TIMEOUT');assert.equal(result.preflight.message,'网络超时，请重新充值。');assert.equal(result.preflight.subscription.accounts[0].active,false);assert.equal(result.preflight.subscription.accounts[0].willRenew,null);assert.equal(result.confirmation,null);
  assert.equal(f.service.links.checks.get(p.attempt_id).session,null);assert.equal(f.calls.filter(c=>c.path==='redeem').length,0);
});
test('lost confirmation response, duplicate clicks and restart only query original order',async t=>{
  const f=fixture(t),p=await f.service.links.prepare(f.card.code,device),input={code:f.card.code,attempt_id:p.attempt_id};
  const ready=await f.service.links.check({...input,session,acknowledged:true},device);f.state.loseRedeem=true;
  const confirmed={...input,confirmed:true,confirmation:ready.confirmation};
  const results=await Promise.all([f.service.links.confirm(confirmed,device),f.service.links.confirm(confirmed,device)]);
  assert.ok(results.every(r=>r.state==='queued'));assert.equal(f.calls.filter(c=>c.path==='redeem').length,1);
  const restarted=new RedemptionService({store:f.store,vault:f.crypt,partner:f.legacy,linkPartner:f.partner,publicOrigin:'https://redeem.example.invalid',mock:true});
  f.state.order='uncertain';await restarted.links.confirm(confirmed,device);
  await assert.rejects(restarted.links.prepare(f.card.code,device,{retry:true}),e=>e.code==='RETRY_UNAVAILABLE');
  assert.equal(f.calls.filter(c=>c.path==='preview').length,1);assert.equal(f.calls.filter(c=>c.path==='redeem').length,1);
});
test('explicit restored-card retry creates a new preview; no-task uses the same order',async t=>{
  const f=fixture(t),p=await f.service.links.prepare(f.card.code,device),input={code:f.card.code,attempt_id:p.attempt_id};
  const ready=await f.service.links.check({...input,session,acknowledged:true},device);await f.service.links.confirm({...input,confirmation:ready.confirmation,confirmed:true},device);
  f.state.order='retryable_no_task';f.due();assert.equal((await f.service.recover(f.card.code,device)).can_check,true);
  const next=await f.service.links.check({...input,session,acknowledged:true},device);assert.equal(next.attempt_id,p.attempt_id);
  f.state.order='failed';f.due();assert.equal((await f.service.recover(f.card.code,device)).can_retry,false);
  f.state.reenabled=true;const retried=await f.service.links.prepare(f.card.code,device,{retry:true});assert.notEqual(retried.attempt_id,p.attempt_id);
  assert.equal(f.calls.filter(c=>c.path==='preview').length,2);
});
test('lookup, paginated orders and independent subscription queries bind capabilities locally',async t=>{
  const f=fixture(t);f.state.request='recovered-request';
  const found=await f.service.recover(f.card.code,other);assert.equal(found.message,'上游实际处理提示');assert.equal(f.calls[0].path,'lookup');
  const first=await f.service.links.orders({code:f.card.code}),second=await f.service.links.orders({code:f.card.code,before_id:first.next_before_id});
  assert.equal(first.orders.length,1);assert.equal(second.next_before_id,null);assert.equal(f.calls.findLast(c=>c.path==='orders').body.before_id,'opaque/next=2');
  const query=await f.service.links.querySubscription({session},device);assert.equal(query.status,'queued');assert.notEqual(query.query_id,'subscription-v2');
  await assert.rejects(f.service.links.subscriptionStatus(query,other),e=>e.code==='QUERY_EXPIRED');
  f.service.links.subscriptions.get(query.query_id).nextPoll=0;const result=await f.service.links.subscriptionStatus(query,device);assert.equal(result.status,'success');assert.equal(result.result.email,session.user.email);
  assert.ok(!JSON.stringify([...f.service.links.subscriptions.values()]).includes(session.accessToken));
  assert.equal(f.calls.filter(c=>['preview','account-check','redeem'].includes(c.path)).length,0);
});
test('cards and confirmation cannot be mixed between orders, and revoked cards remain blocked',async t=>{
  const f=fixture(t),p=await f.service.links.prepare(f.card.code,device),otherCard=f.service.issue([{reference:'other',source_code:'ELLPLUS-OTHER-CARD'}])[0];
  await assert.rejects(f.service.links.check({code:otherCard.code,attempt_id:p.attempt_id,session,acknowledged:true},device),e=>e.code==='ATTEMPT_NOT_FOUND');
  f.store.run("UPDATE cards SET status='revoked' WHERE id=?",f.card.id);
  await assert.rejects(f.service.links.orders({code:f.card.code}),e=>e.code==='CARD_REVOKED');
  assert.equal(f.calls.filter(c=>c.path==='account-check').length,0);
});
test('Session shape and 8192-field bounds reject malformed data without network calls',async t=>{
  const f=fixture(t);
  for(const value of ['not-json','[]',null,Object.fromEntries(Array.from({length:8193},(_,i)=>['field'+i,i])),{token:'x'.repeat(192*1024)}])await assert.rejects(async()=>f.partner.querySubscription(value),e=>e.code==='SESSION_FORMAT_INVALID');
  assert.equal(f.calls.length,0);
});

test('real HTTP delivery to v2 preview, preflight, confirmation and queries preserves original mapping',async t=>{
  const f=fixture(t),integration='integration-v2-simulated-credential-123456789',origin='https://redeem.example.invalid';
  const server=makeServer({service:f.service,config:{publicOrigin:origin,integrationToken:integration,mode:'mock',examples:[]},staticDir:'.'});
  await new Promise(r=>server.listen(0,'127.0.0.1',r));t.after(()=>new Promise(r=>server.close(r)));
  const base='http://127.0.0.1:'+server.address().port;let cookie='';
  const call=async(path,body,{privateAPI=false,badOrigin=false}={})=>{const response=await fetch(base+'/api/'+path,{method:body?'POST':'GET',headers:{'Content-Type':'application/json',...(privateAPI?{Authorization:'Bearer '+integration}:{Origin:badOrigin?'https://other.invalid':origin,'X-Pai-Request':'1',Cookie:cookie})},...(body?{body:JSON.stringify(body)}:{})});if(response.headers.get('set-cookie'))cookie=response.headers.get('set-cookie').split(';')[0];assert.match(response.headers.get('cache-control'),/no-store/);return {status:response.status,...await response.json()};};
  const config=await call('session');assert.equal(config.data.partner_protocol,2);
  const delivery={reference:'shop:1:order:90111',payload:'卡密cdk：ELLPLUS-HTTP-V2-CARD\n兑换站：wudi.cdk.bar',quantity:1,metadata:{order_id:90111,order_no:'SIMULATED-V2-ORDER',product_id:30,sku_id:30,product_name:'Plus'}};
  const first=await call('integration/deliver',delivery,{privateAPI:true});assert.equal(first.status,200);
  assert.deepEqual((await call('integration/deliver',delivery,{privateAPI:true})).data,first.data);assert.ok(!JSON.stringify(first).includes('ELLPLUS-HTTP'));
  const code=first.data.cards[0].code;assert.equal((await call('partner-link',{code},{badOrigin:true})).status,403);
  const preview=await call('partner-link',{code});assert.equal(preview.status,200);assert.equal(preview.data.protocol,2);assert.equal(preview.data.redemption_code,undefined);
  const input={code,attempt_id:preview.data.attempt_id};
  const checked=await call('partner-check',{...input,session,acknowledged:true});assert.equal(checked.status,200);assert.equal(checked.data.preflight.status,'ready');
  const submitted=await call('partner-confirm',{...input,confirmation:checked.data.confirmation,confirmed:true});assert.equal(submitted.data.state,'queued');
  assert.equal(f.calls.find(c=>c.path==='account-check').body.code,'ELLPLUS-HTTP-V2-CARD');assert.equal(f.calls.find(c=>c.path==='redeem').body.code,'ELLPLUS-HTTP-V2-CARD');
  f.state.order='succeeded';f.due();assert.equal((await call('result-by-code',{code})).data.state,'succeeded');
  assert.equal((await call('partner-orders',{code})).data.orders.length,1);
  const query=await call('subscription-query',{session});f.service.links.subscriptions.get(query.data.query_id).nextPoll=0;
  assert.equal((await call('subscription-status',{query_id:query.data.query_id})).data.status,'success');
  assert.equal(f.store.get('SELECT count(*) n FROM deliveries').n,1);assert.equal(f.store.get('SELECT count(*) n FROM partner_redemptions').n,1);
});
test('v2 forwards real business messages as text, strips secrets, and never treats unknown states as success',async()=>{
  const partner=new WudiV2({key,fetchImpl:async()=>Response.json({ok:false,code:'SESSION_INVALID',message:'Token 已失效，请重新获取。 '+key+' '+session.sessionToken},{status:400})});
  await assert.rejects(partner.querySubscription(session),e=>e.code==='SESSION_INVALID'&&e.message.startsWith('Token 已失效')&&!e.message.includes(key)&&!e.message.includes(session.sessionToken));
  const strange=new WudiV2({key,fetchImpl:async()=>Response.json({ok:true,id:'original',status:'new_unknown_status',message:'等待核实'})});
  assert.equal((await strange.status('original')).status,'uncertain');
  await assert.rejects(strange.status('mismatched'),e=>e.status===502);
});
test('per-key limits stop a fifth account check and excessive shared requests before contacting provider',async t=>{
  const f=fixture(t);for(let i=0;i<4;i++)await f.partner.accountCheck('order-v2',source,session);
  await assert.rejects(f.partner.accountCheck('order-v2',source,session),e=>e.status===429);assert.equal(f.calls.length,4);
  for(let i=0;i<106;i++)await f.partner.status('order-v2');
  await assert.rejects(f.partner.status('order-v2'),e=>e.status===429);assert.equal(f.calls.length,110);
});
test('pending async preflights reuse the original check and cap shared concurrency at two',async t=>{
  const f=fixture(t),first=await f.service.links.prepare(f.card.code,device);f.state.preflight='running';
  const input={code:f.card.code,attempt_id:first.attempt_id,session,acknowledged:true};
  await f.service.links.check(input,device);await f.service.links.check(input,device);
  assert.equal(f.calls.filter(c=>c.path==='account-check').length,1);
  await assert.rejects(f.service.links.check({...input,session:{...session,user:{email:'other@example.invalid'}}},device),e=>e.code==='BUSY');
  const cards=f.service.issue([{reference:'concurrent-2',source_code:'ELLPLUS-CONCURRENT-2'},{reference:'concurrent-3',source_code:'ELLPLUS-CONCURRENT-3'}]);
  const second=await f.service.links.prepare(cards[0].code,device);await f.service.links.check({...input,code:cards[0].code,attempt_id:second.attempt_id},device);
  const third=await f.service.links.prepare(cards[1].code,device);await assert.rejects(f.service.links.check({...input,code:cards[1].code,attempt_id:third.attempt_id},device),e=>e.status===429);
  assert.equal(f.calls.filter(c=>c.path==='account-check').length,2);
});
test('promo eligibility, invalid Session and incompatible plan failures never authorize confirmation',async t=>{
  for(const reason of ['PLUS_PROMO_INELIGIBLE','PLUS_PROMO_UNKNOWN','SESSION_INVALID','ACCOUNT_PLAN_INCOMPATIBLE']){
    const f=fixture(t);f.state.promo=true;const p=await f.service.links.prepare(f.card.code,device);assert.equal(p.card.require_plus_promo,true);
    f.state.preflight='failed';f.state.preflightError=reason;
    const input={code:f.card.code,attempt_id:p.attempt_id,session,acknowledged:true},checked=await f.service.links.check(input,device);
    assert.equal(checked.preflight.code,reason);assert.equal(checked.confirmation,null);
    await assert.rejects(f.service.links.confirm({...input,confirmed:true},device),e=>e.code==='PREFLIGHT_EXPIRED');
    assert.equal(f.calls.filter(c=>c.path==='redeem').length,0);
  }
});
