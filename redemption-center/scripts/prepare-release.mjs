import { cp, mkdir, readFile, readdir } from 'node:fs/promises';
import { resolve, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root=resolve(fileURLToPath(new URL('..',import.meta.url)));
const target=join(root,'dist','release');
await mkdir(target,{recursive:true});
await readFile(join(root,'dist','client','index.html'));
for(const item of ['server','ops','README.md','.env.example'])await cp(join(root,item),join(target,item),{recursive:true});
await mkdir(join(target,'scripts'),{recursive:true});
await cp(join(root,'scripts','backup.mjs'),join(target,'scripts','backup.mjs'));
await cp(join(root,'dist','client'),join(target,'dist','client'),{recursive:true});
async function inspect(dir){for(const entry of await readdir(dir,{withFileTypes:true})){const path=join(dir,entry.name);if(['node_modules','runtime','.git'].includes(entry.name)||entry.name==='.env'||/\.db(?:-wal|-shm)?$/.test(entry.name))throw new Error('Private file in release');if(entry.isDirectory())await inspect(path);}}
await inspect(target);
console.log('Prepared Node release: dist/release (server + built frontend; no runtime data)');
