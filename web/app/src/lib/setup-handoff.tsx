import { createContext, useContext, useRef, useState, useEffect, type ReactNode, type MouseEvent } from 'react';
import { useWorkspace } from './workspace-context';
import { useI18n } from '../i18n';
import { workspaceExperience } from '../i18n/workspace-experience';
import { ScopedNotice } from '../components/WorkspacePresentation';

type DraftReference = { kind: 'chat' | 'task'; id: string };
type Handoff = { origin: string; destination: 'models' | 'knowledge'; draft?: DraftReference };
const Context = createContext<{ begin: (handoff: Handoff) => void; returning: DraftReference | undefined; consumed: () => void; requestedKnowledge: boolean; askKnowledge: () => void; consumeKnowledge: () => void }>({ begin: () => {}, returning: undefined, consumed: () => {}, requestedKnowledge: false, askKnowledge: () => {}, consumeKnowledge: () => {} });
export const useSetupHandoff = () => useContext(Context);

// Mounted with the actor/workspace/permissions key by App. No prompts or tokens
// in URLs/storage. Existing draft persistence, not this reference, owns the text.
export function SetupHandoffProvider({ children, refresh }: { children: ReactNode; refresh: () => Promise<void> }) {
  const { workspace, knowledge, admin } = useWorkspace();
  const { locale, messages } = useI18n();
  const [handoff, setHandoff] = useState<Handoff>();
  const [returning, setReturning] = useState<DraftReference>();
  const [requestedKnowledge, setRequestedKnowledge] = useState(false);
  const [route, setRoute] = useState(window.location.hash);
  const mounted = useRef(true);
  useEffect(() => {
    mounted.current = true;
    const change = () => setRoute(window.location.hash);
    window.addEventListener('hashchange', change);
    return () => { mounted.current = false; window.removeEventListener('hashchange', change); };
  }, []);
  const begin = (next: Handoff) => {
    if (!workspace || (next.destination === 'knowledge' ? !knowledge : !admin)) return;
    if (!/^#\/(chat|agents(?:\/(new|workspace|task\/run-[a-f0-9]{32}))?)$/.test(next.origin)) return;
    setHandoff(next); setReturning(undefined);
  };
  const returnDraft = () => {
    if (!handoff) return;
    const origin = handoff.origin;
    setReturning(handoff.draft); setHandoff(undefined);
    // Returning never grants a capability or submits. A fresh mounted composer
    // checks its own readiness; inventory refresh is independently read-only.
    window.location.hash = origin;
    if (mounted.current) void refresh();
  };
  const inSetup = /^#\/(models|knowledge)(?:\/|$)/.test(route);
  const askKnowledge = () => {
    if (!knowledge) return;
    setReturning(undefined); setHandoff(undefined); setRequestedKnowledge(true);
    window.location.hash = '#/chat';
  };
  return <Context.Provider value={{ begin, returning, consumed: () => { setReturning(undefined); setHandoff(undefined); }, requestedKnowledge, askKnowledge, consumeKnowledge: () => setRequestedKnowledge(false) }}>
    {handoff && inSetup && <ScopedNotice className="setup-return" action={<button className="secondary-button" onClick={returnDraft}>{workspaceExperience[locale].returnDraft}</button>}>{messages.nav[handoff.destination]}</ScopedNotice>}
    {children}
  </Context.Provider>;
}

export function SetupLink({ destination, draft, children }: { destination: 'models' | 'knowledge'; draft?: DraftReference; children: ReactNode }) {
  const { begin } = useSetupHandoff();
  const follow = (event: MouseEvent<HTMLAnchorElement>) => {
    if (event.button || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
    begin({ origin: window.location.hash, destination, draft });
  };
  return <a className="secondary-button" href={`#/${destination}`} onClick={follow}>{children}</a>;
}
