import assert from 'node:assert/strict';
import {randomBytes} from 'node:crypto';
import test from 'node:test';
import {Store} from '../server/store.mjs';
import {vault} from '../server/security.mjs';
import {PartnerError} from '../server/partner.mjs';
import {RedemptionService} from '../server/service.mjs';

const address='viewer@example.test',device='device-demo-read-only-test',session=JSON.stringify({user:{email:address},sessionToken:'synthetic-session-secret'});
const bill={summary:{email:address,plan_type:'pro',subscription_plan:'pro_20x',has_active_subscription:true,will_renew:false,
  active_until:'2026-11-26T11:00:44Z',settlement_currency:'USD'},last_payment:{paidAt:'2026-10-26T11:00:12Z',status:'paid'},
  payment_method:{brand:'visa',last4:'4242',number:'private-raw-payment'}};
async function fixture(t,{check={email:address,currentPlan:'pro',subscription_has_active:true},billing=bill,product='gpt',plan='plus',flow}={}){
  const store=new Store(':memory:');t.after(()=>store.close());
  const calls=[];
  const partner={async call(op,body,sourceDevice,options){
    calls.push(op);assert.equal(sourceDevice,device);
    if(op==='preview')return {product,plan,plan_flow:flow,redemption_token:'test-preview'};
    if(op==='preflight'){if(check instanceof Error)throw check;return check;}
    if(op==='billing'){
      assert.deepEqual(body,{token_input:session});assert.ok(options.timeout<=5000);
      if(billing instanceof Error)throw billing;
      return billing;
    }
    assert.fail('Unexpected provider operation: '+op);
  }};
  const service=new RedemptionService({store,vault:vault(randomBytes(32).toString('base64')),partner,publicOrigin:'https://example.test'});
  const card=service.issue([{reference:'read-only-case',source_code:'SYNTHETIC-SOURCE-CODE'}])[0];
  const {attempt_id:id}=await service.preview(card.code,device);
  return {id,store,service,card,calls};
}
test('a blocked legacy check returns subscription and bill only to the fresh response',async t=>{
  const {service,id,card,calls,store}=await fixture(t);
  const a=await service.preflight(id,session,device,()=>{},true);
  assert.equal(a.state,'blocked');assert.equal(a.confirmation,null);
  for(const expected of [address,'Pro 20x','2026-11-26 19:00:44','2026-10-26 19:00:12','已关闭','USD','Visa · 4242'])assert.ok(a.blocking_reason.includes(expected));
  for(const v of [service.data(store.get('SELECT * FROM attempts')),service.recent(device),await service.result(id,device),await service.recover(card.code,'device-another-test-owner')]){
    assert.doesNotMatch(JSON.stringify(v),/last_paid_at|settlement_currency|2026-10-26|4242|private-raw-payment|synthetic-session-secret/);
  }
  await assert.rejects(service.redeem(id,'fake',true,device),{code:'CONFIRM_REQUIRED'});
  assert.deepEqual(calls,['preview','preflight','billing']);
});
test('a rejected preflight can display a separately verified bill and can never grant a charge',async t=>{
  const {service,id,calls,store}=await fixture(t,{check:new PartnerError('UPSTREAM_REJECTED',422)});
  let reads=0;const a=await service.preflight(id,session,device,()=>reads++);
  assert.deepEqual([a.state,a.email,a.confirmation,reads],['blocked',address,null,1]);
  assert.equal(a.preflight_details.current_plan,'pro_20x');assert.equal(a.preflight_details.active_until,Date.parse(bill.summary.active_until));
  assert.equal(a.preflight_details.will_renew,false);assert.doesNotMatch(JSON.stringify(a),/private-raw-payment|synthetic-session-secret/);
  await assert.rejects(service.redeem(id,'forged',true,device),{code:'CONFIRM_REQUIRED'});
  const row=store.get('SELECT * FROM attempts');assert.equal(row.session_enc,'');assert.equal(row.preflight_enc,'');
  assert.deepEqual(calls,['preview','preflight','billing']);
});
test('a structured refusal without an email accepts only the same independently verified Session account',async t=>{
  const {service,id}=await fixture(t,{check:{can_redeem:false}});
  const a=await service.preflight(id,session,device,()=>{});
  assert.equal(a.email,address);assert.equal(a.preflight_details.current_plan,'pro_20x');assert.equal(a.confirmation,null);
});
test('a different account, rejected bill, absent Session proof, or quota failure cannot expose a rejected account',async t=>{
  for(const mode of ['different','denied','missing-proof','quota','no-billing','provider-network']){
    const check=new PartnerError(mode==='provider-network'?'UPSTREAM_NETWORK':'UPSTREAM_REJECTED',mode==='provider-network'?502:422);
    const billing=mode==='different'?{...bill,summary:{...bill.summary,email:'other@example.test'}}:mode==='denied'?new PartnerError('UPSTREAM_REJECTED',422):bill;
    const {service,id,calls}=await fixture(t,{check,billing});
    const credential=mode==='missing-proof'?JSON.stringify({user:{email:address},accessToken:'synthetic-access'}):session;
    const reserve=mode==='no-billing'?undefined:()=>{if(mode==='quota')throw Error('rate limited');};
    await assert.rejects(service.preflight(id,credential,device,reserve),{code:check.code});
    const view=await service.result(id,device);
    assert.equal(view.state,'preview');assert.equal(view.email,null);assert.equal(view.confirmation,null);
    assert.equal(calls.filter(v=>v==='billing').length,['different','denied'].includes(mode)?1:0);
  }
});
test('a legacy eligible row does not use the optional billing quota',async t=>{
  const {service,id,calls}=await fixture(t,{check:{email:address,currentPlan:'free',subscription_has_active:false,preflight_token:'synthetic-ticket'}});
  const a=await service.preflight(id,session,device,()=>assert.fail('unexpected billing'),true);
  assert.equal(a.state,'checked');assert.ok(a.confirmation);assert.deepEqual(calls,['preview','preflight']);
});

