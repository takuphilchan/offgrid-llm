import {useEffect,useState} from 'react';
import {stopOwnedComputer} from './TaskControls';

export type BrowserHandoff={state:'checking'|'none'|'taking_over'|'manual'|'resuming'|'unavailable';origin?:string};

export function useBrowserHandoff(session?:string){
  const supported=window.electron?.browserManualProtocol===1&&!!window.electron?.getComputerStatus;
  const [value,setValue]=useState<BrowserHandoff>({state:session&&supported?'checking':'none'});
  useEffect(()=>{
    let live=true,timer:ReturnType<typeof setTimeout>;
    if(!session||!supported){setValue({state:'none'});return;}
    setValue({state:'checking'});
    const poll=async()=>{
      try{
        const host=await window.electron!.getComputerStatus!();
        if(live)setValue(host.target?.id===session&&host.browserManualProtocol===1&&['taking_over','manual','resuming'].includes(host.state)
          ?{state:host.state as BrowserHandoff['state'],origin:host.target.origin}:{state:'none'});
      }catch{if(live)setValue({state:'unavailable'});}
      if(live)timer=setTimeout(()=>void poll(),1000);
    };
    void poll();return()=>{live=false;clearTimeout(timer);};
  },[session,supported]);
  return value;
}

// Quiesce local browser input before asking the service to revoke its old grant.
// Callers must still attempt service revocation when this throws.
export async function takeoverOwnedComputer(session?:string,requestId?:string){
  if(session&&window.electron?.browserManualProtocol===1&&window.electron.takeoverComputerAccess){
    const state=await window.electron.takeoverComputerAccess({session});
    if(state.state==='manual'&&state.target?.id===session)return;
    if(state.state==='not_owned')return;
    if(state.state!=='unsupported')throw Error(state.code??'computer_handoff_unconfirmed');
  }
  await stopOwnedComputer(session,requestId);
}
