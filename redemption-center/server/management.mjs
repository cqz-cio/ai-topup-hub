import { Problem, id, maskedCard, normalizeCard } from './security.mjs';

const messages={
  CARD_INVALID:'卡密无效，销毁失败：该卡密已使用或上游已失效。',
  CARD_REVOKED:'卡密无效，销毁失败：这张本站卡密已销毁。',
  CARD_IN_USE:'销毁失败：卡密正在充值或已有提交记录，请先核对充值结果，不要据此退款。',
  CARD_BUSY:'销毁失败：卡密正在核验，请稍后重试。',
  CHECK_UNAVAILABLE:'销毁失败：暂时无法确认上游卡密是否有效，请稍后重试，不要据此退款。',
};
const statusGroup=(c,links)=>c.revoked_at?'revoked':links&&['ready','disabled'].includes(c.status)?(c.upstream_card_status==='unused'?'unused':'unknown'):({ready:'unused',disabled:'unused',completed:'used',processing:'processing',revoking:'revoking',invalid:'invalid',failed:'invalid'}[c.status]||'unknown');
const short=(v,n=120)=>typeof v==='string'?v.slice(0,n):'';
const fail=code=>new Problem(code,messages[code]||'卡密操作未完成。',409);

// The redemption database remains the sole authority. The shop admin proxies
// these operations; delivered cards never re-enter the shop's sellable stock.
export class CardManagement {
  constructor(service) { this.service=service; this.store=service.store; this.vault=service.vault; this.jobs=new Map(); }
  card(cardID) { const c=this.store.get('SELECT * FROM cards WHERE id=?',String(cardID||'')); if(!c)throw new Problem('NOT_FOUND','卡密不存在。',404); return c; }
  snapshot(c) { return c.upstream_snapshot_enc?JSON.parse(this.vault.decrypt(c.upstream_snapshot_enc)):null; }
  capabilities() {
    const main=typeof this.service.links?.partner.cardStatus==='function';
    return {status_scope:this.service.links&&!main?'partner_order':'card',can_revoke:!this.service.links,
      revoke_message:this.service.links?(main?'当前可查询上游卡密状态，包含主站手动兑换；上游未提供退款销毁接口，退款前仍需人工核对。':'当前上游仅提供本代理的兑换记录，未提供卡密未使用证明及退款销毁接口。请人工核对，不能仅凭未查到记录退款。'):null};
  }
  row(c) {
    const a=c.attempt_id?this.store.get('SELECT state,submitted_at FROM attempts WHERE id=?',c.attempt_id):null;
    const link=this.service.links?.latest(c.id);
    const reference=/^shop:(\d+):order:(\d+):(\d+)$/.exec(c.reference);
    return {id:c.id,code:maskedCard(this.vault.decrypt(c.code_enc)),source_code:'••••••'+this.vault.decrypt(c.source_enc).slice(-4),
      reference:c.reference,order_id:c.order_id||Number(reference?.[2]||0),order_no:c.order_no,product_id:c.product_id,sku_id:c.sku_id,label:c.label,
      status:statusGroup(c,this.service.links),local_status:c.status,created_at:c.created_at,updated_at:c.updated_at,revoked_at:c.revoked_at,
      upstream_checked_at:c.upstream_checked_at,upstream:this.snapshot(c),attempt_state:link?.state||a?.state||null,submitted:!!a?.submitted_at,external_link_created:!!link,capabilities:this.capabilities()};
  }
  list(input={}) {
    const page=Math.max(1,Math.min(100000,Number.isSafeInteger(input.page)?input.page:1));
    const size=Math.max(1,Math.min(100,Number.isSafeInteger(input.page_size)?input.page_size:20));
    const clauses=[],args=[],query=short(input.query,200).trim();
    if(query) {
      if(query.toUpperCase().startsWith('PAIPLUS-')) {
        let normalized;try {normalized=normalizeCard(query);} catch {normalized=query;}
        clauses.push('fingerprint=?');args.push(this.vault.hash('card',normalized));
      } else {clauses.push('(instr(reference,?)>0 OR instr(order_no,?)>0 OR instr(label,?)>0 OR source_hash=?)');args.push(query,query,query,this.vault.hash('source',query));}
    }
    for(const k of ['order_id','product_id'])if(Number.isSafeInteger(input[k])&&input[k]>0){clauses.push(`${k}=?`);args.push(input[k]);}
    const statuses={unused:['ready','disabled'],used:['completed'],processing:['processing'],revoking:['revoking'],revoked:['revoked'],invalid:['invalid','failed']};
    if(input.status==='unknown') {
      clauses.push(this.service.links?"status IN ('ready','disabled') AND upstream_card_status!='unused' AND revoked_at=0":"status NOT IN ('ready','disabled','completed','processing','revoking','revoked','invalid','failed') AND revoked_at=0");
    } else if(input.status) {
      if(!statuses[input.status])throw new Problem('FILTER_INVALID','请选择有效卡密状态。');
      clauses.push(`status IN (${statuses[input.status].map(()=>'?').join(',')})`);args.push(...statuses[input.status]);
      if(input.status==='unused'&&this.service.links)clauses.push("upstream_card_status='unused' AND revoked_at=0");
    }
    const where=clauses.length?' WHERE '+clauses.join(' AND '):'';
    const total=this.store.get('SELECT count(*) n FROM cards'+where,...args).n;
    const cards=this.store.all('SELECT * FROM cards'+where+' ORDER BY created_at DESC,id LIMIT ? OFFSET ?',...args,size,(page-1)*size).map(c=>this.row(c));
    const counts={unused:0,used:0,processing:0,revoking:0,revoked:0,invalid:0,unknown:0};
    for(const c of this.store.all('SELECT status,revoked_at,upstream_card_status,count(*) n FROM cards GROUP BY status,revoked_at,upstream_card_status'))counts[statusGroup(c,this.service.links)]+=c.n;
    return {cards,total,page,page_size:size,counts,capabilities:this.capabilities()};
  }
  detail(cardID,actor) {
    const c=this.card(cardID);this.store.audit('reveal_card_pair',c.id,actor);
    const operations=this.store.all('SELECT * FROM card_operations WHERE card_id=? ORDER BY created_at DESC LIMIT 30',c.id).map(j=>({id:j.id,state:j.state,actor:j.actor,reason:this.vault.decrypt(j.reason_enc),error_code:j.error_code,message:messages[j.error_code]||null,created_at:j.created_at,finished_at:j.finished_at}));
    return {...this.row(c),code:this.vault.decrypt(c.code_enc),source_code:this.vault.decrypt(c.source_enc),operations,
      audit:this.store.all('SELECT action,actor,at FROM audit WHERE subject=? ORDER BY id DESC LIMIT 50',c.id)};
  }
  async readSource(c,attemptID=c.attempt_id) {
    const a=attemptID?this.store.get('SELECT * FROM attempts WHERE id=?',attemptID):null;
    const links=this.service.links;
    // Submitted legacy attempts retain their original provider. Management
    // queries must never create a preview/link or retry a pending creation.
    if(links && !a?.submitted_at) {
      if(typeof links.partner.cardStatus==='function')return links.partner.cardStatus(this.vault.decrypt(c.source_enc));
      const row=links.latest(c.id),data=row?links.data(row):null;
      if(row && row.key_hash!==links.keyHash)throw new Error('Partner credential changed');
      let result;
      if(data?.id)result=await links.partner.status(data.id);
      else if(links.partner.protocol===2) {
        try {result=(await links.partner.lookup(this.vault.decrypt(c.source_enc))).order;}
        catch(error) {if(error?.code!=='INTENT_NOT_FOUND')throw error;}
        if(!result)return {status:'no_record',used:null,can_resubmit:false,scope:'partner_order',message:'未查到本代理的兑换记录；这不代表上游卡密未使用或可以退款。'};
      } else return {status:'unverified',used:null,can_resubmit:false,scope:'partner_order',message:'尚未建立可查询的兑换记录，当前接口不能直接核验卡密是否未使用。'};
      return {status:short(result.status,40),used:result.status==='succeeded'?true:null,can_resubmit:false,scope:'partner_order',message:'已查询本代理兑换记录；记录状态不等同于卡密未使用证明。'};
    }
    const device=a?this.service.device(a):'management-'+this.vault.hash('management-device',c.id).slice(0,48);
    const source=this.vault.decrypt(c.source_enc);
    const data=await this.service.partner.call('lookup',{code:source},device,{timeout:15000});
    if(data?.cdk_code!==source || typeof data.used!=='boolean' || typeof data.can_resubmit!=='boolean' || typeof data.status!=='string')throw new Error('Unverifiable upstream card status');
    // Keep only structured status evidence. No arbitrary source text or tokens.
    return {status:short(data.status,40),used:data.used,can_resubmit:data.can_resubmit,plan:short(data.plan,80),used_at:typeof data.used_at==='number'?data.used_at:short(data.used_at,80)};
  }
  saveSnapshot(c,snapshot) {
    const cardStatus=snapshot.scope==='card'&&snapshot.source==='wudi_main'?snapshot.status:'';
    this.store.run('UPDATE cards SET upstream_snapshot_enc=?,upstream_checked_at=?,upstream_card_status=? WHERE id=?',this.vault.encrypt(JSON.stringify(snapshot)),Date.now(),cardStatus,c.id);
  }
  async refresh(cardID,actor='system') {
    const c=this.card(cardID);
    let snapshot;
    try {snapshot=await this.readSource(c);} catch {throw new Problem('CHECK_UNAVAILABLE','暂时无法确认上游状态，保留原状态，请稍后再查。',502);}
    this.store.transaction(()=>{
      this.saveSnapshot(c,snapshot);
      const now=this.card(c.id),a=now.attempt_id?this.store.get('SELECT submitted_at FROM attempts WHERE id=?',now.attempt_id):null;
      if(!now.revoked_at && now.status!=='revoking' && !a?.submitted_at) {
        let status=now.status;
        if(snapshot.used || ['used','completed'].includes(snapshot.status))status='completed';
        else if(snapshot.scope==='partner_order') {
          // An expired/failed redemption is not an invalid card. Do not let a
          // record-only lookup undo completion, disabling or permanent holds.
          if(['ready','processing'].includes(status) && ['awaiting_customer','queued','running','captcha_required','manual_review','uncertain','retryable_no_task'].includes(snapshot.status))status='processing';
        }
        else if(['queued','processing','review','submitting','uncertain'].includes(snapshot.status)&&['ready','processing'].includes(status))status='processing';
        else if(['unknown','invalid','expired','disabled','revoked'].includes(snapshot.status)&&['ready','processing'].includes(status))status='invalid';
        // Never automatically re-enable invalid/used/disabled cards.
        if(status!==now.status)this.store.run('UPDATE cards SET status=?,updated_at=? WHERE id=?',status,Date.now(),c.id);
      }
      this.store.audit('check_card_status',c.id,actor);
    });
    return this.row(this.card(c.id));
  }
  async revoke(input,actor) {
    if(this.service.links)throw new Problem('CHECK_UNAVAILABLE','当前兑换协议不提供卡密未使用核验或撤销链接，不能自动销毁确认退款，请人工核对。',409);
    const {card_id:cardID,request_id:requestID,confirmed}=input,reason=short(input.reason,500).trim();
    if(confirmed!==true || !reason || !/^[A-Za-z0-9_-]{16,80}$/.test(requestID||''))throw new Problem('CONFIRM_REQUIRED','请填写销毁原因，并确认永久停用这张本站卡密。');
    let job=this.store.transaction(()=>{
      const old=this.store.get('SELECT * FROM card_operations WHERE id=?',requestID);
      if(old) {if(old.card_id!==cardID || old.actor!==actor)throw new Problem('OPERATION_CONFLICT','操作编号不匹配。',409);return old;}
      const c=this.card(cardID),a=c.attempt_id?this.store.get('SELECT * FROM attempts WHERE id=?',c.attempt_id):null;
      if(c.revoked_at || c.status==='revoked')throw fail('CARD_REVOKED');
      const pending=this.store.get("SELECT * FROM card_operations WHERE card_id=? AND state='pending'",c.id);
      if(pending)return pending;
      if(['completed','invalid','failed'].includes(c.status))throw fail('CARD_INVALID');
      if(a?.submitted_at || c.status==='processing')throw fail('CARD_IN_USE');
      if(!['ready','disabled'].includes(c.status) || ['preparing','checking'].includes(a?.state))throw fail('CARD_BUSY');
      const now=Date.now();
      this.store.run("INSERT INTO card_operations(id,card_id,actor,reason_enc,previous_status,attempt_id,state,created_at) VALUES(?,?,?,?,?,?,'pending',?)",requestID,c.id,actor,this.vault.encrypt(reason),c.status,c.attempt_id,now);
      // The persistent hold is acquired before the network check. Every public
      // submission checks it, so a buyer cannot redeem while destruction runs.
      this.store.run("UPDATE cards SET status='revoking',updated_at=? WHERE id=?",now,c.id);
      this.store.audit('revoke_requested',c.id,actor);
      return this.store.get('SELECT * FROM card_operations WHERE id=?',requestID);
    });
    if(job.state==='failed')throw fail(job.error_code);
    if(job.state==='succeeded')return {card:this.row(this.card(job.card_id)),destroyed:true,operation_id:job.id};
    if(this.jobs.has(job.id))return this.jobs.get(job.id);
    const promise=this.finishRevoke(job);this.jobs.set(job.id,promise);
    try{return await promise;}finally{this.jobs.delete(job.id);}
  }
  async finishRevoke(job) {
    const c=this.card(job.card_id);let snapshot,errorCode='';
    try {
      snapshot=await this.readSource(c,job.attempt_id);
      if(snapshot.used || ['used','completed','invalid','expired','disabled','revoked','unknown'].includes(snapshot.status))errorCode='CARD_INVALID';
      else if(['queued','processing','submitting','uncertain'].includes(snapshot.status))errorCode='CARD_IN_USE';
      else if(snapshot.status!=='unused' || snapshot.used!==false || snapshot.can_resubmit!==true)errorCode='CHECK_UNAVAILABLE';
    } catch(error) {errorCode=error?.code==='UPSTREAM_NOT_FOUND'?'CARD_INVALID':'CHECK_UNAVAILABLE';}
    this.store.transaction(()=>{
      const current=this.card(c.id),stored=this.store.get('SELECT * FROM card_operations WHERE id=?',job.id);
      if(stored.state!=='pending' || current.status!=='revoking')throw new Problem('OPERATION_CONFLICT','销毁状态已变化，请刷新核对。',409);
      if(snapshot)this.saveSnapshot(c,snapshot);
      const now=Date.now();
      if(errorCode) {
        const status=errorCode==='CARD_INVALID'?(snapshot?.used||snapshot?.status==='used'||snapshot?.status==='completed'?'completed':'invalid'):errorCode==='CARD_IN_USE'?'processing':job.previous_status;
        this.store.run('UPDATE cards SET status=?,updated_at=? WHERE id=?',status,now,c.id);
        this.store.run("UPDATE card_operations SET state='failed',error_code=?,finished_at=? WHERE id=?",errorCode,now,job.id);
        this.store.audit('revoke_failed_'+errorCode.toLowerCase(),c.id,job.actor);
      } else {
        this.store.run("UPDATE cards SET status='revoked',revoked_at=?,updated_at=? WHERE id=?",now,now,c.id);
        if(job.attempt_id) {
          const a=this.store.get('SELECT * FROM attempts WHERE id=?',job.attempt_id);
          if(a)this.service.update(a,'cancelled',{confirmation:null},{token_enc:'',preflight_enc:'',session_enc:'',confirmation_hash:''});
        }
        this.store.run("UPDATE card_operations SET state='succeeded',finished_at=? WHERE id=?",now,job.id);
        this.store.audit('revoke_succeeded',c.id,job.actor);
      }
    });
    if(errorCode)throw fail(errorCode);
    return {card:this.row(this.card(c.id)),destroyed:true,operation_id:job.id};
  }
  async reconcile() {
    if(this.reconciling)return this.reconciling;
    this.reconciling=(async()=>{
      // A crash leaves a durable hold. Resume the same read-only validation;
      // this process never purchases, submits a recharge or refunds money.
      for(const job of this.store.all("SELECT * FROM card_operations WHERE state='pending' ORDER BY created_at LIMIT 5")) {
        if(this.jobs.has(job.id))continue;
        const promise=this.finishRevoke(job);this.jobs.set(job.id,promise);
        try{await promise;}catch{}finally{this.jobs.delete(job.id);}
      }
      for(const c of this.store.all("SELECT id FROM cards WHERE status IN ('ready','disabled','processing') AND upstream_checked_at<? ORDER BY upstream_checked_at LIMIT 4",Date.now()-300000)) {
        try{await this.refresh(c.id);}catch{}
      }
    })();
    try{await this.reconciling;}finally{this.reconciling=null;}
  }
}
