import {useEffect,useState} from 'react';
import {api} from '../../api/client';

export function useChatCapabilities(scope:string){
 const [capabilities,setCapabilities]=useState<string[]>([]);
 const [settled,setSettled]=useState(false);
 useEffect(()=>{
  let active=true;setCapabilities([]);setSettled(false);
  void api.systemIdentity().then(identity=>{if(active)setCapabilities(identity.capabilities??[]);}).catch(()=>{}).finally(()=>{if(active)setSettled(true);});
  return()=>{active=false;};
 },[scope]);
 return {settled,sourceScopes:capabilities.includes('conversation-source-scopes-v1'),sourceInputs:capabilities.includes('conversation-source-inputs-v1'),conversationAdmission:capabilities.includes('conversation-input-admission-v1'),executionRequestable:capabilities.includes('execution-access-request-v1'),scopedReady:capabilities.includes('scoped-knowledge-chat-v1'),imageServiceReady:capabilities.includes('session-images-v1'),imageOrientationReady:capabilities.includes('chat-image-orientation-v1'),fileServiceReady:capabilities.includes('session-files-v1')};
}
