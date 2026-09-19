import { useWorkspace, useWorkspaceState } from '../../lib/workspace-context';
import { useFocusScope } from '../../lib/focus-scope';
import { interaction } from '../../i18n/interaction';
import { useWorkspaceRefresh } from '../../lib/workspace-refresh';
import { useEffect, useRef, useState, type KeyboardEvent } from 'react';
import { api, type ChatMessage, type ChatSession, type Model, type SessionTurn } from '../../api/client';
import { Icon } from '../../components/Icon';
import { MarkdownMessage } from '../../components/MarkdownMessage';
import { ModelSelect } from '../../components/ModelSelect';
import { HistoryDeleteDialog, type HistoryItem } from '../../components/HistoryDeleteDialog';
import { useI18n } from '../../i18n';
import { copyText } from '../../lib/clipboard';
import { clearSubmittedDraft, draftKey, readDraft, useDraft, writeDraft } from '../../lib/drafts';
import { type ChatMetrics, type ChatPhase } from '../../api/session-stream';
import { readPreference, writePreference } from '../../lib/preferences';

const activeSessionKey = 'offgrid.active-session';

function readPreferences(key: string): { profile: string; maxTokens: number } {
  try {
    const value = JSON.parse(readPreference(key) ?? '{}');
    return { profile: value.profile === 'extended' ? 'extended' : 'interactive', maxTokens: [256, 1024, 4096].includes(value.maxTokens) ? value.maxTokens : 1024 };
  } catch { return { profile: 'interactive', maxTokens: 1024 }; }
}

function newSessionName(prompt: string): string {
  const stamp = new Date().toISOString().replace(/\D/g, '').slice(0, 14);
  const title = prompt.replace(/\s+/g, ' ').slice(0, 42).trim();
  return `${title || 'Chat'} · ${stamp}`;
}

function visibleSessionName(name: string): string {
  return name.replace(/ · \d{14}$/, '');
}

