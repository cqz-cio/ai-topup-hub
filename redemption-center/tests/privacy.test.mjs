import test from 'node:test';
import assert from 'node:assert/strict';
import { randomBytes } from 'node:crypto';
import { mkdtemp, mkdir, writeFile, rm, symlink } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve, dirname, basename } from 'node:path';
import { Store } from '../server/store.mjs';
import { vault, newCard, maskedEmail } from '../server/security.mjs';
import { abuseProtection, clientNetwork } from '../server/abuse.mjs';
import { MockPartner } from '../server/mock.mjs';
import { RedemptionService } from '../server/service.mjs';
import { makeServer } from '../server/http.mjs';

const owner='device-owner-abcdefghijklmnopqrstuv',other='device-other-abcdefghijklmnopqrstuv';
const email='privacy.buyer@example.test',secret='SESSION-PRIVACY-CANARY-NOT-A-REAL-CREDENTIAL';
const session=JSON.stringify({demo:true,email,sessionToken:secret});
function fixture(t,product='gpt') {
  const store=new Store(':memory:'),crypt=vault(randomBytes(32).toString('base64')),partner=new MockPartner(store);
  t.after(()=>store.close());
  const service=new RedemptionService({store,vault:crypt,partner,publicOrigin:'http://127.0.0.1:5191',mock:true});
  const source='PRIVACY-UPSTREAM-TEST-CARD';partner.seed(source,'success',product);
  const card=service.issue([{reference:'privacy-test:1',source_code:source}])[0];
  return {store,crypt,partner,service,card,source};
}
async function checked(f){const a=await f.service.preview(f.card.code,owner);return f.service.preflight(a.attempt_id,session,owner);}
async function serverFixture(t,config={},staticDir='.') {
  const f=fixture(t),origin='http://127.0.0.1:5191';
  const server=makeServer({service:f.service,config:{mode:'mock',publicOrigin:origin,adminToken:'privacy-admin-credential-not-real',integrationToken:'privacy-integration-credential-not-real',examples:[],...config},staticDir});
  await new Promise(r=>server.listen(0,'127.0.0.1',r));
  t.after(()=>new Promise(r=>{server.close(r);server.closeAllConnections();}));
  const url=`http://127.0.0.1:${server.address().port}`;
  const post=(path,body,headers={})=>fetch(url+'/api/'+path,{method:'POST',headers:{'Content-Type':'application/json',Origin:origin,'X-Pai-Request':'1',...headers},body:JSON.stringify(body)});
  return {...f,url,post};
}
test('only fresh account verification reveals full email; history and card-only recovery are masked',async t=>{
  const f=fixture(t),a=await checked(f);
  assert.equal(a.email,email);
  assert.equal(f.service.recent(owner)[0].email,maskedEmail(email));
  assert.equal(f.service.recent(owner)[0].confirmation,null);
  assert.deepEqual(f.service.recent(other),[]);
  await assert.rejects(f.service.result(a.attempt_id,other),e=>e.code==='ATTEMPT_NOT_FOUND');
  await f.service.redeem(a.attempt_id,a.confirmation,true,owner);
  const views=[await f.service.preview(f.card.code,other),await f.service.recover(f.card.code,other),...f.service.results([f.card.code],other),...await f.service.lookup([f.card.code],other),await f.service.result(a.attempt_id,owner),...f.service.recent(owner)];
  for(const view of views){const raw=JSON.stringify(view);for(const value of [email,secret,session,f.source])assert.ok(!raw.includes(value));}
  assert.ok(views.some(v=>v.email===maskedEmail(email)));
});
test('knowing a code is insufficient to read billing; reject before contacting upstream',async t=>{
  const f=fixture(t);let n=f.partner.calls.length;
  await assert.rejects(f.service.billing({code:f.card.code},owner),e=>e.code==='BILLING_PROOF_REQUIRED');assert.equal(f.partner.calls.length,n);
  const a=await checked(f);await f.service.redeem(a.attempt_id,a.confirmation,true,owner);n=f.partner.calls.length;
  await assert.rejects(f.service.billing({code:f.card.code},other),e=>e.status===403);assert.equal(f.partner.calls.length,n);
  const own=await f.service.billing({code:f.card.code},owner);assert.equal(own.summary.email,maskedEmail(email));
  const proven=await f.service.billing({session},other);assert.equal(proven.summary.email,maskedEmail(email));
});
test('credential is encrypted while Grok awaits confirmation, expires and is cleared before sending recharge',async t=>{
  const f=fixture(t,'grok');let a=await checked(f),row=f.store.get('SELECT * FROM attempts');
  assert.equal(f.crypt.decrypt(row.session_enc),session);assert.ok(!JSON.stringify(row).includes(secret));
  f.store.run('UPDATE attempts SET checked_at=?',Date.now()-16*60000);f.service.expire();assert.equal(f.store.get('SELECT session_enc FROM attempts').session_enc,'');
  a=await f.service.preflight(a.attempt_id,session,owner);
  const original=f.partner.call.bind(f.partner);
  f.partner.call=async(op,body,d)=>{if(op==='redeem'){assert.equal(body.credential.session,session);assert.equal(f.store.get('SELECT session_enc FROM attempts').session_enc,'');}return original(op,body,d);};
  await f.service.redeem(a.attempt_id,a.confirmation,true,owner);
  const database=JSON.stringify(f.store.all('SELECT * FROM attempts'))+JSON.stringify(f.store.all('SELECT * FROM audit'));
  assert.ok(!database.includes(secret));assert.ok(!database.includes(email));
});
test('rechecking an account removes the prior credential even while upstream has not replied',async t=>{
  const f=fixture(t,'grok'),a=await checked(f);
  f.partner.call=async()=>{assert.equal(f.store.get('SELECT session_enc FROM attempts').session_enc,'');throw new Error('synthetic network failure');};
  await assert.rejects(f.service.preflight(a.attempt_id,session,owner));assert.equal(f.store.get('SELECT session_enc FROM attempts').session_enc,'');
});
test('upstream extra credential fields never appear in public responses or saved account metadata',async t=>{
  const f=fixture(t),call=f.partner.call.bind(f.partner);
  f.partner.call=async(...args)=>({...await call(...args),session,session_token:secret,accessToken:secret,debug:{session,source:f.source}});
  const a=await checked(f);assert.ok(!JSON.stringify(a).includes(secret));
  assert.ok(!JSON.stringify(f.service.data(f.store.get('SELECT * FROM attempts'))).includes(secret));
  assert.ok(!JSON.stringify(await f.service.lookup([session],owner)).includes(secret));
});
test('IPv4 mapped addresses share a budget and IPv6 rotation within /64 is grouped',()=>{
  assert.equal(clientNetwork('::ffff:192.0.2.9'),'192.0.2.9');assert.equal(clientNetwork('::ffff:c000:209'),'192.0.2.9');
  assert.equal(clientNetwork('2001:db8:ab:cd::1'),clientNetwork('2001:0db8:00ab:00cd:1:2:3:4'));
  assert.notEqual(clientNetwork('2001:db8:ab:ce::1'),clientNetwork('2001:db8:ab:cd::1'));
});
test('budgets expire, concurrent calls release exactly once, and memory is bounded',()=>{
  let time=1000;const a=abuseProtection({now:()=>time,maxBuckets:2});
  a.hit('one',2);a.hit('one',2);assert.throws(()=>a.hit('one',2),e=>e.status===429);a.hit('two',2);assert.throws(()=>a.hit('three',2));
  time+=60001;a.hit('three',2);const releases=Array.from({length:6},()=>a.acquire('ip'));assert.throws(()=>a.acquire('ip'));releases[0]();releases[0]();const release=a.acquire('ip');assert.throws(()=>a.acquire('ip'));release();releases.slice(1).forEach(f=>f());
});
test('rotating cookies and forged forwarding headers cannot bypass the IP billing limit',async t=>{
  const f=await serverFixture(t);for(let i=0;i<21;i++){
    const r=await f.post('billing',{session},{Cookie:'pai_device=device-'+randomBytes(24).toString('base64url'),'X-Real-IP':`198.51.100.${i+1}`,'X-Forwarded-For':`203.0.113.${i+1}`});
    assert.equal(r.status,i<20?200:429);if(i===20)assert.equal(r.headers.get('retry-after'),'60');await r.text();
  }
  assert.equal(f.partner.calls.filter(c=>c.operation==='billing').length,20);
});
test('weighted batch quota is shared across new browsers and rejects before more upstream work',async t=>{
  const f=await serverFixture(t);for(let i=0;i<3;i++){
    const r=await f.post('lookup-batch',{codes:Array(100).fill(f.card.code)});assert.equal(r.status,i<2?200:429);await r.text();
  }
  assert.equal(f.partner.calls.filter(c=>c.operation==='lookup-batch').length,2);
});
test('repeated invalid-code enumeration is blocked even with a fresh cookie per request',async t=>{
  const f=await serverFixture(t);for(let i=0;i<31;i++){
    const r=await f.post('preview',{code:newCard()});assert.equal(r.status,i<30?404:429);await r.text();
  }
  assert.equal(f.partner.calls.length,0);
});
test('public HTTP never serves accidental database, log, source-map, or config files',async t=>{
  const dir=await mkdtemp(join(tmpdir(),'paiplus-privacy-'));
  t.after(async()=>{assert.equal(dirname(resolve(dir)),resolve(tmpdir()));assert.ok(basename(dir).startsWith('paiplus-privacy-'));await rm(dir,{recursive:true,force:true});});
  await mkdir(join(dir,'assets'));await writeFile(join(dir,'index.html'),'<html>private test application</html>');
  for(const name of ['database.db','debug.log','app.js.map','config.json'])await writeFile(join(dir,'assets',name),secret);
  const f=await serverFixture(t,{},dir);
  for(const path of ['/assets/database.db','/assets/debug.log','/assets/app.js.map','/assets/config.json','/server/http.mjs','/.env','/.git/config']){const r=await fetch(f.url+path);assert.equal(r.status,404);assert.ok(!(await r.text()).includes(secret));}
  const html=await fetch(f.url+'/');assert.equal(html.status,200);assert.equal(html.headers.get('cache-control'),'private, no-store');assert.match(html.headers.get('x-robots-tag'),/noindex/);assert.equal(html.headers.get('referrer-policy'),'no-referrer');await html.text();
  const robots=await fetch(f.url+'/robots.txt');assert.match(await robots.text(),/Disallow: \//);
  const query=await fetch(f.url+'/api/session?session='+secret);assert.equal(query.status,400);assert.ok(!(await query.text()).includes(secret));
  const fresh=await fetch(f.url+'/api/session');const data=await fresh.json();assert.deepEqual(data.data.recent,[]);assert.deepEqual(data.data.examples,[]);
});
test('a symbolic public-asset directory cannot escape the public root',async t=>{
  const dir=await mkdtemp(join(tmpdir(),'paiplus-privacy-'));
  t.after(async()=>{assert.equal(dirname(resolve(dir)),resolve(tmpdir()));assert.ok(basename(dir).startsWith('paiplus-privacy-'));await rm(dir,{recursive:true,force:true});});
  await mkdir(join(dir,'client'));await mkdir(join(dir,'private'));await writeFile(join(dir,'private','secret.png'),secret);
  await symlink(join(dir,'private'),join(dir,'client','assets'),process.platform==='win32'?'junction':'dir');
  const f=await serverFixture(t,{},join(dir,'client'));const r=await fetch(f.url+'/assets/secret.png');assert.equal(r.status,404);assert.ok(!(await r.text()).includes(secret));
});
