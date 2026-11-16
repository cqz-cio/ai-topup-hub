import test from 'node:test';
import assert from 'node:assert/strict';
import { randomBytes } from 'node:crypto';
import { Store } from '../server/store.mjs';
import { vault, id } from '../server/security.mjs';
import { RedemptionService } from '../server/service.mjs';
import { MockPartner } from '../server/mock.mjs';
import { WudiV2 } from '../server/wudi-v2.mjs';
import { makeServer } from '../server/http.mjs';

const source='ELL-PRIVATE-MANAGEMENT-TEST',actor='shop-admin:12';
function fixture(t,main=false) {
  const store=new Store(':memory:'),crypt=vault(randomBytes(32).toString('base64')),legacy=new MockPartner(store),calls=[];
  const controls={response:{ok:false,code:'INTENT_NOT_FOUND'},http:404};
  const partner=new WudiV2({key:'wpk_management_test_not_a_real_credential',fetchImpl:async(url,options)=>{
    const op=url==='https://wudi.cdk.bar/api/status'?'card-status':url.split('/').at(-1),body=JSON.parse(options.body);calls.push({op,body});
    assert.ok(['status','lookup','card-status'].includes(op),'Management must not create or redeem');
    if(op==='card-status')assert.deepEqual(options.headers,{Accept:'application/json','Content-Type':'application/json',Origin:'https://wudi.cdk.bar'});
    return Response.json(controls.response,{status:controls.http});
  }});
  // Retain coverage of record-only provider adapters and their limited scope.
  if(!main)partner.cardStatus=undefined;
  const service=new RedemptionService({store,vault:crypt,partner:legacy,linkPartner:partner,publicOrigin:'https://redeem.example.test',mock:true});
  legacy.seed(source);
  const issued=service.issue([{reference:'management-v2',source_code:source,label:'Test'}])[0],card=service.card(issued.code);
  t.after(()=>store.close());
  const respond=(status)=>{controls.http=200;controls.response={ok:true,order:{id:'remote-order-1',status}};};
  return {store,crypt,legacy,service,card,calls,controls,respond,partner};
}

test('v2 missing records return a successful scoped lookup, never an unused/refundable proof',async t=>{
  for(const absent of ['404','null']) {
    const f=fixture(t);
    if(absent==='null'){f.controls.http=200;f.controls.response={ok:true,order:null};}
    const result=await f.service.management.refresh(f.card.id,actor);
    assert.equal(result.upstream.status,'no_record');assert.equal(result.upstream.used,null);
    assert.equal(result.upstream.can_resubmit,false);assert.equal(result.capabilities.can_revoke,false);
    assert.equal(result.local_status,'ready');assert.ok(result.upstream_checked_at>0);
    assert.equal(result.status,'unknown');assert.equal(f.service.management.list({status:'unknown'}).total,1);
    assert.equal(f.service.management.list({status:'unused'}).total,0);
    assert.deepEqual(f.calls,[{op:'lookup',body:{code:source}}]);
    assert.equal(f.store.get('SELECT count(*) n FROM partner_redemptions').n,0);
    assert.equal(f.store.get('SELECT count(*) n FROM card_operations').n,0);
    assert.equal(f.store.get("SELECT actor FROM audit WHERE action='check_card_status'").actor,actor);
    assert.ok(!JSON.stringify(result).includes(source));
    await assert.rejects(f.service.management.revoke({card_id:f.card.id,request_id:id(),reason:'refund',confirmed:true},actor),{code:'CHECK_UNAVAILABLE'});
    assert.equal(f.store.get('SELECT count(*) n FROM card_operations').n,0);
  }
});

