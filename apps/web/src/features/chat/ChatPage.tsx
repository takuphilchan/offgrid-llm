import { useWorkspace, useWorkspaceState } from '../../lib/workspace-context';
import { SidebarContext, SidebarSlot } from '../../components/shell/SidebarSlot';
import { interaction } from '../../i18n/interaction';
import { useWorkspaceRefresh } from '../../lib/workspace-refresh';
import { useContext, useEffect, useLayoutEffect, useRef, useState, type KeyboardEvent } from 'react';
import { workspaceRefinement } from '../../i18n/workspace-refinement';
import { api, type AgentProfile, type AgentResourceRestrictions, type ChatMessage, type ChatSession, type Model, type SessionTurn } from '../../api/client';
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
import { HistoryItemActions } from '../../components/HistoryItemActions';
import { SetupLink, useSetupHandoff } from '../../lib/setup-handoff';
import { useDraftScope } from '../../lib/draft-scope';
import { missingConversationText } from '../../i18n/draft-recovery';
import { useWorkActions, selectChatEvent } from '../../lib/active-work';
import { unavailableTurnText } from '../../i18n/known-work';
import { workspaceManagement } from '../../i18n/workspace-management';
import { AgentSelector, AgentTools } from './AgentSelector';
import { ConversationWork, workSettled } from './ConversationWork';
import { useTaskRun } from '../agents/useTaskRun';
import { TaskAccessPolicy, type TaskComputerPolicy } from '../agents/TaskAccessPolicy';
import { conversationWorkText } from '../../i18n/conversation-work';

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
  const sidebar = useContext(SidebarContext);
  const refinement = workspaceRefinement[locale];
  const [showLatest, setShowLatest] = useState(false);
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
  const [agent, setAgent] = useWorkspaceState<AgentProfile | null>('chat.agent', null);
  const [agentResources, setAgentResources] = useWorkspaceState<{ profile: string; restrictions: AgentResourceRestrictions }>('chat.agentResources', { profile: '', restrictions: {} });
  const [computerPolicy, setComputerPolicy] = useWorkspaceState<TaskComputerPolicy>('chat.agentPolicy', 'full_task');
  const agentKey = agent ? `${agent.id}:${agent.revision}` : '';
  const restrictions = agentResources.profile === agentKey ? agentResources.restrictions : {};
  const effectiveModel = agent?.settings.model ?? model;
  const [linkedTurn, setLinkedTurn] = useState<SessionTurn | null>(null);
  const [recoveryNeeded, setRecoveryNeeded] = useState(false);
  const { run: linkedRun, connection: jobConnection, error: jobReadError, refresh: refreshJob } = useTaskRun(linkedTurn?.job_id ?? '');
  const currentLinked = useRef<SessionTurn | null>(null); currentLinked.current = linkedTurn;
  const linkedUnsettled = !!linkedTurn && !workSettled(linkedRun?.status ?? linkedTurn.status);
  const canSendInstruction = linkedUnsettled && linkedRun?.can_steer === true;
  const jobCopy = conversationWorkText[locale];
  const spokenJob = useRef('');
  const submittedJob = useRef('');
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
  const [deleteWork, setDeleteWork] = useState<'' | 'retain' | 'stop'>('');
  useEffect(() => setDeleteWork(''), [deleteItems]);
  const [historyQuery, setHistoryQuery] = useWorkspaceState('chat.historyQuery', '');
  const [historyLimit, setHistoryLimit] = useWorkspaceState('chat.historyLimit', 20);
  const [historyNotice, setHistoryNotice] = useState('');
  const [copiedMessage, setCopiedMessage] = useState('');
  const controller = useRef<AbortController | null>(null);
  const conversationPane = useRef<HTMLDivElement | null>(null);
  const conversationContent = useRef<HTMLDivElement | null>(null);
  const followOutput = useRef(true);
  const userScrollUntil = useRef(0);
  const composerInput = useRef<HTMLTextAreaElement | null>(null);
  const activeTurn = useRef<{ name: string; id: string } | null>(null);
  const mounted = useRef(true);
  const selectionRevision = useRef(0);
  const historyRevision = useRef(0);
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

  const acceptLinkedTurn = (session: ChatSession, turn: SessionTurn, revision: number) => {
    if (!mounted.current || revision !== selectionRevision.current) return;
    activeTurn.current = { name: session.name, id: turn.id };
    setLinkedTurn(turn); setRecoveryNeeded(false); setBusy(false); setStreamed(''); setInterrupted(false);
    const saved = session.messages.some(message => message.turn_id === turn.id && message.role === 'user');
    setConversation(saved ? session.messages : [...session.messages, { role: 'user', content: turn.prompt }]);
    work.rememberChat(session.name, turn.status);
  };

  const refreshLinkedConversation = async (turn: SessionTurn, explicitRecovery = false) => {
    const revision = selectionRevision.current, name = activeName;
    if (explicitRecovery) await api.recoverAgentTurn(name, turn.id);
    const session = await api.session(name);
    if (!mounted.current || revision !== selectionRevision.current || currentLinked.current?.id !== turn.id) return;
    const saved = session.messages.some(message => message.turn_id === turn.id && message.role === 'assistant');
    setRecoveryNeeded(!saved && linkedRun?.status === 'completed');
    if (saved) {
      setConversation(session.messages);
      setSessions(current => [session, ...current.filter(item => item.name !== name)]);
      const key = draftKey(draftScope, 'chat', name), previous = readDraft(key);
      if (previous.trim() === turn.prompt) clearSubmittedDraft(key, previous);
      if (onboardingPending) onFirstResponse();
    }
  };
  useEffect(() => {
    if (!linkedRun || !linkedTurn) return;
    work.rememberChat(activeName, linkedRun.status);
    if (workSettled(linkedRun.status)) {
      void refreshLinkedConversation(linkedTurn).catch(reason => { if (currentLinked.current?.id === linkedTurn.id) setError(reason instanceof Error ? reason.message : text.common.error); });
      if (spokenJob.current === linkedRun.run_id) {
        spokenJob.current = '';
        if (linkedRun.status === 'completed' && speakResponses && linkedRun.output) {
          // Tool/delegation time is not audio preparation time. Begin the audio
          // deadline only when the parent's final response is ready to speak.
          stopSpeech();
          speech.current = new ResponseSpeech(setSpeechState, setSpeechError, voicePreferences, locale);
          speech.current.finish(linkedRun.output);
        }
      }
    }
  }, [linkedRun?.run_id, linkedRun?.status, linkedRun?.event_cursor]);

  const recoverTurn = async (session: ChatSession, turn: SessionTurn, revision: number) => {
    if (!mounted.current || revision !== selectionRevision.current) return;
    if (turn.agent || turn.job_id) { acceptLinkedTurn(session, turn, revision); return; }
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
    followOutput.current = true;
    const revision = ++selectionRevision.current;
    controller.current?.abort(); controller.current = null; activeTurn.current = null; setBusy(false); setCheckingTurn(Boolean(session)); setError('');
    setLinkedTurn(null); setRecoveryNeeded(false); spokenJob.current = ''; submittedJob.current = '';
    const name = session?.name ?? '';
    setActiveName(name);
    setMissingSession(false);
    setConversation(session?.messages ?? []);
    setStreamed(''); setInterrupted(false); setMetrics(undefined); setLimited(false);
    if (session?.model_id && models.some(item => item.id === session.model_id)) setModel(session.model_id);
    writeDraft(sessionKey, name);
    sidebar.close();
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
        if (window.location.hash === '#/chat/new') {
          activate(undefined); window.history.replaceState(null, '', '#/chat');
        } else if (preferred && !selected) {
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
  const followConversation = () => {
    const pane = conversationPane.current;
    // Explicit Latest also wins over a still-running native keyboard scroll.
    // The scroll handler corrects drift only when no fresh user intent exists.
    if (pane && followOutput.current) pane.scrollTo({ top: pane.scrollHeight, behavior: 'instant' });
  };
  useLayoutEffect(followConversation, [conversation, busy, streamed, phase, linkedRun?.event_cursor, linkedRun?.status, error]);
  useEffect(() => {
    const content = conversationContent.current, pane = conversationPane.current;
    if (!content || !pane) return;
    const observer = new ResizeObserver(followConversation);
    observer.observe(content);
    observer.observe(pane);
    return () => observer.disconnect();
  }, []);
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
    if (!prompt || !effectiveModel || busy || checkingTurn || missingSession || controller.current || deleteItems) return;
    if (linkedUnsettled) {
      if (!linkedRun || !canSendInstruction) { setError(jobCopy.unavailable); return; }
      setBusy(true); setError('');
      const submitted = draft, key = draftKey(draftScope, 'chat-agent-instruction', linkedRun.run_id);
      let intent: { request_id: string; instruction: string } | undefined;
      try { intent = JSON.parse(readDraft(key)); } catch { /* No unacknowledged instruction. */ }
      if (!intent || intent.instruction !== prompt) intent = { request_id: crypto.randomUUID(), instruction: prompt };
      writeDraft(key, JSON.stringify(intent));
      try {
        await api.jobAction(linkedRun.run_id, 'steer', intent);
        clearSubmittedDraft(key, JSON.stringify(intent)); clearSubmittedDraft(composerDraftKey, submitted); refreshJob();
      } catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); refreshJob(); }
      finally { setBusy(false); }
      return;
    }
    const revision = selectionRevision.current;
    const currentView = () => mounted.current && revision === selectionRevision.current;
    const requestController = new AbortController();
    const requestID = crypto.randomUUID();
    window.dispatchEvent(new Event(stopSpeechEvent));
    setSpeechError('');
    if (speakResponses && !agent) speech.current = new ResponseSpeech(setSpeechState, setSpeechError, voicePreferences, locale);
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
        const created = await api.createSession(newSessionName(prompt), effectiveModel, requestController.signal);
        if (!currentView()) return;
        sessionName = created.name;
        writeDraft(draftKey(draftScope, 'chat', sessionName), readDraft(composerDraftKey));
        writeDraft(composerDraftKey, '');
        setActiveName(sessionName);
        writeDraft(sessionKey, sessionName);
        setSessions(current => [created, ...current]);
      }
      if (agent) {
        const intentKey = draftKey(draftScope, 'chat-agent-request', sessionName);
        const payload = { content: prompt, model_id: agent.settings.model, profile: profile === 'extended' ? 'extended' as const : 'interactive' as const, max_tokens: maxTokens, agent: { id: agent.id, revision: agent.revision }, restrictions, ...(agent.settings.computer && restrictions.computer !== false ? { computer_scope: 'desktop' as const, computer_approval_mode: computerPolicy } : {}) };
        let intent: { request_id: string; payload: typeof payload } | undefined;
        try { intent = JSON.parse(readDraft(intentKey)); } catch { /* No pending admission. */ }
        if (!intent || JSON.stringify(intent.payload) !== JSON.stringify(payload)) intent = { request_id: requestID, payload };
        writeDraft(intentKey, JSON.stringify(intent));
        activeTurn.current = { name: sessionName, id: intent.request_id };
        work.rememberChat(sessionName, 'pending');
        const accepted = await api.submitAgentTurn(sessionName, { ...intent.payload, request_id: intent.request_id }, requestController.signal);
        if (!currentView()) return;
        clearSubmittedDraft(intentKey, JSON.stringify(intent));
        clearSubmittedDraft(draftKey(draftScope, 'chat', sessionName), submitted);
        setConversation(current => [...current, { role: 'user', content: prompt }]);
        setLinkedTurn(accepted.turn); setRecoveryNeeded(false); setStreamed('');
        submittedJob.current = accepted.job.run_id;
        spokenJob.current = accepted.job.run_id;
        return;
      }
      setLinkedTurn(null); setRecoveryNeeded(false);
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
  const linkedOutputSaved = !!linkedTurn && conversation.some(message => 'turn_id' in message && message.turn_id === linkedTurn.id && message.role === 'assistant');
  return <div className="chat-workspace">
    {deleteItems && <HistoryDeleteDialog items={deleteItems} kind="chats" remove={name => api.deleteSession(name, deleteWork || undefined)} onDeleted={historyDeleted} onClose={() => setDeleteItems(null)} options={locked => <><label>{jobCopy.deleteChoice}<select disabled={locked} value={deleteWork} onChange={event => setDeleteWork(event.target.value as '' | 'retain' | 'stop')}><option value="">{jobCopy.preserveActive}</option><option value="retain">{jobCopy.retain}</option><option value="stop">{jobCopy.stop}</option></select></label><p>{jobCopy.deleteHint}</p></>} />}
    <SidebarSlot action><button className="new-chat-button" title={text.chat.newChat} aria-label={text.chat.newChat} disabled={busy} onClick={() => { sidebar.close(); startNew(); }}><Icon name="plus" size={18} /><span>{text.chat.newChat}</span></button></SidebarSlot>
    <SidebarSlot><section id="conversation-history" className="conversation-history" aria-label={text.chat.conversations}>
      <HistoryToolbar query={historyQuery} onQuery={value => { setHistoryQuery(value); setHistoryLimit(20); }} searchLabel={text.history.searchChats} title={text.chat.conversations} count={sessions.length} busy={busy || loading} refresh={() => void loadSessions()} clearLabel={text.history.clearChats} clearDisabled={!filteredSessions.length} clear={() => setDeleteItems(filteredSessions.map(session => ({ id: session.name, label: visibleSessionName(session.name) })))} />
      {historyNotice && <p className="history-notice" role="status">{historyNotice}</p>}
      {loading ? <div className="history-empty" role="status">{text.common.loading}</div> : sessions.length === 0 ? <EmptyState className="history-empty" title={text.chat.noConversations} /> : <div className="history-list">
        {filteredSessions.length === 0 && <div className="history-empty">{text.history.noMatches}</div>}
        {filteredSessions.slice(0, historyLimit).map(session => <div className={activeName === session.name ? 'history-row active' : 'history-row'} key={session.name}>
          <button className="history-open" disabled={busy} aria-current={activeName === session.name ? 'true' : undefined} onClick={() => { sidebar.close(); selectSession(session); }} title={`${visibleSessionName(session.name)} · ${new Date(session.updated_at).toLocaleDateString(locale)}`}><strong>{visibleSessionName(session.name)}</strong></button>
          <HistoryItemActions title={visibleSessionName(session.name)} label={text.chat.deleteConversation} disabled={busy} onDelete={() => setDeleteItems([{ id: session.name, label: visibleSessionName(session.name) }])} />
        </div>)}
        {filteredSessions.length > historyLimit && <button className="secondary-button history-more" onClick={() => setHistoryLimit(limit => limit + 20)}>{text.history.showMore} ({filteredSessions.length - historyLimit})</button>}
      </div>}
    </section></SidebarSlot>
    <div className="chat-layout">
      {(!model || onboardingPending) && <div className="setup-inline" role="status"><span>{model ? text.onboarding.firstReplyHint : text.onboarding.modelHint}</span>{!model && <button className="secondary-button" onClick={() => { handoff.begin({ origin: '#/chat', destination: 'models', draft: { kind: 'chat', id: activeName } }); onOpenModels(); }}>{text.onboarding.chooseModel}</button>}</div>}
