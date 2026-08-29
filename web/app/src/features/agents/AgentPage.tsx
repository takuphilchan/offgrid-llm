import { useEffect, useMemo, useState, type FormEvent } from 'react';
import { APIError, api, type AgentStep, type AgentTask, type AgentTool, type ComputerStatus, type MCPServer, type Model, type ToolApproval } from '../../api/client';
import { Icon } from '../../components/Icon';
import { ModelSelect } from '../../components/ModelSelect';
import { useI18n } from '../../i18n';

export function AgentPage({ models, model, setModel }: { models: Model[]; model: string; setModel: (model: string) => void }) {
  const { messages: text } = useI18n();
  const [task, setTask] = useState('');
  const [style, setStyle] = useState('react');
  const [busy, setBusy] = useState(false);
  const [result, setResult] = useState('');
  const [steps, setSteps] = useState<AgentStep[]>([]);
  const [error, setError] = useState('');
  const [approval, setApproval] = useState<ToolApproval | null>(null);
  const [grants, setGrants] = useState<ToolApproval[]>([]);
  const [tools, setTools] = useState<AgentTool[]>([]);
  const [enabledTools, setEnabledTools] = useState(0);
  const [tasks, setTasks] = useState<AgentTask[]>([]);
  const [servers, setServers] = useState<MCPServer[]>([]);
  const [computer, setComputer] = useState<ComputerStatus | null>(null);
  const [loadingRuntime, setLoadingRuntime] = useState(true);
  const [toolBusy, setToolBusy] = useState('');
  const [connectionName, setConnectionName] = useState('');
  const [connectionURL, setConnectionURL] = useState('');
  const [connectionBusy, setConnectionBusy] = useState<'test' | 'connect' | ''>('');
  const [connectionMessage, setConnectionMessage] = useState('');

  const refreshRuntime = async () => {
    setLoadingRuntime(true);
    const [toolResult, taskResult, serverResult, computerResult] = await Promise.allSettled([api.agentTools(), api.agentTasks(), api.mcpServers(), api.computerStatus()]);
    if (toolResult.status === 'fulfilled') { setTools(toolResult.value.tools); setEnabledTools(toolResult.value.enabled_count); }
    if (taskResult.status === 'fulfilled') setTasks([...taskResult.value].sort((a, b) => Date.parse(b.created_at) - Date.parse(a.created_at)));
    if (serverResult.status === 'fulfilled') setServers(serverResult.value);
    if (computerResult.status === 'fulfilled') setComputer(computerResult.value);
    const failed = [toolResult, taskResult, serverResult, computerResult].find(item => item.status === 'rejected');
    if (failed?.status === 'rejected') setError(failed.reason instanceof Error ? failed.reason.message : text.common.error);
    setLoadingRuntime(false);
  };

  useEffect(() => { void refreshRuntime(); }, []);

  const execute = async (approved: ToolApproval[]) => {
    if (!task.trim() || !model) return;
    setBusy(true); setResult(''); setSteps([]); setError(''); setApproval(null);
    try {
      const response = await api.runAgent(model, task.trim(), style, approved);
      setResult(response.output);
      setSteps(Array.isArray(response.steps) ? response.steps : []);
      await refreshRuntime();
    } catch (reason) {
      if (reason instanceof APIError && reason.status === 409 && reason.data?.run_id) {
        try {
          const events = await api.runEvents(String(reason.data.run_id));
          const request = [...events].reverse().find(item => item.type === 'approval.required');
          if (request?.data?.tool) { setApproval({ tool: String(request.data.tool), arguments: request.data.arguments ?? {} }); await refreshRuntime(); return; }
        } catch { /* Preserve the policy error below. */ }
      }
      setError(reason instanceof Error ? reason.message : text.common.error);
      await refreshRuntime();
    } finally { setBusy(false); }
  };

  const run = (event: FormEvent) => { event.preventDefault(); setGrants([]); void execute([]); };
  const approve = () => { if (!approval) return; const next = [...grants, approval]; setGrants(next); void execute(next); };
  const deny = () => { setApproval(null); setError(text.agents.denied); };
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

  const recentTasks = useMemo(() => tasks.slice(0, 8), [tasks]);
  return <div className="stack agents-page">
    {error && <div className="inline-error" role="alert">{error}</div>}
    <div className="metric-grid agent-metrics">
      <Metric label={text.agentRuntime.tools} value={loadingRuntime ? '…' : `${enabledTools}/${tools.length} ${text.agentRuntime.enabled}`} />
      <Metric label={text.agentRuntime.history} value={loadingRuntime ? '…' : String(tasks.length)} />
      <Metric label={text.agentRuntime.connectors} value={loadingRuntime ? '…' : String(servers.length)} />
      <Metric label={text.agentRuntime.computer} value={computer?.available ? text.agentRuntime.available : text.agentRuntime.unavailable} danger={computer?.available === false} />
    </div>

    <div className="agent-workspace"><form className="task-card" onSubmit={run}><div className="form-row"><ModelSelect models={models} value={model} onChange={setModel} /></div><label><span>{text.agentRuntime.style}</span><select value={style} onChange={event => setStyle(event.target.value)}><option value="react">{text.agentRuntime.react}</option><option value="plan-execute">{text.agentRuntime.plan}</option><option value="cot">{text.agentRuntime.reasoning}</option></select></label><label><span>{text.agents.task}</span><textarea rows={7} value={task} onChange={event => setTask(event.target.value)} placeholder={text.agents.placeholder} /></label><button className="primary-button" disabled={busy || !task.trim() || !model}>{busy ? text.agents.running : text.agents.run}</button></form><section className="result-card"><span className="eyebrow">{text.agents.result}</span>{approval ? <div className="approval-card" role="alertdialog" aria-labelledby="approval-title"><span className="status-pill danger">{text.agents.approvalTitle}</span><h2 id="approval-title">{approval.tool}</h2><p>{text.agents.approvalBody}</p><pre>{JSON.stringify(approval.arguments, null, 2)}</pre><div><button className="danger-button" onClick={deny}>{text.agents.deny}</button><button className="primary-button" onClick={approve}>{text.agents.approve}</button></div></div> : result ? <><pre>{result}</pre>{steps.length > 0 && <div className="agent-steps"><span className="eyebrow">{text.agentRuntime.steps}</span>{steps.map((step, index) => <article key={step.id ?? index}><strong>{step.type ?? `#${index + 1}`}{step.tool_name ? ` · ${step.tool_name}` : ''}</strong><p>{step.content || step.tool_result}</p></article>)}</div>}</> : <div className="quiet-state"><Icon name="agents" size={30} /><p>{text.agents.subtitle}</p></div>}</section></div>

    <div className="agent-runtime-grid">
      <section className="runtime-panel"><div className="section-heading"><div><span className="eyebrow">{text.agentRuntime.tools}</span><h2>{enabledTools}/{tools.length} {text.agentRuntime.enabled}</h2></div><button className="secondary-button" onClick={() => void refreshRuntime()}>{text.common.refresh}</button></div>{tools.length === 0 ? <p className="compact-empty">{text.agentRuntime.noTools}</p> : <div className="tool-list">{tools.map(tool => <article key={tool.name}><div><strong>{tool.name}</strong><p>{tool.description}</p><small>{tool.source}{tool.capability ? ` · ${tool.capability.risk} ${text.agentRuntime.risk}` : ''}</small></div><label className="switch"><input type="checkbox" checked={tool.enabled} disabled={toolBusy === tool.name} onChange={() => void toggleTool(tool)} /><span /></label></article>)}</div>}</section>
      <section className="runtime-panel"><span className="eyebrow">{text.agentRuntime.history}</span>{recentTasks.length === 0 ? <p className="compact-empty">{text.agentRuntime.noTasks}</p> : <div className="task-history">{recentTasks.map(item => <article key={item.id}><i className={item.status} /><div><strong>{item.prompt}</strong><small>{item.status} · {new Date(item.created_at).toLocaleString()}</small>{item.error && <p>{item.error}</p>}</div></article>)}</div>}</section>
    </div>

    <section className="runtime-panel connector-panel"><div><span className="eyebrow">{text.agentRuntime.connectors}</span><div className="connector-list">{servers.length === 0 ? <p>{text.agentRuntime.noConnectors}</p> : servers.map(server => <article key={server.name}><i /><div><strong>{server.name}</strong><small>{server.transport} · {server.tools} {text.agentRuntime.tools.toLowerCase()} · {server.status}</small></div></article>)}</div></div><form onSubmit={connect}><label><span>{text.agentRuntime.connectorName}</span><input value={connectionName} onChange={event => setConnectionName(event.target.value)} /></label><label><span>{text.agentRuntime.connectorURL}</span><input type="url" placeholder="http://127.0.0.1:3000/mcp" value={connectionURL} onChange={event => setConnectionURL(event.target.value)} /></label>{connectionMessage && <small className="connection-success">{connectionMessage}</small>}<div><button type="button" className="secondary-button" onClick={() => void testConnection()} disabled={!connectionURL.trim() || connectionBusy !== ''}>{connectionBusy === 'test' ? text.agentRuntime.testing : text.agentRuntime.test}</button><button className="primary-button" disabled={!connectionName.trim() || !connectionURL.trim() || connectionBusy !== ''}>{connectionBusy === 'connect' ? text.agentRuntime.connecting : text.agentRuntime.connect}</button></div></form></section>
  </div>;
}

function Metric({ label, value, danger = false }: { label: string; value: string; danger?: boolean }) { return <article className={danger ? 'metric warning' : 'metric'}><span>{label}</span><strong>{value}</strong></article>; }
