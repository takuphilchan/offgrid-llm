import {mkdir,lstat} from 'node:fs/promises';
import {isAbsolute,join} from 'node:path';
import {BrowserFileSnapshot,validateBrowserUploadGrant} from './browser-file-snapshot.mjs';
import {BrowserUploadLedger} from './browser-upload-ledger.mjs';
import {uploadBinding,uploadIdentityFromStat} from './browser-upload-ledger-values.mjs';
import {browserFileAction} from './browser-file-binding.mjs';

const bindingKey=value=>JSON.stringify([value.actorId,value.workspaceId,value.runId,value.inputId,value.sessionId]);
const sameIdentity=(first,last)=>Object.keys(first).every(key=>first[key]===last[key]);

// The caller must establish private host storage before opening this owner.
// This owns allocation records and bounded files, not OS ACL policy, task
// approval or proof of browser termination. Reopened rows are never adopted.
export class BrowserOwnedUploadStore {
 static async open({directory,binding,maxRetainedBytes,maxActiveRecords}={}){
  binding=uploadBinding(binding);
  if(!isAbsolute(directory??''))throw Error('upload_store_invalid');
  const ledger=await BrowserUploadLedger.open({directory,maxRetainedBytes,maxActiveRecords});
  try{
   const root=join(directory,'uploads');await mkdir(root,{mode:0o700}).catch(error=>{if(error.code!=='EEXIST')throw error;});
   const stat=await lstat(root,{bigint:true});if(!stat.isDirectory()||stat.isSymbolicLink())throw Error('upload_store_invalid');
   return new BrowserOwnedUploadStore(ledger,root,uploadIdentityFromStat(stat),binding);
  }catch(error){ledger.close();throw error;}
 }
 constructor(ledger,root,identity,binding){
  this.ledger=ledger;this.root=root;this.rootIdentity=identity;this.binding=binding;
  this.records=new Map();this.stopped=false;this.closed=false;this.browserClosure=false;
 }
 rebind(value){
  const binding=uploadBinding(value);
  if(this.stopped||this.preparing||binding.actorId!==this.binding.actorId||binding.workspaceId!==this.binding.workspaceId)throw Error('upload_store_scope_invalid');
  this.binding=binding;
 }
 async checkRoot(){
  const stat=await lstat(this.root,{bigint:true});
  if(!stat.isDirectory()||stat.isSymbolicLink()||!sameIdentity(this.rootIdentity,uploadIdentityFromStat(stat)))throw Error('upload_store_ownership_changed');
 }
 validateAction(action){
  if(!action||bindingKey(action.binding??{})!==bindingKey(this.binding))throw Error('upload_store_scope_invalid');
  return browserFileAction({id:action.actionId,file_binding:{version:1,actor_id:action.binding.actorId,workspace_id:action.binding.workspaceId,run_id:action.binding.runId,input_id:action.binding.inputId,session_id:action.binding.sessionId}},this.binding);
 }
 async get(grant,action){
  if(this.stopped||this.preparing)throw Error('upload_store_unavailable');
  grant=Object.freeze({...grant});validateBrowserUploadGrant(grant);this.validateAction(action);
  const pending=this.prepare(grant);this.preparing=pending;
  try{return await pending;}finally{if(this.preparing===pending)this.preparing=null;}
 }
 async prepare(grant){
  await this.checkRoot();if(this.stopped)throw Error('upload_store_unavailable');
  const existing=this.records.get(grant.id);
  if(existing){
   if(existing.key!==bindingKey(this.binding)||existing.snapshot.state!=='ready'||existing.grant.name!==grant.name||existing.grant.size!==grant.size||existing.grant.sha256!==grant.sha256)throw Error('upload_store_scope_invalid');
   const verifier=new BrowserFileSnapshot(this.root);this.verifier=verifier;
   try{await verifier.verifySource(grant);if(this.stopped)throw Error('upload_store_unavailable');}
   finally{await verifier.close();this.verifier=null;}
   return existing.snapshot;
  }
  const metadata={id:grant.id,name:grant.name,size:grant.size,sha256:grant.sha256};
  const resource=this.ledger.reserve({binding:this.binding,grant:metadata});
  const record={resource,grant:metadata,key:bindingKey(this.binding)};
  const snapshot=new BrowserFileSnapshot(this.root,{allocation:{
   resourceId:resource.resourceId,
   directory:stat=>{record.resource=this.ledger.recordDirectory(resource.resourceId,uploadIdentityFromStat(stat));},
   file:stat=>{record.resource=this.ledger.recordFile(resource.resourceId,uploadIdentityFromStat(stat));},
   ready:metadata=>{record.resource=this.ledger.ready(resource.resourceId,metadata);},
   beforeCleanup:async()=>{await this.checkRoot();record.resource=this.ledger.beginCleanup(resource.resourceId);},
   afterCleanup:()=>{record.resource=this.ledger.removed(resource.resourceId);},
  }});
  record.snapshot=snapshot;this.records.set(grant.id,record);
  try{await snapshot.prepare(grant);if(this.stopped)throw Error('upload_store_unavailable');return snapshot;}
  catch(error){
   try{record.resource=this.ledger.abortPreparation(resource.resourceId);await snapshot.close();this.records.delete(grant.id);}
   catch(cleanup){throw new AggregateError([error,cleanup],'Upload preparation failed; ownership remains retained.');}
   throw error;
  }
 }
 async inUse(snapshot,action,documentId){
  if(this.stopped||this.preparing)throw Error('upload_store_unavailable');action=this.validateAction(action);
  const record=this.records.get(snapshot.metadata?.id);
  if(!record||record.snapshot!==snapshot||record.key!==bindingKey(this.binding)||snapshot.state!=='ready')throw Error('upload_store_scope_invalid');
  await this.checkRoot();if(this.stopped)throw Error('upload_store_unavailable');
  record.resource=this.ledger.inUse(record.resource.resourceId,{actionId:action.actionId,documentId});
 }
 stopPreparing(){this.stopped=true;this.verifier?.abort.abort();for(const record of this.records.values())record.snapshot.abort.abort();}
 async browserClosed(){
  this.stopPreparing();await this.preparing?.catch(()=>{});
  if(this.verifier){await this.verifier.close();this.verifier=null;}
  this.ledger.browserClosed();this.browserClosure=true;
 }
 close(){
  if(this.closed)return Promise.resolve();if(this.closing)return this.closing;
  this.stopPreparing();
  this.closing=(async()=>{
   await this.preparing?.catch(()=>{});
   if(!this.browserClosure)throw Error('upload_store_browser_close_unconfirmed');
   if(this.verifier){await this.verifier.close();this.verifier=null;}
   for(const [id,record] of this.records){await record.snapshot.close();this.records.delete(id);}
   this.ledger.close();this.closed=true;
  })().finally(()=>{this.closing=null;});
  return this.closing;
 }
 retained(){return this.ledger.listRetained();}
}