<div ref={conversationPane} className="conversation" tabIndex={0} onWheel={event => { userScrollUntil.current = performance.now() + 1200; if (event.deltaY < 0) followOutput.current = false; }} onTouchStart={() => { userScrollUntil.current = performance.now() + 1200; }} onKeyDown={event => { if (['ArrowUp','ArrowDown','PageUp','PageDown','Home','End',' '].includes(event.key)) userScrollUntil.current = performance.now() + 1200; }} onScroll={event => { const el = event.currentTarget; const distance = el.scrollHeight - el.scrollTop - el.clientHeight; const atBottom = distance < 100; if (performance.now() <= userScrollUntil.current) followOutput.current = atBottom; else if (followOutput.current && distance > 1) followConversation(); setShowLatest(!atBottom && !followOutput.current); }}>
        <div ref={conversationContent} className="conversation-content">
        {conversation.length === 0 && !loading && <div className="empty-chat"><div className="orb"><Icon name="models" size={28} /></div><h2>{text.chat.emptyTitle}</h2><p>{text.chat.emptyBody}</p></div>}
        {conversation.map((message, index) => {
          const content = messageText(message.content);
          const messageKey = `${index}-${'timestamp' in message ? message.timestamp : content.slice(0, 24)}`;
          return <article className={`message ${message.role}`} key={messageKey}>
            <div className="message-heading"><div className="message-label">{message.role === 'user' ? text.chat.you : message.role === 'assistant' ? text.chat.assistant : message.role}</div></div>
            <div className="message-body">{message.role === 'assistant' ? <MarkdownMessage content={content} /> : content}</div>
            {message.role === 'assistant' && <footer className="message-actions">
              <button type="button" className="message-copy" onClick={() => void copyMessage(messageKey, content)} aria-label={copiedMessage === messageKey ? text.chat.copied : text.chat.copyResponse}><Icon name={copiedMessage === messageKey ? 'check' : 'copy'} size={14} />{copiedMessage === messageKey ? text.chat.copied : text.chat.copy}</button>
              <ReadAloudButton text={content} />
              {'job_id' in message && typeof message.job_id === 'string' && message.job_id !== linkedTurn?.job_id && <a className="secondary-button message-job-link" href={`#/agents/task/${encodeURIComponent(message.job_id)}`}>{jobCopy.inspect}</a>}
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
        {linkedTurn?.job_id && linkedRun && <ConversationWork key={linkedRun.run_id} run={linkedRun} refresh={refreshJob} connection={jobConnection} readError={jobReadError} outputSaved={linkedOutputSaved} recoveryNeeded={recoveryNeeded} recover={() => refreshLinkedConversation(linkedTurn, true)} liveSubmission={submittedJob.current === linkedRun.run_id} />}
        {linkedTurn && !linkedRun && <ScopedNotice kind={jobReadError ? 'error' : 'status'} action={<button className="secondary-button" onClick={() => { if (linkedTurn.job_id) refreshJob(); else void api.recoverAgentTurn(activeName, linkedTurn.id).then(result => setLinkedTurn(result.turn)).catch(reason => setError(reason instanceof Error ? reason.message : text.common.error)); }}>{linkedTurn.job_id ? text.common.refresh : jobCopy.attach}</button>}>{jobReadError || (linkedTurn.job_id ? text.common.loading : jobCopy.savedJob)}</ScopedNotice>}
        {((busy && !linkedRun) || streamed) && <article className="message assistant streaming-response"><div className="message-heading"><div className="message-label">{text.chat.assistant}</div>{streamed && <button type="button" className="message-copy" onClick={() => void copyMessage('partial', streamed)}>{copiedMessage === 'partial' ? text.chat.copied : text.chat.copy}</button>}</div>
          {streamed && <div className="message-body"><MarkdownMessage content={streamed} /></div>}
          <div className="generation-status" role="status">{interrupted ? text.chatStreaming.unsaved : text.chatStreaming[phase]}</div>
        </article>}
        {limited && <div className="generation-status" role="status">{text.chatStreaming.limitReached}</div>}
        {error && <ScopedNotice kind="error" action={errorAction && <button className="secondary-button" onClick={() => void (errorAction === 'cancel' ? stop() : loadSessions())}>{errorAction === 'cancel' ? text.chat.stop : errorAction === 'history' ? interaction[locale].refreshHistory : interaction[locale].checkRequest}</button>}>{error}</ScopedNotice>}
        {unsaved && <div className="inline-error" role="alert">{text.recovery.draftWarning}</div>}
        </div>
      </div>
      {showLatest && <button type="button" className="secondary-button chat-latest" onClick={() => { userScrollUntil.current = 0; followOutput.current = true; const pane = conversationPane.current; pane?.focus({ preventScroll: true }); setShowLatest(false); requestAnimationFrame(() => { pane?.scrollTo({ top: pane.scrollHeight, behavior: 'instant' }); }); }}>{refinement.latest}</button>}
      <div className="composer">
        <div className="composer-input"><textarea ref={composerInput} value={draft} onChange={event => setDraft(event.target.value)} onKeyDown={keyDown} placeholder={text.chat.placeholder} aria-label={text.chat.placeholder} rows={1} disabled={loading || checkingTurn} /><small>{text.chat.enterHint}</small></div>
        <div className="composer-context">
          <AgentSelector value={agent} onChange={value => { setAgent(value); setAgentResources({ profile: value ? `${value.id}:${value.revision}` : '', restrictions: {} }); }} disabled={busy} />
          {agent ? <><div className="chat-agent-model"><span>{text.chat.model}</span>{agent.settings.model}</div><AgentTools profile={agent} restrictions={restrictions} onChange={value => setAgentResources({ profile: agentKey, restrictions: value })} disabled={busy} />{agent.settings.computer && restrictions.computer !== false && <TaskAccessPolicy value={computerPolicy} onChange={setComputerPolicy} disabled={busy} scope="desktop" workspace={permissions.workspace} />}</> : <ModelSelect models={models} value={model} onChange={setModel} />}
          <UtilityPopover label={experience.context} trigger={<><Icon name="settings" size={18} />{!agent && knowledge && <span className="composer-choice">{experience.knowledgeOn}</span>}</>} triggerClassName={!agent && knowledge ? 'secondary-button' : 'icon-button'} className="composer-context-menu">
            {!agent && <label className="switch"><input type="checkbox" checked={knowledge} disabled={busy || (!knowledge && knowledgeState !== 'ready')} onChange={event => setKnowledge(event.target.checked)} /><span />{text.chat.knowledge}</label>}
            {!agent && permissions.knowledge && knowledgeState !== 'ready' && <div className="knowledge-readiness">
              {knowledgeState === 'checking' ? <span role="status">{feedback.checkingKnowledge}</span>
                : knowledgeState === 'failed' ? <><span role="status">{feedback.knowledgeCheckFailed}</span> <button className="text-button" onClick={() => void refreshKnowledge()}>{feedback.retryKnowledge}</button></>
                : <SetupLink destination="knowledge" draft={{ kind: 'chat', id: activeName }}>{feedback.configureKnowledge}</SetupLink>}
            </div>}
            <label className="field">{text.chatStreaming.profile}<select value={profile} disabled={busy} onChange={event => updatePreferences({ ...preferences, profile: event.target.value })}><option value="interactive">{text.chatStreaming.interactive}</option><option value="extended">{text.chatStreaming.extended}</option></select></label>
            <label className="field">{text.chatStreaming.responseLimit}<select value={maxTokens} disabled={busy} onChange={event => updatePreferences({ ...preferences, maxTokens: Number(event.target.value) })}>{[256, 1024, 4096].map(count => <option value={count} key={count}>{count.toLocaleString(locale)}</option>)}</select></label>
            <p>{text.chatStreaming.profileHint}</p>
          </UtilityPopover>
          <VoiceSettings compact><label className="switch"><input type="checkbox" checked={speakResponses} disabled={checkingSpeech || (busy && !speakResponses)} onChange={event => void toggleSpeech(event.target.checked)} /><span />{voiceCopy.speakResponses}</label></VoiceSettings>
        </div>
        {!agent && knowledge && knowledgeState !== 'ready' && <ScopedNotice action={<button className="secondary-button" onClick={() => void refreshKnowledge()}>{feedback.retryKnowledge}</button>}>{knowledgeState === 'checking' ? feedback.checkingKnowledge : knowledgeState === 'failed' ? feedback.knowledgeCheckFailed : interaction[locale].knowledgeUnavailable}</ScopedNotice>}
        {!agent && knowledge && <p className="composer-retrieval-scope">{workspaceManagement(locale).retrievalScope}</p>}
        <div className="composer-footer">
      <div className="composer-voice-toolbar chat-speech-controls">
        {speakResponses && <small>{voiceCopy.speakResponses}</small>}
        {speechState !== 'idle' && <button type="button" className="text-button" onClick={stopSpeech}>{voiceCopy.stopSpeaking}</button>}
        {speechState !== 'idle' && <small role="status">{speechState === 'waiting' ? voiceCopy.speechWaiting : speechState === 'preparing' ? voiceCopy.preparingHint : busy ? voiceCopy.provisionalSpeech : voiceCopy.speak}</small>}
        {speechError && <small role="alert">{speechError} <a className="secondary-button" href="#/models">{text.nav.models}</a></small>}
      </div>
      <div className="composer-actions"><VoiceInputButton contextKey={composerDraftKey} disabled={loading || checkingTurn || busy} onTranscript={value => setDraft(draft ? `${draft} ${value}` : value)} /><button aria-label={busy ? text.chat.stop : canSendInstruction ? jobCopy.update : text.chat.send} disabled={busy ? false : loading || checkingTurn || missingSession || !draft.trim() || !effectiveModel || (linkedUnsettled && !canSendInstruction)} onClick={busy ? () => void stop() : () => void send()}>{busy ? text.chat.stop : <><span>{canSendInstruction ? jobCopy.update : text.chat.send}</span><Icon name="send" size={18} /></>}</button></div>
        </div>
      </div>
    </div>
  </div>;
}
