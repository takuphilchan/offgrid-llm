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
import { ReadAloudButton, VoiceInputButton } from '../../components/VoiceInputButton';
import { ResponseSpeech, stopSpeechEvent, type SpeechState } from '../../lib/response-speech';
import { voiceText } from '../../i18n/voice';
import { VoiceSettings } from '../../components/VoiceSettings';
import { selectSpeech, useVoicePreferences } from '../../lib/voice-preferences';
import { operationFeedback } from '../../i18n/operation-feedback';
import { EmptyState, ScopedNotice } from '../../components/WorkspacePresentation';
import { UtilityPopover } from '../../components/UtilityPopover';
import { workspaceExperience } from '../../i18n/workspace-experience';
import { HistoryToolbar } from '../../components/HistoryToolbar';
import { SetupLink, useSetupHandoff } from '../../lib/setup-handoff';
import { useDraftScope } from '../../lib/draft-scope';
import { missingConversationText } from '../../i18n/draft-recovery';
import { useWorkActions, selectChatEvent } from '../../lib/active-work';
import { unavailableTurnText } from '../../i18n/known-work';
import { workspaceManagement } from '../../i18n/workspace-management';

function messageText(content: ChatMessage['content']): string {
  if (typeof content === 'string') return content;
  return content.filter(part => part.type === 'text').map(part => part.text).join('');
}

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
  const voiceCopy = voiceText(locale);
  const feedback = operationFeedback(locale);
  const experience = workspaceExperience[locale];
  const handoff = useSetupHandoff();
  const draftScope = useDraftScope();
  const work = useWorkActions();
  const { preferences: voicePreferences } = useVoicePreferences();
  const [speakResponses, setSpeakResponses] = useState(false);
  const [speechState, setSpeechState] = useState<SpeechState>('idle');
  const [speechError, setSpeechError] = useState('');
  const [checkingSpeech, setCheckingSpeech] = useState(false);
  const speechCheck = useRef<AbortController | null>(null);
  const speech = useRef<ResponseSpeech | null>(null);
  const stopSpeech = () => { speech.current?.stop(); speech.current = null; };
  useEffect(() => {
    window.addEventListener(stopSpeechEvent, stopSpeech);
    return () => { window.removeEventListener(stopSpeechEvent, stopSpeech); speechCheck.current?.abort(); stopSpeech(); };
  }, []);
  const toggleSpeech = async (enabled: boolean) => {
    speechCheck.current?.abort();
    setSpeechError('');
    if (!enabled) { setSpeakResponses(false); stopSpeech(); return; }
    const check = new AbortController(); speechCheck.current = check;
    setCheckingSpeech(true);
    try {
      const status = await api.audioStatus(check.signal);
      if (check.signal.aborted) return;
      selectSpeech(status, voicePreferences, 'tts', locale);
      setSpeakResponses(true);
    } catch (reason) { if (!check.signal.aborted) setSpeechError(reason instanceof Error ? reason.message : voiceCopy.speechUnavailable); }
    finally { if (!check.signal.aborted) setCheckingSpeech(false); }
  };
  const sessionKey = draftKey(draftScope, 'active-chat');
  const preferencesKey = `offgrid.chat.preferences:${encodeURIComponent(scope)}`;
  const [sessions, setSessions] = useState<ChatSession[]>([]);
  const [activeName, setActiveName] = useState(() => handoff.returning?.kind === 'chat' ? handoff.returning.id : readDraft(sessionKey));
  const [missingSession, setMissingSession] = useState(false);
  useEffect(() => work.claimChat(activeName), [activeName, work]);
  useEffect(() => { if (handoff.returning?.kind === 'chat') handoff.consumed(); }, []);
  const [conversation, setConversation] = useState<ChatMessage[]>([]);
  const { value: draft, setValue: setDraft, unsaved, key: composerDraftKey } = useDraft(draftScope, 'chat', activeName);
  const [knowledge, setKnowledge] = useWorkspaceState('chat.knowledge', false);
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
  const [historyLimit, setHistoryLimit] = useWorkspaceState('chat.historyLimit', 20);
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
  const historyRevision = useRef(0);
  const historyPanel = useRef<HTMLElement>(null);
  useFocusScope(historyPanel, historyOpen && !deleteItems, () => setHistoryOpen(false));
  const permissions = useWorkspace();
  useEffect(() => {
    if (handoff.requestedKnowledge && permissions.knowledge) { setKnowledge(true); handoff.consumeKnowledge(); }
  }, [handoff.requestedKnowledge, permissions.knowledge]);
  const [knowledgeState, setKnowledgeState] = useState<'checking' | 'ready' | 'unavailable' | 'failed'>('checking');
  const knowledgeCheck = useRef<AbortController | null>(null);
  const refreshKnowledge = async () => {
    knowledgeCheck.current?.abort();
    if (!permissions.knowledge) { setKnowledgeState('unavailable'); return; }
    const check = new AbortController(); knowledgeCheck.current = check;
    setKnowledgeState('checking');
    try {
      const status = await api.ragStatus(check.signal);
      if (!check.signal.aborted) setKnowledgeState(status.enabled === true ? 'ready' : 'unavailable');
    } catch { if (!check.signal.aborted) setKnowledgeState('failed'); }
  };
  useEffect(() => { void refreshKnowledge(); return () => knowledgeCheck.current?.abort(); }, [permissions.knowledge]);
  useWorkspaceRefresh(refreshKnowledge);

  const recoverTurn = async (session: ChatSession, turn: SessionTurn, revision: number) => {
    if (!mounted.current || revision !== selectionRevision.current) return;
    activeTurn.current = { name: session.name, id: turn.id };
    work.rememberChat(session.name, turn.status);
    if (!['pending', 'running'].includes(turn.status)) {
      if (turn.status === 'completed') {
        const key = draftKey(draftScope, 'chat', session.name), savedDraft = readDraft(key);
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
      work.rememberChat(session.name, 'completed');
      const key = draftKey(draftScope, 'chat', session.name), savedDraft = readDraft(key);
      if (savedDraft.trim() === turn.prompt) clearSubmittedDraft(key, savedDraft);
      setSessions(current => [result.session, ...current.filter(item => item.name !== session.name)]);
    } catch (reason) {
      if (mounted.current && revision === selectionRevision.current) { setStreamed(partial || turn.output); setInterrupted(true); setErrorAction('request'); setError(reason instanceof Error ? reason.message : text.common.error); }
    } finally { if (controller.current === watch) { controller.current = null; if (mounted.current) setBusy(false); } }
  };

  const activate = (session?: ChatSession) => {
    stopSpeech(); setSpeechError('');
    const revision = ++selectionRevision.current;
    controller.current?.abort(); controller.current = null; activeTurn.current = null; setBusy(false); setCheckingTurn(Boolean(session)); setError('');
    const name = session?.name ?? '';
    setActiveName(name);
    setMissingSession(false);
    setConversation(session?.messages ?? []);
    setStreamed(''); setInterrupted(false); setMetrics(undefined); setLimited(false);
    if (session?.model_id && models.some(item => item.id === session.model_id)) setModel(session.model_id);
    writeDraft(sessionKey, name);
    setHistoryOpen(false);
    if (session) void api.currentTurn(session.name).then(({ turn }) => {
      if (!mounted.current || revision !== selectionRevision.current) return;
      if (turn?.id) void recoverTurn(session, turn, revision);
      else if (work.needsRecovery(session.name)) {
        setMissingSession(true); setErrorAction('request'); setError(unavailableTurnText[locale]);
      }
    }).catch(reason => {
      if (mounted.current && revision === selectionRevision.current) {
        if (work.needsRecovery(session.name)) setMissingSession(true);
        setError(reason instanceof Error ? reason.message : text.common.error);
      }
    }).finally(() => { if (mounted.current && revision === selectionRevision.current) setCheckingTurn(false); });
  };

  const loadSessions = async (preferred = activeName) => {
    const request = ++historyRevision.current;
    const selection = selectionRevision.current;
    setErrorAction('history');
    setLoading(true);
    setError('');
    try {
      const next = await api.sessions();
      if (!mounted.current || request !== historyRevision.current) return;
      setSessions(next);
      // A late startup/retry must never replace a newer selection or its draft.
      if (selection === selectionRevision.current) {
        const selected = next.find(item => item.name === preferred);
        if (preferred && !selected) {
          // Never silently bind a returning draft to the first unrelated chat.
          activate(undefined); setActiveName(preferred); writeDraft(sessionKey, preferred);
          setMissingSession(true); setError(missingConversationText[locale]);
        } else activate(!preferred && readDraft(draftKey(draftScope, 'chat')) ? undefined : selected ?? next[0]);
      }
      if (onboardingPending && next.some(item => item.messages.some(message => message.role === 'assistant' && messageText(message.content).trim()))) onFirstResponse();
    } catch (reason) {
      if (mounted.current && request === historyRevision.current) setError(reason instanceof Error ? reason.message : text.common.error);
    } finally {
      if (mounted.current && request === historyRevision.current) setLoading(false);
    }
  };

  useEffect(() => { void loadSessions(); }, []);
  useEffect(() => {
    const selectKnown = (event: Event) => {
      const name = (event as CustomEvent<string>).detail;
      if (typeof name === 'string' && name !== activeName) void loadSessions(name);
    };
    window.addEventListener(selectChatEvent, selectKnown);
    return () => window.removeEventListener(selectChatEvent, selectKnown);
  }, [activeName]);
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
    names.forEach(name => { writeDraft(draftKey(draftScope, 'chat', name), ''); work.forgetChat(name); });
    if (names.includes(activeName)) activate(undefined);
    if (names.length) setHistoryNotice(text.history.removed.replace('{count}', String(names.length)));
  };

  const send = async () => {
    setErrorAction('request');
    const prompt = draft.trim();
    if (!prompt || !model || busy || checkingTurn || missingSession || controller.current || deleteItems) return;
    const revision = selectionRevision.current;
    const currentView = () => mounted.current && revision === selectionRevision.current;
    const requestController = new AbortController();
    const requestID = crypto.randomUUID();
    window.dispatchEvent(new Event(stopSpeechEvent));
    setSpeechError('');
    if (speakResponses) speech.current = new ResponseSpeech(setSpeechState, setSpeechError, voicePreferences, locale);
    const responseSpeech = speech.current;
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
        if (!currentView()) return;
        sessionName = created.name;
        writeDraft(draftKey(draftScope, 'chat', sessionName), readDraft(composerDraftKey));
        writeDraft(composerDraftKey, '');
        setActiveName(sessionName);
        writeDraft(sessionKey, sessionName);
        setSessions(current => [created, ...current]);
      }
      setConversation(current => [...current, { role: 'user', content: prompt }]);
      activeTurn.current = { name: sessionName, id: requestID };
      work.rememberChat(sessionName, 'pending');
      const result = await api.streamSession(sessionName, prompt, model, knowledge, profile, maxTokens, event => {
        if (!currentView()) return;
        if (event.type === 'phase') setPhase(event.phase);
        else {
          partial += event.delta;
          responseSpeech?.append(event.delta);
          // Bound React/Markdown updates during fast GPU decoding.
          if (!flush) flush = setTimeout(() => { if (currentView()) setStreamed(partial); flush = undefined; }, 40);
        }
      }, requestController.signal, requestID);
      clearTimeout(flush); flush = undefined;
      if (!currentView()) return;
      setStreamed(''); setMetrics(result.metrics); setLimited(result.finish_reason === 'length');
      clearSubmittedDraft(draftKey(draftScope, 'chat', sessionName), submitted);
      setConversation(result.session.messages);
      work.rememberChat(sessionName, 'completed');
      responseSpeech?.finish(messageText(result.message.content));
      setSessions(current => [result.session, ...current.filter(item => item.name !== result.session.name)]);
      if (onboardingPending && result.message.role === 'assistant' && messageText(result.message.content).trim()) onFirstResponse();
    } catch (reason) {
      responseSpeech?.stop();
      clearTimeout(flush); flush = undefined;
      if (!currentView()) return;
      setStreamed(partial); setInterrupted(true);
      setError(requestController.signal.aborted ? text.chatStreaming.cancelled : reason instanceof Error ? reason.message : text.common.error);
      if (sessionName) {
        try {
          const stored = await api.session(sessionName);
          if (!currentView()) return;
          setConversation(stored.messages);
          setSessions(current => [stored, ...current.filter(item => item.name !== stored.name)]);
        } catch { /* The primary error is more useful. */ }
      }
    } finally {
      clearTimeout(flush);
      if (currentView()) setBusy(false);
      if (controller.current === requestController) controller.current = null;
    }
  };

  const stop = async () => {
    stopSpeech();
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
  // Metrics describe the just-committed turn, never an older answer or a new
  // in-flight response. They are cleared when starting/selecting a conversation.
  const lastAssistant = conversation.reduce((last, message, index) => message.role === 'assistant' ? index : last, -1);
  return <div className="chat-workspace">
    {deleteItems && <HistoryDeleteDialog items={deleteItems} kind="chats" remove={api.deleteSession} onDeleted={historyDeleted} onClose={() => setDeleteItems(null)} />}
    {historyOpen && <button className="history-scrim" onClick={() => setHistoryOpen(false)} aria-label={text.chat.closeHistory} />}
    <aside ref={historyPanel} role={historyOpen && !deleteItems ? 'dialog' : undefined} aria-modal={(historyOpen && !deleteItems) || undefined} id="conversation-history" className={historyOpen ? 'conversation-history open' : 'conversation-history'} aria-label={text.chat.conversations}>
      <button className="history-close icon-button" onClick={() => setHistoryOpen(false)} aria-label={text.chat.closeHistory}><Icon name="close" size={18} /></button>
      <button className="new-chat-button" disabled={busy} onClick={startNew}><Icon name="plus" size={16} />{text.chat.newChat}</button>
      <div className="history-label">{text.chat.conversations}</div>
      <HistoryToolbar query={historyQuery} onQuery={value => { setHistoryQuery(value); setHistoryLimit(20); }} searchLabel={text.history.searchChats} count={sessions.length} busy={busy || loading} refresh={() => void loadSessions()} clearLabel={text.history.clearChats} clearDisabled={!filteredSessions.length} clear={() => setDeleteItems(filteredSessions.map(session => ({ id: session.name, label: visibleSessionName(session.name) })))} />
      {historyNotice && <p className="history-notice" role="status">{historyNotice}</p>}
      {loading ? <div className="history-empty" role="status">{text.common.loading}</div> : sessions.length === 0 ? <EmptyState className="history-empty" title={text.chat.noConversations} /> : <div className="history-list">
        {filteredSessions.length === 0 && <div className="history-empty">{text.history.noMatches}</div>}
        {filteredSessions.slice(0, historyLimit).map(session => <div className={activeName === session.name ? 'history-row active' : 'history-row'} key={session.name}>
          <button className="history-open" disabled={busy} onClick={() => selectSession(session)} title={visibleSessionName(session.name)}><strong>{visibleSessionName(session.name)}</strong><small>{session.messages.length} · {new Date(session.updated_at).toLocaleDateString(locale)}</small></button>
          <button className="history-delete" disabled={busy} aria-label={`${text.chat.deleteConversation}: ${visibleSessionName(session.name)}`} title={text.chat.deleteConversation} onClick={() => setDeleteItems([{ id: session.name, label: visibleSessionName(session.name) }])}><Icon name="trash" size={16} /></button>
        </div>)}
        {filteredSessions.length > historyLimit && <button className="secondary-button history-more" onClick={() => setHistoryLimit(limit => limit + 20)}>{text.history.showMore} ({filteredSessions.length - historyLimit})</button>}
      </div>}
    </aside>
    <div className="chat-layout">
      <div className="chat-toolbar"><button className="history-toggle secondary-button" onClick={() => setHistoryOpen(true)} aria-label={text.chat.showHistory} aria-expanded={historyOpen} aria-controls="conversation-history"><Icon name="menu" size={19} />{text.chat.conversations}</button></div>
      {(!model || onboardingPending) && <div className="setup-inline" role="status"><span>{model ? text.onboarding.firstReplyHint : text.onboarding.modelHint}</span>{!model && <button className="secondary-button" onClick={() => { handoff.begin({ origin: '#/chat', destination: 'models', draft: { kind: 'chat', id: activeName } }); onOpenModels(); }}>{text.onboarding.chooseModel}</button>}</div>}
      <div className="conversation" onScroll={event => { const el = event.currentTarget; followOutput.current = el.scrollHeight - el.scrollTop - el.clientHeight < 100; }}>
        {conversation.length === 0 && !loading && <div className="empty-chat"><div className="orb"><div /></div><h2>{text.chat.emptyTitle}</h2><p>{text.chat.emptyBody}</p></div>}
        {conversation.map((message, index) => {
          const content = messageText(message.content);
          const messageKey = `${index}-${'timestamp' in message ? message.timestamp : content.slice(0, 24)}`;
          return <article className={`message ${message.role}`} key={messageKey}>
            <div className="message-heading"><div className="message-label">{message.role === 'user' ? text.chat.you : message.role === 'assistant' ? text.chat.assistant : message.role}</div></div>
            <div className="message-body">{message.role === 'assistant' ? <MarkdownMessage content={content} /> : content}</div>
            {message.role === 'assistant' && <footer className="message-actions">
              <button type="button" className="message-copy" onClick={() => void copyMessage(messageKey, content)} aria-label={copiedMessage === messageKey ? text.chat.copied : text.chat.copyResponse}><Icon name={copiedMessage === messageKey ? 'check' : 'copy'} size={14} />{copiedMessage === messageKey ? text.chat.copied : text.chat.copy}</button>
              <ReadAloudButton text={content} />
              {metrics && index === lastAssistant && <UtilityPopover label={experience.responseDetails} className="response-details">
                <dl className="response-metrics">
                  <div><dt>{text.chatStreaming.firstText}</dt><dd>{(metrics.first_text_ms / 1000).toLocaleString(locale, { maximumFractionDigits: 2 })} s</dd></div>
                  {!!metrics.tokens_per_second && <div><dt>{text.chatStreaming.tokensPerSecond}</dt><dd>{metrics.tokens_per_second.toLocaleString(locale, { maximumFractionDigits: 1 })}</dd></div>}
                  <div><dt>{text.chatStreaming.contextTokens}</dt><dd>{metrics.context_window.toLocaleString(locale)}</dd></div>
                </dl>
              </UtilityPopover>}
            </footer>}
          </article>;
        })}
        {(busy || streamed) && <article className="message assistant streaming-response"><div className="message-heading"><div className="message-label">{text.chat.assistant}</div>{streamed && <button type="button" className="message-copy" onClick={() => void copyMessage('partial', streamed)}>{copiedMessage === 'partial' ? text.chat.copied : text.chat.copy}</button>}</div>
          {streamed && <div className="message-body"><MarkdownMessage content={streamed} /></div>}
          <div className="generation-status" role="status">{interrupted ? text.chatStreaming.unsaved : text.chatStreaming[phase]}</div>
        </article>}
        {limited && <div className="generation-status" role="status">{text.chatStreaming.limitReached}</div>}
        {error && <ScopedNotice kind="error" action={errorAction && <button className="secondary-button" onClick={() => void (errorAction === 'cancel' ? stop() : loadSessions())}>{errorAction === 'cancel' ? text.chat.stop : errorAction === 'history' ? interaction[locale].refreshHistory : interaction[locale].checkRequest}</button>}>{error}</ScopedNotice>}<div ref={end} />
        {unsaved && <div className="inline-error" role="alert">{text.recovery.draftWarning}</div>}
      </div>
      <div className="composer">
        <div className="composer-input"><textarea ref={composerInput} value={draft} onChange={event => setDraft(event.target.value)} onKeyDown={keyDown} placeholder={text.chat.placeholder} aria-label={text.chat.placeholder} rows={1} disabled={loading || checkingTurn} /><small>{text.chat.enterHint}</small></div>
        <div className="composer-context">
          <ModelSelect models={models} value={model} onChange={setModel} />
          <UtilityPopover label={experience.context} trigger={<>{experience.context}{knowledge && <span className="composer-choice">{experience.knowledgeOn}</span>}</>} className="composer-context-menu">
            <label className="switch"><input type="checkbox" checked={knowledge} disabled={busy || (!knowledge && knowledgeState !== 'ready')} onChange={event => setKnowledge(event.target.checked)} /><span />{text.chat.knowledge}</label>
            {permissions.knowledge && knowledgeState !== 'ready' && <div className="knowledge-readiness">
              {knowledgeState === 'checking' ? <span role="status">{feedback.checkingKnowledge}</span>
                : knowledgeState === 'failed' ? <><span role="status">{feedback.knowledgeCheckFailed}</span> <button className="text-button" onClick={() => void refreshKnowledge()}>{feedback.retryKnowledge}</button></>
                : <SetupLink destination="knowledge" draft={{ kind: 'chat', id: activeName }}>{feedback.configureKnowledge}</SetupLink>}
            </div>}
            <label className="field">{text.chatStreaming.profile}<select value={profile} disabled={busy} onChange={event => updatePreferences({ ...preferences, profile: event.target.value })}><option value="interactive">{text.chatStreaming.interactive}</option><option value="extended">{text.chatStreaming.extended}</option></select></label>
            <label className="field">{text.chatStreaming.responseLimit}<select value={maxTokens} disabled={busy} onChange={event => updatePreferences({ ...preferences, maxTokens: Number(event.target.value) })}>{[256, 1024, 4096].map(count => <option value={count} key={count}>{count.toLocaleString(locale)}</option>)}</select></label>
            <p>{text.chatStreaming.profileHint}</p>
          </UtilityPopover>
          <VoiceSettings />
        </div>
        {knowledge && knowledgeState !== 'ready' && <ScopedNotice action={<button className="secondary-button" onClick={() => void refreshKnowledge()}>{feedback.retryKnowledge}</button>}>{knowledgeState === 'checking' ? feedback.checkingKnowledge : knowledgeState === 'failed' ? feedback.knowledgeCheckFailed : interaction[locale].knowledgeUnavailable}</ScopedNotice>}
        {knowledge && <p className="composer-retrieval-scope">{workspaceManagement(locale).retrievalScope}</p>}
        <div className="composer-footer">
      <div className="composer-voice-toolbar chat-speech-controls">
        <label className="switch"><input type="checkbox" checked={speakResponses} disabled={checkingSpeech || (busy && !speakResponses)} onChange={event => void toggleSpeech(event.target.checked)} /><span />{voiceCopy.speakResponses}</label>
        {speechState !== 'idle' && <button type="button" className="text-button" onClick={stopSpeech}>{voiceCopy.stopSpeaking}</button>}
        {speechState !== 'idle' && <small role="status">{speechState === 'waiting' ? voiceCopy.speechWaiting : speechState === 'preparing' ? voiceCopy.preparingHint : busy ? voiceCopy.provisionalSpeech : voiceCopy.speak}</small>}
        {speechError && <small role="alert">{speechError} <a className="secondary-button" href="#/models">{text.nav.models}</a></small>}
      </div>
      <div className="composer-actions"><VoiceInputButton contextKey={composerDraftKey} disabled={loading || checkingTurn || busy} onTranscript={value => setDraft(draft ? `${draft} ${value}` : value)} /><button disabled={busy ? false : loading || checkingTurn || missingSession || !draft.trim() || !model} onClick={busy ? () => void stop() : () => void send()}>{busy ? text.chat.stop : <><span>{text.chat.send}</span><Icon name="send" size={18} /></>}</button></div>
        </div>
      </div>
    </div>
  </div>;
}
