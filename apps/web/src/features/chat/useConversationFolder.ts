import {useEffect, useState} from 'react';
import {api, type ExecutionWorkspace} from '../../api/client';

// Conversation-owned service state survives reload and changes of client.
// Local pending selection never leaks into a different conversation.
export function useConversationFolder(name: string, id: string) {
  const [selection, setSelection] = useState<{name: string; value: ExecutionWorkspace | null; changed: boolean}>({name, value: null, changed: false});
  const current = selection.name === name ? selection : {name, value: null, changed: false};
  useEffect(() => {
    if (!id) return;
    let alive = true;
    void api.conversations.configuration(id).then(async configuration => {
      const hosts = configuration.folder?.available ? (await api.executionWorkspaces()).workspaces.filter(item => item.reference.session === configuration.folder!.session) : [];
      if (alive) setSelection(previous => previous.name === name && previous.changed ? previous : {name, value: hosts.length === 1 ? hosts[0] : null, changed: false});
    }).catch(() => { /* Configuration inspection reports errors in its own surface. No authority is inferred. */ });
    return () => {alive = false;};
  }, [name, id]);
  return {
    value: current.value, changed: current.changed,
    change: (value: ExecutionWorkspace | null) => setSelection({name, value, changed: true}),
    command: current.value ?? (current.changed ? null : undefined),
    accepted: () => setSelection(previous => previous.name === name ? {...previous, changed: false} : previous),
  };
}
