// Host-local duplicate-dispatch protection. Never include credentials or action
// arguments in this journal. It is not restored with service/workspace backups.
import {createHash} from 'node:crypto';
import {DatabaseSync} from 'node:sqlite';

function canonical(value) {
  if (value === null || typeof value !== 'object') return JSON.stringify(value);
  if (Array.isArray(value)) return `[${value.map(canonical).join(',')}]`;
  return `{${Object.keys(value).sort().map(key => `${JSON.stringify(key)}:${canonical(value[key])}`).join(',')}}`;
}

export class DispatchJournal {
  constructor(path) {
    this.db = new DatabaseSync(path);
    try {
      this.db.exec('PRAGMA journal_mode=DELETE; PRAGMA synchronous=FULL; PRAGMA busy_timeout=5000; PRAGMA secure_delete=ON');
      const version = this.db.prepare('PRAGMA user_version').get().user_version;
      if (version !== 0 && version !== 2) throw Error('journal_incompatible');
      const integrity = this.db.prepare('PRAGMA quick_check').get().quick_check;
      if (integrity !== 'ok') throw Error('journal_corrupt');
      this.db.exec('BEGIN IMMEDIATE');
      try {
        this.db.exec('CREATE TABLE IF NOT EXISTS dispatch (id TEXT PRIMARY KEY, hash TEXT NOT NULL, status TEXT NOT NULL)');
        if (version === 0) {
          // Keep every old tombstone. Its old hash lacks a session/workspace
          // binding, so it must never authorize execution or result replay.
          this.db.exec("ALTER TABLE dispatch ADD COLUMN binding TEXT NOT NULL DEFAULT ''; ALTER TABLE dispatch ADD COLUMN reply TEXT; PRAGMA user_version=2");
        }
        this.db.exec('COMMIT');
      } catch (error) { this.db.exec('ROLLBACK'); throw error; }
    } catch (error) { this.db.close(); throw error; }
  }

  prepare(action, binding) {
    if (!action || !/^[a-zA-Z0-9:_-]{1,256}$/.test(action.id) ||
        typeof action.kind !== 'string' || !action.arguments || Array.isArray(action.arguments) ||
        typeof action.arguments !== 'object' || typeof binding !== 'string' || !binding) throw Error('invalid_action');
    const encoded = canonical([action.kind, action.arguments]);
    if (Buffer.byteLength(encoded) > 20 * 1024) throw Error('invalid_action');
    const hash = createHash('sha256').update(encoded).digest('hex');
    const scope = createHash('sha256').update(binding).digest('hex');
    // BEGIN IMMEDIATE also prevents two processes from claiming the same ID.
    this.db.exec('BEGIN IMMEDIATE');
    try {
      const previous = this.db.prepare('SELECT hash,status,binding,reply FROM dispatch WHERE id=?').get(action.id);
      if (previous) {
        if (previous.binding === '') throw Error('action_uncertain');
        if (previous.hash !== hash || previous.binding !== scope) throw Error('action_conflict');
        if (previous.status !== 'completed' || previous.reply === null) throw Error('action_uncertain');
        this.db.exec('COMMIT');
        return {execute:false, reply:JSON.parse(previous.reply)};
      }
      this.db.prepare('INSERT INTO dispatch(id,hash,status,binding) VALUES(?,?,?,?)').run(action.id,hash,'dispatched',scope);
      this.db.exec('COMMIT');
      return {execute:true};
    } catch (error) { try { this.db.exec('ROLLBACK'); } catch {} throw error; }
  }

  complete(id, reply) {
    if (reply.id !== id || typeof reply.result !== 'string' || Buffer.byteLength(reply.result) > 48 * 1024 ||
        (reply.error !== undefined && (typeof reply.error !== 'string' || Buffer.byteLength(reply.error) > 1024))) throw Error('invalid_result');
    const changed = this.db.prepare("UPDATE dispatch SET status='completed',reply=? WHERE id=? AND status='dispatched'").run(JSON.stringify(reply),id);
    if (changed.changes !== 1) throw Error('action_conflict');
  }

  // Results may contain page text. Retain them only for the live session; keep
  // permanent hash/status tombstones so cleanup never makes an ID executable.
  forgetResults(binding) {
    const scope = createHash('sha256').update(binding).digest('hex');
    this.db.prepare('UPDATE dispatch SET reply=NULL WHERE binding=?').run(scope);
  }
  discardPreviousSessionResults(binding) {
    const scope = createHash('sha256').update(binding).digest('hex');
    this.db.prepare('UPDATE dispatch SET reply=NULL WHERE binding<>?').run(scope);
  }
  close() { this.db.close(); }
}
