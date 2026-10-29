import { Problem, id, newCard, normalizeCard, maskedCard, credential, terminal, safeURL } from './security.mjs';
import { parseDelivery, customerDelivery } from './delivery.mjs';

const active = s => ['submitting','queued','processing','uncertain'].includes(s);
const string = (v,n=120) => typeof v==='string'?v.slice(0,n):null;
const productName = v => ['gpt','grok','claude','x'].includes(v)?v:'unknown';
const email = v => typeof v==='string' && /^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(v) ? v.slice(0,254) : null;
const nullableBoolean = v => typeof v==='boolean'?v:null;
const amount = v => (typeof v==='number' && Number.isFinite(v)) || (typeof v==='string' && /^-?\d+(\.\d+)?$/.test(v)) ? v : null;
const dateValue = v => typeof v==='number' && Number.isFinite(v) ? v : string(v,80);

export class RedemptionService {
  constructor({store,vault,partner,publicOrigin,mock=false}) {
    this.store=store; this.vault=vault; this.partner=partner; this.publicOrigin=publicOrigin; this.mock=mock; this.polls=new Map();
    const fingerprint=vault.hash('key','paiplus-key-check'), saved=store.get("SELECT value FROM meta WHERE key='key_fingerprint'");
    if(saved && saved.value!==fingerprint)throw new Error('Master key does not match this database');
    store.run("INSERT OR IGNORE INTO meta(key,value) VALUES('key_fingerprint',?)",fingerprint);
    const mode=mock?'mock':'live', priorMode=store.get("SELECT value FROM meta WHERE key='mode'");
    if(priorMode && priorMode.value!==mode)throw new Error('Use separate databases for mock and live modes');
    store.run("INSERT OR IGNORE INTO meta(key,value) VALUES('mode',?)",mode);
    store.run("UPDATE attempts SET state='uncertain',session_enc='' WHERE state='submitting'");
    store.run("UPDATE attempts SET state='preview_failed',session_enc='',preflight_enc='',confirmation_hash='' WHERE state IN ('preparing','checking')");
    this.expire();
  }
  expire() {
    this.store.run("UPDATE attempts SET session_enc='',preflight_enc='',confirmation_hash='',state='preview' WHERE state='checked' AND checked_at<?",Date.now()-15*60000);
  }
  data(a) { return a?.data_enc?JSON.parse(this.vault.decrypt(a.data_enc)):{}; }
  update(a,state,data={},extra={}) {
    const allowed = new Set(['token_enc','preflight_enc','session_enc','confirmation_hash','checked_at','submitted_at','last_poll']);
    const patch={state,data_enc:this.vault.encrypt(JSON.stringify({...this.data(a),...data})),updated_at:Date.now(),...extra};
    for(const k of Object.keys(extra))if(!allowed.has(k))throw new Error('Invalid attempt update');
    this.store.run(`UPDATE attempts SET ${Object.keys(patch).map(k=>`${k}=?`).join(',')},revision=revision+1 WHERE id=?`,...Object.values(patch),a.id);
    return this.store.get('SELECT * FROM attempts WHERE id=?',a.id);
  }
  view(a,owned=true) {
    const d=this.data(a), c=this.store.get('SELECT code_enc FROM cards WHERE id=?',a.card_id);
    return {attempt_id:a.id,state:a.state,code:maskedCard(this.vault.decrypt(c.code_enc)),product:d.product||'unknown',plan:d.plan||null,
      country:d.country||null,currency:d.currency||null,email:d.email||null,current_plan:d.current_plan||null,
      has_active_subscription:d.has_active_subscription??null,blocked:!!d.blocked,
      blocking_reason:d.blocked?'当前账号状态需要人工处理，暂时不能提交充值。':null,
      confirmation:a.state==='checked' && owned && a.confirmation_hash?d.confirmation:null,
      message:terminal(a.state)?(a.state==='completed'?'充值已完成。':'本次充值未完成，请联系客服核对。'):a.state==='uncertain'?'提交结果待确认，正在查询原记录，请勿重新兑换。':a.state==='blocked'?'账号状态需要人工核对。':active(a.state)?'已提交，正在确认充值结果。':null,
      created_at:a.created_at,updated_at:a.updated_at,submitted:!!a.submitted_at,recover_by_code:!owned};
  }
  card(value) {
    const code=normalizeCard(value), c=this.store.get('SELECT * FROM cards WHERE fingerprint=?',this.vault.hash('card',code));
    if(!c)throw new Problem('CARD_NOT_FOUND','未找到这张本站卡密，请核对订单交付内容。',404);
    return c;
  }
  attempt(attemptID,device) {
    const a=this.store.get('SELECT * FROM attempts WHERE id=? AND device_hash=?',String(attemptID||''),this.vault.hash('device',device));
    if(!a)throw new Problem('ATTEMPT_NOT_FOUND','兑换记录不存在，请使用原卡密恢复进度。',404);
    return a;
  }
  device(a) { return this.vault.decrypt(a.device_enc); }
  _issue(reference,source,label='') {
    if(typeof reference!=='string' || !reference.trim() || reference.length>200 || typeof source!=='string' || !/^[A-Za-z0-9_-]{8,256}$/.test(source) || source.toUpperCase().startsWith('PAIPLUS-')) throw new Problem('IMPORT_INVALID','来源编号或源头卡密格式不正确。');
    const hash=this.vault.hash('source',source), existing=this.store.get('SELECT * FROM cards WHERE reference=?',reference);
    if(existing) { if(existing.source_hash!==hash)throw new Problem('REFERENCE_CONFLICT','来源编号已对应另一张卡密。',409); return {id:existing.id,code:this.vault.decrypt(existing.code_enc),reference:existing.reference}; }
    if(this.store.get('SELECT id FROM cards WHERE source_hash=?',hash))throw new Problem('SOURCE_DUPLICATE','该源头卡密已经生成过本站码。',409);
    const code=newCard(), cardID=id(), now=Date.now();
    this.store.run('INSERT INTO cards(id,fingerprint,code_enc,source_hash,source_enc,reference,label,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?,?)',cardID,this.vault.hash('card',code),this.vault.encrypt(code),hash,this.vault.encrypt(source),reference,String(label).slice(0,120),now,now);
    this.store.audit('issue',cardID);
    return {id:cardID,code,reference};
  }
  issue(rows) {
    if(!Array.isArray(rows) || !rows.length || rows.length>100)throw new Problem('IMPORT_LIMIT','每批导入 1–100 张卡密。');
    if(new Set(rows.map(r=>r?.reference)).size!==rows.length || new Set(rows.map(r=>r?.source_code)).size!==rows.length)throw new Problem('IMPORT_DUPLICATE','同一批中存在重复来源编号或卡密。');
    return this.store.transaction(()=>rows.map(r=>this._issue(r.reference,r.source_code,r.label)));
  }
  deliver({reference,payload,quantity}) {
    if(typeof reference!=='string' || reference.length>150 || !reference || typeof payload!=='string' || Buffer.byteLength(payload)>200000 || !Number.isSafeInteger(quantity) || quantity<1 || quantity>100)throw new Problem('DELIVERY_INVALID','交付参数不完整。');
    const records=parseDelivery(payload,quantity);
    return this.store.transaction(()=>{
      const fingerprint=this.vault.hash('delivery',payload), old=this.store.get('SELECT * FROM deliveries WHERE reference=?',reference);
      if(old && old.fingerprint!==fingerprint)throw new Problem('DELIVERY_CONFLICT','同一交付编号的源头内容发生变化。',409);
      const cards=records.map(({source},i)=>this._issue(`${reference}:${i+1}`,source));
      this.store.run('INSERT OR IGNORE INTO deliveries(reference,fingerprint,card_ids) VALUES(?,?,?)',reference,fingerprint,JSON.stringify(cards.map(c=>c.id)));
      return customerDelivery(cards.map((c,i)=>({code:c.code,...(records[i].account?{account:records[i].account}:{})})),this.publicOrigin);
    });
  }
  async preview(value,device) {
    this.expire();
    const c=this.card(value);
    if(c.status==='disabled')throw new Problem('CARD_DISABLED','该卡密已停用，请联系客服。',409);
    let a=c.attempt_id?this.store.get('SELECT * FROM attempts WHERE id=?',c.attempt_id):null;
    if(a) {
      if(a.device_hash!==this.vault.hash('device',device)) {
        if(active(a.state)||terminal(a.state))return this.view(await this.pollRecord(a),false);
        throw new Problem('OTHER_BROWSER','该卡密已在另一浏览器开始核验，请在原浏览器继续。',409);
      }
      if(active(a.state)||terminal(a.state))return this.view(await this.pollRecord(a));
      if(a.state!=='preview_failed')return this.view(a);
      a=this.update(a,'preparing',{}, {token_enc:'',preflight_enc:'',session_enc:'',confirmation_hash:''});
    } else {
      const now=Date.now(), attemptID=id();
      this.store.transaction(()=>{
        const current=this.store.get('SELECT attempt_id,status FROM cards WHERE id=?',c.id);
        if(current.attempt_id || current.status!=='ready')throw new Problem('CARD_BUSY','卡密正在处理，请稍后恢复进度。',409);
        this.store.run('INSERT INTO attempts(id,card_id,device_hash,device_enc,request_id,state,created_at,updated_at) VALUES(?,?,?,?,?,?,?,?)',attemptID,c.id,this.vault.hash('device',device),this.vault.encrypt(device),'pai-'+id(),'preparing',now,now);
        this.store.run('UPDATE cards SET attempt_id=? WHERE id=?',attemptID,c.id);
      });
      a=this.store.get('SELECT * FROM attempts WHERE id=?',attemptID);
    }
    try {
      const data=await this.partner.call('preview',{code:this.vault.decrypt(c.source_enc)},this.device(a));
      if(typeof data.redemption_token!=='string' || !data.redemption_token || data.redemption_token.length>2048 || productName(data.product)==='unknown')throw new Problem('PREVIEW_UNSUPPORTED','源头未返回可支持的商品信息，请联系客服核对。',422);
      a=this.update(a,'preview',{product:productName(data.product),plan:string(data.plan,80),country:string(data.payment_country,10),currency:string(data.payment_currency,10)}, {token_enc:this.vault.encrypt(data.redemption_token)});
      return this.view(a);
    } catch(e) { this.update(a,'preview_failed'); throw e; }
  }
  async preflight(attemptID,session,device) {
    this.expire(); session=credential(session);
    let a=this.attempt(attemptID,device);
    if(!['preview','checked','blocked'].includes(a.state))throw new Problem('WRONG_STEP','请先验证卡密，处理中不能更换账号。',409);
    const c=this.store.get('SELECT * FROM cards WHERE id=?',a.card_id);
    if(c.status==='disabled')throw new Problem('CARD_DISABLED','该卡密已停用。',409);
    if(!this.store.run("UPDATE attempts SET state='checking',revision=revision+1 WHERE id=? AND revision=?",a.id,a.revision).changes)throw new Problem('BUSY','账号正在核验，请稍后。',409);
    a=this.store.get('SELECT * FROM attempts WHERE id=?',a.id);
    try {
      const data=await this.partner.call('preflight',{code:this.vault.decrypt(c.source_enc),redemption_token:this.vault.decrypt(a.token_enc),credential:{mode:'session',session}},this.device(a));
      const product=this.data(a).product, blocked=!!data.blocking_reason || data.subscription_recovery_required===true || data.can_redeem===false;
      const account=email(data.email||data.account_email), ticket=string(data.preflight_token,65536);
      if(!blocked && (!account || (product!=='grok' && !ticket)))throw new Problem('PREFLIGHT_INCOMPLETE','未能完整核对账号，请重新核验或联系客服。',422);
      const confirmation=id();
      a=this.update(a,blocked?'blocked':'checked',{email:account,current_plan:string(data.currentPlan||data.current_plan,80),has_active_subscription:nullableBoolean(data.has_active_subscription),blocked,confirmation},
        {preflight_enc:blocked?'':this.vault.encrypt(ticket),session_enc:!blocked && product==='grok'?this.vault.encrypt(session):'',confirmation_hash:blocked?'':this.vault.hash('confirmation',confirmation),checked_at:Date.now()});
      return this.view(a);
    } catch(e) { this.update(a,'preview',{}, {preflight_enc:'',session_enc:'',confirmation_hash:''}); throw e; }
  }
  async redeem(attemptID,confirmation,confirmed,device) {
    this.expire(); let a=this.attempt(attemptID,device);
    if(active(a.state)||terminal(a.state))return this.view(a);
    if(confirmed!==true || !confirmation || a.state!=='checked' || !a.confirmation_hash || a.confirmation_hash!==this.vault.hash('confirmation',confirmation))throw new Problem('CONFIRM_REQUIRED','请重新核验并确认本次账号与套餐。',409);
    const c=this.store.get('SELECT * FROM cards WHERE id=?',a.card_id);
    if(c.status==='disabled')throw new Problem('CARD_DISABLED','该卡密已停用。',409);
    this.store.transaction(()=>{
      if(!this.store.run("UPDATE attempts SET state='submitting',submitted_at=?,updated_at=?,revision=revision+1 WHERE id=? AND state='checked' AND revision=?",Date.now(),Date.now(),a.id,a.revision).changes)throw new Problem('BUSY','兑换已经提交，请查询原记录。',409);
      this.store.run("UPDATE cards SET status='processing',updated_at=? WHERE id=?",Date.now(),c.id);
      this.store.audit('submit',a.id);
    });
    a=this.store.get('SELECT * FROM attempts WHERE id=?',a.id);
    const request={redemption_token:this.vault.decrypt(a.token_enc),client_request_id:a.request_id};
    if(this.data(a).product==='grok')request.credential={mode:'session',session:this.vault.decrypt(a.session_enc)};
    else request.preflight_token=this.vault.decrypt(a.preflight_enc);
    try {
      const response=await this.partner.call('redeem',request,this.device(a));
      return this.view(this.setResult(a,response));
    } catch {
      a=this.store.get('SELECT * FROM attempts WHERE id=?',a.id);
      return this.view(terminal(a.state)?a:this.update(a,'uncertain',{}, {session_enc:'',preflight_enc:'',confirmation_hash:''}));
    }
  }
  setResult(a,response) {
    // A late submit/poll response cannot replace an already known terminal result.
    a=this.store.get('SELECT * FROM attempts WHERE id=?',a.id);
    if(terminal(a.state))return a;
    const status=response?.uncertain===true?'uncertain':terminal(response?.status)?response.status:['queued','processing'].includes(response?.status)?response.status:'uncertain';
    return this.store.transaction(()=>{
      const result=this.update(a,status,{email:email(response?.account_email)||this.data(a).email}, {session_enc:'',preflight_enc:'',confirmation_hash:'',last_poll:Date.now()});
      this.store.run('UPDATE cards SET status=?,updated_at=? WHERE id=?',status==='completed'?'completed':terminal(status)?'failed':'processing',Date.now(),a.card_id);
      return result;
    });
  }
  async pollRecord(a) {
    if(!active(a.state) || Date.now()-a.last_poll<2800)return a;
    if(this.polls.has(a.id))return this.polls.get(a.id);
    const promise=(async()=>{
      try {
        this.store.run('UPDATE attempts SET last_poll=? WHERE id=?',Date.now(),a.id);
        let data;
        try { data=await this.partner.call('result',{token:this.vault.decrypt(a.token_enc)},this.device(a)); }
        catch(error) {
          if(error.status!==404)throw error;
          const card=this.store.get('SELECT source_enc FROM cards WHERE id=?',a.card_id);
          data=await this.partner.call('result-by-code',{code:this.vault.decrypt(card.source_enc)},this.device(a));
        }
        return this.setResult(a,data);
      } catch {
        // Missing or failed queries do not prove the original submission failed.
        return this.store.get('SELECT * FROM attempts WHERE id=?',a.id);
      } finally { this.polls.delete(a.id); }
    })();
    this.polls.set(a.id,promise); return promise;
  }
  async result(attemptID,device) { return this.view(await this.pollRecord(this.attempt(attemptID,device))); }
  // The server also reconciles after the buyer closes the page. It only queries;
  // it never replays a submitted charge or creates another request ID.
  async reconcile() {
    if(this.reconciling)return this.reconciling;
    this.reconciling=(async()=>{
      const now=Date.now();
      const pending=this.store.all("SELECT * FROM attempts WHERE state IN ('queued','processing','uncertain') AND ?-last_poll > CASE WHEN ?-submitted_at>86400000 THEN 300000 WHEN ?-submitted_at>3600000 THEN 60000 ELSE 3000 END ORDER BY last_poll ASC LIMIT 20",now,now,now);
      let index=0;
      await Promise.all(Array.from({length:Math.min(4,pending.length)},async()=>{while(index<pending.length)await this.pollRecord(pending[index++]);}));
    })();
    try {await this.reconciling;} finally {this.reconciling=null;}
  }
  results(values,device) {
    if(!Array.isArray(values)||!values.length||values.length>100)throw new Problem('RESULT_LIMIT','每批查询 1–100 条兑换记录。');
    return values.map(code=>{
      const c=this.card(code),a=c.attempt_id?this.store.get('SELECT * FROM attempts WHERE id=?',c.attempt_id):null;
      if(!a)return {state:'unused',code:maskedCard(normalizeCard(code)),submitted:false};
      const owned=a.device_hash===this.vault.hash('device',device);
      if(!owned&&!a.submitted_at)throw new Problem('OTHER_BROWSER','请在原浏览器继续核验。',409);
      return this.view(a,owned);
    });
  }
  async recover(value,device) {
    const c=this.card(value), a=c.attempt_id?this.store.get('SELECT * FROM attempts WHERE id=?',c.attempt_id):null;
    if(!a)return {state:'unused',code:maskedCard(normalizeCard(value)),submitted:false};
    if(!active(a.state) && !terminal(a.state) && a.device_hash!==this.vault.hash('device',device))throw new Problem('OTHER_BROWSER','请在原浏览器继续核验。',409);
    return this.view(await this.pollRecord(a),a.device_hash===this.vault.hash('device',device));
  }
  recent(device) { return this.store.all('SELECT * FROM attempts WHERE device_hash=? ORDER BY created_at DESC LIMIT 30',this.vault.hash('device',device)).map(a=>this.view(a)); }
  async lookup(values,device) {
    if(!Array.isArray(values) || !values.length || values.length>100)throw new Problem('LOOKUP_LIMIT','每次查询 1–100 张卡密。');
    const results=[], groups=new Map();
    for(let i=0;i<values.length;i++) {
      try {
        const c=this.card(values[i]), a=c.attempt_id?this.store.get('SELECT * FROM attempts WHERE id=?',c.attempt_id):null;
        if(c.status==='disabled') { results[i]={row:i+1,code:normalizeCard(values[i]),status:'disabled',used:null,can_resubmit:false}; continue; }
        const d=a?this.device(a):device;
        if(!groups.has(d))groups.set(d,[]); groups.get(d).push({i,c,a});
      } catch { results[i]={row:i+1,code:String(values[i]||'').slice(0,256),status:'unknown',used:null,can_resubmit:false}; }
    }
    for(const [d,items] of groups) {
      try {
        const sourceCodes=items.map(({c})=>this.vault.decrypt(c.source_enc));
        const data=await this.partner.call('lookup-batch',{codes:sourceCodes},d);
        if(!Array.isArray(data.results) || data.results.length!==items.length)throw new Error('Invalid batch response');
        items.forEach(({i,a},n)=>{
          const result=data.results[n];
          if(result.cdk_code && result.cdk_code!==sourceCodes[n])throw new Error('Mismatched batch row');
          const localSubmitted=!!a?.submitted_at;
          results[i]={row:i+1,code:normalizeCard(values[i]),status:localSubmitted?a.state:string(result.status,40)||'unknown',used:nullableBoolean(result.used),can_resubmit:!localSubmitted && result.can_resubmit===true,account_email:email(result.account_email),plan:string(result.plan,80),used_at:dateValue(result.used_at)};
        });
      } catch { for(const {i,a} of items)results[i]={row:i+1,code:normalizeCard(values[i]),status:a?.submitted_at?a.state:'unavailable',used:null,can_resubmit:false}; }
    }
    return results;
  }
  async billing({code,session},device) {
    if(!!code===!!session)throw new Problem('BILLING_INPUT','请选择卡密或账号凭证中的一种查询方式。');
    let body,d=device;
    if(code) { const c=this.card(code), a=c.attempt_id?this.store.get('SELECT * FROM attempts WHERE id=?',c.attempt_id):null; if(c.status==='disabled')throw new Problem('CARD_DISABLED','该卡密已停用。',409); body={cdk_code:this.vault.decrypt(c.source_enc)}; if(a)d=this.device(a); }
    else body={token_input:credential(session)};
    const data=await this.partner.call('billing',body,d), summary=data.summary||{};
    const payment=p=>p && typeof p==='object'?{amount:amount(p.amount),currency:string(p.currency,12),date:dateValue(p.date??p.paid_at??p.created_at),status:string(p.status,40),invoice_url:safeURL(p.invoice_url||p.hosted_invoice_url||p.url)}:null;
    return {summary:{email:email(summary.email),plan_type:string(summary.plan_type,80),has_active_subscription:nullableBoolean(summary.has_active_subscription)},last_payment:payment(data.last_payment),invoices:Array.isArray(data.invoices)?data.invoices.slice(0,100).map(payment):null,invoice_url:safeURL(data.invoice_url),invoice_data_available:nullableBoolean(data.invoice_data_available)};
  }
  listCards(query='') {
    query=String(query).trim().slice(0,200);
    let rows;
    if(query.toUpperCase().startsWith('PAIPLUS-')) {try{rows=[this.card(query)];}catch{rows=[];}}
    else rows=query?this.store.all('SELECT * FROM cards WHERE instr(reference,?)>0 OR instr(label,?)>0 ORDER BY created_at DESC LIMIT 500',query,query):this.store.all('SELECT * FROM cards ORDER BY created_at DESC LIMIT 500');
    return rows.map(c=>({id:c.id,reference:c.reference,label:c.label,code:maskedCard(this.vault.decrypt(c.code_enc)),status:c.status,created_at:c.created_at}));
  }
  reveal(cardID) { const c=this.store.get('SELECT * FROM cards WHERE id=?',cardID); if(!c)throw new Problem('NOT_FOUND','卡密不存在。',404); this.store.audit('reveal_local_code',c.id); return {code:this.vault.decrypt(c.code_enc)}; }
  disable(cardID,disabled) {
    return this.store.transaction(()=>{
      const c=this.store.get('SELECT * FROM cards WHERE id=?',cardID);
      if(!c)throw new Problem('NOT_FOUND','卡密不存在。',404);
      const a=c.attempt_id?this.store.get('SELECT * FROM attempts WHERE id=?',c.attempt_id):null;
      if(a?.submitted_at || !['ready','disabled'].includes(c.status))throw new Problem('CARD_IN_USE','已提交或已使用卡密不能在此停用或重新启用，请核对原兑换。',409);
      this.store.run('UPDATE cards SET status=?,updated_at=? WHERE id=?',disabled?'disabled':'ready',Date.now(),c.id); this.store.audit(disabled?'disable':'enable',c.id); return {ok:true};
    });
  }
  releaseCheck(cardID) {
    return this.store.transaction(()=>{
      const c=this.store.get('SELECT * FROM cards WHERE id=?',cardID);
      if(!c)throw new Problem('NOT_FOUND','卡密不存在。',404);
      const a=c.attempt_id?this.store.get('SELECT * FROM attempts WHERE id=?',c.attempt_id):null;
      if(!a)return {ok:true};
      if(a.submitted_at || ['preparing','checking'].includes(a.state))throw new Problem('CARD_IN_USE','正在核验或已经提交，不能解除绑定。',409);
      this.update(a,'cancelled',{}, {session_enc:'',preflight_enc:'',confirmation_hash:''});
      this.store.run('UPDATE cards SET attempt_id=NULL,updated_at=? WHERE id=?',Date.now(),c.id);
      this.store.audit('release_unsubmitted_check',c.id);return {ok:true};
    });
  }
}