test('main-site manual redemption is discovered without a partner order and prevents new previews',async t=>{
  const f=fixture(t,true),updatedAt='2026-11-15T09:57:55Z';
  f.controls.http=200;f.controls.response={ok:true,card:{status:'used'},redemption:{id:'private-main-id',status:'succeeded',email:'private@example.invalid',message:source,updatedAt}};
  const result=await f.service.management.refresh(f.card.id,actor);
  assert.equal(result.local_status,'completed');assert.equal(result.status,'used');assert.equal(result.upstream.used,true);
  assert.equal(result.upstream.redemption_status,'succeeded');assert.equal(result.upstream.redemption_updated_at,updatedAt);
  assert.equal(result.capabilities.status_scope,'card');assert.equal(result.capabilities.can_revoke,false);
  assert.deepEqual(f.calls,[{op:'card-status',body:{code:source}}]);
  assert.equal(f.service.management.list({status:'used'}).total,1);assert.equal(f.service.management.list().counts.unknown,0);
  assert.equal(f.store.get('SELECT count(*) n FROM partner_redemptions').n,0);
  const code=f.crypt.decrypt(f.card.code_enc);
  await assert.rejects(f.service.links.prepare(code,'device-main-test'));
  assert.equal(f.calls.length,1);assert.equal(f.store.get('SELECT count(*) n FROM partner_redemptions').n,0);
  assert.ok(!JSON.stringify(result).includes(source));assert.ok(!JSON.stringify(result).includes('private'));
  const snapshot=f.crypt.decrypt(f.service.management.card(f.card.id).upstream_snapshot_enc);
  assert.ok(!snapshot.includes('private'));assert.ok(!snapshot.includes(source));
});

test('unverified cards enter the unused filter only after an explicit main-site unused result',async t=>{
  const f=fixture(t,true),m=f.service.management;
  assert.equal(m.list({status:'unknown'}).total,1);assert.equal(m.list({status:'unused'}).total,0);
  f.controls.http=200;f.controls.response={ok:true,card:{status:'unused'},redemption:null};
  const result=await m.refresh(f.card.id,actor);
  assert.equal(result.local_status,'ready');assert.equal(result.status,'unused');assert.equal(result.upstream.used,false);
  assert.equal(result.upstream.can_resubmit,false);assert.equal(result.capabilities.can_revoke,false);
  assert.equal(m.list({status:'unused'}).total,1);assert.equal(m.list({status:'unknown'}).total,0);
  assert.equal(m.list().counts.unused,1);assert.equal(m.list().counts.unknown,0);
  await assert.rejects(m.revoke({card_id:f.card.id,request_id:id(),reason:'refund',confirmed:true},actor),{code:'CHECK_UNAVAILABLE'});
  assert.equal(f.store.get('SELECT count(*) n FROM card_operations').n,0);
});

test('main-site lookup rejects missing, mismatched, contradictory and unavailable evidence',async t=>{
  for(const [http,response] of [[200,{ok:true,order:null}],[200,{ok:true,card:{status:'new-state'}}],[200,{ok:true,card:{status:'used',code:'another-card'}}],[200,{ok:true,card:{status:'unused'},redemption:{status:'succeeded'}}],[200,{ok:true,card:{status:'used'},redemption:[]}],[403,{ok:false,code:'ORIGIN_REJECTED'}],[404,{ok:false,code:'CARD_NOT_FOUND'}],[429,{ok:false,code:'RATE_LIMITED'}],[503,{ok:false}]]) {
    const f=fixture(t,true);f.controls.http=200;f.controls.response={ok:true,card:{status:'used'},redemption:{status:'succeeded'}};
    await f.service.management.refresh(f.card.id,actor);const before=f.service.management.card(f.card.id);
    f.controls.http=http;f.controls.response=response;
    await assert.rejects(f.service.management.refresh(f.card.id,actor),{code:'CHECK_UNAVAILABLE'});
    const after=f.service.management.card(f.card.id);
    assert.equal(after.status,'completed');assert.equal(after.upstream_snapshot_enc,before.upstream_snapshot_enc);
    assert.equal(after.upstream_checked_at,before.upstream_checked_at);assert.equal(after.upstream_card_status,'used');
    assert.ok(f.calls.every(c=>c.op==='card-status'),'An error must not turn a scoped empty lookup into unused proof');
  }
});

test('main-site active and disabled states remain distinct and never undo terminal holds',async t=>{
  for(const [state,expected] of [['processing','processing'],['review','processing'],['disabled','invalid']]) {
    const f=fixture(t,true);f.controls.http=200;f.controls.response={ok:true,card:{status:state},redemption:null};
    assert.equal((await f.service.management.refresh(f.card.id,actor)).local_status,expected);
  }
  for(const state of ['completed','disabled','invalid','revoked','revoking'])for(const upstream of ['unused','processing','disabled']) {
    const f=fixture(t,true);f.store.run('UPDATE cards SET status=?',state);
    f.controls.http=200;f.controls.response={ok:true,card:{status:upstream},redemption:null};
    assert.equal((await f.service.management.refresh(f.card.id,actor)).local_status,state);
  }
});

