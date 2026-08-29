import { useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { api, type ChatMessage, type ChatSession, type Model } from '../../api/client';
import { Icon } from '../../components/Icon';
import { ModelSelect } from '../../components/ModelSelect';
import { useI18n } from '../../i18n';

const activeSessionKey = 'offgrid.active-session';

function newSessionName(): string {
  const stamp = new Date().toISOString().replace(/\D/g, '').slice(0, 14);
  return `Chat ${stamp}`;
}

export function ChatPage({ models, model, setModel }: { models: Model[]; model: string; setModel: (model: string) => void }) {
  const { messages: text } = useI18n();
  const [sessions, setSessions] = useState<ChatSession[]>([]);
  const [activeName, setActiveName] = useState(localStorage.getItem(activeSessionKey) ?? '');
  const [conversation, setConversation] = useState<ChatMessage[]>([]);
  const [draft, setDraft] = useState('');
  const [knowledge, setKnowledge] = useState(false);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [confirmDelete, setConfirmDelete] = useState('');
  const controller = useRef<AbortController | null>(null);
  const end = useRef<HTMLDivElement | null>(null);

  const activate = (session?: ChatSession) => {
    const name = session?.name ?? '';
    setActiveName(name);
    setConversation(session?.messages ?? []);
    if (session?.model_id && models.some(item => item.id === session.model_id)) setModel(session.model_id);
    if (name) localStorage.setItem(activeSessionKey, name);
    else localStorage.removeItem(activeSessionKey);
  };

  const loadSessions = async (preferred = activeName) => {
    setLoading(true);
    setError('');
    try {
      const next = await api.sessions();
      setSessions(next);
      activate(next.find(item => item.name === preferred) ?? next[0]);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : text.common.error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void loadSessions(); }, []);
  useEffect(() => { end.current?.scrollIntoView({ behavior: 'smooth' }); }, [conversation, busy]);

  const startNew = () => {
    if (busy) return;
    activate(undefined);
    setDraft('');
    setError('');
  };

  const selectSession = (session: ChatSession) => {
    if (!busy) {
      setConfirmDelete('');
      activate(session);
    }
  };

  const removeSession = async (session: ChatSession) => {
    if (busy) return;
    if (confirmDelete !== session.name) {
      setConfirmDelete(session.name);
      return;
    }
    setError('');
    try {
      await api.deleteSession(session.name);
      setConfirmDelete('');
      const next = sessions.filter(item => item.name !== session.name);
      setSessions(next);
      if (activeName === session.name) activate(next[0]);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : text.common.error);
    }
  };

  const send = async () => {
    const prompt = draft.trim();
    if (!prompt || !model || busy) return;
    setBusy(true);
    setDraft('');
    setError('');
    let sessionName = activeName;
    try {
      if (!sessionName) {
        const created = await api.createSession(newSessionName(), model);
        sessionName = created.name;
        setActiveName(sessionName);
        localStorage.setItem(activeSessionKey, sessionName);
        setSessions(current => [created, ...current]);
      }
      setConversation(current => [...current, { role: 'user', content: prompt }]);
      controller.current = new AbortController();
      const result = await api.generateSession(sessionName, prompt, model, knowledge, controller.current.signal);
      setConversation(result.session.messages);
      setSessions(current => [result.session, ...current.filter(item => item.name !== result.session.name)]);
    } catch (reason) {
      if ((reason as Error).name !== 'AbortError') setError(reason instanceof Error ? reason.message : text.common.error);
      if (sessionName) {
        try {
          const stored = await api.session(sessionName);
          setConversation(stored.messages);
          setSessions(current => [stored, ...current.filter(item => item.name !== stored.name)]);
        } catch { /* The primary error is more useful. */ }
      }
    } finally {
      setBusy(false);
      controller.current = null;
    }
  };

  const keyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      void send();
    }
  };

  return <div className="chat-workspace">
    <aside className="conversation-history" aria-label={text.chat.conversations}>
      <button className="new-chat-button" onClick={startNew}><Icon name="plus" size={16} />{text.chat.newChat}</button>
      <div className="history-label">{text.chat.conversations}</div>
      {loading ? <div className="history-empty">{text.common.loading}</div> : sessions.length === 0 ? <div className="history-empty">{text.chat.noConversations}</div> : <div className="history-list">
        {sessions.map(session => <div className={activeName === session.name ? 'history-row active' : 'history-row'} key={session.name}>
          <button className="history-open" onClick={() => selectSession(session)}><strong>{session.name}</strong><small>{session.messages.length} · {new Date(session.updated_at).toLocaleDateString()}</small></button>
          <button className={confirmDelete === session.name ? 'history-delete armed' : 'history-delete'} aria-label={confirmDelete === session.name ? text.models.confirmDelete : text.chat.deleteConversation} title={confirmDelete === session.name ? text.models.confirmDelete : text.chat.deleteConversation} onClick={() => void removeSession(session)}><Icon name="trash" size={14} /></button>
        </div>)}
      </div>}
    </aside>
    <div className="chat-layout">
      <div className="chat-toolbar"><ModelSelect models={models} value={model} onChange={setModel} /><label className="switch"><input type="checkbox" checked={knowledge} onChange={event => setKnowledge(event.target.checked)} /><span />{text.chat.knowledge}</label></div>
      <div className="conversation" aria-live="polite">
        {conversation.length === 0 && !loading && <div className="empty-chat"><div className="orb"><div /></div><h2>{text.chat.emptyTitle}</h2><p>{text.chat.emptyBody}</p></div>}
        {conversation.map((message, index) => <article className={`message ${message.role}`} key={`${index}-${'timestamp' in message ? message.timestamp : message.content.slice(0, 12)}`}><div className="message-label">{message.role === 'user' ? text.chat.you : text.chat.assistant}</div><div className="message-body">{message.content}</div></article>)}
        {busy && <article className="message assistant"><div className="message-label">{text.chat.assistant}</div><div className="thinking"><i /><i /><i /></div></article>}
        {error && <div className="inline-error" role="alert">{error}</div>}<div ref={end} />
      </div>
      <div className="composer"><textarea value={draft} onChange={event => setDraft(event.target.value)} onKeyDown={keyDown} placeholder={text.chat.placeholder} rows={1} /><button disabled={busy ? false : !draft.trim() || !model} onClick={busy ? () => controller.current?.abort() : () => void send()}>{busy ? text.chat.stop : <><span>{text.chat.send}</span><Icon name="send" size={18} /></>}</button></div>
    </div>
  </div>;
}
