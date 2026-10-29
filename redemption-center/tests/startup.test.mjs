import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, symlink, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join, resolve, dirname, basename } from 'node:path';
import { fileURLToPath } from 'node:url';
import { spawn } from 'node:child_process';
import { randomBytes } from 'node:crypto';

test('service starts through an absolute current-release directory link', {timeout:15000}, async()=>{
  const dir=await mkdtemp(join(tmpdir(),'paiplus-startup-'));
  const server=resolve(fileURLToPath(new URL('../server',import.meta.url)));
  await symlink(server,join(dir,'current'),process.platform==='win32'?'junction':'dir');
  const child=spawn(process.execPath,[join(dir,'current','main.mjs')],{cwd:dir,windowsHide:true,env:{...process.env,
    REDEMPTION_MODE:'mock',REDEMPTION_HOST:'127.0.0.1',REDEMPTION_PORT:'0',REDEMPTION_PUBLIC_ORIGIN:'http://127.0.0.1:5191',
    REDEMPTION_DB:join(dir,'test.db'),REDEMPTION_MASTER_KEY:randomBytes(32).toString('base64'),
    REDEMPTION_ADMIN_TOKEN:randomBytes(32).toString('hex'),REDEMPTION_INTEGRATION_TOKEN:randomBytes(32).toString('hex'),
  }});
  const closed=new Promise(r=>child.once('close',r));
  try {
    await new Promise((accept,reject)=>{
      const timeout=setTimeout(()=>reject(new Error('linked service did not start')),8000);
      child.once('error',reject);
      child.once('exit',code=>{clearTimeout(timeout);reject(new Error(`linked service exited before ready (${code})`));});
      child.stdout.on('data',chunk=>{if(String(chunk).includes('PAIPLUS mock API listening')){clearTimeout(timeout);accept();}});
    });
    assert.equal(child.exitCode,null);
  } finally {
    child.kill();await closed;
    assert.equal(dirname(resolve(dir)),resolve(tmpdir()));
    assert.ok(basename(dir).startsWith('paiplus-startup-'));
    await rm(dir,{recursive:true,force:true});
  }
});
