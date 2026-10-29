import test from 'node:test';
import assert from 'node:assert/strict';
import { randomBytes } from 'node:crypto';
import { Store } from '../server/store.mjs';
import { vault,id } from '../server/security.mjs';
import { RedemptionService } from '../server/service.mjs';
import { MockPartner } from '../server/mock.mjs';
import { makeServer } from '../server/http.mjs';
import { mkdtempSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';

const actor='shop-admin:12',device='device-customer-abcdefghijklmnopqrst',session=JSON.stringify({demo:true,email:'test@example.invalid'});
function fixture(t) {
  const store=new Store(':memory:'),crypt=vault(randomBytes(32).toString('base64')),partner=new MockPartner(store);
  const service=new RedemptionService({store,vault:crypt,partner,publicOrigin:'https://redeem.example.test',mock:true});
  const source='ELLPLUS-ABCDEFGHIJ-23456789AB';partner.seed(source);
  const delivery=service.deliver({reference:'shop:1:order:100',payload:source,quantity:1,metadata:{order_id:100,order_no:'ORDER-100',product_id:30,sku_id:30,product_name:'ChatGPT Plus'}});
  const card=service.card(delivery.cards[0].code),manager=service.management;
  t.after(()=>store.close());
  return {store,crypt,partner,service,source,card,manager,code:delivery.cards[0].code};
}
const request=f=>({card_id:f.card.id,request_id:id(),confirmed:true,reason:'客户申请退款'});
async function checked(f) {const p=await f.service.preview(f.code,device);return f.service.preflight(p.attempt_id,session,device);}

test('legacy encrypted database upgrades without changing its card pairs or prior audit',t=>{
  const f=fixture(t),folder=mkdtempSync(join(tmpdir(),'paiplus-management-migration-')),path=join(folder,'old.db');
  t.after(()=>rmSync(folder,{recursive:true,force:true}));
  const old=new Store(path);
  old.db.exec('DROP TABLE card_operations');
  for(const column of ['order_id','order_no','product_id','sku_id','upstream_snapshot_enc','upstream_checked_at','revoked_at'])old.db.exec(`ALTER TABLE cards DROP COLUMN ${column}`);
  old.db.exec('ALTER TABLE audit DROP COLUMN actor');
  old.run('INSERT INTO cards(id,fingerprint,code_enc,source_hash,source_enc,reference,label,status,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?,?)',f.card.id,f.card.fingerprint,f.card.code_enc,f.card.source_hash,f.card.source_enc,f.card.reference,'旧卡','ready',1,1);
  old.run("INSERT INTO audit(action,subject,at) VALUES('issue',?,1)",f.card.id);old.close();
  const upgraded=new Store(path);
  try {
    const card=upgraded.get('SELECT * FROM cards');assert.equal(f.crypt.decrypt(card.code_enc),f.code);assert.equal(f.crypt.decrypt(card.source_enc),f.source);
    assert.equal(card.revoked_at,0);assert.equal(card.status,'ready');assert.equal(upgraded.get('SELECT * FROM audit').actor,'system');
    assert.equal(upgraded.get('SELECT count(*) n FROM card_operations').n,0);
  } finally {upgraded.close();}
});

test('shop deliveries appear once with order/product metadata and protected pair details',t=>{
  const f=fixture(t),list=f.manager.list({query:'ORDER-100',status:'unused'});
  assert.equal(list.total,1);assert.equal(list.cards[0].product_id,30);assert.equal(list.cards[0].order_id,100);
  assert.ok(!JSON.stringify(list).includes(f.source));assert.ok(!JSON.stringify(list).includes(f.code));
  const detail=f.manager.detail(f.card.id,actor);assert.equal(detail.code,f.code);assert.equal(detail.source_code,f.source);
  assert.equal(f.store.get("SELECT actor FROM audit WHERE action='reveal_card_pair'").actor,actor);
  assert.equal(f.manager.list({query:f.code}).total,1);assert.equal(f.manager.list({status:'used'}).total,0);
});

test('unused valid card is permanently destroyed, pair retained, and every redemption path refuses it',async t=>{
  const f=fixture(t),a=await checked(f),req=request(f),result=await f.manager.revoke(req,actor);
  assert.equal(result.destroyed,true);assert.equal(result.card.status,'revoked');
  assert.equal(f.manager.detail(f.card.id,actor).source_code,f.source);
  await assert.rejects(f.service.preview(f.code,device),{code:'CARD_REVOKED'});
  await assert.rejects(f.service.preflight(a.attempt_id,session,device));
  assert.notEqual((await f.service.redeem(a.attempt_id,a.confirmation,true,device)).state,'queued');
  await assert.rejects(f.service.billing({code:f.code},device),{code:'CARD_REVOKED'});
  await assert.rejects(f.service.recover(f.code,device),{code:'CARD_REVOKED'});
  assert.throws(()=>f.service.results([f.code],device),{code:'CARD_REVOKED'});
  assert.throws(()=>f.service.disable(f.card.id,false));assert.throws(()=>f.service.releaseCheck(f.card.id));
  assert.equal((await f.service.lookup([f.code],device))[0].status,'revoked');
  assert.equal(f.partner.calls.filter(c=>c.operation==='redeem').length,0);
  assert.equal(f.store.get('SELECT token_enc,session_enc,confirmation_hash FROM attempts').token_enc,'');
  assert.equal((await f.manager.revoke(req,actor)).destroyed,true);
  await assert.rejects(f.manager.revoke(request(f),actor),{code:'CARD_REVOKED'});
  assert.equal(f.partner.calls.filter(c=>c.operation==='lookup').length,1);
  assert.throws(()=>f.service.issue([{reference:'another-order',source_code:f.source}]),{code:'SOURCE_DUPLICATE'});
});

test('upstream used card fails destruction and is listed as used',async t=>{
  const f=fixture(t);f.store.run("UPDATE mock_tasks SET status='completed'");
  await assert.rejects(f.manager.revoke(request(f),actor),e=>e.code==='CARD_INVALID'&&e.message.includes('卡密无效，销毁失败'));
  assert.equal(f.manager.list({status:'used'}).total,1);assert.equal(f.manager.card(f.card.id).revoked_at,0);
});

test('false used alone never authorizes destruction',async t=>{
  for(const data of [{status:'unknown',used:false,can_resubmit:false},{status:'unused',used:false,can_resubmit:false},{status:'processing',used:false,can_resubmit:false},{status:'unused',used:false}]) {
    const f=fixture(t);f.partner.call=async()=>({cdk_code:f.source,...data});
    await assert.rejects(f.manager.revoke(request(f),actor));assert.equal(f.manager.card(f.card.id).revoked_at,0);
  }
});

test('unreachable or mismatched upstream cannot produce destruction success',async t=>{
  for(const mode of ['network','wrong-code']) {
    const f=fixture(t);f.partner.call=async()=>{if(mode==='network')throw new Error('timeout');return {cdk_code:'OTHER-SOURCE',status:'unused',used:false,can_resubmit:true};};
    await assert.rejects(f.manager.revoke(request(f),actor),{code:'CHECK_UNAVAILABLE'});
    assert.equal(f.manager.card(f.card.id).status,'ready');assert.equal(f.manager.card(f.card.id).revoked_at,0);
    assert.equal(f.store.get('SELECT state FROM card_operations').state,'failed');
  }
});

test('persistent hold wins racing customer confirmation; concurrent admin retries validate once',async t=>{
  const f=fixture(t),a=await checked(f),call=f.partner.call.bind(f.partner),req=request(f);let release;
  f.partner.call=async(op,...args)=>{if(op==='lookup')await new Promise(resolve=>release=resolve);return call(op,...args);};
  const pending=f.manager.revoke(req,actor);assert.equal(f.manager.card(f.card.id).status,'revoking');
  const duplicate=f.manager.revoke(req,actor);
  await assert.rejects(f.service.redeem(a.attempt_id,a.confirmation,true,device),{code:'CARD_REVOKING'});
  await assert.rejects(f.service.preflight(a.attempt_id,session,device),{code:'CARD_REVOKING'});
  release();assert.equal((await pending).destroyed,true);assert.equal((await duplicate).destroyed,true);
  assert.equal(f.partner.calls.filter(c=>c.operation==='lookup').length,1);assert.equal(f.partner.calls.filter(c=>c.operation==='redeem').length,0);
});

test('submitted or uncertain recharge cannot be destroyed even if lookup falsely says unused',async t=>{
  const f=fixture(t),a=await checked(f);await f.service.redeem(a.attempt_id,a.confirmation,true,device);
  f.partner.call=async()=>({cdk_code:f.source,status:'unused',used:false,can_resubmit:true});
  await assert.rejects(f.manager.revoke(request(f),actor),{code:'CARD_IN_USE'});
  assert.equal(f.manager.card(f.card.id).revoked_at,0);
});

test('in-flight account validation blocks destruction and cannot resurrect a revoked card',async t=>{
  const f=fixture(t),p=await f.service.preview(f.code,device),call=f.partner.call.bind(f.partner);let release;
  f.partner.call=async(op,...args)=>{if(op==='preflight')await new Promise(resolve=>release=resolve);return call(op,...args);};
  const checking=f.service.preflight(p.attempt_id,session,device);
  await assert.rejects(f.manager.revoke(request(f),actor),{code:'CARD_BUSY'});release();await checking;
  await f.manager.revoke(request(f),actor);assert.equal(f.manager.card(f.card.id).status,'revoked');
});

test('restart resumes a durable pending destruction without making any recharge request',async t=>{
  const f=fixture(t),operation=id();
  f.store.run("INSERT INTO card_operations(id,card_id,actor,reason_enc,previous_status,state,created_at) VALUES(?,?,?,?,'ready','pending',?)",operation,f.card.id,actor,f.crypt.encrypt('退款'),Date.now());
  f.store.run("UPDATE cards SET status='revoking'");
  const service=new RedemptionService({store:f.store,vault:f.crypt,partner:f.partner,publicOrigin:'https://redeem.example.test',mock:true});
  await assert.rejects(service.preview(f.code,device),{code:'CARD_REVOKING'});
  await service.management.reconcile();assert.equal(service.management.card(f.card.id).status,'revoked');
  assert.equal(f.partner.calls.filter(c=>c.operation==='redeem').length,0);
});

test('status refresh learns external use but never reactivates a destroyed card',async t=>{
  const f=fixture(t);await f.manager.revoke(request(f),actor);
  assert.equal((await f.manager.refresh(f.card.id,actor)).status,'revoked');
  const g=fixture(t);g.store.run("UPDATE mock_tasks SET status='completed'");
  assert.equal((await g.manager.refresh(g.card.id,actor)).status,'used');
  assert.equal(g.manager.list({status:'used'}).counts.used,1);
});

test('management API requires its separate credential and administrator identity',async t=>{
  const f=fixture(t),config={mode:'live',publicOrigin:'https://redeem.example.test',integrationToken:'ship-secret'.repeat(5),managementToken:'manage-secret'.repeat(5),adminToken:'admin-secret'.repeat(5)};
  const server=makeServer({service:f.service,config,staticDir:'.'});await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
  t.after(()=>new Promise(resolve=>server.close(resolve)));
  const url=`http://127.0.0.1:${server.address().port}/api/management/detail`;
  for(const token of ['',config.integrationToken,config.adminToken]) {const res=await fetch(url,{method:'POST',headers:{'Content-Type':'application/json',Authorization:'Bearer '+token},body:JSON.stringify({actor,card_id:f.card.id})});assert.equal(res.status,401);assert.ok(!(await res.text()).includes(f.source));}
  const res=await fetch(url,{method:'POST',headers:{'Content-Type':'application/json',Authorization:'Bearer '+config.managementToken},body:JSON.stringify({actor,card_id:f.card.id})});
  assert.equal(res.status,200);assert.match(res.headers.get('cache-control'),/no-store/);assert.equal((await res.json()).data.source_code,f.source);
});
