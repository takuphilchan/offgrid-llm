import {BrowserOwnedUploadStore} from '../../browser-owned-upload-store.mjs';
import {BrowserFileSnapshot} from '../../browser-file-snapshot.mjs';

const options=JSON.parse(process.env.OFFGRID_UPLOAD_FIXTURE);
if(process.env.OFFGRID_UPLOAD_CHILD!=='1')throw Error('isolated fixture required');
const {directory,binding,grant,phase}=options;
const store=await BrowserOwnedUploadStore.open({directory,binding,maxRetainedBytes:grant.size});
const hold=()=>new Promise(resolve=>{process.send({phase});process.once('message',resolve);});
// Stop only this owned child at a real committed boundary. Resource mutations
// remain production SQLite/file operations; no synthetic ledger rows are used.
const capture=BrowserFileSnapshot.prototype.capture;
BrowserFileSnapshot.prototype.capture=function(...args){
 if(['directory','file','ready'].includes(phase)){
  const original=this.allocation[phase];
  this.allocation[phase]=async(...values)=>{await original(...values);await hold();};
 }
 return capture.apply(this,args);
};
const action={actionId:binding.runId+':upload',binding};
const snapshot=await store.get(grant,action);
if(phase==='in_use'){await store.inUse(snapshot,action);await hold();}
if(phase==='browser_closed'){await store.inUse(snapshot,action);await store.browserClosed();await hold();}
if(phase==='removed'){
 await store.browserClosed();
 const original=snapshot.allocation.afterCleanup;
 snapshot.allocation.afterCleanup=async(...args)=>{await original(...args);await hold();};
 await store.close();
}
