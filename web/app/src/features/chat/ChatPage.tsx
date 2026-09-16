import { useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { api, type ChatMessage, type ChatSession, type Model } from '../../api/client';
import { Icon } from '../../components/Icon';
import { MarkdownMessage } from '../../components/MarkdownMessage';
import { ModelSelect } from '../../components/ModelSelect';
import { useI18n } from '../../i18n';
import { copyText } from '../../lib/clipboard';

const activeSessionKey = 'offgrid.active-session';

function newSessionName(prompt: string): string {
  const stamp = new Date().toISOString().replace(/\D/g, '').slice(0, 14);
  const title = prompt.replace(/\s+/g, ' ').slice(0, 42).trim();
  return `${title || 'Chat'} · ${stamp}`;
}

function visibleSessionName(name: string): string {
  return name.replace(/ · \d{14}$/, '');
}

export function ChatPage({ models, model, setModel, onboardingPending, onFirstResponse, onOpenModels }: {
  models: Model[];
  model: string;
  setModel: (model: string) => void;
  onboardingPending: boolean;
  onFirstResponse: () => void;
  onOpenModels: () => void;
}) {
  const { messages: text, locale } = useI18n();
  const [sessions, setSessions] = useState<ChatSession[]>([]);
  const [activeName, setActiveName] = useState(localStorage.getItem(activeSessionKey) ?? '');
  const [conversation, setConversation] = useState<ChatMessage[]>([]);
  const [draft, setDraft] = useState('');
  const [knowledge, setKnowledge] = useState(false);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState('');
  const [confirmDelete, setConfirmDelete] = useState('');
  const [historyOpen, setHistoryOpen] = useState(false);
  const [copiedMessage, setCopiedMessage] = useState('');
  const controller = useRef<AbortController | null>(null);
  const end = useRef<HTMLDivElement | null>(null);
  const composerInput = useRef<HTMLTextAreaElement | null>(null);

  const activate = (session?: ChatSession) => {
    const name = session?.name ?? '';
    setActiveName(name);
    setConversation(session?.messages ?? []);
    if (session?.model_id && models.some(item => item.id === session.model_id)) setModel(session.model_id);
    if (name) localStorage.setItem(activeSessionKey, name);
    else localStorage.removeItem(activeSessionKey);
    setHistoryOpen(false);
  };

  const loadSessions = async (preferred = activeName) => {
    setLoading(true);
    setError('');
    try {
      const next = await api.sessions();
      setSessions(next);
      activate(next.find(item => item.name === preferred) ?? next[0]);
      if (onboardingPending && next.some(item => item.messages.some(message => message.role === 'assistant' && message.content.trim()))) onFirstResponse();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : text.common.error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void loadSessions(); }, []);
  useEffect(() => { end.current?.scrollIntoView({ behavior: 'smooth' }); }, [conversation, busy]);
  useEffect(() => {
    if (!composerInput.current) return;
    composerInput.current.style.height = '0';
    composerInput.current.style.height = `${Math.min(composerInput.current.scrollHeight, 160)}px`;
  }, [draft]);

  const startNew = () => {
    if (busy) return;
    activate(undefined);
    setDraft('');
    setError('');
    setCopiedMessage('');
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
        const created = await api.createSession(newSessionName(prompt), model);
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
      if (onboardingPending && result.message.role === 'assistant' && result.message.content.trim()) onFirstResponse();
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

  const copyMessage = async (key: string, content: string) => {
    try {
      await copyText(content);
      setCopiedMessage(key);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : text.common.error);
    }
  };

  return <div className="chat-workspace">
    {historyOpen && <button className="history-scrim" onClick={() => setHistoryOpen(false)} aria-label={text.chat.closeHistory} />}
    <aside id="conversation-history" className={historyOpen ? 'conversation-history open' : 'conversation-history'} aria-label={text.chat.conversations}>
      <button className="history-close icon-button" onClick={() => setHistoryOpen(false)} aria-label={text.chat.closeHistory}><Icon name="close" size={18} /></button>
      <button className="new-chat-button" onClick={startNew}><Icon name="plus" size={16} />{text.chat.newChat}</button>
      <div className="history-label">{text.chat.conversations}</div>
      {loading ? <div className="history-empty">{text.common.loading}</div> : sessions.length === 0 ? <div className="history-empty">{text.chat.noConversations}</div> : <div className="history-list">
        {sessions.map(session => <div className={activeName === session.name ? 'history-row active' : 'history-row'} key={session.name}>
          <button className="history-open" onClick={() => selectSession(session)} title={visibleSessionName(session.name)}><strong>{visibleSessionName(session.name)}</strong><small>{session.messages.length} · {new Date(session.updated_at).toLocaleDateString(locale)}</small></button>
          <button className={confirmDelete === session.name ? 'history-delete armed' : 'history-delete'} aria-label={confirmDelete === session.name ? text.models.confirmDelete : text.chat.deleteConversation} title={confirmDelete === session.name ? text.models.confirmDelete : text.chat.deleteConversation} onClick={() => void removeSession(session)}><Icon name="trash" size={14} /></button>
        </div>)}
      </div>}
    </aside>
    <div className="chat-layout">
      <div className="chat-toolbar"><div className="chat-toolbar-leading"><button className="history-toggle icon-button" onClick={() => setHistoryOpen(true)} aria-label={text.chat.showHistory} aria-expanded={historyOpen} aria-controls="conversation-history"><Icon name="menu" size={19} /></button><ModelSelect models={models} value={model} onChange={setModel} /></div><label className="switch"><input type="checkbox" checked={knowledge} onChange={event => setKnowledge(event.target.checked)} /><span />{text.chat.knowledge}</label></div>
      {(!model || onboardingPending) && <div className="setup-inline" role="status"><span>{model ? text.onboarding.firstReplyHint : text.onboarding.modelHint}</span>{!model && <button className="secondary-button" onClick={onOpenModels}>{text.onboarding.chooseModel}</button>}</div>}
      <div className="conversation" aria-live="polite">
        {conversation.length === 0 && !loading && <div className="empty-chat"><div className="orb"><div /></div><h2>{text.chat.emptyTitle}</h2><p>{text.chat.emptyBody}</p></div>}
        {conversation.map((message, index) => {
          const messageKey = `${index}-${'timestamp' in message ? message.timestamp : message.content.slice(0, 24)}`;
          return <article className={`message ${message.role}`} key={messageKey}>
            <div className="message-heading"><div className="message-label">{message.role === 'user' ? text.chat.you : message.role === 'assistant' ? text.chat.assistant : message.role}</div>{message.role === 'assistant' && <button type="button" className="message-copy" onClick={() => void copyMessage(messageKey, message.content)} aria-label={copiedMessage === messageKey ? text.chat.copied : text.chat.copyResponse}><Icon name={copiedMessage === messageKey ? 'check' : 'copy'} size={14} />{copiedMessage === messageKey ? text.chat.copied : text.chat.copy}</button>}</div>
            <div className="message-body">{message.role === 'assistant' ? <MarkdownMessage content={message.content} /> : message.content}</div>
          </article>;
        })}
        {busy && <article className="message assistant"><div className="message-label">{text.chat.assistant}</div><div className="thinking"><i /><i /><i /></div></article>}
        {error && <div className="inline-error" role="alert">{error}</div>}<div ref={end} />
      </div>
      <div className="composer"><div className="composer-input"><textarea ref={composerInput} value={draft} onChange={event => setDraft(event.target.value)} onKeyDown={keyDown} placeholder={text.chat.placeholder} aria-label={text.chat.placeholder} rows={1} /><small>{text.chat.enterHint}</small></div><button disabled={busy ? false : !draft.trim() || !model} onClick={busy ? () => controller.current?.abort() : () => void send()}>{busy ? text.chat.stop : <><span>{text.chat.send}</span><Icon name="send" size={18} /></>}</button></div>
    </div>
  </div>;
}
