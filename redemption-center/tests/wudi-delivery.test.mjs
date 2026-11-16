import test from 'node:test';
import assert from 'node:assert/strict';
import { randomBytes } from 'node:crypto';
import { Store } from '../server/store.mjs';
import { vault } from '../server/security.mjs';
import { RedemptionService } from '../server/service.mjs';
import { WudiPartner } from '../server/wudi.mjs';
import { makeServer } from '../server/http.mjs';

// The production issuer, HTTP API, SQLite persistence and Wudi serializer are
// real. Only the supplier's external create/status responses are simulated.
export async function deliveryScenario({quantity=1,account=false,supplierLabels=false}={}) {
  const store=new Store(':memory:'),crypt=vault(randomBytes(32).toString('base64'));
  const integration='test-shop-integration-token-never-use-in-production',key='wpk_simulated_partner_key_not_a_real_credential';
  const requests=new Map(),intents=new Map(),calls=[];let loseResponse=true,customerConfirmed=false;
  const partner=new WudiPartner({key,fetchImpl:async(url,options)=>{
    const body=JSON.parse(options.body);calls.push({url,body});
    assert.equal(options.headers.Authorization,'Bearer '+key);assert.equal(options.headers.Origin,undefined);assert.equal(options.headers.Cookie,undefined);
    if(url.endsWith('/redemptions')) {
      let result=requests.get(body.client_request_id);
      if(result)assert.equal(result.code,body.code);
      else {
        const id='intent-'+(requests.size+1);
        result={code:body.code,response:{ok:true,id,client_request_id:body.client_request_id,status:'awaiting_customer',redeem_url:'https://wudi.cdk.bar/#partner=simulated-private-token-'+id,expires_at:'2099-11-15T12:00:00Z'}};
        requests.set(body.client_request_id,result);intents.set(id,result);
      }
      if(loseResponse){loseResponse=false;throw new Error('Simulated response lost after provider persisted intent');}
      return Response.json(result.response);
    }
    assert.ok(url.endsWith('/status'));assert.ok(intents.has(body.id));
    return Response.json({ok:true,id:body.id,status:customerConfirmed?'succeeded':'awaiting_customer'});
  }});
  const unavailableLegacy={call:async()=>{throw new Error('No legacy network access in the new delivery test');}};
  let service=new RedemptionService({store,vault:crypt,partner:unavailableLegacy,linkPartner:partner,publicOrigin:'https://redeem.example.test',mock:true});
  const server=makeServer({service,config:{mode:'mock',publicOrigin:'https://redeem.example.test',integrationToken:integration,examples:[]},staticDir:'.'});
  await new Promise(r=>server.listen(0,'127.0.0.1',r));
  const url='http://127.0.0.1:'+server.address().port;
  let cookie='';
  const post=async(path,body,privateAPI=false)=>{
    const response=await fetch(url+'/api/'+path,{method:'POST',headers:{'Content-Type':'application/json',...(privateAPI?{Authorization:'Bearer '+integration}:{Origin:'https://redeem.example.test','X-Pai-Request':'1',Cookie:cookie})},body:JSON.stringify(body)});
    if(response.headers.has('set-cookie'))cookie=response.headers.get('set-cookie').split(';')[0];
    assert.equal(response.status,200,path);assert.match(response.headers.get('cache-control'),/no-store/);
    const result=await response.json();assert.equal(result.ok,true,path);return result.data;
  };
  try {
    const sources=Array.from({length:quantity},(_,i)=>'ELLPLUS-TESTSOURCE'+String(i).padStart(5,'0')+'-ABCDEFGH');
    const raw=account?'邮箱号：buyer@example.invalid 邮箱密码：Test-password#1 密钥2fa：TESTSEED 密钥2fa在线获取：https://auth.example.invalid/2fa 兑换卡密：'+sources[0]:supplierLabels?sources.map(code=>'卡密cdk：'+code).join('\n')+'\n兑换站：wudi.cdk.bar':sources.join('\n');
    const order={reference:'shop:1:order:90001',payload:raw,quantity,metadata:{order_id:90001,order_no:'SIMULATED-ORDER-90001',product_id:30,sku_id:30,product_name:'模拟 ChatGPT Plus'}};
    const delivered=await post('integration/deliver',order,true);
    assert.equal(delivered.format,'paiplus-v1');assert.equal(delivered.count,quantity);assert.equal(delivered.cards.length,quantity);
    const codes=delivered.cards.map(c=>c.code);assert.equal(new Set(codes).size,quantity);
    for(const code of codes){assert.match(code,/^PAIPLUS-[A-HJ-NP-Z2-9]{10}-[A-HJ-NP-Z2-9]{10}$/);assert.ok(delivered.payload.includes(code));}
    assert.ok(delivered.payload.includes('https://redeem.example.test/'));for(const source of sources)assert.ok(!JSON.stringify(delivered).includes(source));
    if(account){assert.ok(delivered.payload.includes('buyer@example.invalid'));assert.ok(delivered.payload.includes('Test-password#1'));assert.ok(delivered.payload.includes('TESTSEED'));}
    // Simulate the shop retrying a lost delivery response: exactly the same codes.
    assert.deepEqual(await post('integration/deliver',order,true),delivered);
    assert.equal(store.get('SELECT count(*) n FROM deliveries').n,1);assert.equal(store.get('SELECT count(*) n FROM cards').n,quantity);assert.equal(calls.length,0);
    const initial=await post('partner-link',{code:codes[0]});assert.equal(initial.state,'link_pending');assert.equal(initial.redemption_code,undefined);
    const requestID=store.get('SELECT request_id FROM partner_redemptions').request_id;
    // Rebuild the service around its durable database to simulate restart.
    service=new RedemptionService({store,vault:crypt,partner:unavailableLegacy,linkPartner:partner,publicOrigin:'https://redeem.example.test',mock:true});
    store.run('UPDATE partner_redemptions SET next_poll=0');await service.reconcile();
    assert.equal(store.get('SELECT request_id FROM partner_redemptions').request_id,requestID);
    for(const [i,code] of codes.entries()) {
      const link=await post('partner-link',{code});assert.equal(link.state,'awaiting_customer');assert.equal(link.redemption_code,sources[i]);assert.match(link.redeem_url,/^https:\/\/wudi\.cdk\.bar\/#partner=/);
      if(i===0){const repeated=await post('partner-link',{code});assert.equal(repeated.attempt_id,link.attempt_id);assert.equal(repeated.redeem_url,link.redeem_url);}
    }
    assert.equal(requests.size,quantity);assert.equal(store.get("SELECT count(*) n FROM cards WHERE status='completed'").n,0);
    customerConfirmed=true;store.run('UPDATE partner_redemptions SET next_poll=0');
    for(let i=0;i<Math.ceil(quantity/4);i++)await service.reconcile();
    const results=await post('results',{codes});assert.equal(results.results.length,quantity);
    assert.ok(results.results.every(r=>r.state==='succeeded'));assert.equal(store.get("SELECT count(*) n FROM cards WHERE status='completed'").n,quantity);
    for(const source of sources)assert.ok(!JSON.stringify(results).includes(source));assert.ok(!JSON.stringify(results).includes('simulated-private-token'));
    assert.equal(requests.size,quantity);
    return {quantity,account,delivery_format:'paiplus-v1',delivery_idempotent:true,private_source_hidden_in_delivery:true,request_ids_retained_after_restart:true,unique_partner_intents:requests.size,completed:quantity,real_upstream_requests:0};
  } finally {await new Promise(r=>{server.close(r);server.closeAllConnections();});store.close();}
}

for(const scenario of [{quantity:1},{quantity:100},{quantity:1,account:true}])test(`shop HTTP delivery to Wudi completion: ${scenario.quantity} card(s), account=${!!scenario.account}`,async()=>{await deliveryScenario(scenario);});
test('supplier cdk/station labels can be delivered and redeemed without exposing upstream text',async()=>{await deliveryScenario({supplierLabels:true});});
