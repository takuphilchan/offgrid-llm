import {basename} from 'node:path';
import {browserFileLimit} from './browser-file-snapshot.mjs';

export const uploadResourceID=value=>typeof value==='string'&&/^[a-f0-9]{32}$/.test(value);
const opaque=value=>typeof value==='string'&&/^[a-zA-Z0-9_-]{1,128}$/.test(value);
const exact=(value,fields)=>value&&Object.getPrototypeOf(value)===Object.prototype&&Object.keys(value).sort().join(',')===fields;
const reject=()=>{throw Error('upload_ledger_invalid');};
export function uploadTaskBinding(value){
  if(!exact(value,'actorId,inputId,runId,workspaceId')||!opaque(value.actorId)||!opaque(value.workspaceId)||!/^run-[a-f0-9]{32}$/.test(value.runId??'')||!/^input-[a-zA-Z0-9_-]{1,122}$/.test(value.inputId??''))reject();
  return Object.freeze({...value});
}
export function uploadBinding(value){
  if(!exact(value,'actorId,inputId,runId,sessionId,workspaceId')||!uploadResourceID(value.sessionId))reject();
  const {sessionId,...task}=value;uploadTaskBinding(task);
  return Object.freeze({...value});
}
export function uploadGrant(value){
  if(!exact(value,'id,name,sha256,size')||!uploadResourceID(value.id)||typeof value.name!=='string'||!value.name||value.name.length>255||basename(value.name)!==value.name||['.','..'].includes(value.name)||value.name.includes('\0')||!Number.isSafeInteger(value.size)||value.size<0||value.size>browserFileLimit||!/^[a-f0-9]{64}$/.test(value.sha256??''))reject();
  return Object.freeze({...value});
}
export function uploadNativeIdentity(value){
  if(!exact(value,'birthtimeNs,device,inode,platform')||!['win32','linux','darwin'].includes(value.platform))reject();
  for(const key of ['device','inode','birthtimeNs'])if(typeof value[key]!=='string'||! /^[1-9][0-9]{0,19}$/.test(value[key])||BigInt(value[key])>18446744073709551615n)reject();
  return Object.freeze({platform:value.platform,device:value.device,inode:value.inode,birthtimeNs:value.birthtimeNs});
}
export function uploadIdentityFromStat(stat,platform=process.platform){
  if(!stat||['dev','ino','birthtimeNs'].some(key=>typeof stat[key]!=='bigint'))reject();
  return uploadNativeIdentity({platform,device:String(stat.dev),inode:String(stat.ino),birthtimeNs:String(stat.birthtimeNs)});
}
export function uploadAction(value,run){
  if(!value||Object.getPrototypeOf(value)!==Object.prototype||Object.keys(value).some(key=>!['actionId','documentId'].includes(key))||typeof value.actionId!=='string'||value.actionId.length>256||!value.actionId.startsWith(run+':')||! /^[a-zA-Z0-9:_-]{1,256}$/.test(value.actionId)||value.actionId.length===run.length+1||value.documentId!==undefined&&! /^[a-f0-9]{8}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{4}-[a-f0-9]{12}$/.test(value.documentId))reject();
  return {actionId:value.actionId,documentId:value.documentId??''};
}
