import { useWorkspaceRefresh } from '../../lib/workspace-refresh';
import { useEffect, useMemo, useRef, useState, type FormEvent } from 'react';
import { APIError, api, type AgentRun, type AgentTask, type AgentTool, type ComputerStatus, type ExternalIntegration, type IntegrationSetup, type MCPServer, type Model } from '../../api/client';
import { Icon } from '../../components/Icon';
import { MarkdownMessage } from '../../components/MarkdownMessage';
import { ModelSelect } from '../../components/ModelSelect';
import { HistoryDeleteDialog, type HistoryItem } from '../../components/HistoryDeleteDialog';
import { copyText } from '../../lib/clipboard';
import { useI18n } from '../../i18n';
import { presentation } from '../../i18n/presentation';
import { clearSubmittedDraft, draftKey, useDraft, writeDraft } from '../../lib/drafts';
import { agentActive } from '../../api/agent-stream';
import { readPreference, writePreference } from '../../lib/preferences';
import { AgentPreview, AgentProgress } from './AgentProgress';
import { useAgentOutputScroll } from './useAgentOutputScroll';
import { useWorkspace, useWorkspaceState } from '../../lib/workspace-context';
import { interaction } from '../../i18n/interaction';
import { ComputerSetup } from './ComputerSetup';
import { computerRecovery, computerModelCopy } from '../../i18n/computer-recovery';
import { computerExperience } from '../../i18n/computer-experience';
import { computerModeText, nativeAppText } from '../../i18n/native-app';
import { BrowserActionSummary, BrowserActivity } from './BrowserActionSummary';

type AgentView = 'workspace' | 'tools' | 'connections';

function viewFromLocation(): AgentView {
  const candidate = window.location.hash.split('/')[2];
  return candidate === 'tools' || candidate === 'connections' ? candidate : 'workspace';
}

export function AgentPage({ scope, models, model, setModel }: { scope: string; models: Model[]; model: string; setModel: (model: string) => void }) {
  const { admin } = useWorkspace();
  const { locale, messages } = useI18n();
  const [workspace, setWorkspace] = useState<string | null>(null);
  const [identityError, setIdentityError] = useState(false);
  useEffect(() => {
    if (!admin) return;
    let disposed = false;
    const refresh = () => api.systemIdentity().then(identity => { if (!disposed) { setWorkspace(identity.workspace_id ?? 'legacy'); setIdentityError(false); } }).catch(() => { if (!disposed) setIdentityError(true); });
    void refresh(); const timer = setInterval(() => void refresh(), 5000);
    return () => { disposed = true; clearInterval(timer); };
  }, [admin]);
  if (!admin) return <p className="permission-notice" role="status">{interaction[locale].adminOnly}</p>;
  // Do not mount an interactive form that will immediately be replaced when
  // identity arrives: that loses focus and can discard a just-selected mode.
  if (!workspace) return <p role={identityError ? 'alert' : 'status'}>{identityError ? messages.common.error : messages.common.loading}</p>;
  return <AgentWorkspace key={workspace} scope={scope} selectionScope={`${scope}:workspace:${workspace}`} models={models} model={model} setModel={setModel} />;
}

