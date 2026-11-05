import { id } from './security.mjs';
import { PartnerError } from './partner.mjs';

// This provider is available only in explicit, loopback-only mock mode.
export class MockPartner {
  constructor(store) { this.store=store; this.calls=[]; }
  seed(code,scenario='success',product='gpt') { this.store.run("INSERT OR IGNORE INTO mock_tasks(code,status,scenario,product) VALUES(?,'unused',?,?)",code,scenario,product); }
  async call(operation,body,device) {
    this.calls.push({operation,device,requestId:body.client_request_id});
    if(this.calls.length>1000)this.calls.shift();
    if(operation==='lookup-batch')return {results:await Promise.all(body.codes.map(code=>this.call('lookup',{code},device).catch(()=>({status:'unknown'}))))};
    if(operation==='billing') {
      let account;
      if(body.cdk_code)account=this.store.get('SELECT email FROM mock_tasks WHERE code=?',body.cdk_code)?.email;
      else {let value;try{value=JSON.parse(body.token_input);}catch{}if(value?.demo===true)account=value.email;}
      if(!account)throw new PartnerError('UPSTREAM_REJECTED',422);
      return {summary:{email:account,plan_type:'plus',has_active_subscription:true},last_payment:null,invoices:[],invoice_data_available:true,invoice_url:null};
    }
    let task=this.store.get('SELECT * FROM mock_tasks WHERE code=? OR token=?',body.code||'',body.redemption_token||body.token||'');
    if(!task)throw new PartnerError('UPSTREAM_NOT_FOUND',404);
    if(operation==='preview') {
      const token=task.token||id(); this.store.run('UPDATE mock_tasks SET token=? WHERE code=?',token,task.code);
      return {redemption_token:token,product:task.product,plan:task.product==='grok'?'super':'plus',payment_country:'PH',payment_currency:'PHP'};
    }
    if(operation==='preflight') {
      let value; try { value=JSON.parse(body.credential.session); } catch { throw new PartnerError('UPSTREAM_REJECTED',422); }
      if(!value.email || value.demo!==true)throw new PartnerError('UPSTREAM_REJECTED',422);
      this.store.run('UPDATE mock_tasks SET email=? WHERE code=?',value.email,task.code);
      return {email:value.email,currentPlan:'free',...(task.product==='grok'?{}:{preflight_token:'mock-'+id()}),blocking_reason:task.scenario==='blocked'?'原订阅需要人工处理':'',subscription_recovery_required:task.scenario==='blocked'};
    }
    if(operation==='redeem') {
      if(task.submitted_at)throw new Error('Duplicate submission detected by mock');
      this.store.run("UPDATE mock_tasks SET status='processing',submitted_at=? WHERE code=?",Date.now(),task.code);
      if(task.scenario==='uncertain')throw new PartnerError('UPSTREAM_NETWORK',502,true);
      return {status:'queued'};
    }
    if(['result','result-by-code'].includes(operation)) {
      if(task.status==='unused')throw new PartnerError('UPSTREAM_NOT_FOUND',404);
      const status=Date.now()-task.submitted_at>1500?(task.scenario==='failed'?'failed_precharge':'completed'):'processing';
      this.store.run('UPDATE mock_tasks SET status=? WHERE code=?',status,task.code);
      return {status,account_email:task.email};
    }
    if(operation==='lookup')return {cdk_code:task.code,status:task.status==='completed'?'used':task.status,used:task.status==='completed',can_resubmit:task.status==='unused',account_email:task.email,plan:task.product==='grok'?'super':'plus'};
    throw new Error('Unsupported mock operation');
  }
}
