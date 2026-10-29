import { Problem } from './security.mjs';

const sourcePattern = /^[A-Za-z0-9_-]{8,256}$/;
const accountPattern = /^邮箱号[：:]\s*(.+?)\s*邮箱密码[：:]\s*(.+?)\s*密钥2fa[：:]\s*([A-Z2-7]+)\s*密钥2fa在线获取[：:]\s*(https:\/\/\S+?)\s*兑换卡密[：:]\s*([A-Za-z0-9_-]{8,256})$/i;
const invalid = () => new Problem('DELIVERY_FORMAT','交付卡密数量或格式不匹配，保留原交付等待人工核对。',422);

// Only known delivery grammars are accepted. Supplier prose, alternate links and
// unexpected fields are never copied into a customer delivery.
export function parseDelivery(payload, quantity) {
  const records=[];
  for(const line of payload.split(/\r?\n/).map(v=>v.trim()).filter(Boolean)) {
    if(/^(?:兑换地址|充值地址|兑换链接|充值链接)[：:]\s*https?:\/\/\S+$/.test(line))continue;
    const code=line.replace(/^(?:兑换)?卡密[：:]\s*/,'');
    if(sourcePattern.test(code)) { records.push({source:code});continue; }
    const account=line.match(accountPattern);
    if(!account)throw invalid();
    const [,email,password,twoFactor,twoFactorURL,source]=account;
    let url;
    try { url=new URL(twoFactorURL); } catch { throw invalid(); }
    if(!/^[^\s@]+@[^\s@]+\.[^\s@]+$/.test(email) || email.length>254 || !/^\S{1,256}$/.test(password) ||
      twoFactor.length>128 || url.username || url.password || url.search || url.hash ||
      url.hostname==='cdk.bar' || url.hostname.endsWith('.cdk.bar') || twoFactorURL.length>512)throw invalid();
    records.push({source,account:{email,password,two_factor:twoFactor,two_factor_url:twoFactorURL}});
  }
  const sources=records.map(r=>r.source);
  if(records.length!==quantity || new Set(sources).size!==quantity || sources.some(c=>c.toUpperCase().startsWith('PAIPLUS-')))throw invalid();
  // A supplier code repeated in an account field must not bypass the grammar.
  if(records.some(r=>r.account && sources.some(c=>JSON.stringify(r.account).includes(c))))throw invalid();
  return records;
}

export function customerDelivery(cards, publicOrigin) {
  const blocks=cards.map(({code,account})=>account?
    `邮箱号：${account.email}\n邮箱密码：${account.password}\n密钥2fa：${account.two_factor}\n密钥2fa在线获取：${account.two_factor_url}\n兑换卡密：${code}`:
    `卡密：${code}`);
  return {format:'paiplus-v1',cards,count:cards.length,payload:blocks.join('\n\n')+`\n兑换地址：${publicOrigin}/`};
}
