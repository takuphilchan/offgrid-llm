import { useEffect, useRef, useState, type FormEvent } from 'react';
import { externalIntegrations, externalSetupLabel } from '../../i18n/external-integrations';
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
import { SearchField } from '../../components/SearchField';
import { ProviderSetupDialog } from './ProviderSetupDialog';
import { agentManagement } from '../../i18n/agent-management';

// Management does not mount a task runner or poll execution/history endpoints.
export function AgentManagement({ model, view }: { model: string; view: 'tools' | 'connections' | 'external' }) {
  const { messages: text, locale , language} = useI18n();
  const [tools, setTools] = useState<AgentTool[]>([]);
  const [toolQuery, setToolQuery] = useWorkspaceState('agent.tools.filter', '');
  const visibleTools = tools.filter(tool => `${tool.name} ${tool.description} ${tool.source}`.toLocaleLowerCase(language).includes(toolQuery.trim().toLocaleLowerCase(language)));
  const [enabledTools, setEnabledTools] = useState(0);
  const [servers, setServers] = useState<MCPServer[]>([]);
  const [loadingRuntime, setLoadingRuntime] = useState(true);
  const [toolBusy, setToolBusy] = useState('');
  const [error, setError] = useState('');
  const [runtimeError, setRuntimeError] = useState('');
  const [connectionError, setConnectionError] = useState('');
  const [integrationError, setIntegrationError] = useState('');
  const [connectionName, setConnectionName] = useWorkspaceState('agent.connectorName', '');
  const [connectionURL, setConnectionURL] = useWorkspaceState('agent.connectorURL', '');
  const [connectionBusy, setConnectionBusy] = useState<'test' | 'connect' | 'remove' | ''>('');
  const [removeServer, setRemoveServer] = useState<MCPServer | null>(null);
  const mcpText = mcpConnectionText(locale);
  const management = agentManagement[locale];
  const [addingConnection, setAddingConnection] = useWorkspaceState('agent.connectorOpen', view === 'connections');
  const [connectionMessage, setConnectionMessage] = useState('');
  const [integrations, setIntegrations] = useState<ExternalIntegration[]>([]);
  const [integrationSetup, setIntegrationSetup] = useState<{ id: string; name: string; setup: IntegrationSetup } | null>(null);
  const [integrationBusy, setIntegrationBusy] = useState('');
  const [copied, setCopied] = useState(false);
  const runtimeRequest = useRef(0);
  const integrationRequest = useRef(0);

  const refreshRuntime = async () => {
    const request = ++runtimeRequest.current;
    setLoadingRuntime(true); setRuntimeError(''); setConnectionError(''); setIntegrationError('');
    const [toolResult, serverResult, integrationResult] = await Promise.allSettled([
      view !== 'external' ? api.agentTools() : Promise.resolve(null),
      view !== 'external' ? api.mcpServers() : Promise.resolve(null),
      view === 'external' ? api.integrations(model) : Promise.resolve(null),
    ]);
    if (request !== runtimeRequest.current) return;
    if (toolResult.status === 'fulfilled' && toolResult.value) { setTools(toolResult.value.tools); setEnabledTools(toolResult.value.enabled_count); }
    if (serverResult.status === 'fulfilled' && serverResult.value) setServers(serverResult.value);
    if (integrationResult.status === 'fulfilled' && integrationResult.value) setIntegrations(integrationResult.value.integrations);
    if (toolResult.status === 'rejected') setRuntimeError(toolResult.reason instanceof Error ? toolResult.reason.message : text.common.error);
    if (serverResult.status === 'rejected') setConnectionError(serverResult.reason instanceof Error ? serverResult.reason.message : text.common.error);
    if (integrationResult.status === 'rejected') setIntegrationError(integrationResult.reason instanceof Error ? integrationResult.reason.message : text.common.error);
    setLoadingRuntime(false);
  };
  useEffect(() => {
    integrationRequest.current++; setIntegrationBusy(''); setIntegrationSetup(null);
    if (view === 'connections') setAddingConnection(true);
    void refreshRuntime();
    return () => { integrationRequest.current++; runtimeRequest.current++; };
  }, [model, view]);
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
    {runtimeError && <div className="inline-error" role="alert">{runtimeError} <button type="button" className="secondary-button" disabled={loadingRuntime} onClick={() => void refreshRuntime()}>{text.common.retry}</button></div>}
    {error && <div className="inline-error" role="alert">{error}</div>}
    {view !== 'external' && <div id="agent-connections-panel" className="agent-view" role="region" aria-label={text.agentRuntime.connectors}>
      <section className="runtime-panel connector-panel">
        <div>
          <SectionHeading title={text.agentRuntime.connectors} description={management.connections} actions={<button type="button" className="secondary-button library-create-action" aria-expanded={addingConnection} aria-controls="mcp-connect-form" disabled={connectionBusy !== ''} onClick={() => setAddingConnection(value => !value)}><Icon name={addingConnection ? 'close' : 'plus'} size={16} />{addingConnection ? text.models.cancel : text.agentRuntime.connect}</button>} />
          <div className="connector-list">
            {connectionError && <div className="inline-error" role="alert">{connectionError} <button type="button" className="secondary-button" disabled={loadingRuntime} onClick={() => void refreshRuntime()}>{text.common.retry}</button></div>}
            {servers.length === 0 ? loadingRuntime ? <p role="status">{text.common.loading}</p> : connectionError ? null : <p className="management-empty">{text.agentRuntime.noConnectors}</p> : servers.map(server => <article key={server.name}>
              <i aria-hidden="true" data-connected={server.status === 'connected'} />
              <div>
                <h3>{server.name}</h3>
                <small>{server.transport} · {server.tools} {text.agentRuntime.tools.toLowerCase()} · {server.status === 'connected' ? mcpText.connected : server.status === 'disabled' ? mcpText.disabled : mcpText.disconnected}</small>
              </div>
              <button type="button" className="text-button connector-remove" aria-label={`${mcpText.remove}: ${server.name}`} disabled={connectionBusy !== ''} onClick={() => setRemoveServer(server)}><Icon name="trash" size={14} />{mcpText.remove}</button>
            </article>)}
          </div>
          {connectionMessage && <p className="connection-success" role="status">{connectionMessage}</p>}
        </div>
        {addingConnection && <form id="mcp-connect-form" onSubmit={connect} aria-label={text.agentRuntime.connect}>
          <h3>{text.agentRuntime.connect}</h3>
          <label><span>{text.agentRuntime.connectorName}</span><input disabled={connectionBusy !== ''} value={connectionName} onChange={event => { setConnectionName(event.target.value); setConnectionMessage(''); }} /></label>
          <label><span>{text.agentRuntime.connectorURL}</span><input type="url" disabled={connectionBusy !== ''} placeholder="https://mcp.example.com/mcp" value={connectionURL} onChange={event => { setConnectionURL(event.target.value); setConnectionMessage(''); }} /></label>
          <ActionGroup>
            <button type="button" className="secondary-button" onClick={() => void testConnection()} disabled={!connectionURL.trim() || connectionBusy !== ''}>{connectionBusy === 'test' ? text.agentRuntime.testing : text.agentRuntime.test}</button>
            <button className="primary-button" disabled={!connectionName.trim() || !connectionURL.trim() || connectionBusy !== ''}>{connectionBusy === 'connect' ? text.agentRuntime.connecting : text.agentRuntime.connect}</button>
          </ActionGroup>
        </form>}
      </section>
      {removeServer && <ConfirmDialog title={mcpText.title.replace('{name}', removeServer.name)} body={mcpText.body} confirmLabel={mcpText.remove} confirm={removeConnection} close={() => setRemoveServer(null)} />}
    </div>}
    {view !== 'external' && <div id="agent-tools-panel" className="agent-view" role="region" aria-labelledby="agent-tools-tab"><section className="runtime-panel management-list-panel">
      <SectionHeading title={text.agentRuntime.tools} description={`${enabledTools}/${tools.length} ${text.agentRuntime.enabled}`} actions={!!tools.length && <SearchField label={text.agentRuntime.tools} value={toolQuery} onChange={setToolQuery} />} />
      {!!tools.length && !visibleTools.length && <p role="status">{text.history.noMatches}</p>}
      {tools.length === 0 ? loadingRuntime ? <p role="status">{text.common.loading}</p> : runtimeError ? null : <p className="management-empty">{text.agentRuntime.noTools}</p> : <div className="tool-list">{visibleTools.map(tool => <article key={tool.name}><div><h3>{tool.name}</h3><p>{tool.description}</p><details><summary>{presentation[locale].details}</summary><small>{tool.source}{tool.capability ? ` · ${tool.capability.risk} ${text.agentRuntime.risk}` : ''}</small></details></div><label className="switch"><input type="checkbox" aria-label={tool.name} checked={tool.enabled} disabled={toolBusy === tool.name} onChange={() => void toggleTool(tool)} /><span /></label></article>)}</div>}
    </section></div>}

    {view === 'external' && <div className="agent-view" role="region" aria-labelledby="agent-external-tab"><ExternalProvidersPanel integrations={integrations} loading={loadingRuntime} error={integrationError} onRetry={() => void refreshRuntime()} busy={integrationBusy} setup={integrationSetup} copied={copied} onSetup={showIntegrationSetup} onCopy={copyIntegrationSetup} onClose={() => setIntegrationSetup(null)} /></div>}
  </div>;
}

