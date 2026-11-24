import {DatabaseSync} from 'node:sqlite';
import {randomBytes} from 'node:crypto';
import {mkdir,lstat,open,chmod} from 'node:fs/promises';
import {isAbsolute,join} from 'node:path';
import {browserFileLimit} from './browser-file-snapshot.mjs';
import {uploadResourceID,uploadBinding,uploadGrant,uploadNativeIdentity,uploadAction} from './browser-upload-ledger-values.mjs';

const phases=['reserved','directory','file','ready','in_use','preparation_aborted','deleting','removed'];
const schema=`
CREATE TABLE IF NOT EXISTS browser_upload_schema(singleton INTEGER PRIMARY KEY CHECK(singleton=1),version INTEGER NOT NULL CHECK(version>0),max_bytes INTEGER NOT NULL CHECK(max_bytes BETWEEN 0 AND 68719476736),max_active INTEGER NOT NULL CHECK(max_active BETWEEN 1 AND 1024)) STRICT;
CREATE TABLE IF NOT EXISTS browser_upload_resources(
 id TEXT PRIMARY KEY CHECK(length(id)=32), owner TEXT NOT NULL CHECK(length(owner)=32),
 actor_id TEXT NOT NULL CHECK(length(actor_id) BETWEEN 1 AND 128), workspace_id TEXT NOT NULL CHECK(length(workspace_id) BETWEEN 1 AND 128), run_id TEXT NOT NULL CHECK(length(run_id)=36), input_id TEXT NOT NULL CHECK(length(input_id) BETWEEN 7 AND 128), session_id TEXT NOT NULL CHECK(length(session_id)=32),
 grant_id TEXT NOT NULL CHECK(length(grant_id)=32), name TEXT NOT NULL CHECK(length(name) BETWEEN 1 AND 255), size INTEGER NOT NULL CHECK(size BETWEEN 0 AND 536870912), sha256 TEXT NOT NULL CHECK(length(sha256)=64),
 state TEXT NOT NULL CHECK(state IN ('reserved','directory','file','ready','in_use','preparation_aborted','deleting','removed')),
 directory_identity TEXT NOT NULL DEFAULT '' CHECK(length(directory_identity)<=256), file_identity TEXT NOT NULL DEFAULT '' CHECK(length(file_identity)<=256),
 action_id TEXT NOT NULL DEFAULT '' CHECK(length(action_id)<=256), document_id TEXT NOT NULL DEFAULT '' CHECK(length(document_id)<=36),
 browser_closed INTEGER NOT NULL DEFAULT 0 CHECK(browser_closed IN (0,1)), created_ms INTEGER NOT NULL CHECK(created_ms>=0)
) STRICT;
CREATE INDEX IF NOT EXISTS browser_upload_active ON browser_upload_resources(state,id);
`;
const freshID=()=>randomBytes(16).toString('hex');
const frozenRecord=row=>{
  if(!uploadResourceID(row.id)||!uploadResourceID(row.owner)||!phases.includes(row.state)||![0,1].includes(row.browser_closed)||!Number.isSafeInteger(row.created_ms)||row.created_ms<0)throw Error('upload_ledger_corrupt');
  const binding=uploadBinding({actorId:row.actor_id,workspaceId:row.workspace_id,runId:row.run_id,inputId:row.input_id,sessionId:row.session_id});
  const grant=uploadGrant({id:row.grant_id,name:row.name,size:row.size,sha256:row.sha256});
  const directoryIdentity=row.directory_identity?uploadNativeIdentity(JSON.parse(row.directory_identity)):null;
  const fileIdentity=row.file_identity?uploadNativeIdentity(JSON.parse(row.file_identity)):null;
  if(row.state!=='reserved'&&row.state!=='preparation_aborted'&&row.state!=='deleting'&&row.state!=='removed'&&!directoryIdentity||['file','ready','in_use'].includes(row.state)&&!fileIdentity||fileIdentity&&!directoryIdentity)throw Error('upload_ledger_corrupt');
  if(row.action_id)uploadAction({actionId:row.action_id,...(row.document_id?{documentId:row.document_id}:{})},binding.runId);
  else if(row.document_id||row.state==='in_use')throw Error('upload_ledger_corrupt');
  if(row.action_id&&(!['in_use','deleting','removed'].includes(row.state)||row.state!=='in_use'&&!row.browser_closed))throw Error('upload_ledger_corrupt');
  return Object.freeze({resourceId:row.id,relativeDirectory:`uploads/${row.id}`,binding,grant,state:row.state,directoryIdentity,fileIdentity,actionId:row.action_id||null,documentId:row.document_id||null,browserClosed:row.browser_closed===1,createdAt:row.created_ms});
};

