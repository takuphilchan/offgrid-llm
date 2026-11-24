import {open} from 'node:fs/promises';
import {join} from 'node:path';
import {DispatchJournal} from '../../journal.mjs';

// Only the parent fixture's newly created directory is used. There is no live
// service, account, browser or automatic retry in this process-crash oracle.
process.once('message',async({directory,phase})=>{
 try{
  if(!['intent','effect','complete'].includes(phase))throw Error('Invalid crash phase');
  const action={id:'run:crash',kind:'browser_click',arguments:{element:'fixture',observation_id:'fixture'}};
  const journal=new DispatchJournal(join(directory,'dispatch.sqlite'));
  if(!journal.prepare(action,'fixture-workspace:session').execute)throw Error('Unexpected prior action');
  if(phase!=='intent'){
   const effect=await open(join(directory,'independent-effect'),'wx',0o600);
   try{await effect.writeFile('one effect');await effect.sync();}finally{await effect.close();}
  }
  if(phase==='complete')journal.complete(action.id,{id:action.id,result:'{"fixture_effect":true}'});
  process.send({phase});
  // Keep the journal/process live until the parent terminates this exact child.
  setInterval(()=>{},1000);
 }catch(error){process.send({error:error.message});process.exitCode=1;process.disconnect();}
});