export function ChatPage({ scope, models, model, setModel, onboardingPending, onFirstResponse, onOpenModels }: {
  scope: string;
  models: Model[];
  model: string;
  setModel: (model: string) => void;
  onboardingPending: boolean;
  onFirstResponse: () => void;
  onOpenModels: () => void;
}) {
  const { messages: text, locale } = useI18n();
  const sessionKey = `${activeSessionKey}:${encodeURIComponent(scope)}`;
  const preferencesKey = `offgrid.chat.preferences:${encodeURIComponent(scope)}`;
  const [sessions, setSessions] = useState<ChatSession[]>([]);
  const [activeName, setActiveName] = useState(() => readPreference(sessionKey) ?? '');
  const [conversation, setConversation] = useState<ChatMessage[]>([]);
  const { value: draft, setValue: setDraft, unsaved, key: composerDraftKey } = useDraft(scope, 'chat', activeName);
  const [knowledge, setKnowledge] = useState(false);
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [checkingTurn, setCheckingTurn] = useState(false);
  const [phase, setPhase] = useState<ChatPhase>('queued');
  const [streamed, setStreamed] = useState('');
  const [interrupted, setInterrupted] = useState(false);
  const [metrics, setMetrics] = useState<ChatMetrics>();
  const [limited, setLimited] = useState(false);
  const [preferences, setPreferences] = useState(() => readPreferences(preferencesKey));
  const { profile, maxTokens } = preferences;
  const updatePreferences = (next: typeof preferences) => {
    setPreferences(next);
    writePreference(preferencesKey, JSON.stringify(next));
  };
  const [error, setError] = useState('');
  const [errorAction, setErrorAction] = useState<'history' | 'request' | 'cancel' | null>('history');
  const [deleteItems, setDeleteItems] = useState<HistoryItem[] | null>(null);
  const [historyQuery, setHistoryQuery] = useWorkspaceState('chat.historyQuery', '');
  const [historyNotice, setHistoryNotice] = useState('');
  const [historyOpen, setHistoryOpen] = useState(false);
  const [copiedMessage, setCopiedMessage] = useState('');
  const controller = useRef<AbortController | null>(null);
  const end = useRef<HTMLDivElement | null>(null);
  const followOutput = useRef(true);
  const composerInput = useRef<HTMLTextAreaElement | null>(null);
  const activeTurn = useRef<{ name: string; id: string } | null>(null);
  const mounted = useRef(true);
  const selectionRevision = useRef(0);
  const historyPanel = useRef<HTMLElement>(null);
  useFocusScope(historyPanel, historyOpen && !deleteItems, () => setHistoryOpen(false));
  const permissions = useWorkspace();
  const [knowledgeReady, setKnowledgeReady] = useState<boolean | null>(null);
  const refreshKnowledge = async () => {
    if (!permissions.knowledge) { setKnowledgeReady(false); return; }
    try { setKnowledgeReady((await api.ragStatus()).enabled); } catch { setKnowledgeReady(null); }
  };
  useEffect(() => { void refreshKnowledge(); }, [permissions.knowledge]);
  useWorkspaceRefresh(refreshKnowledge);

  const recoverTurn = async (session: ChatSession, turn: SessionTurn, revision: number) => {
    if (!mounted.current || revision !== selectionRevision.current) return;
    activeTurn.current = { name: session.name, id: turn.id };
    if (!['pending', 'running'].includes(turn.status)) {
      if (turn.status === 'completed') {
        const key = draftKey(scope, 'chat', session.name), savedDraft = readDraft(key);
        if (savedDraft.trim() === turn.prompt) clearSubmittedDraft(key, savedDraft);
        setMetrics(turn.metrics); setLimited(turn.finish_reason === 'length');
      } else { setConversation([...session.messages, { role: 'user', content: turn.prompt }]); setStreamed(turn.output); setInterrupted(true); setError(turn.error || text.chatStreaming.unsaved); }
      return;
    }
    const watch = new AbortController(); controller.current = watch;
    setBusy(true); setConversation([...session.messages, { role: 'user', content: turn.prompt }]);
    let partial = '';
    try {
      const result = await api.followTurn(session.name, turn.id, event => {
        if (!mounted.current || revision !== selectionRevision.current) return;
        if (event.type === 'phase') setPhase(event.phase);
        else { partial += event.delta; setStreamed(partial); }
      }, watch.signal);
      if (!mounted.current || revision !== selectionRevision.current) return;
      setConversation(result.session.messages); setStreamed(''); setMetrics(result.metrics); setLimited(result.finish_reason === 'length');
      const key = draftKey(scope, 'chat', session.name), savedDraft = readDraft(key);
      if (savedDraft.trim() === turn.prompt) clearSubmittedDraft(key, savedDraft);
      setSessions(current => [result.session, ...current.filter(item => item.name !== session.name)]);
    } catch (reason) {
      if (mounted.current && revision === selectionRevision.current) { setStreamed(partial || turn.output); setInterrupted(true); setErrorAction('request'); setError(reason instanceof Error ? reason.message : text.common.error); }
    } finally { if (controller.current === watch) { controller.current = null; if (mounted.current) setBusy(false); } }
  };

  const activate = (session?: ChatSession) => {
    const revision = ++selectionRevision.current;
    controller.current?.abort(); controller.current = null; activeTurn.current = null; setBusy(false); setCheckingTurn(Boolean(session)); setError('');
    const name = session?.name ?? '';
    setActiveName(name);
    setConversation(session?.messages ?? []);
    setStreamed(''); setInterrupted(false); setMetrics(undefined); setLimited(false);
    if (session?.model_id && models.some(item => item.id === session.model_id)) setModel(session.model_id);
    writePreference(sessionKey, name || null);
    setHistoryOpen(false);
    if (session) void api.currentTurn(session.name).then(({ turn }) => { if (turn?.id) void recoverTurn(session, turn, revision); }).catch(reason => { if (mounted.current && revision === selectionRevision.current) setError(reason instanceof Error ? reason.message : text.common.error); }).finally(() => { if (mounted.current && revision === selectionRevision.current) setCheckingTurn(false); });
  };

  const loadSessions = async (preferred = activeName) => {
    setErrorAction('history');
    setLoading(true);
    setError('');
    try {
      const next = await api.sessions();
      setSessions(next);
      activate(!preferred && readDraft(draftKey(scope, 'chat')) ? undefined : next.find(item => item.name === preferred) ?? next[0]);
      if (onboardingPending && next.some(item => item.messages.some(message => message.role === 'assistant' && message.content.trim()))) onFirstResponse();
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : text.common.error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void loadSessions(); }, []);
  useWorkspaceRefresh(async () => {
    // Refresh history without resetting the selected conversation or stream.
    try { setSessions(await api.sessions()); }
    catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); }
  });
  useEffect(() => { if (followOutput.current) end.current?.scrollIntoView({ behavior: 'instant' }); }, [conversation, busy, streamed, phase]);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; selectionRevision.current++; controller.current?.abort(); }; }, []);
  useEffect(() => {
    if (!composerInput.current) return;
    composerInput.current.style.height = '0';
    composerInput.current.style.height = `${Math.min(composerInput.current.scrollHeight, 160)}px`;
  }, [draft]);

  const startNew = () => {
    if (busy || deleteItems) return;
    activate(undefined);
    setError('');
    setCopiedMessage('');
  };

  const selectSession = (session: ChatSession) => {
    if (!busy) {
      activate(session);
    }
  };

  const historyDeleted = (names: string[]) => {
    setSessions(current => current.filter(item => !names.includes(item.name)));
    names.forEach(name => writeDraft(draftKey(scope, 'chat', name), ''));
    if (names.includes(activeName)) activate(undefined);
    if (names.length) setHistoryNotice(text.history.removed.replace('{count}', String(names.length)));
  };

  const send = async () => {
    setErrorAction('request');
    const prompt = draft.trim();
    if (!prompt || !model || busy || checkingTurn || controller.current || deleteItems) return;
    const requestController = new AbortController();
    const requestID = crypto.randomUUID();
    controller.current = requestController;
    const submitted = draft;
    setBusy(true);
    setPhase('queued'); setStreamed(''); setInterrupted(false); setMetrics(undefined); setLimited(false);
    followOutput.current = true;
    setError('');
    let sessionName = activeName;
    let partial = '';
    let flush: ReturnType<typeof setTimeout> | undefined;
    try {
      if (!sessionName) {
        const created = await api.createSession(newSessionName(prompt), model, requestController.signal);
        sessionName = created.name;
        writeDraft(draftKey(scope, 'chat', sessionName), readDraft(composerDraftKey));
        writeDraft(composerDraftKey, '');
        setActiveName(sessionName);
        writePreference(sessionKey, sessionName);
        setSessions(current => [created, ...current]);
      }
      setConversation(current => [...current, { role: 'user', content: prompt }]);
      activeTurn.current = { name: sessionName, id: requestID };
      const result = await api.streamSession(sessionName, prompt, model, knowledge, profile, maxTokens, event => {
        if (event.type === 'phase') setPhase(event.phase);
        else {
          partial += event.delta;
          // Bound React/Markdown updates during fast GPU decoding.
          if (!flush) flush = setTimeout(() => { setStreamed(partial); flush = undefined; }, 40);
        }
      }, requestController.signal, requestID);
      clearTimeout(flush); flush = undefined;
      setStreamed(''); setMetrics(result.metrics); setLimited(result.finish_reason === 'length');
      clearSubmittedDraft(draftKey(scope, 'chat', sessionName), submitted);
      setConversation(result.session.messages);
      setSessions(current => [result.session, ...current.filter(item => item.name !== result.session.name)]);
      if (onboardingPending && result.message.role === 'assistant' && result.message.content.trim()) onFirstResponse();
    } catch (reason) {
      clearTimeout(flush); flush = undefined;
      setStreamed(partial); setInterrupted(true);
      setError(requestController.signal.aborted ? text.chatStreaming.cancelled : reason instanceof Error ? reason.message : text.common.error);
      if (sessionName) {
        try {
          const stored = await api.session(sessionName);
          setConversation(stored.messages);
          setSessions(current => [stored, ...current.filter(item => item.name !== stored.name)]);
        } catch { /* The primary error is more useful. */ }
      }
    } finally {
      clearTimeout(flush);
      setBusy(false);
      controller.current = null;
    }
  };

  const stop = async () => {
    setErrorAction('cancel');
    if (!activeTurn.current) { controller.current?.abort(); return; }
    try { await api.cancelTurn(activeTurn.current.name, activeTurn.current.id); controller.current?.abort(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); }
  };

  const keyDown = (event: KeyboardEvent<HTMLTextAreaElement>) => {
    // Enter can confirm an IME candidate rather than submit. Some WebKit
    // composition-end events clear isComposing but retain keyCode 229.
    if (event.nativeEvent.isComposing || event.nativeEvent.keyCode === 229) return;
    if (event.key === 'Enter' && !event.shiftKey) {
      event.preventDefault();
      void send();
    }
  };

  const copyMessage = async (key: string, content: string) => {
    setErrorAction(null);
    try {
      await copyText(content);
      setCopiedMessage(key);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : text.common.error);
    }
  };

  const filteredSessions = sessions.filter(session => visibleSessionName(session.name).toLocaleLowerCase(locale).includes(historyQuery.trim().toLocaleLowerCase(locale)));
  return <div className="chat-workspace">
    {deleteItems && <HistoryDeleteDialog items={deleteItems} kind="chats" remove={api.deleteSession} onDeleted={historyDeleted} onClose={() => setDeleteItems(null)} />}
    {historyOpen && <button className="history-scrim" onClick={() => setHistoryOpen(false)} aria-label={text.chat.closeHistory} />}
    <aside ref={historyPanel} role={historyOpen && !deleteItems ? 'dialog' : undefined} aria-modal={(historyOpen && !deleteItems) || undefined} id="conversation-history" className={historyOpen ? 'conversation-history open' : 'conversation-history'} aria-label={text.chat.conversations}>
      <button className="history-close icon-button" onClick={() => setHistoryOpen(false)} aria-label={text.chat.closeHistory}><Icon name="close" size={18} /></button>
      <button className="new-chat-button" disabled={busy} onClick={startNew}><Icon name="plus" size={16} />{text.chat.newChat}</button>
      <div className="history-label">{text.chat.conversations}</div>
      <label className="history-search"><Icon name="search" size={15} /><input type="search" value={historyQuery} onChange={event => setHistoryQuery(event.target.value)} placeholder={text.history.searchChats} aria-label={text.history.searchChats} /></label>
      <div className="history-tools"><button className="text-button" disabled={busy || loading} onClick={() => void loadSessions()}>{text.common.refresh}</button><button className="text-button" disabled={busy || loading || !filteredSessions.length} onClick={() => setDeleteItems(filteredSessions.map(session => ({ id: session.name, label: visibleSessionName(session.name) })))}>{text.history.clearChats}</button></div>
      {historyNotice && <p className="history-notice" role="status">{historyNotice}</p>}
      {loading ? <div className="history-empty">{text.common.loading}</div> : sessions.length === 0 ? <div className="history-empty">{text.chat.noConversations}</div> : <div className="history-list">
        {filteredSessions.length === 0 && <div className="history-empty">{text.history.noMatches}</div>}
        {filteredSessions.map(session => <div className={activeName === session.name ? 'history-row active' : 'history-row'} key={session.name}>
          <button className="history-open" disabled={busy} onClick={() => selectSession(session)} title={visibleSessionName(session.name)}><strong>{visibleSessionName(session.name)}</strong><small>{session.messages.length} · {new Date(session.updated_at).toLocaleDateString(locale)}</small></button>
          <button className="history-delete" disabled={busy} aria-label={`${text.chat.deleteConversation}: ${visibleSessionName(session.name)}`} title={text.chat.deleteConversation} onClick={() => setDeleteItems([{ id: session.name, label: visibleSessionName(session.name) }])}><Icon name="trash" size={16} /></button>
        </div>)}
      </div>}
    </aside>
    <div className="chat-layout">
      <div className="chat-toolbar"><div className="chat-toolbar-leading"><button className="history-toggle icon-button" onClick={() => setHistoryOpen(true)} aria-label={text.chat.showHistory} aria-expanded={historyOpen} aria-controls="conversation-history"><Icon name="menu" size={19} /></button><ModelSelect models={models} value={model} onChange={setModel} /></div><label className="switch"><input type="checkbox" checked={knowledge} disabled={busy || knowledgeReady !== true} onChange={event => setKnowledge(event.target.checked)} /><span />{text.chat.knowledge}</label></div>
      {knowledgeReady !== true && <p className="history-notice" role="status">{interaction[locale].knowledgeUnavailable} <a href="#/knowledge">{text.nav.knowledge}</a></p>}
      {(!model || onboardingPending) && <div className="setup-inline" role="status"><span>{model ? text.onboarding.firstReplyHint : text.onboarding.modelHint}</span>{!model && <button className="secondary-button" onClick={onOpenModels}>{text.onboarding.chooseModel}</button>}</div>}
      <details className="chat-performance"><summary>{text.chatStreaming.options}</summary><div>
        <label>{text.chatStreaming.profile}<select value={profile} disabled={busy} onChange={event => updatePreferences({ ...preferences, profile: event.target.value })}><option value="interactive">{text.chatStreaming.interactive}</option><option value="extended">{text.chatStreaming.extended}</option></select></label>
        <label>{text.chatStreaming.responseLimit}<select value={maxTokens} disabled={busy} onChange={event => updatePreferences({ ...preferences, maxTokens: Number(event.target.value) })}>{[256, 1024, 4096].map(count => <option value={count} key={count}>{count.toLocaleString(locale)}</option>)}</select></label>
        <p>{text.chatStreaming.profileHint}</p>
      </div></details>
      <div className="conversation" onScroll={event => { const el = event.currentTarget; followOutput.current = el.scrollHeight - el.scrollTop - el.clientHeight < 100; }}>
        {conversation.length === 0 && !loading && <div className="empty-chat"><div className="orb"><div /></div><h2>{text.chat.emptyTitle}</h2><p>{text.chat.emptyBody}</p></div>}
        {conversation.map((message, index) => {
          const messageKey = `${index}-${'timestamp' in message ? message.timestamp : message.content.slice(0, 24)}`;
          return <article className={`message ${message.role}`} key={messageKey}>
            <div className="message-heading"><div className="message-label">{message.role === 'user' ? text.chat.you : message.role === 'assistant' ? text.chat.assistant : message.role}</div>{message.role === 'assistant' && <button type="button" className="message-copy" onClick={() => void copyMessage(messageKey, message.content)} aria-label={copiedMessage === messageKey ? text.chat.copied : text.chat.copyResponse}><Icon name={copiedMessage === messageKey ? 'check' : 'copy'} size={14} />{copiedMessage === messageKey ? text.chat.copied : text.chat.copy}</button>}</div>
            <div className="message-body">{message.role === 'assistant' ? <MarkdownMessage content={message.content} /> : message.content}</div>
          </article>;
        })}
        {(busy || streamed) && <article className="message assistant streaming-response"><div className="message-heading"><div className="message-label">{text.chat.assistant}</div>{streamed && <button type="button" className="message-copy" onClick={() => void copyMessage('partial', streamed)}>{copiedMessage === 'partial' ? text.chat.copied : text.chat.copy}</button>}</div>
          {streamed && <div className="message-body"><MarkdownMessage content={streamed} /></div>}
          <div className="generation-status" role="status">{interrupted ? text.chatStreaming.unsaved : text.chatStreaming[phase]}</div>
        </article>}
        {metrics && <div className="generation-status">{text.chatStreaming.firstText}: {(metrics.first_text_ms / 1000).toLocaleString(locale, { maximumFractionDigits: 2 })} s{metrics.tokens_per_second ? ` · ${metrics.tokens_per_second.toLocaleString(locale, { maximumFractionDigits: 1 })} ${text.chatStreaming.tokensPerSecond}` : ''} · {metrics.context_window.toLocaleString(locale)} {text.chatStreaming.contextTokens}</div>}
        {limited && <div className="generation-status" role="status">{text.chatStreaming.limitReached}</div>}
        {error && <div className="inline-error" role="alert">{error}{errorAction && <button className="secondary-button" onClick={() => void (errorAction === 'cancel' ? stop() : loadSessions())}>{errorAction === 'cancel' ? text.chat.stop : errorAction === 'history' ? interaction[locale].refreshHistory : interaction[locale].checkRequest}</button>}</div>}<div ref={end} />
        {unsaved && <div className="inline-error" role="alert">{text.recovery.draftWarning}</div>}
      </div>
      <div className="composer"><div className="composer-input"><textarea ref={composerInput} value={draft} onChange={event => setDraft(event.target.value)} onKeyDown={keyDown} placeholder={text.chat.placeholder} aria-label={text.chat.placeholder} rows={1} disabled={loading || checkingTurn} /><small>{text.chat.enterHint}</small></div><button disabled={busy ? false : loading || checkingTurn || !draft.trim() || !model} onClick={busy ? () => void stop() : () => void send()}>{busy ? text.chat.stop : <><span>{text.chat.send}</span><Icon name="send" size={18} /></>}</button></div>
    </div>
  </div>;
}
