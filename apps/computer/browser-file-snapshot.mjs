import {constants} from 'node:fs';
import {open,lstat,mkdir,mkdtemp,chmod,unlink,rmdir} from 'node:fs/promises';
import {createHash,randomUUID} from 'node:crypto';
import {basename,dirname,isAbsolute,join,resolve} from 'node:path';
import {tmpdir} from 'node:os';

export const browserFileLimit=512*1024*1024;
const sameFile=(first,last)=>first.dev===last.dev&&first.ino===last.ino&&first.birthtimeNs===last.birthtimeNs;
export const validateBrowserUploadGrant=grant=>{
 if(!grant||!isAbsolute(grant.path??'')||typeof grant.name!=='string'||!grant.name||basename(grant.name)!==grant.name||basename(grant.path)!==grant.name||['.','..'].includes(grant.name)||grant.name.includes('\0')||grant.name.length>255||
  !Number.isSafeInteger(grant.size)||grant.size<0||grant.size>browserFileLimit||!/^[a-f0-9]{32}$/.test(grant.id??'')||!/^[a-f0-9]{64}$/.test(grant.sha256??''))throw Error('Selected upload grant is invalid.');
 if(process.platform==='win32'&&(/[<>:"|?*\x00-\x1f]|[. ]$/.test(grant.name)||/^(con|prn|aux|nul|clock\$|conin\$|conout\$|com[1-9¹²³]|lpt[1-9¹²³])(?:\.|$)/i.test(grant.name)))throw Error('Selected upload name is unsupported.');
};

// The caller owns this object before prepare starts, including partial files and
// failed cleanup. Its root must be private host storage, never a selected agent
// workspace. Read-only mode is defence in depth, not protection from another
// process already holding the user's full OS authority.
export class BrowserFileSnapshot {
 constructor(root,{allocation}={}){
  if(!isAbsolute(root??''))throw Error('Upload staging root must be absolute.');
  if(allocation&&(!/^[a-f0-9]{32}$/.test(allocation.resourceId??'')||['directory','file','ready','beforeCleanup','afterCleanup'].some(key=>typeof allocation[key]!=='function')))throw Error('Upload allocation is invalid.');
  this.root=resolve(root);this.allocation=allocation;this.abort=new AbortController();this.state='new';
 }
 prepare(grant,{signal}={}){
  return this.start(grant,signal,true);
 }
 verifySource(grant,{signal}={}){
  return this.start(grant,signal,false);
 }
 start(grant,signal,copy){
  if(this.state!=='new')return Promise.reject(Error('Upload preparation has already started.'));
  this.state='preparing';this.preparation=this.capture(Object.freeze({...grant}),signal,copy);
  return this.preparation;
 }
 async capture(grant,signal,copy){
  const cancelled=()=>{this.abort.signal.throwIfAborted();signal?.throwIfAborted();};
  let source,output;
  try{
   validateBrowserUploadGrant(grant);cancelled();
   const observed=await lstat(grant.path,{bigint:true});
   if(!observed.isFile()||observed.isSymbolicLink()||observed.size!==BigInt(grant.size))throw Error('Selected upload changed; select it again.');
   source=this.source=await open(grant.path,constants.O_RDONLY|(constants.O_NOFOLLOW??0)|(constants.O_NONBLOCK??0));
   const initial=await source.stat({bigint:true});
   if(!initial.isFile()||!sameFile(initial,observed)||initial.size!==BigInt(grant.size))throw Error('Selected upload changed; select it again.');
   cancelled();
   if(copy){
    await mkdir(this.root,{recursive:true,mode:0o700});
    const root=await lstat(this.root);if(!root.isDirectory()||root.isSymbolicLink())throw Error('Upload staging root is unavailable.');
    if(this.allocation){const directory=join(this.root,this.allocation.resourceId);await mkdir(directory,{mode:0o700});this.directory=directory;}
    else this.directory=await mkdtemp(join(this.root,'upload-'));
    if(dirname(resolve(this.directory))!==this.root)throw Error('Upload staging escaped its owner.');
    this.directoryIdentity=await lstat(this.directory,{bigint:true});
    await this.allocation?.directory(this.directoryIdentity);
    this.path=join(this.directory,grant.name);
    output=this.output=await open(this.path,'wx',0o600);this.fileIdentity=await output.stat({bigint:true});
    await this.allocation?.file(this.fileIdentity);
   }
   const hash=createHash('sha256'),buffer=Buffer.allocUnsafe(64*1024);let size=0;
   try{
    for(;;){
     cancelled();const {bytesRead}=await source.read(buffer,0,Math.min(buffer.length,grant.size-size+1),null);
     cancelled();if(!bytesRead)break;size+=bytesRead;
     if(size>grant.size)throw Error('Selected upload grew; select it again.');
     hash.update(buffer.subarray(0,bytesRead));
     let written=0;
     while(output&&written<bytesRead){
      cancelled();const result=await output.write(buffer,written,bytesRead-written,null);
      if(!result.bytesWritten)throw Error('Upload snapshot made no write progress.');written+=result.bytesWritten;
     }
    }
   }finally{buffer.fill(0);}
   const final=await source.stat({bigint:true});
   if(size!==grant.size||!sameFile(initial,final)||final.size!==initial.size||final.mtimeNs!==initial.mtimeNs||hash.digest('hex')!==grant.sha256)throw Error('Selected upload changed; select it again.');
   cancelled();await output?.sync();await output?.chmod(0o400);cancelled();
   this.metadata=Object.freeze({id:grant.id,name:grant.name,size,sha256:grant.sha256});
   await this.closeFiles();cancelled();
   if(copy)await this.allocation?.ready({size,sha256:grant.sha256});
   this.state=copy?'ready':'verified';return this;
  }catch(error){this.state='failed';throw error;}
  finally{
   try{await this.closeFiles();}
   catch(error){this.state='failed';throw error;}
  }
 }
 async closeFiles(){
  const results=await Promise.allSettled(['source','output'].map(async key=>{
   const file=this[key];if(!file)return;
   await file.close();if(this[key]===file)this[key]=null;
  }));
  const errors=results.filter(result=>result.status==='rejected').map(result=>result.reason);
  if(errors.length)throw new AggregateError(errors,'Upload file closure could not be confirmed.');
 }
 close(){
  if(this.state==='closed')return Promise.resolve();
  if(this.closing)return this.closing;
  this.abort.abort();
  const closing=this.cleanup();this.closing=closing;
  return closing.finally(()=>{if(this.closing===closing)this.closing=null;});
 }
 async cleanup(){
  await this.preparation?.catch(()=>{});
  try{
   await this.closeFiles();
   await this.allocation?.beforeCleanup();
   if(this.directory){
    const current=await lstat(this.directory,{bigint:true});
    if(current.isSymbolicLink()||!current.isDirectory()||!sameFile(current,this.directoryIdentity))throw Error('Upload cleanup ownership changed.');
    if(this.path&&this.fileIdentity){
     const file=await lstat(this.path,{bigint:true});
     if(!file.isFile()||file.isSymbolicLink()||!sameFile(file,this.fileIdentity))throw Error('Upload snapshot ownership changed.');
     await chmod(this.path,0o600);await unlink(this.path);this.path=null;this.fileIdentity=null;
    }
    await rmdir(this.directory);this.directory=null;this.directoryIdentity=null;
   }
   await this.allocation?.afterCleanup();
   this.state='closed';
  }catch(error){this.state='cleanup_failed';throw error;}
 }
}

// A context has one locally selected upload grant. Its selected File may be read
// by Chromium well after setInputFiles returns, so retain the snapshot through
// tab changes/manual control until actual context closure, not merely dispatch.
export class BrowserUploadStore {
 constructor(){this.root=join(tmpdir(),'offgrid-browser-upload-'+randomUUID());this.stopped=false;}
 async get(grant){
  if(this.stopped)throw Error('Upload preparation stopped.');validateBrowserUploadGrant(grant);
  if(this.snapshot){
   if(this.snapshot.state!=='ready'||this.snapshot.metadata.id!==grant.id||this.snapshot.metadata.sha256!==grant.sha256||this.snapshot.metadata.size!==grant.size||this.snapshot.metadata.name!==grant.name)throw Error('Upload snapshot requires confirmed cleanup before a new grant.');
   if(this.verifier)throw Error('Upload source verification still owns a resource.');
   const verifier=new BrowserFileSnapshot(this.root);this.verifier=verifier;
   try{await verifier.verifySource(grant);if(this.stopped)throw Error('Upload preparation stopped.');}
   finally{await verifier.close();if(this.verifier===verifier)this.verifier=null;}
   return this.snapshot;
  }
  const snapshot=new BrowserFileSnapshot(this.root);this.snapshot=snapshot;
  try{await snapshot.prepare(grant);if(this.stopped)throw Error('Upload preparation stopped.');return snapshot;}
  catch(error){
   try{await snapshot.close();this.snapshot=null;}catch(cleanup){throw new AggregateError([error,cleanup],'Upload preparation failed and cleanup is unconfirmed.');}
   throw error;
  }
 }
 stopPreparing(){this.stopped=true;this.snapshot?.abort.abort();this.verifier?.abort.abort();}
 async close(){
  this.stopPreparing();
  if(this.verifier){await this.verifier.close();this.verifier=null;}
  if(this.snapshot){await this.snapshot.close();this.snapshot=null;}
  try{await rmdir(this.root);}catch(error){if(error.code!=='ENOENT')throw error;}
 }
}
