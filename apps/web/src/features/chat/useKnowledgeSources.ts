import { useEffect, useMemo, type Dispatch, type SetStateAction } from 'react';
import {useDraft, draftKey, writeDraft} from '../../lib/drafts';
import {useDraftScope} from '../../lib/draft-scope';
import { useSetupHandoff, type KnowledgeSource } from '../../lib/setup-handoff';

// Source scope belongs to a conversation draft, not the global model selector.
// Use the same actor/service-bound private storage as the unsent question.
export function useKnowledgeSources(name: string, loading: boolean, allowed: boolean, setKnowledge: Dispatch<SetStateAction<boolean>>) {
  const handoff = useSetupHandoff();
  const scope=useDraftScope(), draft=useDraft(scope,'chat-knowledge-sources',name);
  const sources=useMemo<KnowledgeSource[]>(()=>{
    try {const value:unknown=JSON.parse(draft.value);return Array.isArray(value)?value.filter(item=>item&&typeof item.id==='string'&&typeof item.name==='string').slice(0,16):[];}catch{return [];}
  },[draft.value]);
  useEffect(() => {
    // Initial history loading can select a saved conversation. Consuming the
    // handoff earlier would attach it to the empty draft and lose the selection.
    if (!handoff.requestedKnowledge || !allowed || loading) return;
    setKnowledge(handoff.requestedSources.length === 0);
    draft.setValue(JSON.stringify(handoff.requestedSources));
    handoff.consumeKnowledge();
  }, [handoff.requestedKnowledge, allowed, loading, name]);
  const transfer = (createdName: string) => {
    if (sources.length && createdName !== name) {
      writeDraft(draftKey(scope,'chat-knowledge-sources',createdName),JSON.stringify(sources));
      draft.setValue('');
    }
  };
  const clear = () => { draft.setValue(''); setKnowledge(false); };
  return { sources, transfer, clear, unsaved:draft.unsaved };
}
