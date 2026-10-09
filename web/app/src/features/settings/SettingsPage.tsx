import { useWorkspace } from '../../lib/workspace-context';
import { interaction } from '../../i18n/interaction';
import { useWorkspaceRefresh } from '../../lib/workspace-refresh';
import { useEffect, useRef, useState } from 'react';
import { api } from '../../api/client';
import { useI18n } from '../../i18n';
import { presentation } from '../../i18n/presentation';
import type { DesktopPaths, DesktopBackend } from '../../platform';
import type { ThemeChoice } from '../../theme';
import { SectionHeading } from '../../components/WorkspacePresentation';
import { workspaceManagement } from '../../i18n/workspace-management';
import { desktopCompatibility } from '../../i18n/desktop-compatibility';

type Health = 'checking' | 'ready' | 'offline';
type Details = {
  identity?: Awaited<ReturnType<typeof api.systemIdentity>>;
  config?: Awaited<ReturnType<typeof api.systemConfig>>;
  rag?: Awaited<ReturnType<typeof api.ragStatus>>;
  computer?: Awaited<ReturnType<typeof api.computerStatus>>;
};

export function SettingsPage({ health, themeChoice, onThemeChange, onShowOnboarding }: {
  health: Health;
  themeChoice: ThemeChoice;
  onThemeChange: (choice: ThemeChoice) => void;
  onShowOnboarding: () => void;
}) {
  const { messages: text, locale } = useI18n();
  const { admin, knowledge } = useWorkspace();
  const [details, setDetails] = useState<Details>({});
  const [error, setError] = useState('');
  const [desktopPaths, setDesktopPaths] = useState<DesktopPaths | null>(null);
  const [backend, setBackend] = useState<DesktopBackend | null>(null);
  const [loading, setLoading] = useState(true);
  const [stopping, setStopping] = useState(false);
  const stopPending = useRef(false);
  const refreshRevision = useRef(0);
  const management = workspaceManagement(locale);
  const compatibility = desktopCompatibility(locale);
  const unknown = loading ? text.common.loading : presentation[locale].unknown;

  const refresh = async () => {
    const revision = ++refreshRevision.current;
    setLoading(true);
    setError('');
    const [config, rag, computer, identity] = await Promise.allSettled([api.systemConfig(), knowledge ? api.ragStatus() : Promise.resolve(undefined), admin ? api.computerStatus() : Promise.resolve(undefined), api.systemIdentity()]);
    if (revision !== refreshRevision.current) return;
    setDetails({
      identity: identity.status === 'fulfilled' ? identity.value : undefined,
      config: config.status === 'fulfilled' ? config.value : undefined,
      rag: rag.status === 'fulfilled' ? rag.value : undefined,
      computer: computer.status === 'fulfilled' ? computer.value : undefined
    });
    const failed = [config, rag, computer, identity].find(item => item.status === 'rejected');
    if (failed?.status === 'rejected') setError(failed.reason instanceof Error ? failed.reason.message : text.common.error);
    setLoading(false);
  };
  useWorkspaceRefresh(refresh);

  useEffect(() => {
    let disposed = false;
    void refresh();
    if (window.electron) void window.electron.getPaths().then(value => { if (!disposed) setDesktopPaths(value); }).catch(() => { if (!disposed) setDesktopPaths(null); });
    if (window.electron?.getBackendInfo) void window.electron.getBackendInfo().then(value => { if (!disposed) setBackend(value); }).catch(() => { if (!disposed) setBackend(null); });
    return () => { disposed = true; refreshRevision.current++; };
  }, [admin, knowledge]);
  const stopComputer = async () => {
    if (!admin || stopPending.current) return;
    stopPending.current = true; setStopping(true); setError('');
    try { await api.emergencyStop(); await refresh(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); }
    finally { stopPending.current = false; setStopping(false); }
  };

  const options: { choice: ThemeChoice; label: string }[] = [
    { choice: 'system', label: text.shell.systemTheme },
    { choice: 'dark', label: text.shell.darkTheme },
    { choice: 'light', label: text.shell.lightTheme }
  ];

  return <div className="stack settings-page">
    {error && <div className="inline-error" role="alert">{error}</div>}
    {backend?.reason && <div className="inline-error" role="alert">{backend.reason}</div>}
    <section className="settings-group" aria-labelledby="preferences-title">
      <SectionHeading title={management.preferences} id="preferences-title" />
      <div className="settings-panel"><h3>{text.shell.theme}</h3><div className="segmented-control" role="group" aria-label={text.shell.theme}>{options.map(option => <button key={option.choice} aria-pressed={themeChoice === option.choice} onClick={() => onThemeChange(option.choice)}>{option.label}</button>)}</div></div>
      <div className="settings-panel"><div><h3>{text.onboarding.title}</h3><p>{text.onboarding.body}</p></div><button className="secondary-button" onClick={onShowOnboarding}>{text.settings.showGuide}</button></div>
    </section>
    <section className="settings-group" aria-labelledby="workspace-title">
      <SectionHeading title={management.workspace} id="workspace-title" />
      <div className="settings-panel desktop-storage"><h3>{text.settings.service}</h3><dl>
      <div><dt>{text.settings.service}</dt><dd>{backend?.url ?? window.location.origin}</dd></div>
      <div><dt>{management.serviceState}</dt><dd>{health === 'ready' ? text.status.ready : health === 'checking' ? text.status.checking : text.status.offline}</dd></div>
      <div><dt>{text.settings.version}</dt><dd>{details.identity?.version ?? details.config?.version ?? unknown}</dd></div>
      {backend && <div><dt>Desktop</dt><dd>{backend.desktopVersion}</dd></div>}
    </dl></div>
      <div className="settings-panel"><div><h3>{text.settings.computerUse}</h3><p>{!admin ? interaction[locale].adminOnly : !details.computer ? unknown : details.computer.available ? text.settings.available : text.settings.unavailable}</p></div>{admin && details.computer?.available && <div className="settings-actions"><span role="status" className={details.computer.emergency_stop ? 'status-pill danger' : 'status-pill'}>{details.computer.emergency_stop ? text.settings.stopped : `${details.computer.active_sessions} ${text.settings.sessions}`}</span><button className="danger-button" disabled={stopping} onClick={() => void stopComputer()}>{stopping ? text.common.loading : text.settings.emergencyStop}</button></div>}</div>
    </section>
    <details className="settings-diagnostics desktop-storage"><summary>{management.diagnostics}</summary><dl>
      <div><dt>API</dt><dd>{details.identity?.api_version ?? unknown}</dd></div>
      {backend && <><div><dt>{compatibility.title}</dt><dd>{backend.compatibilityBasis === 'declared-contract' ? compatibility.declared : backend.compatibilityBasis === 'reviewed-legacy' ? compatibility.legacy : unknown}{backend.bridgeProtocol ? ` · ${backend.bridgeProtocol}` : ''}</dd></div><div><dt>{text.settings.service}</dt><dd>{backend.managedByDesktop ? compatibility.owned : compatibility.external}</dd></div></>}
      <div><dt>UI SHA-256</dt><dd><code>{details.identity?.ui_build_id || unknown}</code></dd></div>
      <div><dt>{text.settings.inferenceSlots}</dt><dd>{details.config?.inference_slots ?? unknown}</dd></div>
      {knowledge && <div><dt>{text.settings.knowledge}</dt><dd>{!details.rag ? unknown : details.rag.enabled ? text.settings.enabled : text.settings.disabled}</dd></div>}
    </dl>
      {desktopPaths && <div className="desktop-local-paths"><h3>Desktop · {text.shell.storage}</h3><p>{management.desktopStorage}</p><dl><div><dt>{text.shell.modelsPath}</dt><dd>{desktopPaths.models}</dd></div><div><dt>{text.shell.dataPath}</dt><dd>{desktopPaths.data}</dd></div></dl></div>}
    </details>
  </div>;
}