test('an unavailable eligibility check can return a fresh read-only subscription without storing it',async t=>{
  const failures=[new PartnerError('UPSTREAM_ERROR',502,false,503),new PartnerError('UPSTREAM_NETWORK'),
    new PartnerError('UPSTREAM_RESPONSE',502,false,200),new PartnerError('UPSTREAM_ERROR',502,false,409)];
  for(const failure of failures){
    const {service,id,store,calls,card}=await fixture(t,{check:failure});
    const result=await service.preflight(id,session,device,()=>{},false,true);
    assert.deepEqual([result.state,result.subscription_only,result.blocked,result.confirmation,result.email],['preview',true,false,null,address]);
    const details=result.preflight_details;
    assert.equal(details.current_plan,'pro_20x');
    assert.equal(details.active_until,Date.parse(bill.summary.active_until));
    assert.equal(details.last_paid_at,Date.parse(bill.last_payment.paidAt));
    assert.equal(details.will_renew,false);
    assert.equal(details.settlement_currency,'USD');
    assert.doesNotMatch(JSON.stringify(result),/synthetic-session-secret|private-raw-payment/);
    const saved=store.get('SELECT * FROM attempts');
    for(const key of ['session_enc','preflight_enc','confirmation_hash'])assert.equal(saved[key],'');
    assert.equal(saved.checked_at,null);assert.equal(saved.submitted_at,null);
    const other='device-other-read-only-test';
    for(const view of [service.data(saved),service.recent(device),await service.result(id,device),await service.recover(card.code,other)]){
      assert.doesNotMatch(JSON.stringify(view),/viewer@example|pro_20x|4242|active_until|preflight_details|subscription_only/);
    }
    await assert.rejects(service.redeem(id,'forged',true,device),{code:'CONFIRM_REQUIRED'});
    assert.deepEqual(calls,['preview','preflight','billing']);
  }
});