test('background reconciliation discovers main-site use and keeps issued card identity',async t=>{
  const f=fixture(t,true);f.controls.http=200;f.controls.response={ok:true,card:{status:'used'},redemption:{status:'succeeded'}};
  await f.service.management.reconcile();const c=f.service.management.card(f.card.id);
  assert.equal(c.status,'completed');assert.equal(c.code_enc,f.card.code_enc);assert.equal(c.source_enc,f.card.source_enc);assert.equal(c.reference,f.card.reference);
  assert.equal(f.store.get("SELECT actor FROM audit WHERE action='check_card_status'").actor,'system');
  assert.equal(f.calls.length,1);assert.equal(f.store.get('SELECT count(*) n FROM partner_redemptions').n,0);
});

test('main-site transport failures preserve unverified status and never expose provider text',async t=>{
  for(const response of [()=>{throw Error(source);},()=>new Response('<html>'+source+'</html>'),()=>new Response('x'.repeat(65537)),()=>Response.json({ok:false,message:source},{status:500})]) {
    const f=fixture(t,true);f.partner.fetchImpl=async()=>response();
    await assert.rejects(f.service.management.refresh(f.card.id,actor),e=>e.code==='CHECK_UNAVAILABLE'&&!e.message.includes(source));
    assert.equal(f.service.management.list().counts.unknown,1);assert.equal(f.service.management.list().counts.unused,0);
    assert.equal(f.service.management.card(f.card.id).upstream_checked_at,0);
  }
});

test('management HTTP synchronizes a manual redemption and returns consistent used filters',async t=>{
  const f=fixture(t,true),token='main-status-http-test-secret'.repeat(2);
  f.controls.http=200;f.controls.response={ok:true,card:{status:'used'},redemption:{status:'succeeded',updatedAt:'2026-11-15T09:57:55Z'}};
  const server=makeServer({service:f.service,config:{mode:'live',publicOrigin:'https://redeem.example.test',managementToken:token},staticDir:'.'});
  await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));t.after(()=>new Promise(resolve=>server.close(resolve)));
  const call=async(op,body)=>{
    const res=await fetch(`http://127.0.0.1:${server.address().port}/api/management/${op}`,{method:'POST',headers:{'Content-Type':'application/json',Authorization:'Bearer '+token},body:JSON.stringify({...body,actor})});
    assert.equal(res.status,200);assert.match(res.headers.get('cache-control'),/no-store/);return (await res.json()).data;
  };
  const result=await call('check',{card_id:f.card.id});assert.equal(result.status,'used');assert.equal(result.upstream.source,'wudi_main');
  const used=await call('cards',{status:'used'}),unknown=await call('cards',{status:'unknown'});
  assert.equal(used.total,1);assert.equal(used.counts.used,1);assert.equal(unknown.total,0);
  assert.ok(!JSON.stringify(used).includes(source));
});

test('v2 remote success and active orders update local status; failed orders do not invalidate cards',async t=>{
  for(const [state,expected] of [['succeeded','completed'],['running','processing'],['awaiting_customer','processing'],['expired','ready'],['failed','ready'],['cancelled','ready'],['card_unavailable','ready']]) {
    const f=fixture(t);f.respond(state);
    const result=await f.service.management.refresh(f.card.id,actor);
    assert.equal(result.local_status,expected);assert.equal(result.upstream.status,state);
    assert.equal(result.upstream.used,state==='succeeded'?true:null);assert.equal(result.upstream.can_resubmit,false);
    assert.equal(f.store.get('SELECT count(*) n FROM partner_redemptions').n,0);
  }
});

test('v2 existing IDs use fresh read-only status even with a cached terminal record',async t=>{
  const f=fixture(t),rowID=id(),now=Date.now();
  f.store.run('INSERT INTO partner_redemptions(id,card_id,sequence,device_hash,request_id,key_hash,state,data_enc,created_at,updated_at,next_poll) VALUES(?,?,?,?,?,?,?,?,?,?,?)',rowID,f.card.id,1,'device','request-1',f.service.links.keyHash,'expired',f.crypt.encrypt(JSON.stringify({id:'remote-order-1',protocol:2})),now,now,now+600000);
  f.controls.http=200;f.controls.response={ok:true,id:'remote-order-1',status:'succeeded'};
  f.service.links.sync=()=>{throw new Error('Mutation-capable sync is forbidden');};
  const result=await f.service.management.refresh(f.card.id,actor);
  assert.equal(result.local_status,'completed');assert.deepEqual(f.calls,[{op:'status',body:{id:'remote-order-1'}}]);
  f.controls.http=404;f.controls.response={ok:false,code:'INTENT_NOT_FOUND'};
  await assert.rejects(f.service.management.refresh(f.card.id,actor),{code:'CHECK_UNAVAILABLE'});
  assert.equal(f.service.management.card(f.card.id).status,'completed');
});

