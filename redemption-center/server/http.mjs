import { createServer } from 'node:http';
import { readFile, stat } from 'node:fs/promises';
import { resolve, extname, sep } from 'node:path';
import { isIP } from 'node:net';
import { Problem, id, equal } from './security.mjs';

const MIME={'.html':'text/html; charset=utf-8','.js':'text/javascript; charset=utf-8','.css':'text/css; charset=utf-8','.png':'image/png','.webp':'image/webp','.svg':'image/svg+xml','.ico':'image/x-icon','.woff2':'font/woff2'};
function cookies(req) { const result={}; for(const entry of String(req.headers.cookie||'').split(';')) { const i=entry.indexOf('='); if(i>0)result[entry.slice(0,i).trim()]=entry.slice(i+1).trim(); } return result; }
async function body(req) {
  if(!String(req.headers['content-type']||'').startsWith('application/json'))throw new Problem('CONTENT_TYPE','请使用 JSON 请求。',415);
  let length=0,parts=[];
  for await(const part of req) { length+=part.length; if(length>262144)throw new Problem('BODY_LIMIT','请求过大，请分批处理。',413); parts.push(part); }
  try { const value=JSON.parse(Buffer.concat(parts).toString('utf8')); if(!value || Array.isArray(value) || typeof value!=='object')throw 0; return value; } catch { throw new Problem('JSON_INVALID','请求格式不正确。'); }
}
export function makeServer({service,config,staticDir}) {
  const buckets=new Map();
  const limit=(key,max=120,cost=1)=>{
    const now=Date.now(); let bucket=buckets.get(key);
    if(!bucket || now-bucket.at>60000){bucket={at:now,count:0};buckets.set(key,bucket);}
    bucket.count+=cost;
    if(buckets.size>10000)for(const [k,b] of buckets)if(now-b.at>60000)buckets.delete(k);
    if(buckets.size>12000 || bucket.count>max)throw new Problem('RATE_LIMITED','操作较频繁，请稍后再试。',429);
  };
  const root=resolve(staticDir), secure=new URL(config.publicOrigin).protocol==='https:';
  const cookie=(name,value,maxAge)=>`${name}=${value}; Path=/; HttpOnly; SameSite=Strict; Max-Age=${maxAge}${secure?'; Secure':''}`;
  const adminValid=value=>{
    const [expiry,nonce,mac]=String(value||'').split('.');
    return Number(expiry)>Date.now() && Number(expiry)<Date.now()+3601000 && !!nonce && equal(mac,service.vault.hash('admin-session',`${expiry}.${nonce}`));
  };
  return createServer(async(req,res)=>{
    res.setHeader('X-Content-Type-Options','nosniff');res.setHeader('Referrer-Policy','no-referrer');res.setHeader('X-Frame-Options','DENY');
    res.setHeader('Content-Security-Policy',"default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' data:; connect-src 'self'; frame-ancestors 'none'; base-uri 'self'; form-action 'self'");
    const send=(status,data)=>{res.writeHead(status,{'Content-Type':'application/json; charset=utf-8','Cache-Control':'private, no-store'});res.end(JSON.stringify(data));};
    try {
      const path=new URL(req.url,'http://local').pathname;
      if(!path.startsWith('/api/')) {
        if(!['GET','HEAD'].includes(req.method))throw new Problem('METHOD','Method not allowed',405);
        let relative; try { relative=decodeURIComponent(path); } catch { throw new Problem('PATH','Not found',404); }
        if(relative.split('/').some(p=>p.startsWith('.') || p.includes('\\')))throw new Problem('PATH','Not found',404);
        let file=resolve(root,'.'+relative);
        if(file!==root && !file.startsWith(root+sep))throw new Problem('PATH','Not found',404);
        try { if(!(await stat(file)).isFile())file=resolve(root,'index.html'); } catch { if(extname(path))throw new Problem('PATH','Not found',404); file=resolve(root,'index.html'); }
        const data=await readFile(file);res.writeHead(200,{'Content-Type':MIME[extname(file)]||'application/octet-stream','Cache-Control':file.endsWith('index.html')?'no-cache':'public, max-age=3600'});res.end(req.method==='HEAD'?undefined:data);return;
      }
      res.setHeader('Cache-Control','private, no-store');
      const jar=cookies(req), remote=req.socket.remoteAddress||'unknown';
      const forwarded=req.headers['x-real-ip'];
      const ip=config.trustProxy && ['127.0.0.1','::1','::ffff:127.0.0.1'].includes(remote) && typeof forwarded==='string' && isIP(forwarded)?forwarded:remote;
      limit(`ip:${ip}`,2000);
      let device=jar.pai_device;
      if(!/^device-[A-Za-z0-9_-]{32}$/.test(device||'')){device='device-'+id();res.setHeader('Set-Cookie',cookie('pai_device',device,31536000));}
      const deviceKey=service.vault.hash('device',device);
      if(req.method==='GET' && path==='/api/session') {
        send(200,{ok:true,data:{mode:config.mode,brand:'π星卡社',prefix:'PAIPLUS',admin:adminValid(jar.pai_admin),recent:service.recent(device),examples:config.mode==='mock'?config.examples:[]}});return;
      }
      if(req.method!=='POST')throw new Problem('METHOD','Method not allowed',405);
      if(path==='/api/integration/deliver') {
        if(!equal(req.headers.authorization,`Bearer ${config.integrationToken}`))throw new Problem('UNAUTHORIZED','Unauthorized',401);
        send(200,{ok:true,data:service.deliver(await body(req))});return;
      }
      if(path.startsWith('/api/management/')) {
        if(!config.managementToken || !equal(req.headers.authorization,`Bearer ${config.managementToken}`))throw new Problem('UNAUTHORIZED','Unauthorized',401);
        const input=await body(req),actor=input.actor;
        if(!/^shop-admin:[1-9]\d{0,15}$/.test(actor||''))throw new Problem('ACTOR_REQUIRED','缺少管理员身份。',400);
        limit(`management:${actor}`,120);
        let data;
        if(path==='/api/management/cards')data=service.management.list(input);
        else if(path==='/api/management/detail')data=service.management.detail(input.card_id,actor);
        else if(path==='/api/management/check')data=await service.management.refresh(input.card_id,actor);
        else if(path==='/api/management/revoke')data=await service.management.revoke(input,actor);
        else throw new Problem('NOT_FOUND','接口不存在。',404);
        send(200,{ok:true,data});return;
      }
      if(req.headers.origin!==config.publicOrigin || req.headers['x-pai-request']!=='1')throw new Problem('ORIGIN','请求来源不匹配，请刷新页面。',403);
      const input=await body(req);
      if(path==='/api/demo/examples' && config.mode==='mock' && config.newExamples) {
        limit(`examples:${deviceKey}`,5);config.examples=config.newExamples();send(200,{ok:true,data:{examples:config.examples}});return;
      }
      if(path==='/api/admin/login') {
        limit(`login:${ip}`,5);
        if(!(config.mode==='mock' && input.demo===true) && !equal(input.password,config.adminToken))throw new Problem('LOGIN_FAILED','管理密钥不正确。',401);
        const signed=`${Date.now()+3600000}.${id()}`;
        res.setHeader('Set-Cookie',[cookie('pai_device',device,31536000),cookie('pai_admin',`${signed}.${service.vault.hash('admin-session',signed)}`,3600)]);
        service.store.audit('admin_login','admin');send(200,{ok:true,data:{authenticated:true}});return;
      }
      if(path.startsWith('/api/admin/')) {
        if(!adminValid(jar.pai_admin))throw new Problem('ADMIN_REQUIRED','请先登录管理页。',401);
        let data;
        if(path==='/api/admin/logout'){res.setHeader('Set-Cookie',cookie('pai_admin','',0));data={ok:true};}
        else if(path==='/api/admin/cards')data={cards:service.listCards(input.query),attempts:service.store.all('SELECT * FROM attempts ORDER BY created_at DESC LIMIT 100').map(a=>service.view(a,false))};
        else if(path==='/api/admin/import'){limit(`import:${deviceKey}`,5);data={cards:service.issue(input.rows)};}
        else if(path==='/api/admin/reveal')data=service.reveal(input.card_id);
        else if(path==='/api/admin/release-check')data=service.releaseCheck(input.card_id);
        else if(path==='/api/admin/disable'){if(typeof input.disabled!=='boolean')throw new Problem('INVALID_ACTION','请选择停用或启用。');data=service.disable(input.card_id,input.disabled);}
        else throw new Problem('NOT_FOUND','接口不存在。',404);
        send(200,{ok:true,data});return;
      }
      limit(`device:${deviceKey}`,400);
      let data;
      if(path==='/api/preview') {limit(`preview:${deviceKey}`,120);data=await service.preview(input.code,device);}
      else if(path==='/api/preflight') {limit(`preflight:${deviceKey}`,120);data=await service.preflight(input.attempt_id,input.session,device);}
      else if(path==='/api/redeem') {limit(`redeem:${deviceKey}`,120);data=await service.redeem(input.attempt_id,input.confirmation,input.confirmed,device);}
      else if(path==='/api/result')data=await service.result(input.attempt_id,device);
      else if(path==='/api/results')data={results:service.results(input.codes,device)};
      else if(path==='/api/result-by-code')data=await service.recover(input.code,device);
      else if(path==='/api/lookup-batch'){limit(`lookup:${deviceKey}`,200,Array.isArray(input.codes)?input.codes.length:1);data={results:await service.lookup(input.codes,device)};}
      else if(path==='/api/lookup'){limit(`lookup:${deviceKey}`,200);data=(await service.lookup([input.code],device))[0];}
      else if(path==='/api/billing'){limit(`billing:${deviceKey}`,10);data=await service.billing(input,device);}
      else throw new Problem('NOT_FOUND','接口不存在。',404);
      send(200,{ok:true,data});
    } catch(error) {
      const known=error instanceof Problem;
      if(!res.headersSent)send(known?error.status:500,{ok:false,error:{code:known?error.code:'INTERNAL',message:known?error.message:'暂时无法处理，请稍后再试。'}});
      else res.end();
    }
  });
}