function AgentWorkspace({ scope, selectionScope, models, model, setModel }: { scope: string; selectionScope: string; models: Model[]; model: string; setModel: (model: string) => void }) {
  const { messages: text, locale } = useI18n();
  const experience = computerExperience(locale);
  const { value: task, setValue: setTask, key: taskDraftKey, unsaved } = useDraft(scope, 'agent-task');
  const { value: selectedRun, setValue: selectRun } = useDraft(selectionScope, 'agent-run');
  const [selectionNotice, setSelectionNotice] = useState(false);
  const [style, setStyle] = useWorkspaceState('agent.style', 'react');
  const [busy, setBusy] = useState(false);
  const [computerMode, setComputerMode] = useWorkspaceState(`${selectionScope}:computer.mode`, false);
  const [computerTargetMode, setComputerTargetMode] = useWorkspaceState<'app'|'browser'>(`${selectionScope}:computer.target`, window.electron?.discoverComputerApps ? 'app' : 'browser');
  const [computerSession, setComputerSession] = useState('');
  const [computerReady, setComputerReady] = useState(false);
  const [submittingComputer, setSubmittingComputer] = useState(false);
  // Retire only this obsolete UI setting; preserve task drafts and saved runs.
  // Requests never read it, even when storage is unavailable or another tab
  // running an older renderer writes it again.
  useEffect(() => {
    writeDraft(draftKey(selectionScope, 'computer-expected-text'), '');
  }, [selectionScope]);
  const [execution, setExecution] = useState<AgentRun | null>(null);
  const [connection, setConnection] = useState<'connecting' | 'live' | 'reconnecting'>('connecting');
  const [livePreview, setLivePreview] = useState(() => readPreference('offgrid.agent.livePreview') !== 'false');
  const result = execution?.output ?? '';
  const steps = execution?.steps ?? [];
  const [error, setError] = useState('');
  const approval = execution?.pending_approval;
  const [verifiedResult, setVerifiedResult] = useState('');
  const actionLock = useRef(false);
  const working = busy || execution?.status === 'running' || execution?.status === 'pending';
  const computerBlocked = computerMode && (!computerSession || !computerReady);
  const computerText = nativeAppText(locale);
  const modeText = computerModeText(locale);
  const computerRunLabel = computerBlocked
    ? (window.electron?.discoverComputerApps ? computerText.select : experience.openDesktop)
    : submittingComputer ? computerModelCopy[locale].checking
      : working ? text.agents.running : text.agents.run;
  useEffect(() => {
    if (execution?.computer_session && ['running', 'pending', 'waiting_for_approval', 'interrupted', 'uncertain'].includes(execution.status)) {
      setComputerMode(true); setComputerSession(execution.computer_session);
    }
  }, [execution?.run_id, execution?.computer_session]);
  const [tools, setTools] = useState<AgentTool[]>([]);
  const [enabledTools, setEnabledTools] = useState(0);
  const [tasks, setTasks] = useState<AgentTask[]>([]);
  const [servers, setServers] = useState<MCPServer[]>([]);
  const [computer, setComputer] = useState<ComputerStatus | null>(null);
  const [loadingRuntime, setLoadingRuntime] = useState(true);
  const [toolBusy, setToolBusy] = useState('');
  const [connectionName, setConnectionName] = useWorkspaceState('agent.connectorName', '');
  const [connectionURL, setConnectionURL] = useWorkspaceState('agent.connectorURL', '');
  const [connectionBusy, setConnectionBusy] = useState<'test' | 'connect' | ''>('');
  const [connectionMessage, setConnectionMessage] = useState('');
  const [integrations, setIntegrations] = useState<ExternalIntegration[]>([]);
  const [integrationSetup, setIntegrationSetup] = useState<{ id: string; name: string; setup: IntegrationSetup } | null>(null);
  const [integrationBusy, setIntegrationBusy] = useState('');
  const [copied, setCopied] = useState(false);
  const [view, setView] = useState<AgentView>(viewFromLocation);
  const [historyQuery, setHistoryQuery] = useWorkspaceState('agent.historyQuery', '');
  const [historyLimit, setHistoryLimit] = useWorkspaceState('agent.historyLimit', 20);
  const [deleteItems, setDeleteItems] = useState<HistoryItem[] | null>(null);
  const [historyNotice, setHistoryNotice] = useState('');
  const [resultCopied, setResultCopied] = useState(false);
  const outputScroll = useAgentOutputScroll(execution, livePreview, view);
  const runtimeRequest = useRef(0);
  const integrationRequest = useRef(0);
  const [runtimeError, setRuntimeError] = useState('');
  const [historyError, setHistoryError] = useState('');

  const refreshRuntime = async () => {
    const request = ++runtimeRequest.current;
    setLoadingRuntime(true);
    setRuntimeError(''); setHistoryError('');
    const [toolResult, taskResult, serverResult, computerResult, integrationResult] = await Promise.allSettled([api.agentTools(), api.agentTasks(), api.mcpServers(), api.computerStatus(), api.integrations(model)]);
    if (request !== runtimeRequest.current) return;
    if (toolResult.status === 'fulfilled') { setTools(toolResult.value.tools); setEnabledTools(toolResult.value.enabled_count); }
    if (taskResult.status === 'fulfilled') setTasks([...taskResult.value].sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at)));
    if (serverResult.status === 'fulfilled') setServers(serverResult.value);
    if (computerResult.status === 'fulfilled') setComputer(computerResult.value);
    if (integrationResult.status === 'fulfilled') setIntegrations(integrationResult.value.integrations);
    const failed = [toolResult, taskResult, serverResult, computerResult, integrationResult].find(item => item.status === 'rejected');
    if (failed?.status === 'rejected') setRuntimeError(failed.reason instanceof Error ? failed.reason.message : text.common.error);
    if (taskResult.status === 'rejected') setHistoryError(interaction[locale].historyFailed);
    setLoadingRuntime(false);
  };

  useEffect(() => {
    integrationRequest.current++;
    setIntegrationBusy('');
    setIntegrationSetup(null);
    void refreshRuntime();
    return () => { integrationRequest.current++; runtimeRequest.current++; };
  }, [model]);
  useWorkspaceRefresh(refreshRuntime);
  useEffect(() => {
    const syncView = () => setView(viewFromLocation());
    window.addEventListener('hashchange', syncView);
    return () => window.removeEventListener('hashchange', syncView);
  }, []);

  const selectView = (next: AgentView) => {
    setView(next);
    window.location.hash = `#/agents/${next}`;
  };

  useEffect(() => {
    if (!selectedRun) { setExecution(null); return; }
    let disposed = false;
    let timer: ReturnType<typeof setTimeout>;
    let watchdog: ReturnType<typeof setTimeout>;
    let controller: AbortController | undefined;
    let retries = 0;
    setConnection('connecting');
    const follow = async () => {
      let fetchingSnapshot = true;
      try {
        const next = await api.agentRun(selectedRun);
        if (disposed) return;
        setExecution(next);
        if (!agentActive(next)) {
          setConnection('live');
          // Approvals can expire or be resolved in another client. Paused runs
          // must discover that transition without resubmitting any work.
          if (['waiting_for_approval','interrupted','uncertain'].includes(next.status)) timer = setTimeout(() => void follow(), 5000);
          else void refreshRuntime();
          return;
        }
        fetchingSnapshot = false;
        controller = new AbortController();
        const alive = () => {
          if (disposed) return;
          retries = 0; setConnection('live'); clearTimeout(watchdog);
          watchdog = setTimeout(() => controller?.abort(), 20_000);
        };
        watchdog = setTimeout(() => controller?.abort(), 20_000);
        await api.streamAgent(selectedRun, snapshot => { if (!disposed) setExecution(snapshot); }, alive, controller.signal);
        if (!disposed) void refreshRuntime();
      } catch (reason) {
        if (!disposed) {
          if (reason instanceof APIError && ([401,403].includes(reason.status) || (fetchingSnapshot && reason.status === 404))) {
            setExecution(null);
            if (fetchingSnapshot && reason.status === 404) {
              selectRun(''); setError(''); setSelectionNotice(true);
            } else { setError(reason.message); }
            return;
          }
          setConnection('reconnecting');
          // Snapshot polling is also the compatibility fallback for old servers;
          // reconnect never resubmits a task or sends a cancel action.
          timer = setTimeout(() => void follow(), Math.min(10_000, 1000 * 2 ** retries++));
        }
      } finally { clearTimeout(watchdog); }
    };
    void follow();
    return () => { disposed = true; clearTimeout(timer); clearTimeout(watchdog); controller?.abort(); };
  }, [scope, selectedRun, execution?.status]);

  const run = async (event: FormEvent) => {
    event.preventDefault();
    if (actionLock.current || working || approval || deleteItems || !task.trim() || !model || computerBlocked) return;
    actionLock.current = true; setBusy(true); setError('');
    setSubmittingComputer(computerMode);
    const submitted = task;
    try {
      const next = await api.runAgent(model, task.trim(), computerMode ? 'react' : style, computerMode ? computerSession : undefined);
      selectRun(next.run_id); setExecution(next); setResultCopied(false);
      clearSubmittedDraft(taskDraftKey, submitted);
      await refreshRuntime();
    } catch (reason) {
      if (reason instanceof APIError && typeof reason.data?.run_id === 'string') selectRun(reason.data.run_id);
      setError(reason instanceof Error ? reason.message : text.common.error);
    }
    finally { actionLock.current = false; setBusy(false); setSubmittingComputer(false); }
  };

  const act = async (action: 'approve' | 'deny' | 'cancel' | 'resume' | 'reconcile') => {
    if (actionLock.current || !execution) return;
    actionLock.current = true; setBusy(true); setError('');
    try {
      const next = await api.agentAction(execution.run_id, action, {
        ...(action === 'approve' || action === 'deny' ? { approval_id: approval?.id } : {}),
        ...(action === 'reconcile' ? { call_id: execution.uncertain_call_id, result: verifiedResult.trim() } : {})
      });
      setExecution(next); setVerifiedResult('');
      await refreshRuntime();
    } catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); }
    finally { actionLock.current = false; setBusy(false); }
  };
  const approve = () => void act('approve');
  const deny = () => void act('deny');
  const toggleTool = async (tool: AgentTool) => {
    setToolBusy(tool.name); setError('');
    try { await api.setAgentToolEnabled(tool.name, !tool.enabled); await refreshRuntime(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); }
    finally { setToolBusy(''); }
  };
  const testConnection = async () => {
    if (!connectionURL.trim()) return;
    setConnectionBusy('test'); setConnectionMessage(''); setError('');
    try { const response = await api.testMCP(connectionURL.trim()); setConnectionMessage(`${response.tools_count} ${text.agentRuntime.tools.toLowerCase()}`); }
    catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); }
    finally { setConnectionBusy(''); }
  };
  const connect = async (event: FormEvent) => {
    event.preventDefault();
    if (!connectionName.trim() || !connectionURL.trim()) return;
    setConnectionBusy('connect'); setConnectionMessage(''); setError('');
    try {
      const response = await api.connectMCP(connectionName.trim(), connectionURL.trim());
      setConnectionMessage(`${response.tools_added} ${text.agentRuntime.tools.toLowerCase()}`);
      setConnectionName(''); setConnectionURL('');
      await refreshRuntime();
    } catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); }
    finally { setConnectionBusy(''); }
  };

  const showIntegrationSetup = async (item: ExternalIntegration) => {
    const revision = ++integrationRequest.current;
    setIntegrationBusy(item.id); setError(''); setCopied(false);
    try { const response = await api.integrationSetup(item.id, model || item.model_id); if (revision === integrationRequest.current) setIntegrationSetup({ id: item.id, name: item.name, setup: response.setup }); }
    catch (reason) { if (revision === integrationRequest.current) setError(reason instanceof Error ? reason.message : text.common.error); }
    finally { if (revision === integrationRequest.current) setIntegrationBusy(''); }
  };

  const copyIntegrationSetup = async () => {
    if (!integrationSetup) return;
    try {
      if (integrationSetup.id === 'hermes') {
        await navigator.clipboard.writeText(integrationSetup.setup.install_command);
        setCopied(true);
        return;
      }
      const environment = Object.entries(integrationSetup.setup.environment ?? {}).map(([key, value]) => `export ${key}=${JSON.stringify(value)}`).join('\n');
      await navigator.clipboard.writeText([integrationSetup.setup.install_command, environment, integrationSetup.setup.content, ...integrationSetup.setup.verify].filter(Boolean).join('\n\n'));
      setCopied(true);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : text.common.error);
    }
  };

  const matchingTasks = useMemo(() => tasks.filter(item => `${item.prompt} ${item.id}`.toLocaleLowerCase().includes(historyQuery.trim().toLocaleLowerCase())), [tasks, historyQuery]);
  const historyDeleted = (ids: string[]) => {
    runtimeRequest.current++; setLoadingRuntime(false);
    setTasks(current => current.filter(item => !ids.includes(item.id)));
    if (ids.includes(selectedRun)) { selectRun(''); setExecution(null); setError(''); }
    if (ids.length) setHistoryNotice(text.history.removed.replace('{count}', String(ids.length)));
  };
  const copyResult = async () => {
    try { await copyText(result); setResultCopied(true); }
    catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); }
  };
  return <div className="stack agents-page">
    {selectionNotice && <p role="status" className="permission-notice">{computerRecovery[locale].missing}</p>}
    {runtimeError && <div className="inline-error" role="alert">{runtimeError}<button disabled={loadingRuntime} onClick={() => void refreshRuntime()}>{text.common.retry}</button></div>}
    {deleteItems && <HistoryDeleteDialog items={deleteItems} kind="tasks" remove={api.deleteAgentRun} onDeleted={historyDeleted} onClose={() => setDeleteItems(null)} />}
    {error && <div className="inline-error" role="alert">{error}</div>}
    {view !== 'workspace' && <div className="metric-grid agent-metrics">
      <Metric label={text.agentRuntime.tools} value={loadingRuntime ? '…' : `${enabledTools}/${tools.length} ${text.agentRuntime.enabled}`} />
      <Metric label={text.agentRuntime.history} value={loadingRuntime ? '…' : String(tasks.length)} />
      <Metric label={text.agentRuntime.connectors} value={loadingRuntime ? '…' : String(servers.length)} />
      <Metric label={text.agentRuntime.computer} value={!computer ? loadingRuntime ? text.common.loading : presentation[locale].unknown : computer.available ? text.agentRuntime.available : computerRecovery[locale].unpaired} />
    </div>}

    <div className="section-tabs" role="tablist" aria-label={text.nav.agents} onKeyDown={event => {
        const tabs = Array.from(event.currentTarget.querySelectorAll<HTMLButtonElement>('[role="tab"]'));
        const index = tabs.indexOf(document.activeElement as HTMLButtonElement);
        const rtl = document.documentElement.dir === 'rtl';
        const delta = event.key === 'ArrowRight' ? (rtl ? -1 : 1) : event.key === 'ArrowLeft' ? (rtl ? 1 : -1) : 0;
        const next = event.key === 'Home' ? 0 : event.key === 'End' ? tabs.length - 1 : delta ? (index + delta + tabs.length) % tabs.length : -1;
        if (next >= 0) { event.preventDefault(); tabs[next].focus(); tabs[next].click(); }
      }}>
      <button id="agent-workspace-tab" role="tab" tabIndex={view === 'workspace' ? 0 : -1} aria-controls="agent-workspace-panel" aria-selected={view === 'workspace'} onClick={() => selectView('workspace')}>{text.shell.work}</button>
      <button id="agent-tools-tab" role="tab" tabIndex={view === 'tools' ? 0 : -1} aria-controls="agent-tools-panel" aria-selected={view === 'tools'} onClick={() => selectView('tools')}>{experience.tools}</button>
      <button id="agent-connections-tab" role="tab" tabIndex={view === 'connections' ? 0 : -1} aria-controls="agent-connections-panel" aria-selected={view === 'connections'} onClick={() => selectView('connections')}>{experience.connections}</button>
    </div>

    {view === 'workspace' && <div id="agent-workspace-panel" className="agent-view" role="tabpanel" aria-labelledby="agent-workspace-tab">
      <div className="agent-workspace">
        <form className="task-card" onSubmit={run}>
          <label className="agent-task-editor" htmlFor="agent-task-input"><span id="agent-task-label">{text.agents.task}</span><textarea id="agent-task-input" aria-labelledby="agent-task-label" aria-describedby="task-guidance" rows={7} value={task} onChange={event => setTask(event.target.value)} placeholder={text.agents.placeholder} /></label>
          <p id="task-guidance" className="task-guidance">{experience.taskHint}</p>
          <fieldset className="computer-mode-picker">
            <legend>{modeText.title} <span className="status-pill">{modeText.preview}</span></legend>
            <div role="group" aria-label={modeText.title}>
              <button type="button" aria-pressed={!computerMode} disabled={working || !!approval || !!computerSession} onClick={() => { setComputerMode(false); setComputerReady(false); }}>{modeText.agent}</button>
              <button type="button" aria-pressed={computerMode && computerTargetMode === 'app'} disabled={working || !!approval || !!computerSession} onClick={() => { setComputerTargetMode('app'); setComputerMode(true); setComputerSession(''); setComputerReady(false); }}>{computerText.window}</button>
              <button type="button" aria-pressed={computerMode && computerTargetMode === 'browser'} disabled={working || !!approval || !!computerSession} onClick={() => { setComputerTargetMode('browser'); setComputerMode(true); setComputerSession(''); setComputerReady(false); }}>{computerText.browser}</button>
            </div>
          </fieldset>
          {computerMode && <ComputerSetup mode={computerTargetMode} model={model} visionStatus={models.find(item=>item.id===model)?.capability_status?.vision} onReady={setComputerReady} value={computerSession} onChange={setComputerSession} disabled={working || !!approval} onAvailability={available => setComputer(current => ({ emergency_stop: false, active_sessions: available ? 1 : 0, ...current, available }))} />}
          <div className="agent-task-options">
            <ModelSelect models={models} value={model} onChange={setModel} />
            {!computerMode && <label><span>{text.agentRuntime.style}</span><select value={style} onChange={event => setStyle(event.target.value)}><option value="react">{text.agentRuntime.react}</option><option value="plan-execute">{text.agentRuntime.plan}</option><option value="cot">{text.agentRuntime.reasoning}</option></select></label>}
          </div>
          {unsaved && <p role="alert">{text.recovery.draftWarning}</p>}
          {computerBlocked && <p className="task-guidance" role="status">{window.electron?.discoverComputerApps ? computerText.scope : computerText.desktop}</p>}
          <div className="agent-task-actions"><button className="primary-button" disabled={working || !!approval || !task.trim() || !model || computerBlocked}>{computerRunLabel}</button></div>
        </form>
        <section className="result-card">
          <header className="agent-result-header">
            <span id="agent-result-title" className="eyebrow">{text.agents.result}</span>
            {execution && <div className="section-heading"><span role="status">{text.recovery[execution.status as keyof typeof text.recovery] ?? execution.status}</span>
              {result && <button className="secondary-button" onClick={() => void copyResult()}>{resultCopied ? text.chat.copied : text.history.copyResult}</button>}
              {(working || approval || (execution.status === 'interrupted' && execution.resumable)) && <button className="secondary-button" disabled={busy} onClick={() => void act('cancel')}>{text.models.cancel}</button>}
            </div>}
            {execution && <AgentProgress run={execution} connection={connection} showPreview={livePreview} setShowPreview={value => { setLivePreview(value); writePreference('offgrid.agent.livePreview', String(value)); }} />}
          </header>
          <div className="agent-result-body" role="region" aria-labelledby="agent-result-title" tabIndex={0} ref={outputScroll.ref} onScroll={outputScroll.onScroll}>
            <div className="agent-result-content" ref={outputScroll.contentRef}>
            {approval ? <div className="approval-card" role="alertdialog" aria-labelledby="approval-title">
              <span className="status-pill danger">{text.agents.approvalTitle}</span><h2 id="approval-title">{experience.change}</h2><p>{text.agents.approvalBody}</p>
              {execution?.computer_session ? <><BrowserActionSummary tool={approval.tool} args={approval.arguments as Record<string,unknown>} steps={steps} /><details><summary>{experience.details}</summary><pre>{approval.canonical_arguments ?? JSON.stringify(approval.arguments, null, 2)}</pre></details></> : <pre>{approval.canonical_arguments ?? JSON.stringify(approval.arguments, null, 2)}</pre>}
              <div><button className="danger-button" disabled={busy} onClick={deny}>{text.agents.deny}</button>
                {Date.parse(approval.expires_at) <= Date.now()
                  ? <button className="primary-button" disabled={busy} onClick={() => void act('resume')}>{text.common.refresh}</button>
                  : <button className="primary-button" disabled={busy} onClick={approve}>{text.agents.approve}</button>}
              </div>
            </div> : execution?.status === 'uncertain' ? <div className="approval-card">
              <h2>{text.recovery.uncertain}</h2><p>{text.recovery.verifyOutcome}</p>
              <pre>{JSON.stringify(execution.uncertain_call, null, 2)}</pre>
              <label className="field"><span>{text.recovery.reconcile}</span><textarea value={verifiedResult} onChange={event => setVerifiedResult(event.target.value)} /></label>
              <button className="primary-button" disabled={busy || !verifiedResult.trim()} onClick={() => void act('reconcile')}>{text.recovery.reconcile}</button>
            </div> : result ? <div className="message-body markdown-body agent-answer"><MarkdownMessage content={result} /></div> : !execution && <div className="quiet-state"><Icon name="agents" size={30} /><p>{text.agents.subtitle}</p></div>}
            {(execution?.status === 'interrupted' || execution?.status === 'pending') && execution.resumable && <button className="primary-button" disabled={busy} onClick={() => void act('resume')}>{text.models.resume}</button>}
            {execution?.error && <p role="alert">{execution.error}</p>}
            {steps.length > 0 && (execution?.computer_session ? <BrowserActivity steps={steps} /> : <details className="agent-steps" open={working}><summary>{text.agentRuntime.steps} · {steps.length}</summary>{steps.map((step, index) => <article key={step.id ?? index}><strong>{step.type}{step.tool_name ? ` · ${step.tool_name}` : ''}</strong><p>{step.content || step.tool_result}</p></article>)}</details>)}
            {execution && <AgentPreview run={execution} showPreview={livePreview} />}
            </div>
          </div>
        </section>
      </div>
      <section className="runtime-panel agent-history-panel">
        <div className="section-heading"><span className="eyebrow">{text.agentRuntime.history} · {tasks.length}</span><div className="history-tools"><button className="secondary-button" disabled={loadingRuntime || busy} onClick={() => void refreshRuntime()}>{text.common.refresh}</button><button className="secondary-button" disabled={loadingRuntime || busy || !matchingTasks.some(item => item.deletable)} onClick={() => setDeleteItems(matchingTasks.filter(item => item.deletable).map(item => ({ id: item.id, label: item.prompt })))}>{text.history.clearTasks}</button></div></div>
        <label className="history-search"><Icon name="search" size={15} /><input type="search" value={historyQuery} onChange={event => { setHistoryQuery(event.target.value); setHistoryLimit(20); }} placeholder={text.history.searchTasks} aria-label={text.history.searchTasks} /></label>
        <p className="history-notice">{text.history.protectedTasks}</p>
        {historyNotice && <p className="history-notice" role="status">{historyNotice}</p>}
        {loadingRuntime && tasks.length === 0 ? <p role="status">{interaction[locale].loadingHistory}</p> : historyError && tasks.length === 0 ? <p role="alert">{historyError}</p> : matchingTasks.length === 0 ? <p className="compact-empty">{tasks.length ? text.history.noMatches : text.agentRuntime.noTasks}</p> : <div className="task-history">{matchingTasks.slice(0, historyLimit).map(item => <article key={item.id} className={selectedRun === item.id ? 'selected' : undefined}>
          <i className={item.status} /><div><button className="text-button" aria-current={selectedRun === item.id ? 'true' : undefined} disabled={busy} onClick={() => { selectRun(item.id); setExecution(null); setError(''); setResultCopied(false); }}>{item.prompt}</button><small>{text.recovery[item.status as keyof typeof text.recovery] ?? item.status} · {new Date(item.created_at).toLocaleString(locale)}</small>{item.error && <p>{item.error}</p>}
            <div className="history-row-actions"><button className="text-button" disabled={working || !!approval || !!task.trim()} title={task.trim() ? text.history.draftProtected : text.history.reuseTask} onClick={() => { setTask(item.prompt); document.getElementById('agent-task-input')?.focus(); }}>{text.history.reuseTask}</button><button className="text-button" disabled={busy || !item.deletable} title={item.deletable ? text.history.deleteTask : text.history.protectedTasks} onClick={() => setDeleteItems([{ id: item.id, label: item.prompt }])}><Icon name="trash" size={14} />{text.history.deleteTask}</button></div>
          </div></article>)}</div>}
        {matchingTasks.length > historyLimit && <button className="secondary-button history-more" onClick={() => setHistoryLimit(limit => limit + 20)}>{text.history.showMore} ({matchingTasks.length - historyLimit})</button>}
      </section>
    </div>}

    {view === 'tools' && <div id="agent-tools-panel" className="agent-view" role="tabpanel" aria-labelledby="agent-tools-tab"><section className="runtime-panel"><div className="section-heading"><div><span className="eyebrow">{text.agentRuntime.tools}</span><h2>{enabledTools}/{tools.length} {text.agentRuntime.enabled}</h2></div><button className="secondary-button" onClick={() => void refreshRuntime()}>{text.common.refresh}</button></div>{tools.length === 0 ? <p className="compact-empty">{loadingRuntime ? text.common.loading : runtimeError ? presentation[locale].unknown : text.agentRuntime.noTools}</p> : <div className="tool-list">{tools.map(tool => <article key={tool.name}><div><strong>{tool.name}</strong><p>{tool.description}</p><small>{tool.source}{tool.capability ? ` · ${tool.capability.risk} ${text.agentRuntime.risk}` : ''}</small></div><label className="switch"><input type="checkbox" aria-label={tool.name} checked={tool.enabled} disabled={toolBusy === tool.name} onChange={() => void toggleTool(tool)} /><span /></label></article>)}</div>}</section></div>}

    {view === 'connections' && <div id="agent-connections-panel" className="agent-view" role="tabpanel" aria-labelledby="agent-connections-tab">
      <section className="runtime-panel connector-panel"><div><span className="eyebrow">{text.agentRuntime.connectors}</span><div className="connector-list">{servers.length === 0 ? <p>{loadingRuntime ? text.common.loading : runtimeError ? presentation[locale].unknown : text.agentRuntime.noConnectors}</p> : servers.map(server => <article key={server.name}><i /><div><strong>{server.name}</strong><small>{server.transport} · {server.tools} {text.agentRuntime.tools.toLowerCase()} · {server.status}</small></div></article>)}</div></div><form onSubmit={connect}><label><span>{text.agentRuntime.connectorName}</span><input value={connectionName} onChange={event => setConnectionName(event.target.value)} /></label><label><span>{text.agentRuntime.connectorURL}</span><input type="url" placeholder="http://127.0.0.1:3000/mcp" value={connectionURL} onChange={event => setConnectionURL(event.target.value)} /></label>{connectionMessage && <small className="connection-success">{connectionMessage}</small>}<div><button type="button" className="secondary-button" onClick={() => void testConnection()} disabled={!connectionURL.trim() || connectionBusy !== ''}>{connectionBusy === 'test' ? text.agentRuntime.testing : text.agentRuntime.test}</button><button className="primary-button" disabled={!connectionName.trim() || !connectionURL.trim() || connectionBusy !== ''}>{connectionBusy === 'connect' ? text.agentRuntime.connecting : text.agentRuntime.connect}</button></div></form></section>
      <ExternalProvidersPanel integrations={integrations} loading={loadingRuntime} busy={integrationBusy} setup={integrationSetup} copied={copied} onSetup={showIntegrationSetup} onCopy={copyIntegrationSetup} />
    </div>}
  </div>;
}

