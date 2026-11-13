import test from 'node:test';
import assert from 'node:assert/strict';
import { randomBytes } from 'node:crypto';
import { Store } from '../server/store.mjs';
import { vault, newCard, normalizeCard } from '../server/security.mjs';
import { RedemptionService } from '../server/service.mjs';
import { MockPartner } from '../server/mock.mjs';
import { Partner } from '../server/partner.mjs';
import { makeServer } from '../server/http.mjs';
import { parseRows } from '../src/batch.js';

const device='device-test-customer-abcdefghijklmnop', other='device-other-customer-abcdefghijklmn';
const session=JSON.stringify({demo:true,email:'buyer@example.com'});
function fixture(scenario='success',product='gpt') {
  const store=new Store(':memory:'), key=randomBytes(32).toString('base64'), crypt=vault(key),partner=new MockPartner(store);
  const service=new RedemptionService({store,vault:crypt,partner,publicOrigin:'https://redeem.example.com',mock:true});
  const source='ELLPLUS-AAAAAAAAAA-1111111111';partner.seed(source,scenario,product);
  const card=service.issue([{reference:'paid-order:1',source_code:source,label:'Plus'}])[0];
  return {store,key,crypt,partner,service,source,card};
}
async function checked(f){const p=await f.service.preview(f.card.code,device);return f.service.preflight(p.attempt_id,session,device);}
async function submitted(f){const a=await checked(f);return f.service.redeem(a.attempt_id,a.confirmation,true,device);}
test('PAIPLUS generation: 2000 distinct codes, two mixed groups and unambiguous alphabet',()=>{
  const codes=Array.from({length:2000},newCard);assert.equal(new Set(codes).size,2000);
  for(const code of codes){assert.equal(normalizeCard(code.toLowerCase()),code);for(const group of code.split('-').slice(1)){assert.equal(group.length,10);assert.match(group,/[A-Z]/);assert.match(group,/[2-9]/);assert.doesNotMatch(group,/[IO01]/);}}
});
test('issuance is idempotent; source and local credentials encrypted at rest',()=>{
  const f=fixture();assert.equal(f.service.issue([{reference:'paid-order:1',source_code:f.source}])[0].code,f.card.code);
  const row=f.store.get('SELECT * FROM cards');const raw=JSON.stringify(row);assert.ok(!raw.includes(f.source));assert.ok(!raw.includes(f.card.code));assert.equal(f.crypt.decrypt(row.source_enc),f.source);
  assert.throws(()=>f.service.issue([{reference:'another',source_code:f.source}]),/已经生成/);
  assert.throws(()=>f.service.issue([{reference:'paid-order:1',source_code:'DIFFERENT-CARD-CODE'}]),/另一张/);
});
test('import batch rolls back all rows on a later conflict',()=>{
  const f=fixture();assert.throws(()=>f.service.issue([{reference:'new',source_code:'NEW-SOURCE-CODE'},{reference:'late',source_code:f.source}]));
  assert.equal(f.store.get('SELECT count(*) AS n FROM cards').n,1);
});
test('paid delivery wrapper preserves account and 2FA fields, replaces only card and redemption address',()=>{
  const f=fixture();const payload='邮箱号：demo@example.com 邮箱密码：example-only 密钥2fa：TESTSEED 密钥2fa在线获取：https://auth.example.test/2fa 兑换卡密：ELLPLUSHALF-AAAAAAAAAA-BBBBBBBBBB\n兑换地址：https://cdk.bar';
  const input={reference:'order:55',quantity:1,payload};const output=f.service.deliver(input);
  assert.match(output.payload,/邮箱密码：example-only/);assert.match(output.payload,/密钥2fa：TESTSEED/);assert.match(output.payload,/auth\.example\.test\/2fa/);assert.match(output.payload,/兑换卡密：PAIPLUS-/);assert.match(output.payload,/兑换地址：https:\/\/redeem\.example\.com\//);assert.ok(!output.payload.includes('ELLPLUSHALF'));
  assert.deepEqual(f.service.deliver(input),output);assert.throws(()=>f.service.deliver({...input,payload:payload.replace('example-only','changed-password')}),/变化/);
});
test('delivery accepts plain single code and rejects links, duplicate codes and quantity mismatch',()=>{
  const f=fixture();assert.match(f.service.deliver({reference:'plain',payload:'ELLGO-AAAAAAAAAA-CCCCCCCCCC',quantity:1}).payload,/卡密：PAIPLUS/);
  for(const payload of ['https://private.example/token','卡密：ELLGO-AAAAAAAAAA-DDDDDDDDDD\n卡密：ELLGO-AAAAAAAAAA-DDDDDDDDDD'])assert.throws(()=>f.service.deliver({reference:'bad',payload,quantity:2}));
  assert.throws(()=>f.service.deliver({reference:'mismatch',payload:'ELLGO-AAAAAAAAAA-EEEEEEEEEE',quantity:2}));
});
test('delivery rebuilds safe content, accepts bulk source rows, and never forwards unknown supplier text',()=>{
  const f=fixture();
  const source=['ELLPLUS-CCCCCCCCCC-DDDDDDDDDD','ELLPLUS-EEEEEEEEEE-FFFFFFFFFF'];
  const result=f.service.deliver({reference:'bulk-delivery',payload:source.join('\r\n')+'\n充值链接：https://cdk.bar/?source='+source[0],quantity:2});
  assert.equal(result.format,'paiplus-v1');assert.equal(result.cards.length,2);
  assert.equal(new Set(result.cards.map(c=>c.code)).size,2);
  assert.ok(source.every(c=>!JSON.stringify(result).includes(c)));
  assert.ok(!JSON.stringify(result).includes('cdk.bar'));
  assert.match(result.payload,/兑换地址：https:\/\/redeem\.example\.com\//);
  assert.throws(()=>f.service.deliver({reference:'unknown-text',quantity:1,payload:'卡密：ELLPLUS-GGGGGGGGGG-HHHHHHHHHH\n备用卡密信息：ELLPLUS-GGGGGGGGGG-HHHHHHHHHH'}),/格式/);
  assert.equal(f.store.get('SELECT count(*) AS n FROM deliveries').n,1);
});
test('preview/preflight do not redeem; confirmation is required and public views hide source tokens',async()=>{
  const f=fixture();const a=await checked(f);assert.equal(a.email,'buyer@example.com');assert.equal(a.state,'checked');assert.equal(f.partner.calls.filter(c=>c.operation==='redeem').length,0);
  assert.ok(!JSON.stringify(a).includes(f.source));assert.ok(!('redemption_token' in a));assert.ok(!('preflight_token' in a));assert.ok(!('session' in a));
  await assert.rejects(f.service.redeem(a.attempt_id,a.confirmation,false,device));assert.equal(f.partner.calls.filter(c=>c.operation==='redeem').length,0);
});
test('concurrent and repeated confirmations send exactly one durable request',async()=>{
  const f=fixture(),a=await checked(f);const results=await Promise.all(Array.from({length:10},()=>f.service.redeem(a.attempt_id,a.confirmation,true,device)));
  assert.ok(results.every(r=>['submitting','queued'].includes(r.state)));const calls=f.partner.calls.filter(c=>c.operation==='redeem');assert.equal(calls.length,1);
  assert.equal(f.store.get('SELECT request_id FROM attempts').request_id,calls[0].requestId);assert.equal(f.store.get('SELECT session_enc FROM attempts').session_enc,'');
});
test('source completion, not accepted HTTP response, settles a card',async()=>{
  const f=fixture(),a=await submitted(f);assert.equal(a.state,'queued');assert.equal(f.store.get('SELECT status FROM cards').status,'processing');
  f.store.run('UPDATE mock_tasks SET submitted_at=?',Date.now()-10000);f.store.run('UPDATE attempts SET last_poll=0');
  const result=await f.service.result(a.attempt_id,device);assert.equal(result.state,'completed');assert.equal(f.store.get('SELECT status FROM cards').status,'completed');
});
test('timeout remains uncertain and query recovery completes without another submission',async()=>{
  const f=fixture('uncertain'),a=await submitted(f);assert.equal(a.state,'uncertain');await f.service.redeem(a.attempt_id,'anything',true,device);
  f.store.run('UPDATE mock_tasks SET submitted_at=?',Date.now()-10000);f.store.run('UPDATE attempts SET last_poll=0');
  assert.equal((await f.service.recover(f.card.code,device)).state,'completed');assert.equal(f.partner.calls.filter(c=>c.operation==='redeem').length,1);
});
test('late submit network error cannot overwrite a completed reconciliation',async()=>{
  const f=fixture(),a=await checked(f),original=f.partner.call.bind(f.partner);
  let release;
  f.partner.call=async(op,body,d)=>{if(op!=='redeem')return original(op,body,d);await original(op,body,d);await new Promise(resolve=>release=resolve);throw new Error('late timeout');};
  const pending=f.service.redeem(a.attempt_id,a.confirmation,true,device);
  await new Promise(resolve=>setImmediate(resolve));
  f.store.run('UPDATE mock_tasks SET submitted_at=?',Date.now()-10000);f.store.run('UPDATE attempts SET last_poll=0');
  assert.equal((await f.service.result(a.attempt_id,device)).state,'completed');
  release();assert.equal((await pending).state,'completed');assert.equal(f.store.get('SELECT status FROM cards').status,'completed');
});
test('background reconciliation completes submitted records after the page is closed without charging again',async()=>{
  const f=fixture('uncertain'),a=await submitted(f);
  f.store.run('UPDATE mock_tasks SET submitted_at=?',Date.now()-10000);f.store.run('UPDATE attempts SET last_poll=0');
  await Promise.all([f.service.reconcile(),f.service.reconcile()]);
  assert.equal(f.service.results([f.card.code],device)[0].state,'completed');
  assert.equal(f.service.results([f.card.code],other)[0].confirmation,null);
  assert.equal(f.partner.calls.filter(c=>c.operation==='redeem').length,1);
});
test('blocked subscription cannot be submitted',async()=>{const f=fixture('blocked'),a=await checked(f);assert.equal(a.state,'blocked');assert.equal(a.confirmation,null);await assert.rejects(f.service.redeem(a.attempt_id,'fake',true,device));assert.equal(f.partner.calls.filter(c=>c.operation==='redeem').length,0);});

function partnerPlan(f,{target='plus',flow,current='pro',active=true,renew=true,extra={}}={}) {
  const original=f.partner.call.bind(f.partner);
  f.partner.call=async(op,...args)=>{
    const data=await original(op,...args);
    if(op==='preview')return {...data,plan:target,plan_flow:flow};
    if(op==='preflight')return {...data,currentPlan:current,subscription_has_active:active,subscription_will_renew:renew,
      subscription_active_until:new Date(Date.now()+(active===false?-1:23)*86400000).toISOString(),can_redeem:true,...extra};
    return data;
  };
}

for(const [name,params,expected] of [
  ['active Pro to standard Plus',{},'blocked'],
  ['cancelling renewal while Pro is still active',{renew:false},'blocked'],
  ['same-tier Plus half-price card',{current:'plus',target:'plus_half'},'blocked'],
  ['higher Pro tier to lower Pro tier',{current:'pro_20x',target:'pro_5x'},'blocked'],
  ['partner plan aliases',{current:' ChatGPT ProLite ',target:'plus'},'blocked'],
  ['known Pro with omitted active flag',{active:null},'blocked'],
  ['explicitly inactive old Pro subscription',{active:false},'checked'],
  ['free to Plus',{current:'free',active:false},'checked'],
  ['lower tier to higher tier',{current:'plus',target:'pro_5x'},'checked'],
  ['dedicated renewal card',{target:'pro_20x_renew',current:'pro_20x'},'checked'],
  ['explicit card-attach flow',{flow:'card_attach'},'checked'],
  ['explicit opening flow overrides renewal suffix',{flow:'checkout',target:'pro_20x_renew'},'blocked'],
  ['explicit veto on a card-attach flow',{flow:'card_attach',extra:{can_redeem:false}},'blocked'],
]){
  test('GPT card eligibility: '+name,async t=>{
    const f=fixture();
    t.after(()=>f.store.close());
    partnerPlan(f,params);
    const a=await checked(f);
    assert.equal(a.state,expected);
    assert.equal(Boolean(a.confirmation),expected==='checked');
    if(expected==='blocked'){
      if(!params.extra)assert.match(a.blocking_reason,/同档或更高/);
      const row=f.store.get('SELECT * FROM attempts');
      assert.equal(row.preflight_enc,'');
      assert.equal(row.confirmation_hash,'');
      await assert.rejects(f.service.redeem(a.attempt_id,'stale-confirmation',true,device),{code:'CONFIRM_REQUIRED'});
      assert.equal(f.partner.calls.filter(c=>c.operation==='redeem').length,0);
    }else{
      assert.equal((await f.service.redeem(a.attempt_id,a.confirmation,true,device)).state,'queued');
    }
  });
}

test('a new tier veto invalidates the previous account confirmation; the unused card can be rechecked',async t=>{
  const f=fixture();
  t.after(()=>f.store.close());
  const first=await checked(f);
  const original=f.partner.call.bind(f.partner);
  partnerPlan(f);
  const refused=await f.service.preflight(first.attempt_id,session,device);
  assert.equal(refused.state,'blocked');
  await assert.rejects(f.service.redeem(first.attempt_id,first.confirmation,true,device),{code:'CONFIRM_REQUIRED'});
  assert.equal(f.store.get('SELECT status FROM cards').status,'ready');
  f.partner.call=original;
  const renewed=await f.service.preflight(first.attempt_id,session,device);
  assert.equal(renewed.state,'checked');
  assert.notEqual(renewed.confirmation,first.confirmation);
  assert.equal(f.partner.calls.filter(c=>c.operation==='redeem').length,0);
});

test('unverified plan on a standard GPT card cannot be authorized by optional billing',async t=>{
  const f=fixture();
  t.after(()=>f.store.close());
  partnerPlan(f,{current:null,active:null});
  const preview=await f.service.preview(f.card.code,device);
  for(const budget of [undefined,()=>{}]){
    const check=await f.service.preflight(preview.attempt_id,session,device,budget);
    assert.equal(check.confirmation,null);
    assert.match(check.blocking_reason,/未能核对/);
  }
});

test('upgrading invalidates legacy unsubmitted GPT checks once and preserves a submitted request',async t=>{
  const f=fixture();
  t.after(()=>f.store.close());
  const legacy=await checked(f);
  const secondSource='SYNTHETIC-ALREADY-SUBMITTED';
  f.partner.seed(secondSource);
  const [second]=f.service.issue([{reference:'second-card',source_code:secondSource}]);
  const pending=await submitted({...f,card:second});
  const prior=f.store.get('SELECT * FROM attempts WHERE id=?',pending.attempt_id);
  f.store.run("DELETE FROM meta WHERE key='gpt_opening_tiers_v1'");
  const restart=()=>new RedemptionService({store:f.store,vault:f.crypt,partner:f.partner,publicOrigin:'https://redeem.example.com',mock:true});
  f.service=restart();
  assert.deepEqual(f.store.get('SELECT * FROM attempts WHERE id=?',pending.attempt_id),prior);
  const invalid=f.store.get('SELECT * FROM attempts WHERE id=?',legacy.attempt_id);
  assert.equal(invalid.state,'preview_failed');
  for(const field of ['confirmation_hash','preflight_enc','token_enc','session_enc'])assert.equal(invalid[field],'');
  await assert.rejects(f.service.redeem(legacy.attempt_id,legacy.confirmation,true,device),{code:'CONFIRM_REQUIRED'});
  const fresh=await checked(f);
  assert.equal(fresh.state,'checked');
  assert.notEqual(fresh.confirmation,legacy.confirmation);
  f.service=restart();
  assert.equal((await f.service.result(fresh.attempt_id,device)).confirmation,fresh.confirmation);
});

test('an explicit refusal, delinquency, or recovery requirement prevents submission even with a valid ticket',async t=>{
  for(const flag of [{can_redeem:false},{subscription_is_delinquent:true},{subscription_recovery_required:true},{blocking_reason:'private-upstream-reason'}]){
    const f=fixture(), call=f.partner.call.bind(f.partner);
    t.after(()=>f.store.close());
    f.partner.call=async(op,...rest)=>{
      const value=await call(op,...rest);
      return op==='preflight'?{...value,currentPlan:'free',subscription_has_active:false,...flag}:value;
    };
    const a=await checked(f);
    assert.equal(a.state,'blocked');
    assert.equal(a.confirmation,null);
    assert.ok(a.blocking_reason);
    assert.doesNotMatch(a.blocking_reason,/已满足|private-upstream-reason/);
    await assert.rejects(f.service.redeem(a.attempt_id,'old',true,device),{code:'CONFIRM_REQUIRED'});
    assert.equal(f.partner.calls.filter(c=>c.operation==='redeem').length,0);
  }
});

test('optional billing fills only this verified account and never appears in a code lookup or history',async t=>{
  const f=fixture(), call=f.partner.call.bind(f.partner), used=[];
  t.after(()=>f.store.close());
  f.partner.call=async(op,body,upstreamDevice,options)=>{
    used.push(op);
    if(op==='billing'){
      assert.deepEqual(body,{token_input:session});
      assert.equal(upstreamDevice,device);
      assert.ok(options.timeout<=5000);
      return {summary:{email:'BUYER@example.com',plan_type:'pro',subscription_plan:'pro_5x',has_active_subscription:true,active_until:'2026-11-26T19:00:44+08:00',will_renew:true,settlement_currency:'eur'},
        last_payment:{paidAt:1792998000,currency:'USD',status:'paid'},payment_method:{brand:'visa',last4:'9876',number:'SECRET-PAYMENT-CANARY'}};
    }
    const value=await call(op,body,upstreamDevice);
    return op==='preflight'?{...value,currentPlan:'pro',subscription_will_renew:false}:value;
  };
  const p=await f.service.preview(f.card.code,device);
  let budget=0;
  const a=await f.service.preflight(p.attempt_id,session,device,()=>{budget++;});
  assert.equal(budget,1);
  assert.deepEqual(used,['preview','preflight','billing']);
  assert.deepEqual(a.preflight_details,{current_plan:'pro_5x',active:true,active_until:Date.parse('2026-11-26T11:00:44Z'),will_renew:false,
    last_paid_at:1792998000000,settlement_currency:'EUR',payment_method:'Visa · 9876'});
  assert.doesNotMatch(JSON.stringify(a),/SECRET-PAYMENT/);
  for(const result of [f.service.recent(device),await f.service.result(a.attempt_id,device),await f.service.recover(f.card.code,other),f.service.results([f.card.code],other),f.service.data(f.store.get('SELECT * FROM attempts'))]){
    assert.doesNotMatch(JSON.stringify(result),/preflight_details|last_paid_at|settlement_currency|9876/);
  }
});

test('billing failure, a spent quota, or a different account or plan cannot change the checked card',async t=>{
  for(const scenario of ['offline','limited','wrong-account','wrong-plan']){
    const f=fixture(), call=f.partner.call.bind(f.partner);
    t.after(()=>f.store.close());
    let billed=0;
    f.partner.call=async(op,...rest)=>{
      if(op==='billing'){
        billed++;
        if(scenario==='offline')throw new Error('billing unavailable');
        return {summary:{email:scenario==='wrong-account'?'different@example.com':'buyer@example.com',plan_type:scenario==='wrong-plan'?'plus':'pro',
          has_active_subscription:true,expires_at:'2031-01-01T00:00:00Z',settlement_currency:'JPY'}};
      }
      const value=await call(op,...rest);
      if(op==='preview')return {...value,plan:'pro_50x'};
      return op==='preflight'?{...value,currentPlan:'pro',can_redeem:true,last_payment:{created_at:'2026-10-01T00:00:00Z',status:'open',currency:'CAD'}}:value;
    };
    const p=await f.service.preview(f.card.code,device);
    const a=await f.service.preflight(p.attempt_id,session,device,()=>{if(scenario==='limited')throw Error('quota');});
    assert.equal(billed,scenario==='limited'?0:1);
    assert.equal(a.state,'checked');
    assert.ok(a.confirmation);
    assert.deepEqual(a.preflight_details,{current_plan:'pro',active:null,active_until:null,will_renew:null,last_paid_at:null,settlement_currency:null,payment_method:null});
  }
});
test('Grok stores a short-lived encrypted credential, submits without a preflight ticket and clears it',async()=>{
  const f=fixture('success','grok'),a=await checked(f),row=f.store.get('SELECT * FROM attempts');assert.equal(row.preflight_enc,'');assert.notEqual(row.session_enc,session);assert.equal(f.crypt.decrypt(row.session_enc),session);
  const original=f.partner.call.bind(f.partner);let request;f.partner.call=async(op,body,d)=>{if(op==='redeem')request=body;return original(op,body,d);};await f.service.redeem(a.attempt_id,a.confirmation,true,device);
  assert.equal(request.credential.session,session);assert.ok(!('preflight_token' in request));assert.equal(f.store.get('SELECT session_enc FROM attempts').session_enc,'');
});
test('expired check and replaced account confirmation cannot redeem',async()=>{
  const f=fixture(),first=await checked(f);const second=await f.service.preflight(first.attempt_id,JSON.stringify({demo:true,email:'other@example.com'}),device);
  await assert.rejects(f.service.redeem(first.attempt_id,first.confirmation,true,device));
  f.store.run('UPDATE attempts SET checked_at=?',Date.now()-16*60000);await assert.rejects(f.service.redeem(second.attempt_id,second.confirmation,true,device));assert.equal(f.partner.calls.filter(c=>c.operation==='redeem').length,0);
});
test('a submitted record stays with its owner and is recoverable by code with the original upstream device',async()=>{
  const f=fixture(),a=await checked(f);await assert.rejects(f.service.preflight(a.attempt_id,session,other));
  await f.service.redeem(a.attempt_id,a.confirmation,true,device);f.store.run('UPDATE attempts SET last_poll=0');const recovered=await f.service.recover(f.card.code,other);assert.equal(recovered.recover_by_code,true);assert.equal(recovered.confirmation,null);assert.ok(f.partner.calls.every(c=>c.device===device));
  assert.equal((await f.service.preview(f.card.code,other)).recover_by_code,true);
  await assert.rejects(f.service.redeem(a.attempt_id,a.confirmation,true,other),{code:'ATTEMPT_NOT_FOUND'});
  assert.equal(f.partner.calls.filter(c=>c.operation==='redeem').length,1);
});
test('another browser can restart an unsubmitted check but cannot inherit the verified account',async t=>{
  const f=fixture('success','grok');
  t.after(()=>f.store.close());
  const first=await checked(f);
  const requestID=f.store.get('SELECT request_id FROM attempts').request_id;

  // Read-only recovery must neither reveal the previous check nor claim ownership.
  const summary=await f.service.recover(f.card.code,other);
  assert.deepEqual(summary,{code:first.code,state:'unused',submitted:false});
  assert.deepEqual(f.service.results([f.card.code],other),[summary]);
  assert.equal((await f.service.result(first.attempt_id,device)).state,'checked');

  const next=await f.service.preview(f.card.code,other);
  assert.equal(next.attempt_id,first.attempt_id);
  assert.equal(next.state,'preview');
  assert.equal(next.confirmation,null);
  assert.equal(next.email,null);
  assert.equal(next.current_plan,null);
  const row=f.store.get('SELECT * FROM attempts');
  assert.equal(row.request_id,requestID);
  assert.equal(row.checked_at,null);
  for(const key of ['session_enc','preflight_enc','confirmation_hash'])assert.equal(row[key],'');
  assert.equal(f.service.data(row).confirmation,undefined);
  assert.deepEqual(f.service.recent(device),[]);
  await assert.rejects(f.service.preflight(first.attempt_id,session,device),{code:'ATTEMPT_NOT_FOUND'});
  await assert.rejects(f.service.redeem(first.attempt_id,first.confirmation,true,device),{code:'ATTEMPT_NOT_FOUND'});
  await assert.rejects(f.service.redeem(next.attempt_id,first.confirmation,true,other),{code:'CONFIRM_REQUIRED'});

  const checkedAgain=await f.service.preflight(next.attempt_id,JSON.stringify({demo:true,email:'next@example.test'}),other);
  assert.equal(checkedAgain.email,'next@example.test');
  assert.notEqual(checkedAgain.confirmation,first.confirmation);
  await assert.rejects(f.service.redeem(next.attempt_id,first.confirmation,true,other),{code:'CONFIRM_REQUIRED'});
  await Promise.all([1,2].map(()=>f.service.redeem(next.attempt_id,checkedAgain.confirmation,true,other)));
  assert.equal(f.partner.calls.filter(c=>c.operation==='redeem').length,1);
  assert.ok(f.partner.calls.every(c=>c.device===device));
  assert.equal((await f.service.preview(f.card.code,device)).recover_by_code,true);
  await assert.rejects(f.service.billing({code:f.card.code},device),{code:'BILLING_PROOF_REQUIRED'});
});
test('a browser switch waits for in-flight checks and is retryable after a failed preview',async t=>{
  const f=fixture();
  t.after(()=>f.store.close());
  const first=await f.service.preview(f.card.code,device);
  const call=f.partner.call.bind(f.partner);
  let resume;
  f.partner.call=async(...args)=>{
    if(args[0]==='preflight')await new Promise(resolve=>{resume=resolve;});
    return call(...args);
  };
  const checking=f.service.preflight(first.attempt_id,session,device);
  await assert.rejects(f.service.preview(f.card.code,other),{code:'CARD_BUSY'});
  resume();
  await checking;

  f.partner.call=async()=>{throw new Error('synthetic preview failure');};
  await assert.rejects(f.service.preview(f.card.code,other),/synthetic preview failure/);
  const failed=f.store.get('SELECT * FROM attempts');
  assert.equal(failed.state,'preview_failed');
  assert.equal(failed.token_enc,'');
  assert.equal(failed.preflight_enc,'');
  assert.deepEqual(f.service.data(failed),{});
  f.partner.call=call;
  assert.equal((await f.service.preview(f.card.code,device)).state,'preview');
  assert.equal((await f.service.preview(f.card.code,other)).state,'preview');
  assert.equal(f.partner.calls.filter(c=>c.operation==='redeem').length,0);
});
test('a blocked account does not follow the card to a new browser; unavailable cards cannot be claimed',async t=>{
  const f=fixture('blocked');
  t.after(()=>f.store.close());
  const first=await checked(f);
  assert.equal(first.state,'blocked');
  f.service.disable(f.card.id,true);
  await assert.rejects(f.service.preview(f.card.code,other),{code:'CARD_DISABLED'});
  f.service.disable(f.card.id,false);
  f.store.run("UPDATE cards SET status='completed' WHERE id=?",f.card.id);
  await assert.rejects(f.service.preview(f.card.code,other),{code:'CARD_IN_USE'});
  assert.equal((await f.service.recover(f.card.code,other)).state,'completed');
  f.store.run("UPDATE cards SET status='ready' WHERE id=?",f.card.id);
  const fresh=await f.service.preview(f.card.code,other);
  assert.equal(fresh.state,'preview');
  assert.equal(fresh.blocked,false);
  assert.equal(fresh.email,null);
  assert.equal(fresh.blocking_reason,null);
  assert.equal(f.partner.calls.filter(c=>c.operation==='redeem').length,0);
});
test('disabled cards cannot verify or redeem, and submitted cards cannot be re-enabled',async()=>{
  const f=fixture(),a=await checked(f);f.service.disable(f.card.id,true);await assert.rejects(f.service.redeem(a.attempt_id,a.confirmation,true,device));await assert.rejects(f.service.preview(f.card.code,device));f.service.disable(f.card.id,false);await f.service.redeem(a.attempt_id,a.confirmation,true,device);assert.throws(()=>f.service.disable(f.card.id,false));
});
test('restart after durable submit preserves request ID and only queries',async()=>{
  const f=fixture(),a=await checked(f);f.store.run("UPDATE attempts SET state='submitting',submitted_at=?",Date.now());const request=f.store.get('SELECT request_id FROM attempts').request_id;
  const recovered=new RedemptionService({store:f.store,vault:f.crypt,partner:f.partner,publicOrigin:'https://redeem.example.com',mock:true});assert.equal(f.store.get('SELECT state FROM attempts').state,'uncertain');await recovered.redeem(a.attempt_id,a.confirmation,true,device);assert.equal(f.store.get('SELECT request_id FROM attempts').request_id,request);assert.equal(f.partner.calls.filter(c=>c.operation==='redeem').length,0);
});
test('database refuses changed encryption keys or mode changes',()=>{const f=fixture();assert.throws(()=>new RedemptionService({store:f.store,vault:vault(randomBytes(32).toString('base64')),partner:f.partner,mock:true}));assert.throws(()=>new RedemptionService({store:f.store,vault:f.crypt,partner:f.partner,mock:false}));});
test('batch lookup preserves rows, hides upstream codes and does not allow uncertain resubmission',async()=>{
  const f=fixture('uncertain');await submitted(f);const unknown=newCard();const rows=await f.service.lookup([unknown,f.card.code],device);assert.equal(rows[0].status,'unknown');assert.equal(rows[1].code,f.card.code);assert.equal(rows[1].status,'uncertain');assert.equal(rows[1].can_resubmit,false);assert.ok(!JSON.stringify(rows).includes(f.source));
});
test('billing preserves missing values and zero amounts, rejects unsafe links',async()=>{
  const f=fixture();f.partner.call=async()=>({summary:{},last_payment:{amount:0,currency:'USD'},invoices:[],invoice_data_available:true,invoice_url:'javascript:alert(1)'});
  const b=await f.service.billing({session},device);assert.equal(b.summary.has_active_subscription,null);assert.equal(b.last_payment.amount,0);assert.equal(b.invoice_data_available,true);assert.deepEqual(b.invoices,[]);assert.equal(b.invoice_url,null);
});
test('expired upstream token falls back to result-by-code; a missing record never resubmits',async()=>{
  const f=fixture('uncertain'),a=await submitted(f);const original=f.partner.call.bind(f.partner);let fallback=0;
  f.partner.call=async(op,body,d)=>{if(op==='result'){const e=new Error('expired');e.status=404;throw e;}if(op==='result-by-code')fallback++;return original(op,body,d);};
  f.store.run('UPDATE mock_tasks SET submitted_at=?',Date.now()-10000);f.store.run('UPDATE attempts SET last_poll=0');assert.equal((await f.service.result(a.attempt_id,device)).state,'completed');assert.equal(fallback,1);assert.equal(f.partner.calls.filter(c=>c.operation==='redeem').length,1);
});
test('admin can release an abandoned unsubmitted check but cannot reopen a submitted card',async()=>{
  const f=fixture(),a=await checked(f);f.service.releaseCheck(f.card.id);assert.equal((await f.service.preview(f.card.code,other)).state,'preview');
  assert.equal((await f.service.redeem(a.attempt_id,a.confirmation,true,device)).state,'cancelled');assert.equal(f.partner.calls.filter(c=>c.operation==='redeem').length,0);
  const next=await f.service.preview(f.card.code,other),check=await f.service.preflight(next.attempt_id,session,other);await f.service.redeem(check.attempt_id,check.confirmation,true,other);assert.throws(()=>f.service.releaseCheck(f.card.id));
});
test('partner client uses server Basic auth, same device, fixed origin, no redirects, and no retries',async()=>{
  let count=0;const partner=new Partner({password:'test-only-password',fetchImpl:async(url,opts)=>{count++;assert.equal(url,'https://cdk.bar/api/partner/v1/redeem');assert.equal(opts.redirect,'error');assert.equal(opts.headers['X-Redemption-Device'],device);assert.equal(opts.headers.Authorization,'Basic '+Buffer.from('partner:test-only-password').toString('base64'));return new Response('<html>proxy failed</html>',{status:504});}});
  await assert.rejects(partner.call('redeem',{},device),e=>e.uncertain===true && !e.message.includes('proxy'));assert.equal(count,1);await assert.rejects(partner.call('https://evil.example',{},device));assert.equal(count,1);
});
test('upstream error text and credential echoes are not relayed',async()=>{
  const partner=new Partner({password:'test-only-password',fetchImpl:async()=>Response.json({ok:false,error:{code:'SECRET-TOKEN',message:'secret password token'}},{status:422})});
  await assert.rejects(partner.call('preflight',{},device),e=>!e.message.includes('secret') && !e.code.includes('SECRET'));
});
test('batch parser preserves Excel row numbers and rejects blanks/duplicates/missing headers before calls',()=>{
  const a=newCard(),b=newCard();const rows=parseRows([['卡密','Session'],[a,'session-a'],[b,'session-b']]);assert.deepEqual(rows.map(r=>[r.row,r.code,r.session]),[[2,a,'session-a'],[3,b,'session-b']]);
  assert.throws(()=>parseRows([['卡密','Session'],[a,'session-a'],['',''],[b,'session-b']]),/第 3 行/);
  assert.throws(()=>parseRows([['卡密','Session'],[a,'a'],[a.toLowerCase(),'b']]),/第 3 行卡密重复/);
  assert.throws(()=>parseRows([['账号','卡密'],[a,'a']]));assert.throws(()=>parseRows([['卡密','Session'],[a,'']]));
});
test('HTTP privacy, CSRF, admin access and integration authentication',async t=>{
  const f=fixture(),origin='http://127.0.0.1:5190';const config={mode:'mock',publicOrigin:origin,adminToken:'admin-test-token-32-characters-long',integrationToken:'integration-test-token-32-characters',examples:[]};
  const server=makeServer({service:f.service,config,staticDir:new URL('../dist/client',import.meta.url).pathname});await new Promise(r=>server.listen(0,'127.0.0.1',r));t.after(()=>server.close());const url=`http://127.0.0.1:${server.address().port}`;
  const boot=await fetch(url+'/api/session');assert.equal(boot.headers.get('cache-control'),'private, no-store');assert.match(boot.headers.get('set-cookie'),/HttpOnly; SameSite=Strict/);const cookie=boot.headers.get('set-cookie').split(';')[0];const data=await boot.text();assert.ok(!data.includes(config.adminToken));assert.ok(!data.includes(config.integrationToken));
  const post=(path,input,headers={})=>fetch(url+'/api/'+path,{method:'POST',headers:{'Content-Type':'application/json',Origin:origin,'X-Pai-Request':'1',Cookie:cookie,...headers},body:JSON.stringify(input)});
  assert.equal((await post('preview',{code:f.card.code},{Origin:'https://evil.example'})).status,403);
  assert.equal((await post('admin/cards',{})).status,401);
  assert.equal((await post('integration/deliver',{reference:'x',payload:'source-secret',quantity:1})).status,401);
  const login=await post('admin/login',{demo:true});assert.equal(login.status,200);const signed=login.headers.getSetCookie().find(c=>c.startsWith('pai_admin=')).split(';')[0];
  assert.equal((await post('admin/cards',{}, {Cookie:cookie+'; '+signed})).status,200);
  assert.equal((await post('admin/cards',{}, {Cookie:cookie+'; pai_admin=forged'})).status,401);
});

test('one browser can verify, confirm and query 100 rows without exhausting the batch request allowance',async t=>{
  const f=fixture(),origin='http://127.0.0.1:5190';
  const rows=Array.from({length:100},(_,i)=>({reference:`bulk-${i}`,source_code:`MOCK-BULK-SOURCE-${String(i).padStart(4,'0')}`}));
  for(const row of rows)f.partner.seed(row.source_code);
  const cards=f.service.issue(rows);
  const server=makeServer({service:f.service,config:{mode:'mock',publicOrigin:origin,adminToken:'admin-test-token-32-characters-long',integrationToken:'integration-test-token-32-characters',examples:[]},staticDir:'.'});
  await new Promise(r=>server.listen(0,'127.0.0.1',r));t.after(()=>server.close());
  const base=`http://127.0.0.1:${server.address().port}/api/`,boot=await fetch(base+'session'),cookie=boot.headers.get('set-cookie').split(';')[0];
  const post=async(path,input)=>{const response=await fetch(base+path,{method:'POST',headers:{'Content-Type':'application/json',Origin:origin,'X-Pai-Request':'1',Cookie:cookie},body:JSON.stringify(input)});assert.equal(response.status,200,path);return (await response.json()).data;};
  const checks=[];
  for(const c of cards){const a=await post('preview',{code:c.code});checks.push(await post('preflight',{attempt_id:a.attempt_id,session}));}
  assert.equal(f.partner.calls.filter(c=>c.operation==='redeem').length,0);
  for(const a of checks)await post('redeem',{attempt_id:a.attempt_id,confirmation:a.confirmation,confirmed:true});
  const results=await post('results',{codes:cards.map(c=>c.code)});
  assert.equal(results.results.length,100);assert.ok(results.results.every(r=>r.submitted));
  assert.equal(f.partner.calls.filter(c=>c.operation==='redeem').length,100);
});
