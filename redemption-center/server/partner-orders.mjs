import { Problem, id, equal } from './security.mjs';
import { PartnerLinks } from './partner-links.mjs';
import { linkTerminal } from './wudi.mjs';
import { sessionObject } from './wudi-v2.mjs';

export class PartnerOrders extends PartnerLinks {
  constructor(service,partner) {super(service,partner);this.checks=new Map();this.subscriptions=new Map();}
  expire() {
    for(const [key,check] of this.checks)if(Date.now()>check.expires)this.checks.delete(key);
    for(const [key,query] of this.subscriptions)if(Date.now()>query.expires)this.subscriptions.delete(key);
  }
  patch(row,values) {
    const current=this.store.get('SELECT * FROM partner_redemptions WHERE id=?',row.id);
    this.store.run('UPDATE partner_redemptions SET data_enc=?,updated_at=? WHERE id=?',this.vault.encrypt(JSON.stringify({...this.data(current),...values})),Date.now(),row.id);
    return this.store.get('SELECT * FROM partner_redemptions WHERE id=?',row.id);
  }
  view(row) {
    const result=super.view(row),data=this.data(row);
    if(data.protocol!==2)return result;
    return {...result,protocol:2,can_open:false,card:data.card||null,email:data.email||null,
      message:data.message||(row.state==='link_pending'?'正在验证卡密，请稍后查询。':result.message),
      preflight_message:data.preflight_message||null,error_message:data.error_code?data.message:null,
      can_check:!data.submission_pending&&!data.query_error&&['awaiting_customer','retryable_no_task'].includes(row.state)&&Date.parse(data.expires_at)>Date.now()};
  }
  async sync(row,force=false) {
    const result=await super.sync(row,force),data=this.data(result);
    if(data.protocol===2&&['queued','running','manual_review','uncertain','captcha_required','retryable_no_task'].includes(result.state)&&!data.query_error&&result.next_poll>Date.now()+5000)
      this.store.run('UPDATE partner_redemptions SET next_poll=? WHERE id=?',Date.now()+5000,result.id);
    if(linkTerminal(result.state))this.checks.delete(result.id);
    // Only an explicit upstream no-task result permits another preflight on a submitted order.
    if(data.submission_pending&&result.state==='retryable_no_task'&&!data.query_error)return this.patch(result,{submission_pending:false});
    return result;
  }
  owned(input) {
    const card=this.service.card(input.code);this.service.assertUsable(card);
    const row=this.store.get('SELECT * FROM partner_redemptions WHERE id=? AND card_id=?',String(input.attempt_id||''),card.id);
    if(!row||row.key_hash!==this.keyHash||this.data(row).protocol!==2)throw new Problem('ATTEMPT_NOT_FOUND','请用原本站卡密重新验证订单。',404);
    if(this.latest(card.id)?.id!==row.id)throw new Problem('STALE_ORDER','请使用这张卡密的最新订单。',409);
    return {row,card};
  }
  checkView(row,check) {return {...this.view(row),preflight:check.result||{status:'running',message:'正在验证账号，请稍候。'},confirmation:check.result?.status==='ready'&&Date.now()<check.expires?check.confirmation:null};}
  async check(input,device) {
    this.expire();const {row:original,card}=this.owned(input);
    if(input.acknowledged!==true)throw new Problem('ACKNOWLEDGEMENT_REQUIRED','请先阅读并同意账号预检说明。',409);
    const session=sessionObject(input.session),owner=this.vault.hash('device',device),existing=this.checks.get(original.id);
    if(existing?.busy)throw new Problem('BUSY','该订单正在预检，请稍候。',409);
    if(existing&&['queued','running'].includes(existing.result?.status)) {
      if(existing.owner===owner&&JSON.stringify(existing.session)===JSON.stringify(session))return this.checkView(original,existing);
      throw new Problem('BUSY','原账号仍在预检，请等待原预检结果。',409);
    }
    if([...this.checks.values()].filter(check=>check.busy||['queued','running'].includes(check.result?.status)).length>=2)throw new Problem('RATE_LIMITED','已有两个账号正在预检，请稍后再试。',429);
    const row=await this.sync(original,true);
    if(!this.view(row).can_check)throw new Problem('PREFLIGHT_UNAVAILABLE','当前订单不能开始预检，请查询原订单结果。',409);
    // Install the lock before starting the external request. Session exists only in this short-lived map.
    if(this.checks.get(row.id)?.busy)throw new Problem('BUSY','该订单正在预检，请稍候。',409);
    const check={owner,session,busy:true,expires:Date.now()+10*60000,confirmation:id(),nextPoll:0};this.checks.set(row.id,check);
    try {
      check.result=await this.partner.accountCheck(this.data(row).id,this.vault.decrypt(card.source_enc),session);
      this.finishCheck(check);return this.checkView(row,check);
    } catch(error){this.checks.delete(row.id);throw error;}finally{check.busy=false;}
  }
  finishCheck(check) {
    const result=check.result;
    if(result.status==='ready')check.expires=Math.min(Date.parse(result.expiresAt)||0,Date.now()+120000);
    if(result.status==='failed'){check.session=null;check.confirmation=null;check.expires=Date.now()+120000;}
    check.nextPoll=Date.now()+3000;
  }
  async checkStatus(input,device) {
    this.expire();const {row}=this.owned(input),check=this.checks.get(row.id);
    if(!check||check.owner!==this.vault.hash('device',device))throw new Problem('PREFLIGHT_EXPIRED','预检已失效，请在本浏览器重新预检。',409);
    if(check.busy||Date.now()<check.nextPoll||['ready','failed'].includes(check.result?.status))return this.checkView(row,check);
    check.busy=true;
    try{check.result=await this.partner.accountStatus(this.data(row).id,check.result.id);this.finishCheck(check);return this.checkView(row,check);}finally{check.busy=false;}
  }
  async confirm(input,device) {
    this.expire();let {row,card}=this.owned(input),data=this.data(row);
    if(input.confirmed!==true)throw new Problem('CONFIRM_REQUIRED','请核对账号和套餐并确认兑换。',409);
    if(data.submission_pending||!['awaiting_customer','retryable_no_task'].includes(row.state))return this.view(await this.sync(row,true));
    const check=this.checks.get(row.id);
    if(!check||check.owner!==this.vault.hash('device',device)||check.busy||check.result?.status!=='ready'||!equal(check.confirmation,input.confirmation)||!check.session||Date.now()>=check.expires)
      throw new Problem('PREFLIGHT_EXPIRED','预检授权已失效或不匹配，请重新预检后确认。',409);
    // Durable submission marker precedes the request, so crashes/timeouts can only resume status queries.
    row=this.patch(row,{submission_pending:true,message:'正在提交兑换，请稍后查询原订单。'});
    this.store.run("UPDATE partner_redemptions SET state='uncertain',next_poll=0 WHERE id=?",row.id);
    this.checks.delete(row.id);
    try {
      const response=await this.partner.redeem(data.id,this.vault.decrypt(card.source_enc),check.session,check.result.id);
      this.patch(row,response);this.store.run('UPDATE partner_redemptions SET state=?,next_poll=? WHERE id=?',response.status,Date.now()+5000,row.id);
      if(linkTerminal(response.status))this.store.run('UPDATE cards SET status=?,updated_at=? WHERE id=?',response.status==='succeeded'?'completed':'failed',Date.now(),card.id);
    } catch(error) {
      const rejected=error.status<500&&/^(PREFLIGHT_|SESSION_|CONFIRM_REQUIRED)/.test(error.code||'');
      this.patch(row,{query_error:error.message,...(rejected?{submission_pending:false}:{}),preflight_message:rejected?error.message:null});
    } finally {check.session=null;}
    return this.view(await this.sync(this.store.get('SELECT * FROM partner_redemptions WHERE id=?',row.id),true));
  }
  async recover(code) {
    const card=this.service.card(code);this.service.assertUsable(card);
    const current=this.latest(card.id);
    if(current)return super.recover(code);
    // Legacy attempts retain their original provider and cannot be rebound or reissued.
    if(card.attempt_id||card.status==='completed')return null;
    const result=await this.partner.lookup(this.vault.decrypt(card.source_enc));
    return result.order?this.remoteView(result.order):null;
  }
  remoteView(order) {
    return {provider:'wudi',protocol:2,state:order.status,message:order.message,card:order.card||null,email:order.email||null,error_code:order.error_code,preflight_error_code:order.preflight_error_code,preflight_message:order.preflight_message,created_at:order.created_at,updated_at:order.updated_at,can_retry:false,can_check:false};
  }
  async orders(input) {
    const card=this.service.card(input.code);this.service.assertUsable(card);
    const result=await this.partner.orders(this.vault.decrypt(card.source_enc),input.before_id);
    return {orders:result.orders.map(row=>this.remoteView(row)),next_before_id:result.next_before_id};
  }
  async querySubscription(input,device) {
    this.expire();if(this.subscriptions.size>=1000)throw new Problem('QUERY_CAPACITY','查询繁忙，请稍后再试。',429);
    const result=await this.partner.querySubscription(sessionObject(input.session)),queryID=id();
    this.subscriptions.set(queryID,{owner:this.vault.hash('device',device),upstreamID:result.id,expires:Date.now()+600000,nextPoll:Date.now()+4000,result});
    return this.subscriptionView(queryID,result);
  }
  subscriptionView(queryID,result) {return {query_id:queryID,status:result.status,message:result.message,code:result.code,result:result.result};}
  async subscriptionStatus(input,device) {
    this.expire();const query=this.subscriptions.get(String(input.query_id||''));
    if(!query||query.owner!==this.vault.hash('device',device))throw new Problem('QUERY_EXPIRED','查询已过期，请重新粘贴 Session 查询。',404);
    if(!query.busy&&Date.now()>=query.nextPoll&&!['success','failed'].includes(query.result.status)) {
      query.busy=true;query.nextPoll=Date.now()+4000;
      try{query.result=await this.partner.subscriptionStatus(query.upstreamID);}finally{query.busy=false;}
    }
    return this.subscriptionView(input.query_id,query.result);
  }
}