test('pending creation is only looked up, and a changed credential is not used for old records',async t=>{
  const f=fixture(t),rowID=id(),now=Date.now();
  f.store.run('INSERT INTO partner_redemptions(id,card_id,sequence,device_hash,request_id,key_hash,state,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)',rowID,f.card.id,1,'device','request-1',f.service.links.keyHash,'link_pending',now,now);
  const result=await f.service.management.refresh(f.card.id,actor);
  assert.equal(result.upstream.status,'no_record');assert.equal(f.calls[0].op,'lookup');
  f.service.links.keyHash='changed';
  await assert.rejects(f.service.management.refresh(f.card.id,actor),{code:'CHECK_UNAVAILABLE'});
  assert.equal(f.calls.length,1);assert.equal(f.store.get('SELECT state FROM partner_redemptions').state,'link_pending');
});

test('malformed, unauthorized and unavailable lookups preserve the last evidence',async t=>{
  for(const [http,response] of [[200,{ok:true}],[200,{ok:true,order:false}],[200,{ok:true,order:[]}],[401,{ok:false,code:'PARTNER_UNAUTHORIZED'}],[503,{ok:false,code:'SERVICE_UNAVAILABLE'}]]) {
    const f=fixture(t);f.respond('succeeded');await f.service.management.refresh(f.card.id,actor);
    const before=f.service.management.card(f.card.id);f.controls.http=http;f.controls.response=response;
    await assert.rejects(f.service.management.refresh(f.card.id,actor),{code:'CHECK_UNAVAILABLE'});
    const after=f.service.management.card(f.card.id);
    assert.equal(after.status,'completed');assert.equal(after.upstream_snapshot_enc,before.upstream_snapshot_enc);assert.equal(after.upstream_checked_at,before.upstream_checked_at);
  }
});

test('queries do not undo completion, disabling, invalidation or revocation',async t=>{
  for(const status of ['completed','disabled','invalid','revoked','revoking']) {
    const f=fixture(t);f.store.run('UPDATE cards SET status=?',status);f.respond('running');
    assert.equal((await f.service.management.refresh(f.card.id,actor)).local_status,status);
  }
});

test('submitted historical attempts continue to use the legacy card lookup',async t=>{
  const f=fixture(t),links=f.service.links;f.service.links=null;
  const device='management-legacy-device-abcdefghijklmnop';
  const p=await f.service.preview(f.crypt.decrypt(f.card.code_enc),device);
  const a=await f.service.preflight(p.attempt_id,JSON.stringify({demo:true,email:'test@example.invalid'}),device);
  await f.service.redeem(a.attempt_id,a.confirmation,true,device);f.service.links=links;
  await f.service.management.refresh(f.card.id,actor);
  assert.equal(f.calls.length,0);assert.equal(f.legacy.calls.at(-1).operation,'lookup');
});

test('authenticated management HTTP check reports no record as 200 with no-store',async t=>{
  const f=fixture(t),token='management-http-test-secret'.repeat(2);
  const server=makeServer({service:f.service,config:{mode:'live',publicOrigin:'https://redeem.example.test',managementToken:token},staticDir:'.'});
  await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));t.after(()=>new Promise(resolve=>server.close(resolve)));
  const res=await fetch(`http://127.0.0.1:${server.address().port}/api/management/check`,{method:'POST',headers:{'Content-Type':'application/json',Authorization:'Bearer '+token},body:JSON.stringify({card_id:f.card.id,actor})});
  assert.equal(res.status,200);assert.match(res.headers.get('cache-control'),/no-store/);
  const response=await res.json();assert.equal(response.ok,true);assert.equal(response.data.upstream.status,'no_record');
  assert.equal(response.data.capabilities.can_revoke,false);assert.ok(!JSON.stringify(response).includes(source));
});
