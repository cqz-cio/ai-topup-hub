import { Problem, id, maskedCard } from './security.mjs';
import { linkTerminal, reasonMessages, redeemURL } from './wudi.mjs';

const messages={link_pending:'正在创建兑换链接，请稍后查询原记录。',awaiting_customer:'链接已准备好，请前往兑换站核对账号并确认。',queued:'已进入兑换队列。',running:'正在兑换，请稍后查询原记录。',captcha_required:'请返回原兑换链接完成验证。',succeeded:'充值已完成。',failed:'兑换失败，请核对原记录。',cancelled:'本次兑换已取消。',manual_review:'需要人工核实，请联系客服。',uncertain:'兑换结果待确认，请勿重复兑换。',expired:'确认前链接已到期。',card_unavailable:'卡密暂时不可用于本次兑换。',retryable_no_task:reasonMessages.NO_TASK_FOUND};

// Link creation grants an external redemption capability, even before charging.
// Never equate awaiting_customer (or lack of a local attempt) with refundable.
export class PartnerLinks {
  constructor(service,partner) {this.service=service;this.store=service.store;this.vault=service.vault;this.partner=partner;this.jobs=new Map();this.keyHash=this.vault.hash('wudi-key',partner.key);}
  latest(cardID) {return this.store.get('SELECT * FROM partner_redemptions WHERE card_id=? ORDER BY sequence DESC LIMIT 1',cardID);}
  data(row) {return row?.data_enc?JSON.parse(this.vault.decrypt(row.data_enc)):{};}
  view(row) {
    const d=this.data(row),c=this.store.get('SELECT * FROM cards WHERE id=?',row.card_id);
    return {attempt_id:row.id,provider:'wudi',state:row.state,code:maskedCard(this.vault.decrypt(c.code_enc)),label:c.label,
      message:messages[row.state]||messages.uncertain,error_code:d.error_code||null,preflight_error_code:d.preflight_error_code||null,
      preflight_message:reasonMessages[d.preflight_error_code]||null,error_message:reasonMessages[d.error_code]||null,
      query_error:d.query_error||null,expires_at:d.expires_at||null,created_at:row.created_at,updated_at:row.updated_at,
      can_retry:!d.query_error&&(row.state==='expired'||(row.state==='failed'&&d.card_reenabled===true)),
      can_open:!!d.redeem_url&&!['succeeded','failed','cancelled','expired','card_unavailable'].includes(row.state)};
  }
  async prepare(code,device,{retry=false}={}) {
    const c=this.service.card(code);this.service.assertUsable(c);
    let row=this.latest(c.id);
    if(retry) {
      if(!row)throw new Problem('RETRY_UNAVAILABLE','请先创建兑换链接。',409);
      row=await this.sync(row,true);
      if(!this.view(row).can_retry)throw new Problem('RETRY_UNAVAILABLE','当前结果不能新建兑换，请继续查询或使用原链接。',409);
    }
    if(!row || retry) {
      row=this.store.transaction(()=>{
        const card=this.store.get('SELECT * FROM cards WHERE id=?',c.id);this.service.assertUsable(card);
        const current=this.latest(c.id);
        if(current && (!retry || current.id!==row.id))return current;
        const old=card.attempt_id?this.store.get('SELECT * FROM attempts WHERE id=?',card.attempt_id):null;
        if(old?.submitted_at || ['preparing','checking'].includes(old?.state) || (!current && card.status!=='ready'))throw new Problem('LEGACY_RECORD','该卡密有原兑换记录，请查询原记录或联系客服，不能重新提交。',409);
        if(old)this.service.update(old,'cancelled',{}, {session_enc:'',preflight_enc:'',confirmation_hash:''});
        const now=Date.now(),rowID=id(),sequence=(current?.sequence||0)+1;
        this.store.run('INSERT INTO partner_redemptions(id,card_id,sequence,device_hash,request_id,key_hash,state,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)',rowID,c.id,sequence,this.vault.hash('device',device),'pai-'+id(),this.keyHash,'link_pending',now,now);
        this.store.run("UPDATE cards SET status='processing',updated_at=? WHERE id=?",now,c.id);
        this.store.audit('partner_link_requested',rowID);
        return this.store.get('SELECT * FROM partner_redemptions WHERE id=?',rowID);
      });
    }
    row=await this.sync(row);
    const result=this.view(row),d=this.data(row);
    // Private capability material is returned only after explicitly presenting
    // the full PAIPLUS code, never from session/history/result/admin summaries.
    if(result.can_open && !['revoked','revoking','disabled','invalid'].includes(this.store.get('SELECT status FROM cards WHERE id=?',c.id).status)) {
      result.redeem_url=redeemURL(d.redeem_url);result.redemption_code=this.vault.decrypt(c.source_enc);
      this.store.run('UPDATE partner_redemptions SET exposed_at=COALESCE(exposed_at,?) WHERE id=?',Date.now(),row.id);
    }
    return result;
  }
  async sync(row,force=false) {
    if(this.jobs.has(row.id))return this.jobs.get(row.id);
    if(!force&&(linkTerminal(row.state)||Date.now()<row.next_poll))return row;
    const job=(async()=>{
      this.store.run('UPDATE partner_redemptions SET next_poll=? WHERE id=?',Date.now()+15000,row.id);
      try {
        if(row.key_hash!==this.keyHash)throw new Problem('PARTNER_KEY_CHANGED','原兑换使用的服务密钥已变更，请联系客服核对。',409);
        const d=this.data(row),c=this.store.get('SELECT * FROM cards WHERE id=?',row.card_id);
        let response;
        if(!d.id) {
          this.service.assertUsable(c);
          response=await this.partner.create(this.vault.decrypt(c.source_enc),row.request_id);
        } else response=await this.partner.status(d.id);
        this.store.transaction(()=>{
          const current=this.store.get('SELECT * FROM partner_redemptions WHERE id=?',row.id);
          if(linkTerminal(current.state) && current.state!==response.status)return;
          const data={...this.data(current),...response,error_code:response.error_code||null,preflight_error_code:response.preflight_error_code||null,query_error:null};
          const delay=Date.now()-row.created_at>86400000?300000:response.status==='awaiting_customer'?30000:15000;
          this.store.run('UPDATE partner_redemptions SET state=?,data_enc=?,updated_at=?,next_poll=? WHERE id=?',response.status,this.vault.encrypt(JSON.stringify(data)),Date.now(),Date.now()+delay,row.id);
          if(this.latest(c.id)?.id===row.id && !['revoked','revoking','disabled','invalid'].includes(c.status))this.store.run('UPDATE cards SET status=?,updated_at=? WHERE id=?',response.status==='succeeded'?'completed':linkTerminal(response.status)?'failed':'processing',Date.now(),c.id);
        });
      } catch(error) {
        const current=this.store.get('SELECT * FROM partner_redemptions WHERE id=?',row.id);
        const message=error instanceof Problem?error.message:'兑换服务暂不可用，请稍后查询原记录。';
        this.store.run('UPDATE partner_redemptions SET data_enc=?,next_poll=? WHERE id=?',this.vault.encrypt(JSON.stringify({...this.data(current),query_error:message})),Date.now()+60000,row.id);
      }
      return this.store.get('SELECT * FROM partner_redemptions WHERE id=?',row.id);
    })();
    this.jobs.set(row.id,job);try{return await job;}finally{this.jobs.delete(row.id);}
  }
  async result(attemptID,device) {
    const row=this.store.get('SELECT * FROM partner_redemptions WHERE id=? AND device_hash=?',String(attemptID||''),this.vault.hash('device',device));
    if(!row)throw new Problem('ATTEMPT_NOT_FOUND','兑换记录不存在，请用原 PAIPLUS 卡密查询。',404);
    this.service.assertUsable(this.store.get('SELECT * FROM cards WHERE id=?',row.card_id));
    return this.view(await this.sync(row));
  }
  async recover(code) {const c=this.service.card(code);this.service.assertUsable(c);const row=this.latest(c.id);return row?this.view(await this.sync(row)):null;}
  recent(device) {return this.store.all('SELECT * FROM partner_redemptions WHERE device_hash=? ORDER BY created_at DESC LIMIT 30',this.vault.hash('device',device)).map(row=>this.view(row));}
  async reconcile() {
    if(this.reconciling)return this.reconciling;
    this.reconciling=(async()=>{
      const rows=this.store.all("SELECT * FROM partner_redemptions WHERE state NOT IN ('succeeded','failed','cancelled','expired','card_unavailable') AND next_poll<=? ORDER BY next_poll LIMIT 4",Date.now());
      for(const row of rows)await this.sync(row);
    })();
    try{await this.reconciling;}finally{this.reconciling=null;}
  }
}
