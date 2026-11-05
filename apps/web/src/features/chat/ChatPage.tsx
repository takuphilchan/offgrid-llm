import { useWorkspace, useWorkspaceState } from '../../lib/workspace-context';
import { SidebarContext, SidebarSlot } from '../../components/shell/SidebarSlot';
import { interaction } from '../../i18n/interaction';
import { ChatFailureNotice } from './ChatFailureNotice';
import { documentExcerptCoverage } from '../../i18n/document-coverage';
import { chatAvailable } from '../../api/model-capabilities';
import { chatReadiness } from '../../i18n/chat-readiness';
import { ChatWorkBlocker } from './ChatWorkBlocker';
import { reviewConversationWork, submitComposerInput } from './chat-input';
import { APIError } from '../../api/client';
import { useWorkspaceRefresh } from '../../lib/workspace-refresh';
import { useContext, useDeferredValue, useEffect, useLayoutEffect, useRef, useState } from 'react';
import { workspaceRefinement } from '../../i18n/workspace-refinement';
import { api, type AgentProfile, type AgentResourceRestrictions, type ChatMessage, type ChatSession, type Model, type SessionTurn, type ExecutionWorkspace as ExecutionWorkspaceResource } from '../../api/client';
import { composerAttachmentLabel } from '../../i18n/composer';
import { Icon } from '../../components/Icon';
import { MarkdownMessage } from '../../components/MarkdownMessage';
import { AttachmentAnswer } from './AttachmentAnswer';
import { ModelSelect } from '../../components/ModelSelect';
import type { HistoryItem } from '../../components/HistoryDeleteDialog';
import { ChatHistoryDelete } from './ChatHistoryDelete';
import { useI18n } from '../../i18n';
import { clearSubmittedDraft, draftKey, readDraft, useDraft, writeDraft } from '../../lib/drafts';
import { SessionResponseError, type ChatMetrics, type ChatPhase } from '../../api/session-stream';
import { writePreference } from '../../lib/preferences';
import { ReadAloudButton, VoiceInputButton } from '../../components/VoiceInputButton';
import { ResponseSpeech, stopSpeechEvent } from '../../lib/response-speech';
import { voiceText } from '../../i18n/voice';
import { talkText } from '../../i18n/talk';
import { useTalkIdentity, useTalkMode } from './useTalkMode';
import { useChatSpeech } from './useChatSpeech';
import { ComposerSpeech } from './ComposerSpeech';
import { useVoicePreferences } from '../../lib/voice-preferences';
import { operationFeedback } from '../../i18n/operation-feedback';
import { ScopedNotice } from '../../components/WorkspacePresentation';
import { UtilityPopover } from '../../components/UtilityPopover';
import { useSetupHandoff } from '../../lib/setup-handoff';
import { useKnowledgeSources } from './useKnowledgeSources';
import { knowledgeScope } from '../../i18n/knowledge-scope';
import { KnowledgePassages } from './KnowledgePassages';
import {ImageAttachments,useComposerImages} from './ComposerImages';
import {FileAttachments,useComposerFiles,createFileConversation,clearCompletedFileDraft} from './ComposerFiles';
import {useChatCapabilities} from './useChatCapabilities';
import {ChatResponseDetails} from './ChatResponseDetails';
import type { SessionMessage } from '../../api/client';
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
import { ExecutionWorkspace } from './ExecutionWorkspace';
import { executionWorkspaceText } from '../../i18n/execution-workspace';
import { messageText, newSessionName, readPreferences, copyChatMessage } from './chat-presentation';
import { ConversationHistory } from './ConversationHistory';
import './composer-hierarchy.css';
import { ComposerKnowledgeReadiness, ComposerResponseLimits } from './ComposerOptions';
import { DocumentWorkspace } from './DocumentWorkspace';
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
  const talkCopy = talkText(locale);
  const feedback = operationFeedback(locale);
  const handoff = useSetupHandoff();
  const draftScope = useDraftScope();
  const work = useWorkActions();
  const { preferences: voicePreferences } = useVoicePreferences();
  const talk = useTalkMode();
  const { state: talkState, setState: setTalkState, cycle: talkCycle, active: talkActive, revision: talkRevision } = talk;
  const { speakResponses, speechState, setSpeechState, speechError, setSpeechError, checkingSpeech, speech, stopSpeech, toggleSpeech } = useChatSpeech(voicePreferences, locale);
  const [responseSettingsOpen, setResponseSettingsOpen] = useState(false);
  const sessionKey = draftKey(draftScope, 'active-chat');
  const preferencesKey = `offgrid.chat.preferences:${encodeURIComponent(scope)}`;
  const [sessions, setSessions] = useState<ChatSession[]>([]);
  const [activeName, setActiveName] = useState(() => handoff.returning?.kind === 'chat' ? handoff.returning.id : readDraft(sessionKey));
  const [missingSession, setMissingSession] = useState(false);
  useEffect(() => work.claimChat(activeName), [activeName, work]);
  useEffect(() => { if (handoff.returning?.kind === 'chat') handoff.consumed(); }, []);
  const [conversation, setConversation] = useState<(ChatMessage & { knowledge_sources?: SessionMessage['knowledge_sources']; images?: SessionMessage['images']; attachments?: SessionMessage['attachments'] })[]>([]);
  const { value: draft, setValue: setDraft, unsaved, key: composerDraftKey } = useDraft(draftScope, 'chat', activeName);
  const [knowledge, setKnowledge] = useWorkspaceState('chat.knowledge', false);
  const scopeCopy = knowledgeScope[locale];
  const {scopedReady,imageServiceReady,fileServiceReady,executionRequestable}=useChatCapabilities(scope);
  const [agent, setAgent] = useWorkspaceState<AgentProfile | null>('chat.agent', null);
  const [executionWorkspace, setExecutionWorkspace] = useWorkspaceState<ExecutionWorkspaceResource | null>('chat.executionWorkspace', null);
  const [executionReady, setExecutionReady] = useState(false);
  const [agentResources, setAgentResources] = useWorkspaceState<{ profile: string; restrictions: AgentResourceRestrictions }>('chat.agentResources', { profile: '', restrictions: {} });
  const [computerPolicy, setComputerPolicy] = useWorkspaceState<TaskComputerPolicy>('chat.agentPolicy', 'full_task');
  const agentKey = agent ? `${agent.id}:${agent.revision}` : '';
  const restrictions = agentResources.profile === agentKey ? agentResources.restrictions : {};
  const effectiveModel = agent?.settings.model ?? model;
  const unavailableModel = !agent && !!model && !models.some(item => item.id === model && chatAvailable(item));
  const [linkedTurn, setLinkedTurn] = useState<SessionTurn | null>(null);
  const [recoveryNeeded, setRecoveryNeeded] = useState(false);
  const { run: linkedRun, connection: jobConnection, error: jobReadError, refresh: refreshJob } = useTaskRun(linkedTurn?.job_id ?? '');
  useEffect(()=>{
    if(linkedRun?.execution_workspace&&linkedRun.run_id===linkedTurn?.job_id)setExecutionWorkspace(linkedRun.execution_workspace);
  },[linkedRun?.execution?.session,linkedTurn?.job_id]);
  const currentLinked = useRef<SessionTurn | null>(null); currentLinked.current = linkedTurn;
  const linkedUnsettled = !!linkedTurn && !workSettled(linkedRun?.status ?? linkedTurn.status);
  const canSendInstruction = linkedUnsettled && linkedRun?.can_steer === true;
  const workBlocked = linkedUnsettled && !canSendInstruction;
  const jobCopy = conversationWorkText[locale];
  const spokenJob = useRef('');
  const submittedJob = useRef('');
  const [loading, setLoading] = useState(true);
  const [busy, setBusy] = useState(false);
  const [checkingTurn, setCheckingTurn] = useState(false);
  const imageDraft=useComposerImages(draftScope,activeName,loading||busy||checkingTurn,{service:imageServiceReady,agent:!!agent,model:models.find(candidate=>candidate.id===effectiveModel),history:conversation.some(message=>!!message.images?.length)});
  const [phase, setPhase] = useState<ChatPhase>('queued');
  const [streamed, setStreamed] = useState('');
  const displayedStream = useDeferredValue(streamed);
  const [interrupted, setInterrupted] = useState(false);
  const [metrics, setMetrics] = useState<ChatMetrics>();
  const [limited, setLimited] = useState(false);
  const [preferences, setPreferences] = useState(() => readPreferences(preferencesKey));
  const { profile, maxTokens } = preferences;
  const updatePreferences = (next: typeof preferences) => {
    setPreferences(next);
    writePreference(preferencesKey, JSON.stringify(next));
  };
  const [error, setErrorText] = useState('');
  const [errorCode, setErrorCode] = useState('');
  const setError = (message: string, code = '') => { setErrorText(message); setErrorCode(code); };
  const [errorAction, setErrorAction] = useState<'history' | 'request' | 'cancel' | null>('history');
  const [deleteItems, setDeleteItems] = useState<HistoryItem[] | null>(null);
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
  const { sources: selectedSources, transfer: transferSources, clear: clearSources } = useKnowledgeSources(activeName, loading, permissions.knowledge, setKnowledge);
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
        if (savedDraft.trim() === turn.prompt) {clearSubmittedDraft(key, savedDraft);const user=session.messages.filter(message=>message.role==='user').at(-1);if(user?.images?.length)clearSubmittedDraft(draftKey(draftScope,'chat-images',session.name),JSON.stringify(user.images));clearCompletedFileDraft(draftScope,session.name,user?.attachments);}
        setMetrics(turn.metrics); setLimited(turn.finish_reason === 'length');
      } else { setConversation([...session.messages, { role: 'user', content: turn.prompt,images:turn.images,attachments:turn.attachments }]); setStreamed(turn.output); setInterrupted(true); setError(turn.error || text.chatStreaming.unsaved, turn.error_code); }
      return;
    }
    const watch = new AbortController(); controller.current = watch;
    setBusy(true); setConversation([...session.messages, { role: 'user', content: turn.prompt,images:turn.images,attachments:turn.attachments }]);
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
      if (savedDraft.trim() === turn.prompt) {clearSubmittedDraft(key, savedDraft);if(turn.images?.length)clearSubmittedDraft(draftKey(draftScope,'chat-images',session.name),JSON.stringify(turn.images));clearCompletedFileDraft(draftScope,session.name,turn.attachments);}
      setSessions(current => [result.session, ...current.filter(item => item.name !== session.name)]);
    } catch (reason) {
      if (mounted.current && revision === selectionRevision.current) {
        setStreamed(partial || turn.output); setInterrupted(true); setErrorAction('request'); setError(reason instanceof Error ? reason.message : text.common.error, reason instanceof SessionResponseError ? reason.code : '');
        try { const { turn: stored } = await api.currentTurn(session.name); if (mounted.current && revision === selectionRevision.current && stored?.id === turn.id) work.rememberChat(session.name, stored.status); } catch { /* Unknown transport outcome still requires inspection. */ }
      }
    } finally { if (controller.current === watch) { controller.current = null; if (mounted.current) setBusy(false); } }
  };
  const activate = (session?: ChatSession) => {
    talk.reset();
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
    if (session) void Promise.all([
      api.currentTurn(session.name),
      // A detached turn may have saved new messages since history was loaded.
      work.hasChat(session.name) ? api.session(session.name) : Promise.resolve(session),
    ]).then(([{ turn }, latest]) => {
      if (!mounted.current || revision !== selectionRevision.current) return;
      setConversation(latest.messages);
      if (turn?.id) void recoverTurn(latest, turn, revision);
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
    if (deleteItems) return;
    activate(undefined);
    setError('');
    setCopiedMessage('');
  };
  const historyDeleted = (names: string[]) => {
    setSessions(current => current.filter(item => !names.includes(item.name)));
    names.forEach(name => { writeDraft(draftKey(draftScope, 'chat', name), ''); writeDraft(draftKey(draftScope,'chat-images',name),''); writeDraft(draftKey(draftScope,'chat-files',name),''); work.forgetChat(name); });
    if (names.includes(activeName)) activate(undefined);
    if (names.length) setHistoryNotice(text.history.removed.replace('{count}', String(names.length)));
  };
  const reviewLinkedWork = () => reviewConversationWork(conversationContent.current, followOutput, refreshJob);
  const fileDraft=useComposerFiles(draftScope,activeName,loading||busy||checkingTurn||missingSession,fileServiceReady,!!agent,async select=>{
    if(activeName)return activeName;
    const revision=selectionRevision.current;
    return createFileConversation({scope:draftScope,prompt:draft.trim(),model:effectiveModel,draft:composerDraftKey,images:imageDraft,valid:()=>mounted.current&&revision===selectionRevision.current,accept:created=>{select(created.name);setActiveName(created.name);transferSources(created.name);writeDraft(sessionKey,created.name);setSessions(current=>[created,...current]);}});
  });
  const send = async (voicePrompt?: string, fromTalk = false): Promise<boolean> => {
    setErrorAction('request');
    if (!fromTalk && talkActive.current) { talkActive.current = false; talkRevision.current++; setTalkState('off'); }
    const prompt = (voicePrompt ?? draft).trim();
    if (fromTalk && !talkActive.current) return false;
    if ((!prompt&&!imageDraft.images.length&&!fileDraft.files.length) || !effectiveModel || unavailableModel || busy || checkingTurn || missingSession || controller.current || deleteItems || imageDraft.processing || fileDraft.processing) return false;
    if(fileDraft.blocker){setError(fileDraft.blocker);return false;}
    if(imageDraft.blocker){setError(imageDraft.blocker,imageDraft.blockerCode);return false;}
    if (selectedSources.length && (!scopedReady || linkedUnsettled)) { setError(linkedUnsettled ? scopeCopy.active : scopeCopy.unsupported); return false; }
    if (linkedUnsettled) {
      if (!linkedRun || !canSendInstruction) { reviewLinkedWork(); return false; }
      setBusy(true); setError('');
      const submitted = draft, key = draftKey(draftScope, 'chat-agent-instruction', linkedRun.run_id);
      const revision = selectionRevision.current;
      const sameView = () => mounted.current && revision === selectionRevision.current;
      let intent: { request_id: string; instruction: string } | undefined;
      try { intent = JSON.parse(readDraft(key)); } catch { /* No unacknowledged instruction. */ }
      if (!intent || intent.instruction !== prompt) intent = { request_id: crypto.randomUUID(), instruction: prompt };
      writeDraft(key, JSON.stringify(intent));
      try {
        await api.jobAction(linkedRun.run_id, 'steer', intent);
        clearSubmittedDraft(key, JSON.stringify(intent)); clearSubmittedDraft(composerDraftKey, submitted);
        if (sameView()) refreshJob();
      } catch (reason) { if (sameView()) { setError(reason instanceof Error ? reason.message : text.common.error); refreshJob(); } }
      finally { if (sameView()) setBusy(false); }
      return false;
    }
    if (agent && executionWorkspace && !executionReady) { setError(executionWorkspaceText[locale].unavailable); return false; }
    const revision = selectionRevision.current;
    const currentView = () => mounted.current && revision === selectionRevision.current;
    const requestController = new AbortController();
    const requestID = crypto.randomUUID();
    window.dispatchEvent(new Event(stopSpeechEvent));
    setSpeechError('');
    const talkTurnRevision = talkRevision.current;
    if ((speakResponses || fromTalk) && !agent) speech.current = new ResponseSpeech(state => {
      setSpeechState(state);
      if (fromTalk && talkActive.current && talkRevision.current === talkTurnRevision && (state === 'preparing' || state === 'playing')) setTalkState('speaking');
    }, message => {
      setSpeechError(message);
      if (fromTalk && talkActive.current && talkRevision.current === talkTurnRevision) { talkActive.current = false; setTalkState('needs_attention'); }
    }, voicePreferences, locale, fromTalk ? () => {
      window.setTimeout(() => {
        if (talkActive.current && talkRevision.current === talkTurnRevision && mounted.current) { setTalkState('listening'); talk.nextCycle(); }
      }, 0);
    } : undefined);
    const responseSpeech = speech.current;
    controller.current = requestController;
    const submitted = voicePrompt ?? draft;
    const submittedImages=imageDraft.value;
    const images=imageDraft.images;
    const submittedFiles=fileDraft.value,attachments=fileDraft.references;
    setBusy(true);
    setPhase('queued'); setStreamed(''); setInterrupted(false); setMetrics(undefined); setLimited(false);
    followOutput.current = true;
    setError('');
    let sessionName = activeName;
    let partial = '';
    let flush: ReturnType<typeof setTimeout> | undefined;
    try {
      if (!sessionName) {
        const created = await api.createSession(newSessionName(prompt||images[0]?.name||imageDraft.copy.images), effectiveModel, requestController.signal);
        if (!currentView()) return false;
        sessionName = created.name;
        writeDraft(draftKey(draftScope, 'chat', sessionName), readDraft(composerDraftKey));
        writeDraft(composerDraftKey, '');
        writeDraft(draftKey(draftScope,'chat-images',sessionName),submittedImages);
        clearSubmittedDraft(imageDraft.key,submittedImages);
        setActiveName(sessionName);
        transferSources(sessionName);
        writeDraft(sessionKey, sessionName);
        setSessions(current => [created, ...current]);
      }
      if (agent) {
        const intentKey = draftKey(draftScope, 'chat-agent-request', sessionName);
        const payload = { content: prompt, model_id: agent.settings.model, profile: profile === 'extended' ? 'extended' as const : 'interactive' as const, max_tokens: maxTokens, agent: { id: agent.id, revision: agent.revision }, restrictions, ...(selectedSources.length ? { use_knowledge_base: true, knowledge_documents: selectedSources.map(source => source.id) } : {}), ...(executionWorkspace ? {execution: executionWorkspace.reference} : {}), ...(executionRequestable ? {execution_request_version: 1 as const} : {}), ...(agent.settings.computer && restrictions.computer !== false ? { computer_scope: 'desktop' as const, computer_approval_mode: computerPolicy } : {}) };
        let intent: { request_id: string; payload: typeof payload } | undefined;
        try { intent = JSON.parse(readDraft(intentKey)); } catch { /* No pending admission. */ }
        if (!intent || JSON.stringify(intent.payload) !== JSON.stringify(payload)) intent = { request_id: requestID, payload };
        writeDraft(intentKey, JSON.stringify(intent));
        activeTurn.current = { name: sessionName, id: intent.request_id };
        work.rememberChat(sessionName, 'pending');
        const accepted = await api.submitAgentTurn(sessionName, { ...intent.payload, request_id: intent.request_id }, requestController.signal);
        if (!currentView()) return false;
        clearSubmittedDraft(intentKey, JSON.stringify(intent));
        clearSubmittedDraft(draftKey(draftScope, 'chat', sessionName), submitted);
        setConversation(current => [...current, { role: 'user', content: prompt }]);
        setLinkedTurn(accepted.turn); setRecoveryNeeded(false); setStreamed('');
        submittedJob.current = accepted.job.run_id;
        spokenJob.current = accepted.job.run_id;
        return false;
      }
      setLinkedTurn(null); setRecoveryNeeded(false);
      setConversation(current => [...current, { role: 'user', content: prompt,images,attachments }]);
      activeTurn.current = { name: sessionName, id: requestID };
      work.rememberChat(sessionName, 'pending');
      const result = await api.streamSession(sessionName, prompt, model, knowledge || selectedSources.length > 0, profile, maxTokens, event => {
        if (!currentView()) return;
        if (event.type === 'phase') setPhase(event.phase);
        else {
          partial += event.delta;
          // Talk reads only the committed answer; provisional stream text can
          // still change or fail and must not become a spoken final answer.
          if (!fromTalk) responseSpeech?.append(event.delta);
          // Bound React/Markdown updates during fast GPU decoding.
          if (!flush) flush = setTimeout(() => { if (currentView()) setStreamed(partial); flush = undefined; }, 40);
        }
      }, requestController.signal, requestID, selectedSources.length ? selectedSources.map(source => source.id) : undefined, images.length?images:undefined, attachments.length?attachments:undefined);
      clearTimeout(flush); flush = undefined;
      if (!currentView()) return false;
      setStreamed(''); setMetrics(result.metrics); setLimited(result.finish_reason === 'length');
      clearSubmittedDraft(draftKey(draftScope, 'chat', sessionName), submitted);
      clearSubmittedDraft(draftKey(draftScope,'chat-images',sessionName),submittedImages);
      clearSubmittedDraft(draftKey(draftScope,'chat-files',sessionName),submittedFiles);
      setConversation(result.session.messages);
      work.rememberChat(sessionName, 'completed');
      responseSpeech?.finish(messageText(result.message.content));
      setSessions(current => [result.session, ...current.filter(item => item.name !== result.session.name)]);
      if (onboardingPending && result.message.role === 'assistant' && messageText(result.message.content).trim()) onFirstResponse();
      return true;
    } catch (reason) {
      responseSpeech?.stop();
      if (fromTalk && talkRevision.current === talkTurnRevision) { talkActive.current = false; setTalkState('needs_attention'); }
      clearTimeout(flush); flush = undefined;
      if (!currentView()) return false;
      setStreamed(partial); setInterrupted(true);
      setError(requestController.signal.aborted ? text.chatStreaming.cancelled : reason instanceof Error ? reason.message : text.common.error, reason instanceof SessionResponseError ? reason.code : reason instanceof APIError ? reason.data?.error?.code : '');
      if (sessionName) {
        try {
          const stored = await api.session(sessionName);
          if (!currentView()) return false;
          setConversation(stored.messages);
          if (stored.turn) work.rememberChat(sessionName, stored.turn.status);
          setSessions(current => [stored, ...current.filter(item => item.name !== stored.name)]);
        } catch { /* The primary error is more useful. */ }
      }
      return false;
    } finally {
      clearTimeout(flush);
      if (currentView()) setBusy(false);
      if (controller.current === requestController) controller.current = null;
    }
  };
  const stop = async () => {
    stopSpeech();
    setErrorAction('cancel');
    const target = activeTurn.current, watch = controller.current, revision = selectionRevision.current;
    if (!target) { watch?.abort(); return; }
    try { await api.cancelTurn(target.name, target.id); watch?.abort(); }
    catch (reason) { if (mounted.current && revision === selectionRevision.current) setError(reason instanceof Error ? reason.message : text.common.error); }
  };
  const stopTalk = (cancelTurn = false) => {
    talk.reset();
    stopSpeech();
    if (cancelTurn && controller.current) void stop();
  };
  const startTalk = () => talk.start({ eligible: !agent && !!effectiveModel && !unavailableModel && !busy && !loading && !checkingTurn && !linkedUnsettled && !missingSession, hasDraft: !!(draft.trim() || imageDraft.images.length || fileDraft.files.length), preferences: voicePreferences, locale, unavailable: voiceCopy.speechUnavailable, draftError: talkCopy.draft, setError: setSpeechError });
  const onTalkTranscript = (value: string) => talk.acceptTranscript(value, composerDraftKey, transcript => send(transcript, true));
  const speakAgain = () => talk.speakAgain(busy, stopSpeech);
  useTalkIdentity(`${effectiveModel}:${agentKey}`, () => { if (talkActive.current || talkState === 'checking') stopTalk(true); });
  const copyMessage = async (key: string, content: string) => {
    setErrorAction(null);
    await copyChatMessage(key,content,setCopiedMessage,setError,text.common.error);
  };
  // Metrics describe the just-committed turn, never an older answer or a new
  // in-flight response. They are cleared when starting/selecting a conversation.
  const lastAssistant = conversation.reduce((last, message, index) => message.role === 'assistant' ? index : last, -1);
  const linkedOutputSaved = !!linkedTurn && conversation.some(message => 'turn_id' in message && message.turn_id === linkedTurn.id && message.role === 'assistant');
  return <DocumentWorkspace scope={`${draftScope}:${activeName}`}><div className="chat-workspace">
    {deleteItems && <ChatHistoryDelete items={deleteItems} onDeleted={historyDeleted} onClose={() => setDeleteItems(null)} />}
    <SidebarSlot action><button className="new-chat-button" title={text.chat.newChat} aria-label={text.chat.newChat} disabled={!!deleteItems} onClick={() => { sidebar.close(); startNew(); }}><Icon name="plus" size={18} /><span>{text.chat.newChat}</span></button></SidebarSlot>
    <SidebarSlot><ConversationHistory sessions={sessions} activeName={activeName} busy={busy} loading={loading} locked={!!deleteItems} notice={historyNotice} select={activate} remove={setDeleteItems} refresh={() => void loadSessions()} /></SidebarSlot>
    <div className="chat-layout">
      {!agent && (!model || unavailableModel || onboardingPending) && <div className="setup-inline" role="status"><span>{unavailableModel ? chatReadiness[locale].unavailable : model ? text.onboarding.firstReplyHint : chatReadiness[locale].setup}</span>{(!model || unavailableModel) && <button className="secondary-button" onClick={() => { handoff.begin({ origin: '#/chat', destination: 'models', draft: { kind: 'chat', id: activeName } }); onOpenModels(); }}>{text.onboarding.chooseModel}</button>}</div>}
<div ref={conversationPane} className="conversation" tabIndex={0} onWheel={event => { userScrollUntil.current = performance.now() + 1200; if (event.deltaY < 0) followOutput.current = false; }} onTouchStart={() => { userScrollUntil.current = performance.now() + 1200; }} onKeyDown={event => { if (['ArrowUp','ArrowDown','PageUp','PageDown','Home','End',' '].includes(event.key)) userScrollUntil.current = performance.now() + 1200; }} onScroll={event => { const el = event.currentTarget; const distance = el.scrollHeight - el.scrollTop - el.clientHeight; const atBottom = distance < 100; if (performance.now() <= userScrollUntil.current) followOutput.current = atBottom; else if (followOutput.current && distance > 1) followConversation(); setShowLatest(!atBottom && !followOutput.current); }}>
        <div ref={conversationContent} className="conversation-content">
        {conversation.length === 0 && !loading && !error && <div className="empty-chat"><div className="orb"><Icon name="models" size={28} /></div><h2>{text.chat.emptyTitle}</h2><p>{text.chat.emptyBody}</p></div>}
        {conversation.map((message, index) => {
          const content = messageText(message.content);
          const messageKey = `${index}-${'timestamp' in message ? message.timestamp : content.slice(0, 24)}`;
          return <article className={`message ${message.role}`} key={messageKey}>
            <div className="message-heading"><div className="message-label">{message.role === 'user' ? text.chat.you : message.role === 'assistant' ? text.chat.assistant : message.role}</div></div>
            <div className="message-body">{message.role === 'assistant' ? 'attachment_selections' in message && Array.isArray(message.attachment_selections) && message.attachment_selections.length ? <AttachmentAnswer content={content} session={activeName} selections={message.attachment_selections} /> : <MarkdownMessage content={content} /> : content}</div>
            <ImageAttachments images={message.images}/>
            <FileAttachments name={activeName} references={message.attachments} active={busy && index === conversation.length-1}/>
            {message.role === 'assistant' && <KnowledgePassages passages={message.knowledge_sources} />}
            {message.role === 'assistant' && 'attachment_selections' in message && Array.isArray(message.attachment_selections) && message.attachment_selections.some(source => source.mode === 'excerpts') && <p className="document-answer-coverage">{documentExcerptCoverage(locale)}</p>}
            {message.role === 'assistant' && <footer className="message-actions">
              <button type="button" className="message-copy" onClick={() => void copyMessage(messageKey, content)} aria-label={copiedMessage === messageKey ? text.chat.copied : text.chat.copyResponse}><Icon name={copiedMessage === messageKey ? 'check' : 'copy'} size={14} />{copiedMessage === messageKey ? text.chat.copied : text.chat.copy}</button>
              <ReadAloudButton text={content} />
              {'job_id' in message && typeof message.job_id === 'string' && message.job_id !== linkedTurn?.job_id && <a className="secondary-button message-job-link" href={`#/agents/task/${encodeURIComponent(message.job_id)}`}>{jobCopy.inspect}</a>}
              {metrics && index === lastAssistant && <ChatResponseDetails metrics={metrics}/>}
            </footer>}
          </article>;
        })}
        {linkedTurn?.job_id && linkedRun && <ConversationWork key={linkedRun.run_id} run={linkedRun} refresh={refreshJob} connection={jobConnection} readError={jobReadError} outputSaved={linkedOutputSaved} recoveryNeeded={recoveryNeeded} recover={() => refreshLinkedConversation(linkedTurn, true)} liveSubmission={submittedJob.current === linkedRun.run_id} />}
        {linkedTurn && !linkedRun && <ScopedNotice kind={jobReadError ? 'error' : 'status'} action={<button className="secondary-button" onClick={() => { if (linkedTurn.job_id) refreshJob(); else void api.recoverAgentTurn(activeName, linkedTurn.id).then(result => setLinkedTurn(result.turn)).catch(reason => setError(reason instanceof Error ? reason.message : text.common.error)); }}>{linkedTurn.job_id ? text.common.refresh : jobCopy.attach}</button>}>{jobReadError || (linkedTurn.job_id ? text.common.loading : jobCopy.savedJob)}</ScopedNotice>}
        {((busy && !linkedRun) || streamed) && <article className="message assistant streaming-response"><div className="message-heading"><div className="message-label">{text.chat.assistant}</div>{streamed && <button type="button" className="message-copy" onClick={() => void copyMessage('partial', streamed)}>{copiedMessage === 'partial' ? text.chat.copied : text.chat.copy}</button>}</div>
          {streamed && <div className="message-body"><MarkdownMessage content={displayedStream} streaming /></div>}
          <div className="generation-status" role="status">{interrupted ? text.chatStreaming.unsaved : text.chatStreaming[phase]}{!interrupted && phase === 'loading' && <p>{text.guidance.coldStart}</p>}</div>
        </article>}
        {limited && <div className="generation-status" role="status">{text.chatStreaming.limitReached}</div>}
        {unsaved && <div className="inline-error" role="alert">{text.recovery.draftWarning}</div>}
        </div>
      </div>
      <div className="chat-recovery">{error && <ChatFailureNotice error={error} code={errorCode} openSettings={() => setResponseSettingsOpen(true)} chooseModel={() => { handoff.begin({ origin: '#/chat', destination: 'models', draft: { kind: 'chat', id: activeName } }); onOpenModels(); }} action={errorAction && <button className="secondary-button" onClick={() => void (errorAction === 'cancel' ? stop() : loadSessions())}>{errorAction === 'cancel' ? text.chat.stop : errorAction === 'history' ? interaction[locale].refreshHistory : interaction[locale].checkRequest}</button>} />}</div>
      {showLatest && <button type="button" className="secondary-button chat-latest" onClick={() => { userScrollUntil.current = 0; followOutput.current = true; const pane = conversationPane.current; pane?.focus({ preventScroll: true }); setShowLatest(false); requestAnimationFrame(() => { pane?.scrollTo({ top: pane.scrollHeight, behavior: 'instant' }); }); }}>{refinement.latest}</button>}
      {workBlocked && <ChatWorkBlocker run={linkedRun} review={reviewLinkedWork} refresh={refreshJob} startNew={startNew} />}
      <div className="composer composer-compact" onPaste={imageDraft.paste} onDragOver={imageDraft.dragOver} onDrop={event=>{const files=[...event.dataTransfer.files];if(fileServiceReady&&files.some(file=>!file.type.startsWith('image/'))){event.preventDefault();void fileDraft.add(files);}else imageDraft.drop(event);}}>
        <div className="composer-input">{fileDraft.preview}{imageDraft.preview}<textarea ref={composerInput} value={draft} onChange={event => { if (talkActive.current) stopTalk(false); setDraft(event.target.value); }} onKeyDown={event => submitComposerInput(event, async () => { await send(); })} placeholder={text.chat.placeholder} aria-label={text.chat.placeholder} rows={1} disabled={loading || checkingTurn} /><small className="sr-only">{text.chat.enterHint}</small>{fileDraft.recovery}{imageDraft.blocker&&<p className="image-input-notice" role="status">{imageDraft.blocker}{imageDraft.blockerCode==='vision_projector_unavailable'&&<button type="button" className="text-button" onClick={()=>{handoff.begin({origin:'#/chat',destination:'models',draft:{kind:'chat',id:activeName}});onOpenModels();}}>{text.onboarding.chooseModel}</button>}</p>}</div>
        {!agent && knowledge && knowledgeState !== 'ready' && <ScopedNotice action={<button className="secondary-button" onClick={() => void refreshKnowledge()}>{feedback.retryKnowledge}</button>}>{knowledgeState === 'checking' ? feedback.checkingKnowledge : knowledgeState === 'failed' ? feedback.knowledgeCheckFailed : interaction[locale].knowledgeUnavailable}</ScopedNotice>}
        {selectedSources.length > 0 ? <div className="composer-source-scope"><strong>{scopeCopy.selected}</strong><span>{selectedSources.map(source => source.name).join(' · ')}</span><button type="button" className="icon-button" disabled={busy} onClick={clearSources} aria-label={scopeCopy.clear} title={scopeCopy.clear}><Icon name="close" size={16} /></button>{!scopedReady && <p role="status">{scopeCopy.unsupported}</p>}</div> : !agent && knowledge && <p className="composer-retrieval-scope">{workspaceManagement(locale).retrievalScope}</p>}
        <div className="composer-footer">
          <div className="composer-utilities">
          <UtilityPopover label={composerAttachmentLabel(locale)} trigger={<Icon name="plus" size={20}/>} triggerClassName="icon-button" className="composer-attachments-menu">{fileDraft.control}{imageDraft.control}{fileDraft.saved}</UtilityPopover>
          <UtilityPopover label={text.chatStreaming.options} trigger={<Icon name="sliders" size={18} />} triggerClassName="icon-button" className="composer-context-menu" defaultOpen={responseSettingsOpen} onOpenChange={setResponseSettingsOpen}>
        <div className="composer-context">
          <AgentSelector value={agent} onChange={value => { setAgent(value); setAgentResources({ profile: value ? `${value.id}:${value.revision}` : '', restrictions: {} }); }} disabled={busy} />
          {agent ? <><AgentTools profile={agent} restrictions={restrictions} onChange={value => setAgentResources({ profile: agentKey, restrictions: value })} disabled={busy} /></> : <ModelSelect models={models} value={model} onChange={setModel} />}
        </div>
            {!agent && <label className="switch"><input type="checkbox" checked={knowledge || selectedSources.length > 0} disabled={busy || selectedSources.length > 0 || (!knowledge && knowledgeState !== 'ready')} onChange={event => setKnowledge(event.target.checked)} /><span />{text.chat.knowledge}</label>}
            {!agent && permissions.knowledge && knowledgeState !== 'ready' && <ComposerKnowledgeReadiness state={knowledgeState} conversation={activeName} refresh={refreshKnowledge} />}
            <ComposerResponseLimits preferences={preferences} busy={busy} update={updatePreferences} />
            {responseSettingsOpen && <ComposerSpeech presentation="settings" enabled={speakResponses} checking={checkingSpeech} busy={busy} state={speechState} error={speechError} toggle={toggleSpeech} stop={stopSpeech} />}
          </UtilityPopover>
          </div>
      <div className="composer-actions">
        {talkState === 'listening' || talkState === 'transcribing' ? <VoiceInputButton key={`talk-${talkCycle}`} compact autoStart contextKey={composerDraftKey} disabled={loading || checkingTurn || busy} onTranscript={onTalkTranscript} onFailure={message => talk.fail(message, setSpeechError)} onPhaseChange={talk.capturePhase} /> : talkState === 'off' || talkState === 'needs_attention' ? <VoiceInputButton compact contextKey={composerDraftKey} disabled={loading || checkingTurn || busy} onTranscript={value => { const previous = readDraft(composerDraftKey); setDraft(previous ? `${previous} ${value}` : value); }} /> : null}
        {!agent && <span className="talk-controls"><button type="button" className="talk-control" aria-pressed={talkState !== 'off' && talkState !== 'needs_attention'} disabled={talkState === 'off' && (loading || checkingTurn || busy || !effectiveModel || unavailableModel || linkedUnsettled)} onClick={talkState === 'off' || talkState === 'needs_attention' ? () => void startTalk() : () => stopTalk(true)}>{talkState === 'off' || talkState === 'needs_attention' ? talkCopy.start : talkCopy.stop}</button>{talkState === 'speaking' && !busy && <button type="button" className="talk-control" onClick={speakAgain}>{talkCopy.again}</button>}</span>}
        <button aria-label={busy ? text.chat.stop : canSendInstruction ? jobCopy.update : text.chat.send} disabled={busy ? false : loading || checkingTurn || missingSession || (!draft.trim()&&!imageDraft.images.length&&!fileDraft.files.length) || fileDraft.processing || !!fileDraft.blocker || imageDraft.processing || !!imageDraft.blocker || !effectiveModel || unavailableModel || (linkedUnsettled && !canSendInstruction)} onClick={busy ? () => void stop() : () => void send()}>{busy ? <Icon name="close" size={18} /> : <><span className="sr-only">{canSendInstruction ? jobCopy.update : text.chat.send}</span><Icon name="send" size={18} /></>}</button>
      </div>
        </div>
        <ComposerSpeech presentation="status" enabled={speakResponses} checking={checkingSpeech} busy={busy} state={speechState} error={speechError} toggle={toggleSpeech} stop={stopSpeech} />
        {talkState !== 'off' && <div className="talk-status" role="status">{talkState === 'checking' ? talkCopy.checking : talkState === 'listening' ? talkCopy.listening : talkState === 'transcribing' ? talkCopy.transcribing : talkState === 'thinking' ? talkCopy.thinking : talkState === 'speaking' ? talkCopy.speaking : talkCopy.attention}</div>}
        {agent && <div className="composer-active-context"><span className="composer-active-agent"><Icon name="agents" size={15}/>{agent.settings.name}</span>
          {(!executionRequestable || executionWorkspace) && <ExecutionWorkspace value={executionWorkspace} onChange={setExecutionWorkspace} disabled={busy || linkedUnsettled} onReady={setExecutionReady} ceiling={restrictions.execution??agent.settings.execution} />}
          {agent.settings.computer && restrictions.computer !== false && <TaskAccessPolicy value={computerPolicy} onChange={setComputerPolicy} disabled={busy} scope="desktop" workspace={permissions.workspace} />}
        </div>}
      </div>
    </div>
  </div></DocumentWorkspace>;
}
