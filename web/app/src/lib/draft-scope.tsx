import { createContext, useContext, useEffect, useState, type ReactNode } from 'react';
import { useWorkspace } from './workspace-context';
import { forgetDraftMemory, hasLegacyDrafts, resolveLegacyDrafts, workspaceDraftScope } from './drafts';
import { ScopedNotice } from '../components/WorkspacePresentation';
import { useI18n } from '../i18n';
import { draftRecoveryText } from '../i18n/draft-recovery';

const Context = createContext('');
export function useDraftScope() {
  const scope = useContext(Context);
  if (!scope) throw new Error('Workspace draft provider required');
  return scope;
}

// App remounts this boundary when actor/workspace/permissions change. Missing
// workspace identity gets disposable memory, never an origin/account fallback.
export function WorkspaceDraftProvider({ children }: { children: ReactNode }) {
  const { scope: actor, workspace } = useWorkspace();
  const [scope] = useState(() => workspace ? workspaceDraftScope(actor, workspace) : `@memory:${crypto.randomUUID()}`);
  useEffect(() => () => forgetDraftMemory(scope), [scope]);
  return <Context.Provider value={scope}>{children}</Context.Provider>;
}

export function DraftRecoveryNotice() {
  const { scope: actor, workspace } = useWorkspace();
  const { locale } = useI18n();
  const copy = draftRecoveryText[locale];
  const [offered, setOffered] = useState(() => hasLegacyDrafts(actor, workspace));
  const [message, setMessage] = useState('');
  useEffect(() => {
    const changed = () => setOffered(hasLegacyDrafts(actor, workspace));
    window.addEventListener('storage', changed);
    return () => window.removeEventListener('storage', changed);
  }, [actor, workspace]);
  const resolve = (restore: boolean) => {
    try { resolveLegacyDrafts(actor, workspace, restore); setOffered(false); setMessage(restore ? 'restored' : ''); }
    catch { setMessage('failed'); }
  };
  if (!workspace) return <ScopedNotice>{copy.unidentified}</ScopedNotice>;
  if (!offered && !message) return null;
  return <ScopedNotice className="draft-recovery" kind={message === 'failed' ? 'error' : 'status'} action={offered ? <>
    <button type="button" className="secondary-button" onClick={() => resolve(true)}>{copy.restore}</button>
    <button type="button" className="secondary-button" onClick={() => resolve(false)}>{copy.separate}</button>
  </> : <button type="button" className="secondary-button" onClick={() => setMessage('')}>{copy.dismiss}</button>}>
    {message === 'failed' ? copy.failed : message === 'restored' ? copy.restored : copy.offer}
  </ScopedNotice>;
}
