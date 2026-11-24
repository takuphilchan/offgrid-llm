import { ConversationMessages, type ConversationMessage } from './ConversationMessages';
import { useConversationScroll } from './useConversationScroll';
import { useCommandWindowLayout } from './useCommandWindowLayout';
import { useMessageCopy } from './useMessageCopy';
import { WorkHome } from './WorkHome';
import { useWorkspace, useWorkspaceState } from '../../lib/workspace-context';
import { SidebarContext, SidebarSlot } from '../../components/shell/SidebarSlot';
import { interaction } from '../../i18n/interaction';
import { ChatFailureNotice } from './ChatFailureNotice';
import { useConversationConfiguration } from './agent-configuration';
import { canonicalSpeechSource, restoreCanonicalConfiguration, useCanonicalConversation } from './useCanonicalConversation';
import { CanonicalInputActivity, ConversationInputMode, pendingCanonicalInputs } from './CanonicalInputActivity';
import { ConversationSettings } from './ConversationSettings';
import { conversationInputText } from '../../i18n/conversation-input';
import { chatAvailable } from '../../api/model-capabilities';
import { chatReadiness } from '../../i18n/chat-readiness';
import { ChatWorkBlocker } from './ChatWorkBlocker';
import { inspectConversationWork, reviewConversationWork, submitComposerInput } from './chat-input';
import { useWorkspaceRefresh } from '../../lib/workspace-refresh';
import { useContext, useDeferredValue, useEffect, useLayoutEffect, useRef, useState } from 'react';
import { useComposerSizing } from './useComposerSizing';
import { workspaceRefinement } from '../../i18n/workspace-refinement';
import { api, APIError, type ChatMessage, type ChatSession, type SessionTurn } from '../../api/client';
import { composerAttachmentLabel } from '../../i18n/composer';
import { Icon } from '../../components/Icon';
import { MarkdownMessage } from '../../components/MarkdownMessage';
import { ModelSelect } from '../../components/ModelSelect';
import { compactModelName } from '../models/model-display';
import type { HistoryItem } from '../../components/HistoryDeleteDialog';
import { ChatHistoryDelete } from './ChatHistoryDelete';
import { useI18n } from '../../i18n';
import { clearSubmittedDraft, draftKey, readDraft, useDraft, writeDraft } from '../../lib/drafts';
import { SessionResponseError, type ChatMetrics, type ChatPhase } from '../../api/session-stream';
import {useResponsePreferences} from './useResponsePreferences';
import {CanonicalResponsePreview} from './CanonicalResponsePreview';
import { VoiceInputButton } from '../../components/VoiceInputButton';
import { stopSpeechEvent } from '../../lib/response-speech';
import { voiceText } from '../../i18n/voice';
import { talkText } from '../../i18n/talk';
import { useTalkDismiss, useTalkIdentity, useTalkMode } from './useTalkMode';
import { useTalkCapture } from './useTalkCapture';
import {TalkSurface} from './TalkSurface';
import { useChatSpeech } from './useChatSpeech';
import { useKnowledgeReadiness } from './useKnowledgeReadiness';
import { ComposerSpeech } from './ComposerSpeech';
import { useVoicePreferences } from '../../lib/voice-preferences';
import { operationFeedback } from '../../i18n/operation-feedback';
import { ScopedNotice } from '../../components/WorkspacePresentation';
import { UtilityPopover } from '../../components/UtilityPopover';
import { useSetupHandoff } from '../../lib/setup-handoff';
import { useKnowledgeSources } from './useKnowledgeSources';
import { knowledgeScope } from '../../i18n/knowledge-scope';
import {useComposerImages} from './ComposerImages';
import {useComposerFiles,createFileConversation,clearCompletedFileDraft} from './ComposerFiles';
import {useChatCapabilities} from './useChatCapabilities';
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
import {useConversationFolder} from './useConversationFolder';
import { executionWorkspaceText } from '../../i18n/execution-workspace';
import { messageText, newSessionName, conversationOutputState, type ChatPageProps } from './chat-presentation';
import { ConversationHistory } from './ConversationHistory';
import './composer-hierarchy.css';
import { ComposerKnowledgeReadiness } from './ComposerOptions';
import { DocumentWorkspace } from './DocumentWorkspace';
import {knowledgeUnitLabels} from '../../i18n/knowledge-locators';
import {workSurface} from '../../i18n/work-surface';
import { TerminalContextDialog } from './TerminalContextDialog';
export function ChatPage({ onInitialReady, scope, models, model, setModel, onboardingPending, onFirstResponse, onOpenModels }: ChatPageProps) {
  const { messages: text, locale } = useI18n();
  const sidebar = useContext(SidebarContext);
  const refinement = workspaceRefinement[locale];
  const { conversationPane, conversationContent, followOutput, userScrollUntil, showLatest, setShowLatest, followConversation, scrollEvents } = useConversationScroll();
  const voiceCopy = voiceText(locale);
  const talkCopy = talkText(locale);
  const feedback = operationFeedback(locale);
  const handoff = useSetupHandoff();
  const draftScope = useDraftScope();
  const work = useWorkActions();
  const { preferences: voicePreferences } = useVoicePreferences();
  const talk = useTalkMode();
  const { state: talkState, setState: setTalkState, cycle: talkCycle, active: talkActive, revision: talkRevision } = talk;
  const { speakResponses, speechState, speechError, setSpeechError, checkingSpeech, speech, stopSpeech, stopSpeechFor, finishSpeech, toggleSpeech, beginSpeech, retainSpeechIntent } = useChatSpeech(voicePreferences, locale);
  const [responseSettingsOpen, setResponseSettingsOpen] = useState(false);
  const sessionKey = draftKey(draftScope, 'active-chat');
  const preferencesKey = `offgrid.chat.preferences:${encodeURIComponent(scope)}`;
  const [sessions, setSessions] = useState<ChatSession[]>([]);
  const [activeName, setActiveName] = useState(() => handoff.returning?.kind === 'chat' ? handoff.returning.id : readDraft(sessionKey));
  const [missingSession, setMissingSession] = useState(false);
  useEffect(() => work.claimChat(activeName), [activeName, work]);
  useEffect(() => { if (handoff.returning?.kind === 'chat') handoff.consumed(); }, []);
  const [conversation, setConversation] = useState<ConversationMessage[]>([]);
  const { value: draft, setValue: setDraft, unsaved, key: composerDraftKey } = useDraft(draftScope, 'chat', activeName);
  const pageDraft=useDraft(draftScope,'chat-source-page',activeName);
  let selectedPage: {id:string;revision:string;page:number;name:string;session:string}|undefined;
  try {const value=JSON.parse(pageDraft.value);if(value.session===activeName&&typeof value.id==='string'&&typeof value.revision==='string'&&Number.isInteger(value.page)&&value.page>0)selectedPage=value;}catch{/* No explicit page scope. */}
  const [knowledge, setKnowledge] = useWorkspaceState('chat.knowledge', false);
  const scopeCopy = knowledgeScope[locale];
  const {settled:capabilitiesSettled,inputOptions,sourceScopes,sourceInputs,conversationAdmission,scopedReady,imageServiceReady,imageOrientationReady,fileServiceReady,executionRequestable}=useChatCapabilities(scope);
  const [canonicalID, setCanonicalID] = useState('');
  const agentConfiguration=useConversationConfiguration(), {agent,key:agentKey,restrictions}=agentConfiguration;
  const folder = useConversationFolder(activeName, canonicalID), {value: executionWorkspace, change: setExecutionWorkspace} = folder;
  const [executionReady, setExecutionReady] = useState(false);
  const [computerPolicy, setComputerPolicy] = useWorkspaceState<TaskComputerPolicy>('chat.agentPolicy', 'full_task');
  const effectiveModel = canonicalID || conversationAdmission ? model || agent?.settings.model || '' : agent?.settings.model ?? model;
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
  const workBlocked = !canonicalID && linkedUnsettled && !canSendInstruction;
  const jobCopy = conversationWorkText[locale];
  const spokenJob = useRef<{runID: string; turnID: string; session: string; current: () => boolean} | null>(null);
  const submittedJob = useRef('');
  const [loading, setLoading] = useState(true);
  const [requestedAgent, consumeRequestedAgent] = useWorkspaceState<typeof agent>('chat.requestedAgent', null);
  useEffect(() => {
    if (loading || !requestedAgent) return;
    // Explicit handoff wins over asynchronous history restoration. Selection
    // invalidates the restoration revision without submitting or granting access.
    agentConfiguration.select(requestedAgent);
    consumeRequestedAgent(null);
  }, [loading, requestedAgent]);
  const [busy, setBusy] = useState(false);
  const [checkingTurn, setCheckingTurn] = useState(false);
  useEffect(() => { if (!loading && !checkingTurn && capabilitiesSettled) onInitialReady?.(); }, [loading,checkingTurn,capabilitiesSettled,onInitialReady]);
  const imageDraft=useComposerImages(draftScope,activeName,loading||busy||checkingTurn,{service:imageServiceReady,orientation:imageOrientationReady,agent:!!agent&&!sourceInputs,model:models.find(candidate=>candidate.id===effectiveModel),history:conversation});
  const [phase, setPhase] = useState<ChatPhase>('queued');
  const [streamed, setStreamed] = useState('');
  const displayedStream = useDeferredValue(streamed);
  const [interrupted, setInterrupted] = useState(false);
  const [metrics, setMetrics] = useState<ChatMetrics>();
  const [limited, setLimited] = useState(false);
  const [preferences, updatePreferences] = useResponsePreferences(preferencesKey);
  const { profile, maxTokens } = preferences;
  const visibleStream = preferences.streamResponses || interrupted;
  const [error, setErrorText] = useState('');
  const [errorCode, setErrorCode] = useState('');
  const setError = (message: string, code = '') => { setErrorText(message); setErrorCode(code); };
  const [errorAction, setErrorAction] = useState<'history' | 'request' | 'cancel' | null>('history');
  const [deleteItems, setDeleteItems] = useState<HistoryItem[] | null>(null);
  const [historyNotice, setHistoryNotice] = useState('');
  const { copiedMessage, setCopiedMessage, copyMessage } = useMessageCopy(() => setErrorAction(null), setError, text.common.error);
  const controller = useRef<AbortController | null>(null);
  const composerInput = useRef<HTMLTextAreaElement | null>(null);
  const activeTurn = useRef<{ name: string; id: string } | null>(null);
  const mounted = useRef(true);
  const selectionRevision = useRef(0);
  const historyRevision = useRef(0);
  const canonical = useCanonicalConversation(activeName, canonicalID, {
    receive: (session, turn) => {setConversation(session.messages); setSessions(current => [session, ...current.filter(item => item.name !== session.name)]); setLinkedTurn(turn);},
    answered: (answer, input) => {finishSpeech(answer, canonicalSpeechSource(activeName, input)); if (onboardingPending) onFirstResponse();},
    unavailable: input => {if (stopSpeechFor(canonicalSpeechSource(activeName, input)) && talkActive.current) {talkActive.current = false; setTalkState('needs_attention');}},
  });
  const permissions = useWorkspace();
  const { sources: selectedSources, transfer: transferSources, clear: clearSources, unsaved:sourcesUnsaved } = useKnowledgeSources(activeName, loading, permissions.knowledge, setKnowledge);
  const {state: knowledgeState, refresh: refreshKnowledge} = useKnowledgeReadiness(permissions.knowledge);
  const acceptLinkedTurn = (session: ChatSession, turn: SessionTurn, revision: number) => {
    if (!mounted.current || revision !== selectionRevision.current) return;
    activeTurn.current = { name: session.name, id: turn.id };
    setLinkedTurn(turn); setRecoveryNeeded(false); setBusy(false); setStreamed(''); setInterrupted(false);
    const saved = session.messages.some(message => message.turn_id === turn.id && message.role === 'user');
    setConversation(saved ? session.messages : [...session.messages, { role: 'user', content: turn.prompt }]);
    work.rememberChat(session.name, turn.status, turn.job_id);
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
    work.rememberChat(activeName, linkedRun.status, linkedRun.run_id);
    if (workSettled(linkedRun.status)) {
      void refreshLinkedConversation(linkedTurn).catch(reason => { if (currentLinked.current?.id === linkedTurn.id) setError(reason instanceof Error ? reason.message : text.common.error); });
      if (spokenJob.current?.runID === linkedRun.run_id) {
        const intent = spokenJob.current; spokenJob.current = null;
        if (intent.current() && intent.turnID === linkedTurn.id && intent.session === activeName && linkedRun.status === 'completed' && speakResponses && linkedRun.output) {
          // Begin the audio deadline only when the parent's final response is ready to speak.
          beginSpeech(false, talk, mounted, {sessionKey: activeName, requestID: linkedTurn.id}, {kind: 'session', session: activeName, turnID: linkedTurn.id, runID: linkedRun.run_id}).finish(linkedRun.output);
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
    const settingsRevision = agentConfiguration.reset();
    controller.current?.abort(); controller.current = null; activeTurn.current = null; setBusy(false); setCheckingTurn(Boolean(session)); setError('');
    setLinkedTurn(null); setRecoveryNeeded(false); spokenJob.current = null; submittedJob.current = '';
    const name = session?.name ?? '';
    setCanonicalID(session?.canonical_id ?? '');
    canonical.replyID.current = '';
    setActiveName(name);
    setMissingSession(false);
    setConversation(session?.messages ?? []);
    setStreamed(''); setInterrupted(false); setMetrics(undefined); setLimited(false);
    if (session?.model_id && models.some(item => item.id === session.model_id)) setModel(session.model_id);
    writeDraft(sessionKey, name);
    sidebar.close();
    if (session?.canonical_id) {
      restoreCanonicalConfiguration(session.canonical_id, agentConfiguration, settingsRevision, () => mounted.current && revision === selectionRevision.current, setModel, reason => setError(reason instanceof Error ? reason.message : text.common.error), () => setCheckingTurn(false));
      return;
    }
    if (session) void Promise.all([
      api.currentTurn(session.name),
      // A detached turn may have saved new messages since history was loaded.
      work.hasChat(session.name) ? api.session(session.name) : Promise.resolve(session),
    ]).then(([{ turn, job }, latest]) => {
      if (!mounted.current || revision !== selectionRevision.current) return;
      const restoredModel=agentConfiguration.restore(job?.conversation_configuration,settingsRevision);
      if(restoredModel)setModel(restoredModel);
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
        if ((window.location.hash === '#/chat/new' || window.location.hash === '#/home')) {
          activate(undefined); if(window.location.hash !== '#/home') window.history.replaceState(null, '', '#/chat');
        } else if (preferred && !selected) {
          // Never silently bind a returning draft to the first unrelated chat.
          activate(undefined); setActiveName(preferred); writeDraft(sessionKey, preferred);
          setMissingSession(true); setError(missingConversationText[locale]);
        } else activate(!preferred && readDraft(draftKey(draftScope, 'chat')) ? undefined : selected ?? next[0]);
      }
      // Saved history proves a past response, not that this installation's
      // current engine and model can answer. Only response completion verifies setup.
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
  useLayoutEffect(() => {
    // The empty draft has no transcript to follow. Its ResizeObserver already
    // follows later content; avoid a forced layout before anything has arrived.
    if (conversation.length || busy || streamed || linkedRun || error) followConversation();
  }, [conversation, busy, streamed, phase, linkedRun?.event_cursor, linkedRun?.status, error]);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; selectionRevision.current++; controller.current?.abort(); }; }, []);
  useComposerSizing(composerInput, draft);
  const startNew = () => {
    if (deleteItems) return;
    activate(undefined);
    setError('');
    setCopiedMessage('');
  };
  useEffect(() => { const home = () => { if(location.hash === '#/home' || location.hash === '#/chat/new') startNew(); }; window.addEventListener('hashchange',home); return()=>window.removeEventListener('hashchange',home); }, [activeName, deleteItems]);
  const historyDeleted = (names: string[]) => {
    setSessions(current => current.filter(item => !names.includes(item.name)));
    names.forEach(name => { writeDraft(draftKey(draftScope, 'chat', name), ''); writeDraft(draftKey(draftScope,'chat-images',name),''); writeDraft(draftKey(draftScope,'chat-files',name),''); work.forgetChat(name); });
    if (names.includes(activeName)) activate(undefined);
    if (names.length) setHistoryNotice(text.history.removed.replace('{count}', String(names.length)));
  };
  const reviewLinkedWork = () => reviewConversationWork(conversationContent.current, followOutput, refreshJob);
  const fileDraft=useComposerFiles(draftScope,activeName,loading||busy||checkingTurn||missingSession,fileServiceReady,!!agent&&!sourceInputs,!!canonicalID&&!sourceInputs,async (select,newConversation)=>{
    if(activeName&&!newConversation)return activeName;
    const revision=selectionRevision.current;
    return createFileConversation({scope:draftScope,prompt:draft.trim(),model:effectiveModel,draft:composerDraftKey,images:imageDraft,valid:()=>mounted.current&&revision===selectionRevision.current,accept:created=>{select(created.name);if(newConversation)activate(created);else setActiveName(created.name);transferSources(created.name);writeDraft(sessionKey,created.name);setSessions(current=>[created,...current]);}});
  });
  const send = async (voicePrompt?: string, fromTalk = false, audible: () => boolean = () => true): Promise<boolean> => {
    setErrorAction('request');
    const inputMode = fromTalk ? 'auto' : canonical.inputMode, answeringQuestion = canonical.isAnsweringQuestion(inputMode);
    if (!fromTalk && talkActive.current) { talkActive.current = false; talkRevision.current++; setTalkState('off'); }
    const prompt = (voicePrompt ?? draft).trim();
    if (fromTalk && !talkActive.current) return false;
    if ((!prompt&&!imageDraft.images.length&&!fileDraft.files.length) || (!answeringQuestion && (!effectiveModel || unavailableModel)) || busy || checkingTurn || missingSession || controller.current || deleteItems || imageDraft.processing || fileDraft.processing || fileDraft.pendingHandoff) return false;
    if ((canonicalID || conversationAdmission) && !prompt && (!inputOptions || !sourceInputs || answeringQuestion || canonical.inputMode==='new_work')) return false;
    if(fileDraft.blocker){setError(fileDraft.blocker);return false;}
    if(imageDraft.blocker){setError(imageDraft.blocker,imageDraft.blockerCode);return false;}
    if (!answeringQuestion && selectedSources.length && (!scopedReady || linkedUnsettled)) { setError(linkedUnsettled ? scopeCopy.active : scopeCopy.unsupported); return false; }
    if(selectedSources.some(source=>source.page||source.block_ids?.length)&&!sourceScopes){setError(scopeCopy.unsupported);return false;}
    if (canonicalID && !sourceInputs && (imageDraft.images.length || fileDraft.files.length || (!answeringQuestion && (selectedSources.length || knowledge)))) { setError(conversationInputText[locale]); return false; }
    if (linkedUnsettled && !canonicalID) {
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
    if (!answeringQuestion && (agent || folder.changed) && executionWorkspace && !executionReady) { setError(executionWorkspaceText[locale].unavailable); return false; }
    const revision = selectionRevision.current;
    const currentView = () => mounted.current && revision === selectionRevision.current;
    const requestController = new AbortController();
    const requestID = crypto.randomUUID();
    window.dispatchEvent(new Event(stopSpeechEvent));
    setSpeechError('');
    const talkTurnRevision = talkRevision.current;
    if (!answeringQuestion && (speakResponses || fromTalk) && (!fromTalk || audible()) && (!agent || canonicalID || conversationAdmission)) beginSpeech(fromTalk, talk, mounted, {sessionKey: activeName || composerDraftKey, requestID});
    const speechIntent = speakResponses ? retainSpeechIntent() : undefined;
    const responseSpeech = speech.current;
    controller.current = requestController;
    const submitted = voicePrompt ?? draft;
    const submittedImages=imageDraft.value;
    const images=imageDraft.images;
    const submittedFiles=fileDraft.value;
    const attachments=selectedPage&&sourceInputs?[...fileDraft.references.filter(ref=>ref.id!==selectedPage.id),{id:selectedPage.id,revision:selectedPage.revision,page:selectedPage.page}]:fileDraft.references;
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
      const canonicalEligible = canonicalID || ((conversationAdmission || inputOptions) && !linkedUnsettled && (sourceInputs || (!images.length && !attachments.length && !knowledge && !selectedSources.length && !conversation.some(message => message.images?.length || message.attachments?.length))));
      if (canonicalEligible) {
        await canonical.send({name: sessionName, id: canonicalID, key: draftKey(draftScope, 'canonical-request', sessionName), content: prompt, responseProfile: inputOptions ? (profile==='extended'?'extended':'interactive') : undefined, maxTokens: inputOptions ? maxTokens : undefined, attachments, images, knowledgeDocuments: selectedSources.filter(source=>!source.page&&!source.block_ids?.length).map(source => source.id), knowledgeSelections:selectedSources.filter(source=>source.page||source.block_ids?.length).map(source=>({document_id:source.id,revision:source.revision!,page:source.page,block_ids:source.block_ids})), useKnowledge: knowledge, model: effectiveModel, profile: agent ?? undefined, restrictions, folder: executionReady || !executionWorkspace ? folder.command : undefined, folderApplied: folder.accepted, origin: voicePrompt === undefined ? 'typed' : 'dictation', error: text.common.error, imported: id => {if (currentView()) setCanonicalID(id);}}, draftKey(draftScope, 'chat', sessionName), submitted, currentView, input => {responseSpeech?.bindSource(canonicalSpeechSource(sessionName, input)); if (fromTalk && input.incorporation === 'worker_answer' && input.applied_at) talk.resumeAfterInput(true);}, inputMode);
        clearSubmittedDraft(draftKey(draftScope,'chat-images',sessionName),submittedImages);
        return true;
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
        if (speechIntent?.()) spokenJob.current = {runID: accepted.job.run_id, turnID: accepted.turn.id, session: sessionName, current: speechIntent};
        return false;
      }
      setLinkedTurn(null); setRecoveryNeeded(false);
      setConversation(current => [...current, { role: 'user', content: prompt,images,attachments }]);
      activeTurn.current = { name: sessionName, id: requestID };
      work.rememberChat(sessionName, 'pending');
      responseSpeech?.bindSource({kind: 'session', session: sessionName, turnID: requestID});
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
      if (requestController.signal.aborted) setErrorAction('history');
      setError(reason instanceof APIError&&reason.data?.error?.code==='knowledge_scope_unavailable'?workSurface(locale).scopeUnavailable:requestController.signal.aborted ? text.chatStreaming.cancelled : reason instanceof Error ? reason.message : text.common.error, reason instanceof SessionResponseError ? reason.code : reason instanceof APIError ? reason.data?.error?.code : '');
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
    try { await api.cancelTurn(target.name, target.id); if (mounted.current && revision === selectionRevision.current) setErrorAction('history'); watch?.abort(); }
    catch (reason) { if (mounted.current && revision === selectionRevision.current) setError(reason instanceof Error ? reason.message : text.common.error); }
  };
  const stopTalk = (cancelTurn = false) => {
    talk.reset();
    stopSpeech();
    if (cancelTurn && controller.current) void stop();
  };
  useTalkDismiss(talk.reset, stopSpeech);
  const startTalk = () => talk.start({ eligible: (!agent || !!canonicalID) && !!effectiveModel && !unavailableModel && !busy && !loading && !checkingTurn && (!linkedUnsettled || !!canonicalID) && !missingSession, hasDraft: !!(draft.trim() || imageDraft.images.length || fileDraft.files.length), preferences: voicePreferences, locale, unavailable: voiceCopy.speechUnavailable, draftError: talkCopy.draft, setError: setSpeechError });
  const onTalkTranscript = (value: string) => talk.acceptTranscript(value, composerDraftKey, transcript => send(transcript, true));
  // Saving the first utterance changes its draft key, not the owned Talk capture.
  // Conversation navigation resets Talk explicitly through activate().
  const talkCapture = useTalkCapture(talk, () => !busy && !loading && !checkingTurn && !canonical.pendingAnswer,
    () => { window.dispatchEvent(new Event(stopSpeechEvent)); stopSpeech(); }, composerDraftKey,
    (transcript, audible) => send(transcript, true, audible));
  const speakAgain = () => talk.speakAgain(busy, stopSpeech);
  useTalkIdentity(`${effectiveModel}:${agentKey}`, () => { if (talkActive.current || talkState === 'checking') stopTalk(true); });
  const { lastAssistant, linkedOutputSaved } = conversationOutputState(conversation, linkedTurn?.id);
  const linkedAnswerIndex = linkedRun ? conversation.reduce((found, message, index) => message.role === 'assistant' && 'job_id' in message && message.job_id === linkedRun.run_id ? index : found, -1) : -1;
  const linkedWork = linkedTurn?.job_id && linkedRun && <ConversationWork key={linkedRun.run_id} run={linkedRun} refresh={refreshJob} connection={jobConnection} readError={jobReadError} outputSaved={linkedOutputSaved} recoveryNeeded={recoveryNeeded} recover={() => refreshLinkedConversation(linkedTurn, true)} liveSubmission={submittedJob.current === linkedRun.run_id} />;
  const talkRunning = talkState !== 'off' && talkState !== 'needs_attention';
  const canonicalActive = pendingCanonicalInputs(canonical.inputs)[0];
  const generating = busy || !!canonicalActive;
  const commandLayout = talkState !== 'off' ? 'voice' : conversation.length || generating || error || speechError || imageDraft.images.length || fileDraft.files.length || draft.length > 160 ? 'work' : 'input';
  const voiceVisible = useCommandWindowLayout(commandLayout);
  const stopCurrent = async () => { if (canonicalActive) { try { await canonical.control(canonicalActive, 'stop'); } catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); } } else await stop(); };
  const showTalkAction = (!agent || !!canonicalID) && (talkRunning || ((!generating || !!canonicalID && !busy) && !draft.trim() && !imageDraft.images.length && !fileDraft.files.length && !fileDraft.pendingHandoff));
  const showStop=generating && !(canonicalID && !busy && draft.trim());
  return <DocumentWorkspace scope={`${draftScope}:${activeName}`} addFile={fileServiceReady&&!loading&&!busy&&!checkingTurn&&!missingSession&&!fileDraft.processing&&fileDraft.files.length<8?fileDraft.chooseFile:undefined} selectPage={sourceInputs?(source,page)=>{if(source.attachment?.session===activeName){pageDraft.setValue(JSON.stringify({...source.attachment,page,name:source.name}));requestAnimationFrame(()=>composerInput.current?.focus());}}:undefined}><div className="chat-workspace">
    <TerminalContextDialog sessions={sessions} current={activeName} disabled={loading||busy||checkingTurn||talkRunning} accept={(name,value)=>{const key=draftKey(draftScope,'chat',name);const previous=readDraft(key);writeDraft(key,previous?`${previous}\n\n${value}`:value);activate(sessions.find(session=>session.name===name));}}/>
    {deleteItems && <ChatHistoryDelete items={deleteItems} onDeleted={historyDeleted} onClose={() => setDeleteItems(null)} onOpen={name=>{setDeleteItems(null);activate(sessions.find(session=>session.name===name));}} />}
    <SidebarSlot action><button className="new-chat-button" title={text.chat.newChat} aria-label={text.chat.newChat} disabled={!!deleteItems} onClick={() => { sidebar.close(); startNew(); }}><Icon name="plus" size={18} /><span>{text.chat.newChat}</span></button></SidebarSlot>
    <SidebarSlot><ConversationHistory sessions={sessions} activeName={activeName} busy={busy} loading={loading} locked={!!deleteItems} notice={historyNotice} select={activate} remove={setDeleteItems} refresh={() => void loadSessions()} /></SidebarSlot>
    <div className="chat-layout">
<div ref={conversationPane} className="conversation" tabIndex={0} {...scrollEvents} onPointerDownCapture={event => inspectConversationWork(event.target,followOutput,userScrollUntil)} onFocusCapture={event => inspectConversationWork(event.target,followOutput,userScrollUntil)}>
        <div ref={conversationContent} className="conversation-content">
        {conversation.length === 0 && !loading && !error && <WorkHome sessions={sessions} open={session=>{activate(session);location.hash='#/chat';}} addFile={fileDraft.chooseFile}/>}
        <ConversationMessages conversation={conversation} activeName={activeName} busy={busy} linkedAnswerIndex={linkedAnswerIndex} linkedWork={linkedWork} linkedJob={linkedTurn?.job_id} lastAssistant={lastAssistant} metrics={metrics} copiedMessage={copiedMessage} copyMessage={copyMessage} />
        {linkedAnswerIndex < 0 && linkedWork}
        {linkedTurn && !linkedRun && <ScopedNotice kind={jobReadError ? 'error' : 'status'} action={<button className="secondary-button" onClick={() => { if (linkedTurn.job_id) refreshJob(); else void api.recoverAgentTurn(activeName, linkedTurn.id).then(result => setLinkedTurn(result.turn)).catch(reason => setError(reason instanceof Error ? reason.message : text.common.error)); }}>{linkedTurn.job_id ? text.common.refresh : jobCopy.attach}</button>}>{jobReadError || (linkedTurn.job_id ? text.common.loading : jobCopy.savedJob)}</ScopedNotice>}
        {((busy && !linkedRun) || streamed) && <article className="message assistant streaming-response"><div className="message-heading"><div className="message-label">{text.chat.assistant}</div>{streamed && visibleStream && <button type="button" className="message-copy" onClick={() => void copyMessage('partial', streamed)}>{copiedMessage === 'partial' ? text.chat.copied : text.chat.copy}</button>}</div>
          {streamed && visibleStream && <div className="message-body"><MarkdownMessage content={displayedStream} streaming /></div>}
          <div className="generation-status" role="status">{interrupted ? text.chatStreaming.unsaved : text.chatStreaming[phase]}{!interrupted && phase === 'loading' && <p>{text.guidance.coldStart}</p>}</div>
        </article>}
        {canonicalID && preferences.streamResponses && <CanonicalResponsePreview inputs={canonical.inputs} messages={conversation} onChange={followConversation} />}
        {limited && <div className="generation-status" role="status">{text.chatStreaming.limitReached}</div>}
        {(unsaved || sourcesUnsaved || pageDraft.unsaved) && <div className="inline-error" role="alert">{text.recovery.draftWarning}</div>}
        </div>
      </div>
      <div className="chat-recovery">{fileDraft.notice}{error && <ChatFailureNotice error={error} code={errorCode} openSettings={() => setResponseSettingsOpen(true)} chooseModel={() => { handoff.begin({ origin: '#/chat', destination: 'models', draft: { kind: 'chat', id: activeName } }); onOpenModels(); }} action={errorAction && <button className="secondary-button" onClick={() => void (errorAction === 'cancel' ? stop() : loadSessions())}>{errorAction === 'cancel' ? text.chat.stop : errorAction === 'history' ? interaction[locale].refreshHistory : interaction[locale].checkRequest}</button>} />}</div>
      {showLatest && <button type="button" className="secondary-button chat-latest" onClick={event => { userScrollUntil.current = 0; followOutput.current = true; const pane = conversationPane.current; pane?.focus({ preventScroll: true }); if(pane) pane.dataset.keyboardFocus = String(event.detail === 0); setShowLatest(false); requestAnimationFrame(() => { pane?.scrollTo({ top: pane.scrollHeight, behavior: 'instant' }); }); }}>{refinement.latest}</button>}
      {canonicalID && <CanonicalInputActivity id={canonicalID} inputs={canonical.inputs} control={canonical.control} error={canonical.error} question={canonical.question} mode={canonical.inputMode} chooseMode={canonical.setInputMode} disabled={busy || canonical.pendingAnswer} />}
      {workBlocked && <ChatWorkBlocker run={linkedRun} review={reviewLinkedWork} refresh={refreshJob} startNew={startNew} />}
      {unavailableModel && <p className="chat-readiness" role="status">{chatReadiness[locale].unavailable}</p>}
      <div className="composer composer-compact" onPaste={imageDraft.paste} onDragOver={imageDraft.dragOver} onDrop={event=>{const files=[...event.dataTransfer.files];if(fileServiceReady&&files.some(file=>!file.type.startsWith('image/'))){event.preventDefault();void fileDraft.add(files);}else imageDraft.drop(event);}}>
        {selectedPage&&<div className="composer-source-scope"><Icon name="file" size={16}/><span><bdi>{selectedPage.name}</bdi> · {knowledgeUnitLabels[locale].page} {selectedPage.page}</span><button type="button" className="icon-button" aria-label={scopeCopy.clear} title={scopeCopy.clear} onClick={()=>pageDraft.setValue('')}><Icon name="close" size={16}/></button></div>}
        <div className="composer-input">{fileDraft.preview}{imageDraft.preview}<textarea ref={composerInput} value={draft} onChange={event => { if (talkActive.current) stopTalk(false); setDraft(event.target.value); }} onKeyDown={event => submitComposerInput(event, async () => { await send(); })} placeholder={text.chat.placeholder} aria-label={text.chat.placeholder} rows={1} disabled={loading || checkingTurn} /><small className="sr-only">{text.chat.enterHint}</small>{fileDraft.recovery}{imageDraft.blocker&&<p className="image-input-notice" role="status">{imageDraft.blocker}{imageDraft.blockerCode==='vision_projector_unavailable'&&<button type="button" className="text-button" onClick={()=>{handoff.begin({origin:'#/chat',destination:'models',draft:{kind:'chat',id:activeName}});onOpenModels();}}>{text.onboarding.chooseModel}</button>}</p>}</div>
        {!agent && knowledge && knowledgeState !== 'ready' && <ScopedNotice action={<button className="secondary-button" onClick={() => void refreshKnowledge()}>{feedback.retryKnowledge}</button>}>{knowledgeState === 'checking' ? feedback.checkingKnowledge : knowledgeState === 'failed' ? feedback.knowledgeCheckFailed : interaction[locale].knowledgeUnavailable}</ScopedNotice>}
        {selectedSources.length > 0 ? <div className="composer-source-scope"><strong>{scopeCopy.selected}</strong><span>{selectedSources.map(source => source.name+(source.scopeLabel?' \u00b7 '+source.scopeLabel:'')).join(' · ')}</span><button type="button" className="icon-button" disabled={busy} onClick={clearSources} aria-label={scopeCopy.clear} title={scopeCopy.clear}><Icon name="close" size={16} /></button>{!scopedReady && <p role="status">{scopeCopy.unsupported}</p>}</div> : !agent && knowledge && <p className="composer-retrieval-scope">{workspaceManagement(locale).retrievalScope}</p>}
        <div className="composer-footer">
          <div className="composer-utilities">
          <UtilityPopover label={composerAttachmentLabel(locale)} trigger={<Icon name="plus" size={20}/>} triggerClassName="icon-button" className="composer-attachments-menu" compact>{fileDraft.control}{imageDraft.control}{fileDraft.saved}
          {(canonicalID || conversationAdmission || agent && !executionRequestable || executionWorkspace) && <ExecutionWorkspace menu value={executionWorkspace} onChange={setExecutionWorkspace} disabled={busy || linkedUnsettled} onReady={setExecutionReady} ceiling={restrictions.execution??agent?.settings.execution} />}
          </UtilityPopover>
          <UtilityPopover label={text.chatStreaming.options} trigger={<Icon name="sliders" size={18} />} triggerClassName="icon-button" className="composer-context-menu" defaultOpen={responseSettingsOpen} onOpenChange={setResponseSettingsOpen}>
        <div className="composer-context">
          <AgentSelector value={agent} onChange={agentConfiguration.select} disabled={busy} />
          {agent && <AgentTools profile={agent} restrictions={restrictions} onChange={agentConfiguration.update} disabled={busy} />}
        </div>
            {!agent && <label className="switch"><input type="checkbox" checked={knowledge || selectedSources.length > 0} disabled={busy || selectedSources.length > 0 || (!knowledge && knowledgeState !== 'ready')} onChange={event => setKnowledge(event.target.checked)} /><span />{text.chat.knowledge}</label>}
            {!agent && permissions.knowledge && knowledgeState !== 'ready' && <ComposerKnowledgeReadiness state={knowledgeState} conversation={activeName} refresh={refreshKnowledge} />}
            <a className="composer-settings-link" href="#/settings/responses"><Icon name="settings" size={16}/>{text.chatStreaming.responseLimit}</a>{canonicalID && <><ConversationInputMode mode={canonical.inputMode} chooseMode={canonical.setInputMode} question={canonical.question} disabled={busy || canonical.pendingAnswer} /><ConversationSettings key={canonicalID} id={canonicalID} /></>}
            {responseSettingsOpen && <ComposerSpeech presentation="settings" enabled={speakResponses} checking={checkingSpeech} busy={busy} state={speechState} error={speechError} toggle={toggleSpeech} stop={stopSpeech} />}
          </UtilityPopover>
          </div>
      <div className="composer-actions">
        {agent ? <button type="button" className="composer-pinned-model text-button" title={effectiveModel} aria-label={text.chat.model} onClick={()=>setResponseSettingsOpen(true)}><span className="model-trigger-name"><span className="model-label-regular">{compactModelName(effectiveModel)||text.chat.model}</span><span className="model-label-narrow">{compactModelName(effectiveModel,14)||text.chat.model}</span></span><Icon name="chevron-down" size={14}/></button> : <ModelSelect compact models={models} value={model} onChange={value=>{agentConfiguration.invalidate();setModel(value);}} onOpenModels={() => { handoff.begin({ origin: '#/chat', destination: 'models', draft: { kind: 'chat', id: activeName } }); onOpenModels(); }} />}
        {generating&&(!showStop||showTalkAction)&&<button type="button" className="icon-button" aria-label={text.chat.stop} title={text.chat.stop} onClick={()=>void stopCurrent()}><Icon name="stop" size={18}/></button>}
        {talkState === 'off' && <VoiceInputButton compact contextKey={composerDraftKey} disabled={loading || checkingTurn || busy} onTranscript={value => { const previous = readDraft(composerDraftKey); setDraft(previous ? `${previous} ${value}` : value); }} />}
        {showTalkAction && talkState === 'off' && <button type="button" className="talk-control talk-toggle" aria-label={talkCopy.start} title={talkCopy.start} disabled={loading || checkingTurn || busy || !effectiveModel || unavailableModel || (linkedUnsettled && !canonicalID)} onClick={() => void startTalk()}><Icon name="waveform" size={19} /></button>}
        {!showTalkAction && <button className="composer-submit" data-generating={generating} title={showStop ? text.chat.stop : canSendInstruction ? jobCopy.update : text.chat.send} aria-label={showStop ? text.chat.stop : canSendInstruction ? jobCopy.update : text.chat.send} disabled={showStop ? false : loading || checkingTurn || missingSession || ((canonicalID || conversationAdmission) && !draft.trim() && (!inputOptions || !sourceInputs || canonical.answeringQuestion || canonical.inputMode==='new_work')) || (!draft.trim()&&!imageDraft.images.length&&!fileDraft.files.length) || fileDraft.processing || !!fileDraft.blocker || imageDraft.processing || !!imageDraft.blocker || (!canonical.answeringQuestion && (!effectiveModel || unavailableModel)) || (linkedUnsettled && !canSendInstruction && !canonicalID)} onClick={showStop ? () => void stopCurrent() : () => void send()}>{showStop ? <Icon name="stop" size={18} /> : <><span className="sr-only">{canSendInstruction ? jobCopy.update : text.chat.send}</span><Icon name="arrow-up" size={18} /></>}</button>}
      </div>
        </div>
        {executionWorkspace && <div className="composer-source-scope"><Icon name="folder" size={15}/><span dir="auto">{executionWorkspace.root}</span></div>}
        {talkState === 'off' && <ComposerSpeech presentation="status" enabled={speakResponses} checking={checkingSpeech} busy={busy} state={speechState} error={speechError} toggle={toggleSpeech} stop={stopSpeech} />}
        {voiceVisible && <TalkSurface state={talkState} error={speechError} retry={() => void startTalk()} canRetry={!draft.trim()&&!busy} interrupt={speakAgain} canInterrupt={!!canonicalID && !busy && !canonical.pendingAnswer} end={() => { stopTalk(); if (generating) void stopCurrent(); }} renderCapture={(onLevel,onPhase,onCapability) => talkRunning && talkActive.current ? <VoiceInputButton key={`talk-${talk.session.current}`} compact autoStart contextKey={talk.session.current} disabled={loading || checkingTurn || busy} continuous={{sessionID:talk.session.current, monitoring:!busy&&!loading&&!checkingTurn&&!canonical.pendingAnswer, fallbackListening:talkState==='listening'||talkState==='transcribing', playbackActive:speechState==='playing', onCapability, ...talkCapture}} onTranscript={onTalkTranscript} onFailure={message => talk.fail(message, setSpeechError)} onSilence={talk.silence} onPhaseChange={phase=>{talk.capturePhase(phase);onPhase(phase);}} onLevel={onLevel}/> : null}/>}
        {agent && <div className="composer-active-context">{agent && <span className="composer-active-agent"><Icon name="agents" size={15}/>{agent.settings.name}</span>}

          {agent?.settings.computer && restrictions.computer !== false && <TaskAccessPolicy value={computerPolicy} onChange={setComputerPolicy} disabled={busy} scope="desktop" workspace={permissions.workspace} />}
        </div>}
      </div>
    </div>
  </div></DocumentWorkspace>;
}
