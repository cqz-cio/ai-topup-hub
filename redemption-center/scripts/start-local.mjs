import { existsSync, mkdirSync, readFileSync, writeFileSync } from 'node:fs';
import { randomBytes } from 'node:crypto';
import { resolve, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';
import { start } from '../server/main.mjs';
process.chdir(resolve(dirname(fileURLToPath(import.meta.url)),'..'));
mkdirSync('runtime',{recursive:true,mode:0o700});
const path='runtime/local-secrets.json';
if(!existsSync(path))writeFileSync(path,JSON.stringify({master:randomBytes(32).toString('base64'),admin:randomBytes(32).toString('base64url'),integration:randomBytes(32).toString('base64url')}),{mode:0o600});
const secrets=JSON.parse(readFileSync(path,'utf8'));
if(!secrets.management){secrets.management=randomBytes(32).toString('base64url');writeFileSync(path,JSON.stringify(secrets),{mode:0o600});}
start({...process.env,REDEMPTION_MODE:'mock',REDEMPTION_HOST:'127.0.0.1',REDEMPTION_MASTER_KEY:secrets.master,REDEMPTION_ADMIN_TOKEN:secrets.admin,REDEMPTION_INTEGRATION_TOKEN:secrets.integration,REDEMPTION_MANAGEMENT_TOKEN:secrets.management});