// Storage only: this module never deletes files, proves process death, adopts a
// previous owner's rows, grants task authority or changes dispatch tombstones.
export class BrowserUploadLedger {
  #db;#owner=freshID();#closed=false;#browserClosed=false;#maxBytes;#maxActive;
  static async open({directory,maxRetainedBytes=browserFileLimit,maxActiveRecords=128}={}){
    if(!isAbsolute(directory??'')||!Number.isSafeInteger(maxRetainedBytes)||maxRetainedBytes<0||maxRetainedBytes>128*browserFileLimit||!Number.isSafeInteger(maxActiveRecords)||maxActiveRecords<1||maxActiveRecords>1024)throw Error('upload_ledger_invalid');
    await mkdir(directory,{recursive:true,mode:0o700});
    const root=await lstat(directory);if(!root.isDirectory()||root.isSymbolicLink())throw Error('upload_ledger_storage_invalid');
    const path=join(directory,'dispatch.sqlite');
    try{const created=await open(path,'wx',0o600);await created.close();}catch(error){if(error.code!=='EEXIST')throw error;}
    const observed=await lstat(path,{bigint:true});if(!observed.isFile()||observed.isSymbolicLink())throw Error('upload_ledger_storage_invalid');
    let db;
    try{
      db=new DatabaseSync(path);const opened=await lstat(path,{bigint:true});
      if(opened.dev!==observed.dev||opened.ino!==observed.ino||!opened.isFile()||opened.isSymbolicLink())throw Error('upload_ledger_storage_invalid');
      await chmod(path,0o600);
      return new BrowserUploadLedger(db,maxRetainedBytes,maxActiveRecords);
    }catch(error){try{db?.close();}catch{}throw error;}
  }
  constructor(db,maxBytes,maxActive){
    this.#db=db;this.#maxBytes=maxBytes;this.#maxActive=maxActive;
    db.exec('PRAGMA journal_mode=DELETE; PRAGMA synchronous=EXTRA; PRAGMA busy_timeout=5000; PRAGMA secure_delete=ON');
    if(![0,2].includes(db.prepare('PRAGMA user_version').get().user_version))throw Error('upload_ledger_incompatible');
    if(db.prepare('PRAGMA quick_check').get().quick_check!=='ok')throw Error('upload_ledger_corrupt');
    this.#transaction(()=>{
      const existing=db.prepare("SELECT name FROM sqlite_master WHERE type='table' AND name IN ('browser_upload_schema','browser_upload_resources')").all();
      if(existing.length===1)throw Error('upload_ledger_corrupt');
      const meta=existing.length?db.prepare('SELECT * FROM browser_upload_schema WHERE singleton=1').get():undefined;
      if(!meta&&existing.length)throw Error('upload_ledger_corrupt');
      if(meta&&meta.version!==1)throw Error('upload_ledger_incompatible');
      if(meta&&(meta.max_bytes!==maxBytes||meta.max_active!==maxActive))throw Error('upload_ledger_policy_changed');
      db.exec(schema);
      if(!meta)db.prepare('INSERT INTO browser_upload_schema VALUES(1,1,?,?)').run(maxBytes,maxActive);
      const tables=db.prepare('PRAGMA table_list').all();
      for(const name of ['browser_upload_schema','browser_upload_resources'])if(!tables.some(table=>table.name===name&&table.type==='table'&&table.strict===1))throw Error('upload_ledger_corrupt');
    });
  }
  #transaction(action){
    if(this.#closed)throw Error('upload_ledger_closed');
    this.#db.exec('BEGIN IMMEDIATE');
    try{
      if(this.#db.prepare('PRAGMA synchronous').get().synchronous!==3||this.#db.prepare('PRAGMA journal_mode').get().journal_mode!=='delete')throw Error('upload_ledger_durability_unavailable');
      const result=action();this.#db.exec('COMMIT');return result;
    }catch(error){try{this.#db.exec('ROLLBACK');}catch{}throw error;}
  }
  #owned(id){
    if(!uploadResourceID(id))throw Error('upload_ledger_invalid');
    const row=this.#db.prepare('SELECT * FROM browser_upload_resources WHERE id=?').get(id);
    if(!row||row.owner!==this.#owner)throw Error('upload_ledger_owner_mismatch');
    frozenRecord(row);return row;
  }
  #checkWrite(before,after,values){
    for(const key of Object.keys(before))if(after[key]!==((key in values)?values[key]:before[key]))throw Error('upload_ledger_write_unconfirmed');
  }
  #write(id,values){
    const before=this.#owned(id),keys=Object.keys(values);
    if(keys.some(key=>!['state','directory_identity','file_identity','action_id','document_id'].includes(key)))throw Error('upload_ledger_invalid');
    const changed=this.#db.prepare(`UPDATE browser_upload_resources SET ${keys.map(key=>key+'=?').join(',')} WHERE id=?`).run(...keys.map(key=>values[key]),id);
    if(changed.changes!==1)throw Error('upload_ledger_write_unconfirmed');
    const after=this.#owned(id);this.#checkWrite(before,after,values);return frozenRecord(after);
  }
  #change(id,states,values){
    return this.#transaction(()=>{
      const row=this.#owned(id);if(!states.includes(row.state))throw Error('upload_ledger_transition');
      return this.#write(id,values);
    });
  }
  reserve({binding,grant}={}){
    binding=uploadBinding(binding);grant=uploadGrant(grant);
    return this.#transaction(()=>{
      if(this.#browserClosed)throw Error('upload_ledger_owner_closed');
      const totals=this.#db.prepare("SELECT COUNT(*) AS count,COALESCE(SUM(size),0) AS bytes FROM browser_upload_resources WHERE state!='removed'").get();
      if(!Number.isSafeInteger(totals.bytes)||totals.bytes<0||totals.count>=this.#maxActive||grant.size>this.#maxBytes-totals.bytes)throw Error('upload_ledger_capacity');
      const id=freshID(),created=Date.now();
      const changed=this.#db.prepare(`INSERT INTO browser_upload_resources(id,owner,actor_id,workspace_id,run_id,input_id,session_id,grant_id,name,size,sha256,state,created_ms) VALUES(?,?,?,?,?,?,?,?,?,?,?,'reserved',?)`).run(id,this.#owner,binding.actorId,binding.workspaceId,binding.runId,binding.inputId,binding.sessionId,grant.id,grant.name,grant.size,grant.sha256,created);
      if(changed.changes!==1)throw Error('upload_ledger_write_unconfirmed');
      const actual=this.#owned(id),expected={id,owner:this.#owner,actor_id:binding.actorId,workspace_id:binding.workspaceId,run_id:binding.runId,input_id:binding.inputId,session_id:binding.sessionId,grant_id:grant.id,name:grant.name,size:grant.size,sha256:grant.sha256,state:'reserved',directory_identity:'',file_identity:'',action_id:'',document_id:'',browser_closed:0,created_ms:created};
      this.#checkWrite(expected,actual,{});
      const after=this.#db.prepare("SELECT COUNT(*) AS count,COALESCE(SUM(size),0) AS bytes FROM browser_upload_resources WHERE state!='removed'").get();
      if(after.count!==totals.count+1||after.bytes!==totals.bytes+grant.size)throw Error('upload_ledger_write_unconfirmed');
      return frozenRecord(actual);
    });
  }
  recordDirectory(id,identity){return this.#change(id,['reserved'],{directory_identity:JSON.stringify(uploadNativeIdentity(identity)),state:'directory'});}
  recordFile(id,identity){return this.#change(id,['directory'],{file_identity:JSON.stringify(uploadNativeIdentity(identity)),state:'file'});}
  ready(id,metadata){
    return this.#transaction(()=>{
      const row=this.#owned(id);
      if(this.#browserClosed||row.browser_closed||row.state!=='file'||!metadata||Object.keys(metadata).sort().join(',')!=='sha256,size'||metadata.size!==row.size||metadata.sha256!==row.sha256)throw Error('upload_ledger_transition');
      return this.#write(id,{state:'ready'});
    });
  }
  inUse(id,action){
    return this.#transaction(()=>{
      const row=this.#owned(id);action=uploadAction(action,row.run_id);
      if(this.#browserClosed||row.browser_closed||!['ready','in_use'].includes(row.state))throw Error('upload_ledger_transition');
      return this.#write(id,{state:'in_use',action_id:action.actionId,document_id:action.documentId});
    });
  }
  abortPreparation(id){return this.#change(id,['reserved','directory','file','ready','preparation_aborted'],{state:'preparation_aborted'});}
  browserClosed(){
    this.#transaction(()=>{
      const rows=this.#db.prepare("SELECT * FROM browser_upload_resources WHERE owner=? AND state!='removed'").all(this.#owner);
      if(rows.length>this.#maxActive)throw Error('upload_ledger_corrupt');
      for(const row of rows)frozenRecord(row);
      const changed=this.#db.prepare("UPDATE browser_upload_resources SET browser_closed=1 WHERE owner=? AND state!='removed'").run(this.#owner);
      if(changed.changes!==rows.length)throw Error('upload_ledger_write_unconfirmed');
      for(const row of rows)this.#checkWrite(row,this.#owned(row.id),{browser_closed:1});
      const total=this.#db.prepare("SELECT COUNT(*) AS count FROM browser_upload_resources WHERE owner=? AND state!='removed'").get(this.#owner).count;
      if(total!==rows.length)throw Error('upload_ledger_write_unconfirmed');
    });
    this.#browserClosed=true;
  }
  beginCleanup(id){
    return this.#transaction(()=>{
      const row=this.#owned(id);
      if(row.state==='removed'||!row.browser_closed&&row.state!=='preparation_aborted'&&row.state!=='deleting')throw Error('upload_ledger_transition');
      return this.#write(id,{state:'deleting'});
    });
  }
  removed(id){return this.#change(id,['deleting'],{state:'removed'});}
  listRetained({after='',limit=64}={}){
    if(this.#closed)throw Error('upload_ledger_closed');
    if(after!==''&&!uploadResourceID(after)||!Number.isSafeInteger(limit)||limit<1||limit>64)throw Error('upload_ledger_invalid');
    const rows=this.#db.prepare("SELECT * FROM browser_upload_resources WHERE state!='removed' AND id>? ORDER BY id LIMIT ?").all(after,limit+1);
    const hasMore=rows.length>limit;if(hasMore)rows.pop();
    const records=rows.map(row=>Object.freeze({...frozenRecord(row),currentOwner:row.owner===this.#owner,recoveryState:row.owner===this.#owner?'live_owner':'recovery_uncertain'}));
    return Object.freeze({records:Object.freeze(records),next:hasMore?records.at(-1).resourceId:null});
  }
  close(){if(!this.#closed){this.#db.close();this.#closed=true;}}
}