type ExternalProvidersPanelProps = {
  integrations: ExternalIntegration[];
  loading: boolean;
  error: string;
  onRetry: () => void;
  busy: string;
  setup: { id: string; name: string; setup: IntegrationSetup } | null;
  copied: boolean;
  onSetup: (item: ExternalIntegration) => void;
  onCopy: () => void;
  onClose: () => void;
};

function ExternalProvidersPanel({ integrations, loading, error, onRetry, busy, setup, copied, onSetup, onCopy, onClose }: ExternalProvidersPanelProps) {
  const { messages: text } = useI18n();
  const { locale , language} = useI18n();
  const labels = externalIntegrations[locale];
  const setupTrigger = useRef<HTMLButtonElement | null>(null);
  return <section className="runtime-panel external-providers">
    <SectionHeading title={agentManagement[locale].external} description={text.agentRuntime.externalProvidersBody} />
    {loading && <p role="status">{text.common.loading}</p>}
    {error && <div className="inline-error" role="alert">{error} <button type="button" className="secondary-button" disabled={loading} onClick={onRetry}>{text.common.retry}</button></div>}
    {!loading && !error && integrations.length === 0 && <EmptyState title={text.agentRuntime.noExternalProviders} />}
    <div className="provider-grid">{integrations.map(item => <article key={item.id} className="provider-card">
      <header><h3>{item.name}</h3><p>{item.description}</p></header>
      <div className="provider-readiness">
        <p className="provider-readiness-label"><Icon name={item.ready ? 'check' : 'warning'} size={16} />{item.ready ? labels.ready : labels.blocked}</p>
        {item.warnings.length > 0 && <ul>{item.warnings.map(warning => <li key={warning}>{warning}</li>)}</ul>}
        <p className="provider-host-note">{labels.host}</p>
      </div>
      <details><summary>{presentation[locale].details}</summary><dl><div><dt>{text.agentRuntime.provider}</dt><dd>{item.provider_id}</dd></div><div><dt>{text.agentRuntime.transport}</dt><dd>{item.transport}</dd></div><div><dt>{text.agentRuntime.model}</dt><dd>{item.model_id ?? '—'}</dd></div><div><dt>{text.agentRuntime.context}</dt><dd>{item.context_window.toLocaleString(language)} / {item.minimum_context.toLocaleString(language)}</dd></div></dl><p>{labels.workflow}</p></details>
      <footer><button className="secondary-button" disabled={busy === item.id || !item.model_id} onClick={event => { setupTrigger.current = event.currentTarget; onSetup(item); }}>{busy === item.id ? text.common.loading : externalSetupLabel[locale]}<Icon name="chevron-right" size={15} /></button></footer>
    </article>)}</div>
    {setup && <ProviderSetupDialog key={setup.id} name={setup.name} setup={setup.setup} copied={copied} onCopy={onCopy} close={onClose} returnFocus={setupTrigger.current} />}
  </section>;
}
