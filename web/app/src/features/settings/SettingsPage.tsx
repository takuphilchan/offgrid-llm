import { useWorkspace } from '../../lib/workspace-context';
import { interaction } from '../../i18n/interaction';
import { useWorkspaceRefresh } from '../../lib/workspace-refresh';
import { useEffect, useRef, useState } from 'react';
import { api } from '../../api/client';
import { useI18n } from '../../i18n';
import { presentation } from '../../i18n/presentation';
import type { DesktopPaths, DesktopBackend } from '../../platform';
import type { ThemeChoice } from '../../theme';

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
  const refreshRevision = useRef(0);
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
    void refresh();
    if (window.electron) void window.electron.getPaths().then(setDesktopPaths).catch(() => setDesktopPaths(null));
    if (window.electron?.getBackendInfo) void window.electron.getBackendInfo().then(setBackend).catch(() => setBackend(null));
  }, []);
  const stopComputer = async () => {
    try { await api.emergencyStop(); await refresh(); }
    catch (reason) { setError(reason instanceof Error ? reason.message : text.common.error); }
  };

  const options: { choice: ThemeChoice; label: string }[] = [
    { choice: 'system', label: text.shell.systemTheme },
    { choice: 'dark', label: text.shell.darkTheme },
    { choice: 'light', label: text.shell.lightTheme }
  ];

  return <div className="stack settings-page">
    {error && <div className="inline-error" role="alert">{error}</div>}
    {backend?.reason && <div className="inline-error" role="alert">{backend.reason}</div>}
    <section className="settings-panel desktop-storage"><div><span className="eyebrow">{text.settings.service}</span><h2>{text.common.runtime}</h2></div><dl>
      <div><dt>{text.settings.service}</dt><dd>{backend?.url ?? window.location.origin}</dd></div>
      <div><dt>{text.settings.version}</dt><dd>{details.identity?.version ?? '—'}</dd></div>
      {backend && <div><dt>Desktop</dt><dd>{backend.desktopVersion}</dd></div>}
    </dl></section>
    <details className="settings-panel diagnostic-details desktop-storage"><summary>{presentation[locale].details}</summary><dl><div><dt>API</dt><dd>{details.identity?.api_version ?? '—'}</dd></div><div><dt>UI SHA-256</dt><dd><code>{details.identity?.ui_build_id || '—'}</code></dd></div></dl></details>
    <section className="settings-panel"><div><span className="eyebrow">{text.shell.appearance}</span><h2>{text.shell.theme}</h2></div><div className="segmented-control" role="group" aria-label={text.shell.theme}>{options.map(option => <button key={option.choice} aria-pressed={themeChoice === option.choice} onClick={() => onThemeChange(option.choice)}>{option.label}</button>)}</div></section>
    <div className="metric-grid"><Metric label={text.settings.service} value={health === 'ready' ? text.status.ready : health === 'checking' ? text.status.checking : text.status.offline} /><Metric label={text.settings.version} value={details.config?.version ?? '—'} /><Metric label={text.settings.inferenceSlots} value={String(details.config?.inference_slots ?? '—')} /><Metric label={text.settings.knowledge} value={!details.rag ? unknown : details.rag.enabled ? text.settings.enabled : text.settings.disabled} /></div>
    <section className="settings-panel"><div><span className="eyebrow">{text.settings.safety}</span><h2>{text.settings.computerUse}</h2><p>{!admin ? interaction[locale].adminOnly : !details.computer ? unknown : details.computer.available ? text.settings.available : text.settings.unavailable}</p></div>{details.computer?.available && <div className="settings-actions"><span className={details.computer.emergency_stop ? 'status-pill danger' : 'status-pill'}>{details.computer.emergency_stop ? text.settings.stopped : `${details.computer.active_sessions} ${text.settings.sessions}`}</span><button className="danger-button" onClick={() => void stopComputer()}>{text.settings.emergencyStop}</button></div>}</section>
    <section className="settings-panel"><div><span className="eyebrow">{text.settings.setup}</span><h2>{text.onboarding.title}</h2><p>{text.onboarding.body}</p></div><button className="secondary-button" onClick={onShowOnboarding}>{text.settings.showGuide}</button></section>
    {desktopPaths && <section className="settings-panel desktop-storage"><div><span className="eyebrow">{text.shell.storage}</span><h2>{text.shell.storage}</h2></div><dl><div><dt>{text.shell.modelsPath}</dt><dd>{desktopPaths.models}</dd></div><div><dt>{text.shell.dataPath}</dt><dd>{desktopPaths.data}</dd></div></dl></section>}
  </div>;
}

function Metric({ label, value }: { label: string; value: string }) {
  return <article className="metric"><span>{label}</span><strong>{value}</strong></article>;
}
