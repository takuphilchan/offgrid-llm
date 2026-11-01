import { createContext, useContext, useState, type ReactNode } from 'react';
import { useWorkActions } from '../lib/active-work';
import { draftKey, readDraft, writeDraft } from '../lib/drafts';
import { useDraftScope } from '../lib/draft-scope';
import { selectChatEvent } from '../lib/active-work';
import { useI18n } from '../i18n';
import { workSearchText } from '../i18n/work-search';

type Origin = { hash: string; chat: string };
const Context = createContext<{ open: (kind: 'chat' | 'task', id: string) => void; back?: () => void }>({ open: () => {} });
export const useWorkSearchNavigation = () => useContext(Context);
export function WorkSearchNavigation({ children }: { children: ReactNode }) {
  const actions = useWorkActions(), scope = useDraftScope();
  const [origin, setOrigin] = useState<Origin>();
  const open = (kind: 'chat' | 'task', id: string) => {
    setOrigin(previous => previous ?? { hash: window.location.hash || '#/chat', chat: readDraft(draftKey(scope, 'active-chat')) });
    if (kind === 'chat') {
      if (!actions.hasChat(id)) actions.rememberChat(id, 'completed');
      actions.openChat(id);
    } else window.location.hash = `#/agents/task/${encodeURIComponent(id)}`;
  };
  const back = origin ? () => {
    const previous = origin; setOrigin(undefined);
    writeDraft(draftKey(scope, 'active-chat'), previous.chat);
    window.location.hash = previous.hash === '#/chat' && !previous.chat ? '#/chat/new' : previous.hash;
    if (previous.hash === '#/chat') window.dispatchEvent(new CustomEvent(selectChatEvent, { detail: previous.chat }));
  } : undefined;
  return <Context.Provider value={{ open, back }}>{children}</Context.Provider>;
}
export function WorkSearchReturn() {
  const { back } = useWorkSearchNavigation(), { locale } = useI18n();
  return back ? <button className="utility-button work-search-return" onClick={back}>{workSearchText[locale].back}</button> : null;
}
