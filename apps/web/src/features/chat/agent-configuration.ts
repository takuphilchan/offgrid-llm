import { useRef } from 'react';
import type { AgentProfile, AgentResourceRestrictions, AgentRun } from '../../api/client';
import { useWorkspaceState } from '../../lib/workspace-context';

export function useConversationConfiguration() {
  const [agent,setAgent]=useWorkspaceState<AgentProfile|null>('chat.agent',null);
  const [resources,setResources]=useWorkspaceState<{profile:string;restrictions:AgentResourceRestrictions}>('chat.agentResources',{profile:'',restrictions:{}});
  const revision=useRef(0);
  const key=agent?`${agent.id}:${agent.revision}`:'';
  const restrictions=resources.profile===key?resources.restrictions:{};
  const invalidate=()=>++revision.current;
  const select=(profile:AgentProfile|null)=>{
    invalidate();setAgent(profile);
    setResources({profile:profile?`${profile.id}:${profile.revision}`:'',restrictions:narrowProfileRestrictions(restrictions,profile)});
  };
  const update=(value:AgentResourceRestrictions)=>{invalidate();setResources({profile:key,restrictions:value});};
  const reset=()=>{const next=invalidate();setAgent(null);setResources({profile:'',restrictions:{}});return next;};
  const restore=(saved:AgentRun['conversation_configuration'],expectedRevision:number)=>{
    if(saved?.version!==1 || revision.current!==expectedRevision)return null;
    setAgent(saved.profile);setResources({profile:saved.profile?`${saved.profile.id}:${saved.profile.revision}`:'',restrictions:saved.restrictions});
    return saved.model;
  };
  return {agent,key,restrictions,select,update,reset,restore,invalidate};
}

// Selecting a specialization never implicitly clears conversation restrictions.
// The server independently applies the same narrowing at admission.
export function narrowProfileRestrictions(current: AgentResourceRestrictions, profile: AgentProfile | null): AgentResourceRestrictions {
  if (!profile) return current;
  const next = { ...current };
  if (current.tools) next.tools = current.tools.filter(ref => profile.settings.tools?.some(allowed => allowed.tool === ref.tool && allowed.provider === ref.provider && allowed.schema_revision === ref.schema_revision && allowed.location === ref.location));
  if (current.knowledge) next.knowledge = current.knowledge.filter(id => profile.settings.knowledge?.includes(id));
  if (current.computer !== undefined) next.computer = current.computer && profile.settings.computer;
  if (current.execution && profile.settings.execution) next.execution = {
    read: current.execution.read && profile.settings.execution.read,
    write: current.execution.write && profile.settings.execution.write,
    processes: current.execution.processes && profile.settings.execution.processes,
  };
  return next;
}
