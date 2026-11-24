import {mkdir,open,lstat,unlink,rmdir} from 'node:fs/promises';
import {join} from 'node:path';
import {createHash} from 'node:crypto';
import {BrowserUploadLedger} from '../../browser-upload-ledger.mjs';
import {uploadIdentityFromStat} from '../../browser-upload-ledger-values.mjs';

const hold=setInterval(()=>{},1000);
process.once('message',async({directory,phase,size=4,maxBytes=4,maxActive=2})=>{
  try{
    const ledger=await BrowserUploadLedger.open({directory,maxRetainedBytes:maxBytes,maxActiveRecords:maxActive});
    const binding={actorId:'actor',workspaceId:'workspace',runId:'run-'+'1'.repeat(32),inputId:'input-owned',sessionId:'2'.repeat(32)};
    const content=Buffer.alloc(size,0x51),grant={id:'3'.repeat(32),name:'selected.bin',size,sha256:createHash('sha256').update(content).digest('hex')};
    const record=ledger.reserve({binding,grant}),id=record.resourceId,path=join(directory,record.relativeDirectory),file=join(path,grant.name);
    const reached=name=>{
      if(phase!==name)return false;
      process.send({ready:true,resourceId:id,phase:name});return true;
    };
    if(reached('reserved'))return;
    await mkdir(path,{recursive:true});
    if(reached('directory_before_identity'))return;
    ledger.recordDirectory(id,uploadIdentityFromStat(await lstat(path,{bigint:true})));
    if(reached('directory'))return;
    const output=await open(file,'wx',0o600);
    if(reached('file_before_identity'))return;
    ledger.recordFile(id,uploadIdentityFromStat(await output.stat({bigint:true})));
    await output.write(content.subarray(0,Math.min(1,size)));
    if(reached('partial_copy'))return;
    await output.write(content.subarray(Math.min(1,size)));await output.sync();await output.close();
    ledger.ready(id,{size,sha256:grant.sha256});
    if(reached('ready'))return;
    ledger.inUse(id,{actionId:binding.runId+':selected'});
    if(reached('in_use'))return;
    // No browser exists in this storage fixture. This exercises persistence of
    // a caller assertion only, never native descendant-death qualification.
    ledger.browserClosed();
    if(reached('browser_closed'))return;
    ledger.beginCleanup(id);
    if(reached('deleting'))return;
    await unlink(file);await rmdir(path);
    if(reached('deleted_before_receipt'))return;
    ledger.removed(id);if(reached('removed'))return;
    throw Error('fixture_phase_invalid');
  }catch(error){process.send({error:error.message});clearInterval(hold);process.exitCode=2;process.disconnect();}
});
