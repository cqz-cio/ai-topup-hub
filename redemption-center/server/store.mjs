import { DatabaseSync } from 'node:sqlite';
import { mkdirSync, chmodSync } from 'node:fs';
import { dirname } from 'node:path';

export class Store {
  constructor(path) {
    if (path !== ':memory:') mkdirSync(dirname(path),{recursive:true,mode:0o700});
    this.db = new DatabaseSync(path);
    this.db.exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000;
      CREATE TABLE IF NOT EXISTS meta (key TEXT PRIMARY KEY,value TEXT NOT NULL);
      CREATE TABLE IF NOT EXISTS cards (
        id TEXT PRIMARY KEY, fingerprint TEXT NOT NULL UNIQUE, code_enc TEXT NOT NULL,
        source_hash TEXT NOT NULL UNIQUE, source_enc TEXT NOT NULL, reference TEXT NOT NULL UNIQUE,
        label TEXT NOT NULL DEFAULT '', status TEXT NOT NULL DEFAULT 'ready', attempt_id TEXT,
        created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
      );
      CREATE TABLE IF NOT EXISTS attempts (
        id TEXT PRIMARY KEY, card_id TEXT NOT NULL REFERENCES cards(id), device_hash TEXT NOT NULL,
        device_enc TEXT NOT NULL, request_id TEXT NOT NULL UNIQUE, state TEXT NOT NULL,
        token_enc TEXT NOT NULL DEFAULT '', preflight_enc TEXT NOT NULL DEFAULT '',
        session_enc TEXT NOT NULL DEFAULT '', confirmation_hash TEXT NOT NULL DEFAULT '',
        data_enc TEXT NOT NULL DEFAULT '', revision INTEGER NOT NULL DEFAULT 0,
        checked_at INTEGER, submitted_at INTEGER, last_poll INTEGER NOT NULL DEFAULT 0,
        created_at INTEGER NOT NULL, updated_at INTEGER NOT NULL
      );
      CREATE INDEX IF NOT EXISTS attempts_device ON attempts(device_hash,created_at);
      CREATE TABLE IF NOT EXISTS audit (id INTEGER PRIMARY KEY, action TEXT NOT NULL, subject TEXT NOT NULL, at INTEGER NOT NULL);
      CREATE TABLE IF NOT EXISTS deliveries (reference TEXT PRIMARY KEY, fingerprint TEXT NOT NULL, card_ids TEXT NOT NULL);
      CREATE TABLE IF NOT EXISTS mock_tasks (code TEXT PRIMARY KEY, token TEXT UNIQUE, status TEXT, email TEXT, scenario TEXT, product TEXT, submitted_at INTEGER);
      CREATE TABLE IF NOT EXISTS card_operations (
        id TEXT PRIMARY KEY, card_id TEXT NOT NULL REFERENCES cards(id), actor TEXT NOT NULL,
        reason_enc TEXT NOT NULL, previous_status TEXT NOT NULL, attempt_id TEXT,
        state TEXT NOT NULL, error_code TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL,
        finished_at INTEGER
      );
      CREATE UNIQUE INDEX IF NOT EXISTS card_operation_pending ON card_operations(card_id) WHERE state='pending';
    `);
    const columns=new Set(this.all('PRAGMA table_info(cards)').map(c=>c.name));
    for(const [name,type] of Object.entries({order_id:'INTEGER NOT NULL DEFAULT 0',order_no:"TEXT NOT NULL DEFAULT ''",product_id:'INTEGER NOT NULL DEFAULT 0',sku_id:'INTEGER NOT NULL DEFAULT 0',upstream_snapshot_enc:"TEXT NOT NULL DEFAULT ''",upstream_checked_at:'INTEGER NOT NULL DEFAULT 0',revoked_at:'INTEGER NOT NULL DEFAULT 0'})) {
      if(!columns.has(name))this.db.exec(`ALTER TABLE cards ADD COLUMN ${name} ${type}`);
    }
    if(!this.all('PRAGMA table_info(audit)').some(c=>c.name==='actor'))this.db.exec("ALTER TABLE audit ADD COLUMN actor TEXT NOT NULL DEFAULT 'system'");
    if (path !== ':memory:') try { chmodSync(path,0o600); } catch { /* Windows ACLs govern access. */ }
  }
  get(sql,...args) { return this.db.prepare(sql).get(...args); }
  all(sql,...args) { return this.db.prepare(sql).all(...args); }
  run(sql,...args) { return this.db.prepare(sql).run(...args); }
  transaction(fn) { this.db.exec('BEGIN IMMEDIATE'); try { const result=fn(); this.db.exec('COMMIT'); return result; } catch(e) { this.db.exec('ROLLBACK'); throw e; } }
  audit(action,subject,actor='system') { this.run('INSERT INTO audit(action,subject,at,actor) VALUES(?,?,?,?)',action,subject,Date.now(),actor); }
  close() { this.db.close(); }
}