test('independent billing on failure respects identity, upstream denials, client capability and quota',async t=>{
  const serviceError=new PartnerError('UPSTREAM_ERROR',502,false,503);
  const scenarios=[
    {billing:{...bill,summary:{...bill.summary,email:'other@example.test'}},reads:1},
    {billing:new PartnerError('UPSTREAM_NETWORK'),reads:1},
    {credential:JSON.stringify({user:{email:address},accessToken:'synthetic-access'})},
    {reserve:()=>{throw Error('quota');}},{reserve:null},{optIn:false},{product:'grok'},
    {check:new PartnerError('UPSTREAM_ERROR',502,false,401)},{check:new PartnerError('UPSTREAM_RESPONSE',502,false,403)},
    {check:new PartnerError('UPSTREAM_RATE_LIMIT',429,false,429)},{check:new PartnerError('UPSTREAM_RESPONSE',502,false,429)},
    {check:new PartnerError('UPSTREAM_NOT_FOUND',404,false,404)},
  ];
  for(const s of scenarios){
    const check=s.check??serviceError;
    const {service,id,calls}=await fixture(t,{check,billing:s.billing??bill,product:s.product});
    const reserve=Object.hasOwn(s,'reserve')?s.reserve:()=>{};
    await assert.rejects(service.preflight(id,s.credential??session,device,reserve,false,s.optIn??true),{code:check.code});
    assert.equal(calls.filter(op=>op==='billing').length,s.reads??0);
    const view=await service.result(id,device);
    assert.equal(view.state,'preview');assert.equal(view.email,null);assert.equal(view.confirmation,null);
    assert.ok(!view.preflight_details);
  }
});

test('a successful independent bill cannot reuse an earlier recharge confirmation',async t=>{
  const {service,id}=await fixture(t,{check:{email:address,currentPlan:'free',subscription_has_active:false,preflight_token:'synthetic-old-ticket'}});
  const eligible=await service.preflight(id,session,device);
  assert.equal(eligible.state,'checked');assert.ok(eligible.confirmation);
  const original=service.partner.call.bind(service.partner);
  service.partner.call=async(op,...args)=>{
    if(op==='preflight')throw new PartnerError('UPSTREAM_NETWORK');
    return original(op,...args);
  };
  const pending=await service.preflight(id,session,device,()=>{},false,true);
  assert.equal(pending.subscription_only,true);assert.equal(pending.confirmation,null);
  await assert.rejects(service.redeem(id,eligible.confirmation,true,device),{code:'CONFIRM_REQUIRED'});
});

test('only verified same-tier or higher active plans get a mismatch notice on the read-only subscription panel', async t=>{
  const cases=[
    {current:'pro',active:true,flow:'checkout',mismatch:true},
    {current:'plus',active:true,mismatch:true},
    {current:'pro_20x',active:true,flow:'card_attach'},
    {current:'pro_20x',active:true,plan:'pro_20x_renew'},
    {current:'plus',active:true,plan:'pro_20x'},
    {current:'pro',active:true,expired:true},
    {current:'pro',active:false},{current:'pro',active:null},
    {current:'free',active:false},{current:null,active:true},{current:'unrecognized',active:true}
  ];
  for(const c of cases){
    const summary={email:address,plan_type:c.current,has_active_subscription:c.active,
      active_until:new Date(Date.now()+(c.expired?-1:1)*86400000).toISOString()};
    const {service,id}=await fixture(t,{check:new PartnerError('UPSTREAM_NETWORK'),billing:{summary},plan:c.plan,flow:c.flow});
    const result=await service.preflight(id,session,device,()=>{},false,true);
    assert.equal(result.plan_mismatch,Boolean(c.mismatch),JSON.stringify(c));
    assert.equal(result.preflight_details.current_plan,c.current);
    assert.equal(result.subscription_only,true);assert.equal(result.state,'preview');assert.equal(result.confirmation,null);
    assert.ok(!Object.hasOwn(await service.result(id,device),'plan_mismatch'));
  }
});

test('a completed preflight explains the same plan mismatch while keeping subscription recovery instructions',async t=>{
  for(const recovery of [false,true]){
    const {service,id}=await fixture(t,{check:{email:address,currentPlan:'pro',subscription_has_active:true,subscription_recovery_required:recovery},
      billing:{summary:{email:address,plan_type:'pro',has_active_subscription:true}}});
    const result=await service.preflight(id,session,device,()=>{});
    assert.equal(result.state,'blocked');assert.equal(result.confirmation,null);
    assert.equal(result.plan_mismatch===true,!recovery);
    if(recovery)assert.match(result.blocking_reason,/原订阅需要处理/);
  }
});
