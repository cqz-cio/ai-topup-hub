import test from 'node:test';
import assert from 'node:assert/strict';
import { randomBytes } from 'node:crypto';
import { Store } from '../server/store.mjs';
import { vault, Problem } from '../server/security.mjs';
import { WudiPartner, redeemURL } from '../server/wudi.mjs';
import { RedemptionService } from '../server/service.mjs';
import { MockPartner } from '../server/mock.mjs';
import { makeServer } from '../server/http.mjs';
import { parseLinkRows, mergeLinkResult } from '../src/link-flow.js';

const key='wpk_test_partner_key_not_a_real_credential',device='device-wudi-test-abcdefghijklmnopqrst',other='device-wudi-other-abcdefghijklmnopqr';
const link='https://wudi.cdk.bar/#partner=private_test_token_123456',source='WUDI-PRIVATE-TEST-CARD';
const created=request=>({ok:true,id:'intent-test-001',client_request_id:request.client_request_id,status:'awaiting_customer',redeem_url:link,expires_at:'2099-11-15T12:00:00Z'});
function fixture(t) {
  const store=new Store(':memory:'),crypt=vault(randomBytes(32).toString('base64')),legacy=new MockPartner(store),calls=[];
  const controls={state:'awaiting_customer',reenabled:false,failCreate:false,failQuery:false,error:null,delay:null};
  const partner=new WudiPartner({key,fetchImpl:async(url,options)=>{
    const operation=url.split('/').at(-1),body=JSON.parse(options.body);calls.push({operation,body,options});
    if(controls.delay)await controls.delay;
    if(operation==='redemptions') {if(controls.failCreate)throw new Error('network failed '+key);return Response.json(created(body));}
    if(controls.failQuery)return Response.json({ok:false,code:'INTENT_NOT_FOUND',message:source+key},{status:404});
    return Response.json({ok:true,id:body.id,status:controls.state,card_reenabled:controls.reenabled,...(controls.error?{preflight_error_code:controls.error,preflight_message:source+key}:{}),session:source,code:source});
  }});
  const service=new RedemptionService({store,vault:crypt,partner:legacy,linkPartner:partner,publicOrigin:'https://redeem.example.com',mock:true});
  legacy.seed(source,'success','gpt');const card=service.issue([{reference:'wudi-order-1',source_code:source,label:'ChatGPT Plus'}])[0];
  t.after(()=>store.close());
  const due=()=>store.run('UPDATE partner_redemptions SET next_poll=0');
  return {store,crypt,legacy,partner,service,card,calls,controls,due};
}