type ExternalProvidersPanelProps = {
  integrations: ExternalIntegration[];
  loading: boolean;
  busy: string;
  setup: { id: string; name: string; setup: IntegrationSetup } | null;
  copied: boolean;
  onSetup: (item: ExternalIntegration) => void;
  onCopy: () => void;
};

function ExternalProvidersPanel({ integrations, loading, busy, setup, copied, onSetup, onCopy }: ExternalProvidersPanelProps) {
  const { messages: text } = useI18n();
  const managedHermes = setup?.id === 'hermes';
  return <section className="runtime-panel external-providers">
    <div className="section-heading"><div><span className="eyebrow">{text.agentRuntime.externalProviders}</span><h2>{text.agentRuntime.providerPluginsTitle}</h2><p>{text.agentRuntime.externalProvidersBody}</p></div></div>
    {!loading && integrations.length === 0 && <p className="compact-empty">{text.agentRuntime.noExternalProviders}</p>}
    <div className="provider-grid">{integrations.map(item => <article key={item.id} className="provider-card">
      <div><span className={item.ready ? 'status-pill' : 'status-pill warning'}>{item.ready ? text.agentRuntime.providerReady : text.agentRuntime.providerNeedsSetup}</span><h3>{item.name}</h3><p>{item.description}</p></div>
      <dl><div><dt>{text.agentRuntime.provider}</dt><dd>{item.provider_id}</dd></div><div><dt>{text.agentRuntime.transport}</dt><dd>{item.transport}</dd></div><div><dt>{text.agentRuntime.model}</dt><dd>{item.model_id ?? '—'}</dd></div><div><dt>{text.agentRuntime.context}</dt><dd>{item.context_window.toLocaleString()} / {item.minimum_context.toLocaleString()}</dd></div></dl>
      {item.warnings.length > 0 && <ul>{item.warnings.map(warning => <li key={warning}>{warning}</li>)}</ul>}
      <button className="secondary-button" disabled={busy === item.id || !item.model_id} onClick={() => onSetup(item)}>{busy === item.id ? text.common.loading : text.agentRuntime.generateSetup}</button>
    </article>)}</div>
    {setup && <div className="provider-setup"><div className="section-heading"><div><span className="eyebrow">{setup.name}</span><h3>{text.agentRuntime.configuration}</h3></div><button className="secondary-button" onClick={onCopy}>{copied ? text.agentRuntime.copied : text.agentRuntime.copy}</button></div><label>{text.agentRuntime.installPlugin}</label><pre>{setup.setup.install_command}</pre>{!managedHermes && <>{Object.keys(setup.setup.environment ?? {}).length > 0 && <><label>{text.agentRuntime.environment}</label><pre>{Object.entries(setup.setup.environment ?? {}).map(([key, value]) => `export ${key}=${JSON.stringify(value)}`).join('\n')}</pre></>}<label>{setup.setup.config_file}</label><pre>{setup.setup.content}</pre></>}<label>{text.agentRuntime.verification}</label><pre>{setup.setup.verify.join('\n')}</pre>{(setup.setup.notes?.length ?? 0) > 0 && <div className="provider-notes"><strong>{text.agentRuntime.notes}</strong><ul>{setup.setup.notes?.map(note => <li key={note}>{note}</li>)}</ul></div>}</div>}
  </section>;
}

function Metric({ label, value, danger = false }: { label: string; value: string; danger?: boolean }) { return <article className={danger ? 'metric warning' : 'metric'}><span>{label}</span><strong>{value}</strong></article>; }
