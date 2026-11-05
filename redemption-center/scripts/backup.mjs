import { DatabaseSync, backup } from 'node:sqlite';
import { existsSync } from 'node:fs';
import { resolve } from 'node:path';

const [source,target]=process.argv.slice(2);
if(!source||!target)throw new Error('Usage: node scripts/backup.mjs SOURCE_DB NEW_BACKUP_DB');
if(!existsSync(source)||existsSync(target)||resolve(source)===resolve(target))throw new Error('Source must exist; backup destination must be new');
const db=new DatabaseSync(resolve(source),{readOnly:true});
try {await backup(db,resolve(target));console.log('SQLite consistent backup completed');} finally {db.close();}
