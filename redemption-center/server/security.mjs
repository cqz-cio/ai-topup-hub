import { randomBytes, randomInt, createHmac, createCipheriv, createDecipheriv, timingSafeEqual } from 'node:crypto';

export class Problem extends Error {
  constructor(code, message, status = 400) { super(message); this.code = code; this.status = status; }
}
export const id = () => randomBytes(24).toString('base64url');
const alphabet = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789';
export function newCard() {
  const group = () => { let s; do { s = Array.from({length:10}, () => alphabet[randomInt(alphabet.length)]).join(''); } while (!/[A-Z]/.test(s) || !/[2-9]/.test(s)); return s; };
  return `PAIPLUS-${group()}-${group()}`;
}
export function normalizeCard(value) {
  if (typeof value !== 'string') throw new Problem('INVALID_CODE', '请填写本站卡密。');
  const code = value.trim().toUpperCase();
  if (!/^PAIPLUS-[A-HJ-NP-Z2-9]{10}-[A-HJ-NP-Z2-9]{10}$/.test(code)) throw new Problem('INVALID_CODE', '请输入完整的 PAIPLUS 卡密，保留两处分隔横线。');
  return code;
}
export const maskedCard = code => `${code.slice(0,7)}-••••••••••-••••••${code.slice(-4)}`;
export function equal(a, b) { const x = Buffer.from(String(a || '')), y = Buffer.from(String(b || '')); return x.length === y.length && timingSafeEqual(x,y); }
export function vault(key) {
  const secret = Buffer.from(key || '', 'base64');
  if (secret.length !== 32) throw new Error('REDEMPTION_MASTER_KEY must be a base64 encoded 32-byte key');
  const mac = (purpose, value) => createHmac('sha256', secret).update(`${purpose}\0${value}`).digest('hex');
  return {
    hash: mac,
    encrypt(value) { if (!value) return ''; const iv = randomBytes(12), c = createCipheriv('aes-256-gcm', secret, iv); c.setAAD(Buffer.from('paiplus:v1')); return Buffer.concat([iv,c.update(String(value),'utf8'),c.final(),c.getAuthTag()]).toString('base64'); },
    decrypt(value) { if (!value) return ''; const b = Buffer.from(value,'base64'), d = createDecipheriv('aes-256-gcm',secret,b.subarray(0,12)); d.setAAD(Buffer.from('paiplus:v1')); d.setAuthTag(b.subarray(-16)); return Buffer.concat([d.update(b.subarray(12,-16)),d.final()]).toString('utf8'); },
  };
}
export const terminal = s => ['completed','declined','failed_precharge','cancelled','failed'].includes(s);
export function safeURL(value) { try { const u = new URL(value); return u.protocol === 'https:' && !u.username && !u.password ? u.href : null; } catch { return null; } }
export function credential(value) {
  if (typeof value !== 'string' || !value.trim() || Buffer.byteLength(value,'utf8') > 131072) throw new Problem('INVALID_CREDENTIAL','请填写完整账号凭证，长度不能超过 128 KiB。');
  return value.trim();
}

