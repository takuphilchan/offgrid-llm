import {useEffect,useState} from 'react';
import {api} from '../../api/client';

export function useChatCapabilities(scope:string){
 const [capabilities,setCapabilities]=useState<string[]>([]);
 useEffect(()=>{
  let active=true;setCapabilities([]);
  void api.systemIdentity().then(identity=>{if(active)setCapabilities(identity.capabilities??[]);}).catch(()=>{});
  return()=>{active=false;};
 },[scope]);
 return {executionRequestable:capabilities.includes('execution-access-request-v1'),scopedReady:capabilities.includes('scoped-knowledge-chat-v1'),imageServiceReady:capabilities.includes('session-images-v1'),fileServiceReady:capabilities.includes('session-files-v1')};
}
