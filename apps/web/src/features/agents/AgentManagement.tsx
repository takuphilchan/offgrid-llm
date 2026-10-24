import { useEffect, useRef, useState, type FormEvent } from 'react';
import { externalIntegrations } from '../../i18n/external-integrations';
import { api, type AgentTool, type ExternalIntegration, type IntegrationSetup, type MCPServer } from '../../api/client';
import { Icon } from '../../components/Icon';
import { ConfirmDialog } from '../../components/ConfirmDialog';
import { ActionGroup, EmptyState, SectionHeading } from '../../components/WorkspacePresentation';
import { useI18n } from '../../i18n';
import { presentation } from '../../i18n/presentation';
import { mcpConnectionText } from '../../i18n/mcp-connections';
import { useWorkspaceState } from '../../lib/workspace-context';
import { useWorkspaceRefresh } from '../../lib/workspace-refresh';
import { AgentNavigation } from './AgentNavigation';

// Management does not mount a task runner or poll execution/history endpoints.
export function AgentManagement({ model, view }: { model: string; view: 'tools' | 'connections' }) {
  const { messages: text, locale } = useI18n();
  const [tools, setTools] = useState<AgentTool[]>([]);
  const [enabledTools, setEnabledTools] = useState(0);
  const [servers, setServers] = useState<MCPServer[]>([]);
  const [loadingRuntime, setLoadingRuntime] = useState(true);
  const [toolBusy, setToolBusy] = useState('');
  const [error, setError] = useState('');
  const [runtimeError, setRuntimeError] = useState('');
  const [connectionName, setConnectionName] = useWorkspaceState('agent.connectorName', '');
  const [connectionURL, setConnectionURL] = useWorkspaceState('agent.connectorURL', '');
  const [connectionBusy, setConnectionBusy] = useState<'test' | 'connect' | 'remove' | ''>('');
  const [removeServer, setRemoveServer] = useState<MCPServer | null>(null);
  const mcpText = mcpConnectionText(locale);
  const [connectionMessage, setConnectionMessage] = useState('');
  const [integrations, setIntegrations] = useState<ExternalIntegration[]>([]);
  const [integrationSetup, setIntegrationSetup] = useState<{ id: string; name: string; setup: IntegrationSetup } | null>(null);
  const [integrationBusy, setIntegrationBusy] = useState('');
  const [copied, setCopied] = useState(false);
  const runtimeRequest = useRef(0);
  const integrationRequest = useRef(0);

  const refreshRuntime = async () => {
    const request = ++runtimeRequest.current;
    setLoadingRuntime(true); setRuntimeError('');
    const [toolResult, serverResult, integrationResult] = await Promise.allSettled([api.agentTools(), api.mcpServers(), api.integrations(model)]);
    if (request !== runtimeRequest.current) return;
    if (toolResult.status === 'fulfilled') { setTools(toolResult.value.tools); setEnabledTools(toolResult.value.enabled_count); }
    if (serverResult.status === 'fulfilled') setServers(serverResult.value);
    if (integrationResult.status === 'fulfilled') setIntegrations(integrationResult.value.integrations);
    const failed = [toolResult, serverResult, integrationResult].find(item => item.status === 'rejected');
    if (failed?.status === 'rejected') setRuntimeError(failed.reason instanceof Error ? failed.reason.message : text.common.error);
    setLoadingRuntime(false);
  };
  useEffect(() => {
    integrationRequest.current++; setIntegrationBusy(''); setIntegrationSetup(null);
    void refreshRuntime();
    return () => { integrationRequest.current++; runtimeRequest.current++; };
  }, [model]);
  useWorkspaceRefresh(refreshRuntime);

  const toggleTool = async (tool: AgentTool) => {
    setToolBusy(tool.name); setError('');
    try { await api.setAgentToolEnabled(tool.name, !tool.enabled); await refreshRuntime(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); }
    finally { setToolBusy(''); }
  };
  const testConnection = async () => {
    if (!connectionURL.trim() || connectionBusy || removeServer) return;
    setConnectionBusy('test'); setConnectionMessage(''); setError('');
    try { const response = await api.testMCP(connectionURL.trim()); setConnectionMessage(`${response.tools_count} ${text.agentRuntime.tools.toLowerCase()}`); }
    catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); }
    finally { setConnectionBusy(''); }
  };
  const connect = async (event: FormEvent) => {
    event.preventDefault();
    if (!connectionName.trim() || !connectionURL.trim() || connectionBusy || removeServer) return;
    setConnectionBusy('connect'); setConnectionMessage(''); setError('');
    try {
      const response = await api.connectMCP(connectionName.trim(), connectionURL.trim());
      setConnectionMessage(`${response.tools_added} ${text.agentRuntime.tools.toLowerCase()}`);
      setConnectionName(''); setConnectionURL('');
      await refreshRuntime();
    } catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); }
    finally { setConnectionBusy(''); }
  };

  const removeConnection = async () => {
    if (!removeServer || connectionBusy) return;
    setConnectionBusy('remove'); setConnectionMessage('');
    try {
      await api.removeMCP(removeServer.name);
      // Invalidate older refreshes before updating the list. A later refresh
      // failure must not resurrect a connection that was successfully removed.
      runtimeRequest.current++;
      setServers(current => current.filter(server => server.name !== removeServer.name));
      setConnectionMessage(mcpText.removed);
      await refreshRuntime();
    } finally { setConnectionBusy(''); }
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
      await navigator.clipboard.writeText(integrationSetup.setup.install_command);
      setCopied(true);
    } catch (reason) {
      setError(reason instanceof Error ? reason.message : text.common.error);
    }
  };


  return <div className="agents-page agent-management-page">
    <AgentNavigation view={view} />
    {runtimeError && <p className="inline-error" role="alert">{runtimeError}</p>}
    {error && <div className="inline-error" role="alert">{error}</div>}
    {view === 'tools' && <div id="agent-tools-panel" className="agent-view" role="region" aria-labelledby="agent-tools-tab"><section className="runtime-panel management-list-panel">
      <SectionHeading title={text.agentRuntime.tools} description={`${enabledTools}/${tools.length} ${text.agentRuntime.enabled}`} />
      {tools.length === 0 ? loadingRuntime ? <p role="status">{text.common.loading}</p> : runtimeError ? null : <EmptyState title={text.agentRuntime.noTools} /> : <div className="tool-list">{tools.map(tool => <article key={tool.name}><div><h3>{tool.name}</h3><p>{tool.description}</p><details><summary>{presentation[locale].details}</summary><small>{tool.source}{tool.capability ? ` · ${tool.capability.risk} ${text.agentRuntime.risk}` : ''}</small></details></div><label className="switch"><input type="checkbox" aria-label={tool.name} checked={tool.enabled} disabled={toolBusy === tool.name} onChange={() => void toggleTool(tool)} /><span /></label></article>)}</div>}
    </section></div>}

    {view === 'connections' && <div id="agent-connections-panel" className="agent-view" role="region" aria-labelledby="agent-connections-tab">
      <section className="runtime-panel connector-panel">
        <div>
          <SectionHeading title={text.agentRuntime.connectors} />
          <div className="connector-list">
            {servers.length === 0 ? loadingRuntime ? <p role="status">{text.common.loading}</p> : runtimeError ? null : <EmptyState title={text.agentRuntime.noConnectors} /> : servers.map(server => <article key={server.name}>
              <i aria-hidden="true" data-connected={server.status === 'connected'} />
              <div>
                <h3>{server.name}</h3>
                <small>{server.transport} · {server.tools} {text.agentRuntime.tools.toLowerCase()} · {server.status === 'connected' ? mcpText.connected : server.status === 'disabled' ? mcpText.disabled : mcpText.disconnected}</small>
                <ActionGroup><button type="button" className="secondary-button connector-remove" aria-label={`${mcpText.remove}: ${server.name}`} disabled={connectionBusy !== ''} onClick={() => setRemoveServer(server)}><Icon name="trash" size={14} />{mcpText.remove}</button></ActionGroup>
              </div>
            </article>)}
          </div>
          {connectionMessage && <p className="connection-success" role="status">{connectionMessage}</p>}
        </div>
        <form onSubmit={connect} aria-label={text.agentRuntime.connect}>
          <h3>{text.agentRuntime.connect}</h3>
          <label><span>{text.agentRuntime.connectorName}</span><input value={connectionName} onChange={event => setConnectionName(event.target.value)} /></label>
          <label><span>{text.agentRuntime.connectorURL}</span><input type="url" placeholder="https://mcp.example.com/mcp" value={connectionURL} onChange={event => setConnectionURL(event.target.value)} /></label>
          <ActionGroup>
            <button type="button" className="secondary-button" onClick={() => void testConnection()} disabled={!connectionURL.trim() || connectionBusy !== ''}>{connectionBusy === 'test' ? text.agentRuntime.testing : text.agentRuntime.test}</button>
            <button className="primary-button" disabled={!connectionName.trim() || !connectionURL.trim() || connectionBusy !== ''}>{connectionBusy === 'connect' ? text.agentRuntime.connecting : text.agentRuntime.connect}</button>
          </ActionGroup>
        </form>
      </section>
      {removeServer && <ConfirmDialog title={mcpText.title.replace('{name}', removeServer.name)} body={mcpText.body} confirmLabel={mcpText.remove} confirm={removeConnection} close={() => setRemoveServer(null)} />}
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
  const { locale } = useI18n();
  const labels = externalIntegrations[locale];
  return <section className="runtime-panel external-providers">
    <SectionHeading title={text.agentRuntime.providerPluginsTitle} description={text.agentRuntime.externalProvidersBody} />
    {!loading && integrations.length === 0 && <EmptyState title={text.agentRuntime.noExternalProviders} />}
    <div className="provider-grid">{integrations.map(item => <article key={item.id} className="provider-card">
      <div><span className={item.ready ? 'status-pill' : 'status-pill warning'}>{item.ready ? labels.ready : labels.blocked}</span><h3>{item.name}</h3><p>{item.description}</p></div>
      <dl><div><dt>{text.agentRuntime.provider}</dt><dd>{item.provider_id}</dd></div><div><dt>{text.agentRuntime.transport}</dt><dd>{item.transport}</dd></div><div><dt>{text.agentRuntime.model}</dt><dd>{item.model_id ?? '—'}</dd></div><div><dt>{text.agentRuntime.context}</dt><dd>{item.context_window.toLocaleString()} / {item.minimum_context.toLocaleString()}</dd></div></dl>
      <p>{labels.host}<br />{labels.workflow}</p>
      {item.warnings.length > 0 && <ul>{item.warnings.map(warning => <li key={warning}>{warning}</li>)}</ul>}
      <button className="secondary-button" disabled={busy === item.id || !item.model_id} onClick={() => onSetup(item)}>{busy === item.id ? text.common.loading : text.agentRuntime.generateSetup}</button>
    </article>)}</div>
    {setup && <div className="provider-setup"><div className="section-heading"><div><span className="eyebrow">{setup.name}</span><h3>{text.agentRuntime.configuration}</h3></div><button className="secondary-button" onClick={onCopy}>{copied ? text.agentRuntime.copied : text.agentRuntime.copy}</button></div><label>{text.agentRuntime.installPlugin}</label><pre>{setup.setup.install_command}</pre><details><summary>{labels.manual}</summary>{Object.keys(setup.setup.environment ?? {}).length > 0 && <><label>{text.agentRuntime.environment}</label><pre>{Object.entries(setup.setup.environment ?? {}).map(([key, value]) => `export ${key}=${JSON.stringify(value)}`).join('\n')}</pre></>}<label>{setup.setup.config_file}</label><pre>{setup.setup.content}</pre></details><label>{text.agentRuntime.verification}</label><pre>{setup.setup.verify.join('\n')}</pre>{(setup.setup.notes?.length ?? 0) > 0 && <div className="provider-notes"><strong>{text.agentRuntime.notes}</strong><ul>{setup.setup.notes?.map(note => <li key={note}>{note}</li>)}</ul></div>}</div>}
  </section>;
}