test('Wudi uses exact POST paths, Bearer auth and flat envelopes, without browser headers',async t=>{
  const f=fixture(t),result=await f.service.links.prepare(f.card.code,device);
  assert.equal(result.state,'awaiting_customer');assert.equal(result.redeem_url,link);assert.equal(result.redemption_code,source);
  const call=f.calls[0];assert.equal(call.operation,'redemptions');assert.deepEqual(Object.keys(call.options.headers).sort(),['Authorization','Content-Type']);assert.equal(call.options.headers.Authorization,'Bearer '+key);assert.equal(call.options.redirect,'error');assert.equal(call.body.code,source);assert.match(call.body.client_request_id,/^pai-/);
  assert.equal(f.legacy.calls.length,0);assert.equal(f.store.get('SELECT count(*) n FROM attempts').n,0);
});
test('parallel repeated create requests share a durable ID; cross-browser access requires the full code',async t=>{
  const f=fixture(t);let release;f.controls.delay=new Promise(r=>release=r);
  const first=f.service.links.prepare(f.card.code,device),second=f.service.links.prepare(f.card.code,other);release();
  const [a,b]=await Promise.all([first,second]);assert.equal(a.attempt_id,b.attempt_id);assert.equal(f.calls.length,1);
  await assert.rejects(f.service.result(a.attempt_id,other),e=>e.status===404);
  const recover=await f.service.recover(f.card.code,other);assert.equal(recover.attempt_id,a.attempt_id);assert.equal(recover.redemption_code,undefined);assert.equal(recover.redeem_url,undefined);
  assert.equal((await f.service.links.prepare(f.card.code,other)).redeem_url,link);assert.equal(f.calls.length,1);
});
test('lost create response and restart retry only the same code and request ID',async t=>{
  const f=fixture(t);f.controls.failCreate=true;
  const first=await f.service.links.prepare(f.card.code,device);assert.equal(first.state,'link_pending');assert.ok(first.query_error);assert.equal(first.redemption_code,undefined);
  f.controls.failCreate=false;f.due();
  const restarted=new RedemptionService({store:f.store,vault:f.crypt,partner:f.legacy,linkPartner:f.partner,publicOrigin:'https://redeem.example.com',mock:true});
  await restarted.reconcile();const second=await restarted.links.prepare(f.card.code,device);
  assert.equal(second.attempt_id,first.attempt_id);assert.deepEqual(f.calls[0].body,f.calls[1].body);assert.equal(f.store.get('SELECT count(*) n FROM partner_redemptions').n,1);assert.equal(f.legacy.calls.length,0);
});
test('background polling settles succeeded, not queued or running, without another creation',async t=>{
  const f=fixture(t),first=await f.service.links.prepare(f.card.code,device);
  for(const state of ['queued','running','uncertain','succeeded']) {
    f.controls.state=state;f.due();await f.service.reconcile();
    assert.equal((await f.service.result(first.attempt_id,device)).state,state);
    assert.equal(f.store.get('SELECT status FROM cards').status,state==='succeeded'?'completed':'processing');
  }
  assert.equal(f.calls.filter(c=>c.operation==='redemptions').length,1);f.due();await f.service.reconcile();assert.equal(f.calls.length,5);
});
test('404 and query failures retain original state and cannot unlock a fresh attempt',async t=>{
  const f=fixture(t);await f.service.links.prepare(f.card.code,device);f.controls.failQuery=true;f.due();
  const result=await f.service.recover(f.card.code,device);assert.equal(result.state,'awaiting_customer');assert.ok(result.query_error);assert.equal(result.can_retry,false);
  await assert.rejects(f.service.links.prepare(f.card.code,device,{retry:true}),e=>e.code==='RETRY_UNAVAILABLE');assert.equal(f.calls.filter(c=>c.operation==='redemptions').length,1);
  assert.ok(!JSON.stringify(result).includes(key));assert.ok(!JSON.stringify(result).includes(source));
});
test('manual/uncertain/no-task states continue original links; preflight reasons are safe',async t=>{
  const f=fixture(t);await f.service.links.prepare(f.card.code,device);
  for(const state of ['manual_review','uncertain','retryable_no_task','captcha_required']) {
    f.controls.state=state;f.controls.error='PLUS_PROMO_INELIGIBLE';f.due();
    const result=await f.service.links.prepare(f.card.code,device);assert.equal(result.state,state);assert.equal(result.can_retry,false);assert.equal(result.redeem_url,link);assert.match(result.preflight_message,/半价资格/);assert.ok(!result.preflight_message.includes(source));
    await assert.rejects(f.service.links.prepare(f.card.code,device,{retry:true}),e=>e.code==='RETRY_UNAVAILABLE');
  }
  assert.equal(f.calls.filter(c=>c.operation==='redemptions').length,1);
});
test('only explicit retry after failed+reenabled or expiry creates a new request ID',async t=>{
  for(const state of ['failed','expired']) {
    const f=fixture(t);await f.service.links.prepare(f.card.code,device);f.controls.state=state;f.due();
    let result=await f.service.recover(f.card.code,device);
    if(state==='failed') {assert.equal(result.can_retry,false);await assert.rejects(f.service.links.prepare(f.card.code,device,{retry:true}));f.controls.reenabled=true;}
    const original=f.calls[0].body.client_request_id;
    const retried=await f.service.links.prepare(f.card.code,device,{retry:true});assert.equal(retried.state,'awaiting_customer');
    assert.notEqual(f.calls.filter(c=>c.operation==='redemptions')[1].body.client_request_id,original);assert.equal(f.store.get('SELECT count(*) n FROM partner_redemptions').n,2);
  }
});
test('summaries, history and ciphertext never reveal private links, source codes or keys',async t=>{
  const f=fixture(t),created=await f.service.links.prepare(f.card.code,device);
  const responses=[f.service.recent(device),await f.service.result(created.attempt_id,device),await f.service.recover(f.card.code,other),f.service.results([f.card.code],other),await f.service.lookup([f.card.code],other),f.service.management.list()];
  for(const value of responses)for(const privateValue of [source,key,link])assert.ok(!JSON.stringify(value).includes(privateValue));
  for(const row of f.store.all('SELECT * FROM partner_redemptions'))for(const privateValue of [source,key,link])assert.ok(!JSON.stringify(row).includes(privateValue));
  await assert.rejects(f.service.preflight(created.attempt_id,'secret-session',device),e=>e.code==='EXTERNAL_CONFIRMATION');
  await assert.rejects(f.service.redeem(created.attempt_id,'confirmation',true,device),e=>e.code==='EXTERNAL_CONFIRMATION');
  await assert.rejects(f.service.billing({session:'secret-session'},device),e=>e.code==='BILLING_UNAVAILABLE');
});
test('new protocol never certifies unused or permits refund destruction; external links cannot be reset',async t=>{
  const f=fixture(t);const unused=(await f.service.lookup([f.card.code],device))[0];assert.equal(unused.used,null);assert.equal(unused.can_resubmit,false);
  await assert.rejects(f.service.management.revoke({card_id:f.card.id,request_id:'test-revoke-abcdefghijkl',confirmed:true,reason:'test'},'shop-admin:1'),e=>e.code==='CHECK_UNAVAILABLE');
  assert.equal(f.store.get('SELECT count(*) n FROM card_operations').n,0);
  await f.service.links.prepare(f.card.code,device);assert.throws(()=>f.service.disable(f.card.id,true));assert.throws(()=>f.service.releaseCheck(f.card.id));
});
test('revoked cards and previously submitted legacy cards cannot create new external links',async t=>{
  const f=fixture(t);f.store.run("UPDATE cards SET status='revoked'");await assert.rejects(f.service.links.prepare(f.card.code,device),e=>e.code==='CARD_REVOKED');assert.equal(f.calls.length,0);
  f.store.run("UPDATE cards SET status='ready'");const links=f.service.links;f.service.links=null;
  const first=await f.service.preview(f.card.code,device),checked=await f.service.preflight(first.attempt_id,JSON.stringify({demo:true,email:'test@example.com'}),device);
  await f.service.redeem(checked.attempt_id,checked.confirmation,true,device);f.service.links=links;
  await assert.rejects(f.service.links.prepare(f.card.code,device),e=>e.code==='LEGACY_RECORD');assert.equal(f.calls.length,0);
  const historical=await f.service.recover(f.card.code,device);assert.ok(historical.attempt_id);assert.equal(historical.provider,undefined);
});
test('key replacement cannot silently query or recreate old intents',async t=>{
  const f=fixture(t);await f.service.links.prepare(f.card.code,device);f.due();f.service.links.keyHash='different-key';const result=await f.service.recover(f.card.code,device);assert.match(result.query_error,/密钥/);assert.equal(f.calls.length,1);
});
test('malformed, redirected, mismatched and unexpected responses fail closed without leaking provider messages',async()=>{
  for(const response of [Response.json({ok:true,data:{status:'succeeded'}}),Response.json({ok:false,code:source,message:key},{status:401}),new Response('<html>'+key,{status:503}),Response.json({...created({client_request_id:'wrong'}),redeem_url:link}),Response.json({...created({client_request_id:'request-1'}),redeem_url:'https://attacker.example/#partner=abc'})]) {
    const p=new WudiPartner({key,fetchImpl:async()=>response});await assert.rejects(p.create(source,'request-1'),e=>e instanceof Problem&&!e.message.includes(key)&&!e.message.includes(source));
  }
  for(const url of ['http://wudi.cdk.bar/#partner=abcdefgh','https://wudi.cdk.bar.attacker.test/#partner=abcdefgh','https://user:pass@wudi.cdk.bar/#partner=abcdefgh','https://wudi.cdk.bar/?key=private#partner=abcdefgh','javascript:alert(1)'])assert.throws(()=>redeemURL(url));
  const p=new WudiPartner({key,fetchImpl:async()=>Response.json({ok:true,status:'new_unknown_status'})});assert.equal((await p.status('intent-1')).status,'uncertain');
});
test('HTTP enforces origin and device ownership; only explicit code presentation returns capability material',async t=>{
  const f=fixture(t),origin='https://redeem.example.com',server=makeServer({service:f.service,config:{mode:'live',publicOrigin:origin,examples:[]},staticDir:'.'});
  await new Promise(r=>server.listen(0,'127.0.0.1',r));t.after(()=>new Promise(r=>{server.close(r);server.closeAllConnections();}));const url='http://127.0.0.1:'+server.address().port;
  const post=(path,body,headers={})=>fetch(url+'/api/'+path,{method:'POST',headers:{Origin:origin,'Content-Type':'application/json','X-Pai-Request':'1',...headers},body:JSON.stringify(body)});
  assert.equal((await post('partner-link',{code:f.card.code},{Origin:'https://attacker.test'})).status,403);assert.equal(f.calls.length,0);
  const response=await post('partner-link',{code:f.card.code});assert.match(response.headers.get('cache-control'),/no-store/);const data=(await response.json()).data;assert.equal(data.redemption_code,source);
  const cookie=response.headers.get('set-cookie').split(';')[0];assert.equal((await post('result',{attempt_id:data.attempt_id})).status,404);
  const summary=await (await fetch(url+'/api/session',{headers:{Cookie:cookie}})).json();assert.equal(summary.data.provider,'wudi');assert.ok(!JSON.stringify(summary).includes(source));assert.ok(!JSON.stringify(summary).includes(link));
  assert.equal((await post('partner-retry',{code:f.card.code})).status,409);
});
test('batch preserves source rows, rejects duplicate/blank/extra credential columns; polling clears spent secrets',()=>{
  const codes=['PAIPLUS-ABCDEFGH23-ABCDEFGH45','PAIPLUS-ABCDEFGH67-ABCDEFGH89'];
  assert.deepEqual(parseLinkRows([['卡密'],...codes.map(c=>[c])]).map(r=>r.row),[2,3]);
  for(const rows of [[[codes[0]],[codes[0]]],[[codes[0]],[''],[codes[1]]],[[codes[0],'SESSION-SECRET']],[]])assert.throws(()=>parseLinkRows(rows));
  const previous={attempt_id:'1',redeem_url:link,redemption_code:source};assert.equal(mergeLinkResult(previous,{attempt_id:'1',can_open:true}).redemption_code,source);
  assert.equal(mergeLinkResult(previous,{attempt_id:'1',can_open:false,state:'succeeded'}).redemption_code,undefined);
  assert.equal(mergeLinkResult(previous,{attempt_id:'2',can_open:true}).redemption_code,undefined);
});
