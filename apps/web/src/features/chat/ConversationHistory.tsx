import type { ChatSession } from '../../api/client';
import { HistoryToolbar } from '../../components/HistoryToolbar';
import { HistoryItemActions } from '../../components/HistoryItemActions';
import type { HistoryItem } from '../../components/HistoryDeleteDialog';
import { EmptyState } from '../../components/WorkspacePresentation';
import { useI18n } from '../../i18n';
import { useWorkspaceState } from '../../lib/workspace-context';
import { visibleSessionTitle } from './chat-presentation';

/** History browsing is independent of the selected conversation's execution. */
export function ConversationHistory({ sessions, activeName, busy, loading, locked, notice, select, remove, refresh }: {
  sessions: ChatSession[]; activeName: string; busy: boolean; loading: boolean;
  locked: boolean; notice: string; select: (session: ChatSession) => void;
  remove: (items: HistoryItem[]) => void; refresh: () => void;
}) {
  const { messages: text, locale , language} = useI18n();
  const [query, setQuery] = useWorkspaceState('chat.historyQuery', '');
  const [limit, setLimit] = useWorkspaceState('chat.historyLimit', 20);
  const filtered = sessions.filter(session => visibleSessionTitle(session).toLocaleLowerCase(language).includes(query.trim().toLocaleLowerCase(language)));
  const item = (session: ChatSession) => ({ id: session.name, label: visibleSessionTitle(session) });
  return <section id="conversation-history" className="conversation-history" aria-label={text.chat.conversations}>
    <HistoryToolbar query={query} onQuery={value => { setQuery(value); setLimit(20); }} searchLabel={text.history.searchChats} title={text.chat.conversations} count={sessions.length} busy={busy || loading || locked} refresh={refresh} clearLabel={text.history.clearChats} clearDisabled={!filtered.length} clear={() => remove(filtered.map(item))} />
    {notice && <p className="history-notice" role="status">{notice}</p>}
    {loading ? <div className="history-empty" role="status">{text.common.loading}</div> : sessions.length === 0 ? <EmptyState className="history-empty" title={text.chat.noConversations} /> : <div className="history-list">
      {filtered.length === 0 && <div className="history-empty">{text.history.noMatches}</div>}
      {filtered.slice(0, limit).map(session => <div className={activeName === session.name ? 'history-row active' : 'history-row'} key={session.name}>
        <button className="history-open" disabled={locked} aria-current={activeName === session.name ? 'true' : undefined} onClick={() => select(session)} title={`${visibleSessionTitle(session)} · ${new Date(session.updated_at).toLocaleDateString(language)}`}><strong>{visibleSessionTitle(session)}</strong></button>
        <HistoryItemActions title={visibleSessionTitle(session)} label={text.chat.deleteConversation} disabled={busy || locked} onDelete={() => remove([item(session)])} />
      </div>)}
      {filtered.length > limit && <button className="secondary-button history-more" onClick={() => setLimit(count => count + 20)}>{text.history.showMore} ({filtered.length - limit})</button>}
    </div>}
  </section>;
}
