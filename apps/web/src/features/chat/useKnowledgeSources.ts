import { useEffect, type Dispatch, type SetStateAction } from 'react';
import { useWorkspaceState } from '../../lib/workspace-context';
import { useSetupHandoff, type KnowledgeSource } from '../../lib/setup-handoff';

// Source scope belongs to a conversation draft, not the global model selector.
// Account/workspace changes remount the owner and isolate this in-memory state.
export function useKnowledgeSources(name: string, loading: boolean, allowed: boolean, setKnowledge: Dispatch<SetStateAction<boolean>>) {
  const handoff = useSetupHandoff();
  const [scopes, setScopes] = useWorkspaceState<Record<string, KnowledgeSource[]>>('chat.sourceScopes', {});
  const sources = scopes[name] ?? [];
  useEffect(() => {
    // Initial history loading can select a saved conversation. Consuming the
    // handoff earlier would attach it to the empty draft and lose the selection.
    if (!handoff.requestedKnowledge || !allowed || loading) return;
    setKnowledge(handoff.requestedSources.length === 0);
    setScopes(current => ({ ...current, [name]: handoff.requestedSources }));
    handoff.consumeKnowledge();
  }, [handoff.requestedKnowledge, allowed, loading, name]);
  const transfer = (createdName: string) => {
    if (sources.length) setScopes(current => ({ ...current, '': [], [createdName]: sources }));
  };
  const clear = () => { setScopes(current => ({ ...current, [name]: [] })); setKnowledge(false); };
  return { sources, transfer, clear };
}
