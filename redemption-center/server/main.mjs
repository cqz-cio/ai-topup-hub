import { resolve } from 'node:path';
import { realpathSync } from 'node:fs';
import { fileURLToPath } from 'node:url';
import { Store } from './store.mjs';
import { vault, id } from './security.mjs';
import { Partner } from './partner.mjs';
import { MockPartner } from './mock.mjs';
import { RedemptionService } from './service.mjs';
import { makeServer } from './http.mjs';

export function start(env=process.env) {
  const mode=env.REDEMPTION_MODE;
  if(!['mock','live'].includes(mode))throw new Error('Set REDEMPTION_MODE explicitly to mock or live');
  const host=env.REDEMPTION_HOST||'127.0.0.1',port=Number(env.REDEMPTION_PORT||8190);
  if(mode==='mock' && !['127.0.0.1','::1'].includes(host))throw new Error('Mock mode must bind loopback');
  const publicOrigin=new URL(env.REDEMPTION_PUBLIC_ORIGIN||'http://127.0.0.1:5190').origin;
  if(mode==='live' && !publicOrigin.startsWith('https://'))throw new Error('Live mode requires an HTTPS public origin');
  if(mode==='mock' && !['127.0.0.1','localhost','[::1]'].includes(new URL(publicOrigin).hostname))throw new Error('Mock public origin must be loopback');
  for(const key of ['REDEMPTION_ADMIN_TOKEN','REDEMPTION_INTEGRATION_TOKEN'])if((env[key]||'').length<32)throw new Error(`${key} must have at least 32 characters`);
  if(env.REDEMPTION_ADMIN_TOKEN===env.REDEMPTION_INTEGRATION_TOKEN)throw new Error('Use separate admin and integration credentials');
  if(env.REDEMPTION_MANAGEMENT_TOKEN && (env.REDEMPTION_MANAGEMENT_TOKEN.length<32 || [env.REDEMPTION_ADMIN_TOKEN,env.REDEMPTION_INTEGRATION_TOKEN].includes(env.REDEMPTION_MANAGEMENT_TOKEN)))throw new Error('Use a separate management credential with at least 32 characters');
  const store=new Store(env.REDEMPTION_DB||resolve('runtime/redemption.db'));
  const partner=mode==='mock'?new MockPartner(store):new Partner({password:env.CDK_PARTNER_PASSWORD});
  const service=new RedemptionService({store,vault:vault(env.REDEMPTION_MASTER_KEY),partner,publicOrigin,mock:mode==='mock'});
  const config={mode,publicOrigin,trustProxy:env.REDEMPTION_TRUST_PROXY==='loopback',adminToken:env.REDEMPTION_ADMIN_TOKEN,integrationToken:env.REDEMPTION_INTEGRATION_TOKEN,managementToken:env.REDEMPTION_MANAGEMENT_TOKEN,examples:[]};
  const examples=group=>{
    const result=[];
    for(const [i,scenario,product,label] of [[1,'success','gpt','GPT 正常充值'],[2,'success','grok','Grok 无预检票据'],[3,'blocked','gpt','原订阅阻断'],[4,'uncertain','gpt','提交超时后恢复'],[5,'failed','gpt','终态失败'],[6,'success','claude','Claude 充值'],[7,'success','x','X 充值'],[8,'success','gpt','批量备用']]) {
      const source=group?`MOCKCARD-${group}-${i}`:`MOCKCARD-${String(i).padStart(10,'0')}-ABCDEFGHIJ`;
      partner.seed(source,scenario,product);
      const card=service.issue([{reference:group?`mock:${group}:${i}`:`mock:${i}`,source_code:source,label}])[0];
      result.push({code:card.code,label,session:JSON.stringify({demo:true,email:`demo${i}@example.com`})});
    }
    return result;
  };
  if(mode==='mock') {
    config.examples=examples(store.get("SELECT value FROM meta WHERE key='mock_group'")?.value);
    config.newExamples=()=>{const group=id();const cards=examples(group);store.run("INSERT OR REPLACE INTO meta(key,value) VALUES('mock_group',?)",group);return cards;};
  }
  const server=makeServer({service,config,staticDir:env.REDEMPTION_STATIC_DIR||resolve('dist/client')});
  server.requestTimeout=180000;server.headersTimeout=15000;
  const cleanup=setInterval(()=>service.expire(),60000);cleanup.unref();
  const reconcile=setInterval(()=>service.reconcile().catch(()=>{}),3000);reconcile.unref();
  const cardSync=setInterval(()=>service.management.reconcile().catch(()=>{}),30000);cardSync.unref();
  server.listen(port,host,()=>console.log(`PAIPLUS ${mode} API listening on ${host}:${port}`));
  let stopped=false;
  const stop=()=>{if(stopped)return;stopped=true;clearInterval(cleanup);clearInterval(reconcile);clearInterval(cardSync);server.close(async()=>{try{await Promise.allSettled([service.reconciling,service.management.reconciling,...service.management.jobs.values()]);}finally{store.close();}});};
  process.once('SIGTERM',stop);process.once('SIGINT',stop);
  return {server,service,config,stop};
}
if(process.argv[1] && realpathSync(process.argv[1])===fileURLToPath(import.meta.url))start();

