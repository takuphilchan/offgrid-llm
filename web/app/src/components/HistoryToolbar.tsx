import { useI18n } from '../i18n';
import { workspaceExperience } from '../i18n/workspace-experience';
import { ListSearch } from './WorkspacePresentation';
import { UtilityPopover } from './UtilityPopover';

/** The caller supplies the confirmed-list snapshot and existing delete authority. */
export function HistoryToolbar({ query, onQuery, searchLabel, count, busy, refresh, clear, clearLabel, clearDisabled, protection }: {
  query: string; onQuery: (value: string) => void; searchLabel: string; count: number;
  busy?: boolean; refresh: () => void; clear: () => void; clearLabel: string; clearDisabled?: boolean; protection?: string;
}) {
  const { locale, messages } = useI18n();
  if (!count && !query) return null;
  return <div className="workspace-history-toolbar">
    <ListSearch label={searchLabel} value={query} onChange={onQuery} />
    {count > 0 && <UtilityPopover label={workspaceExperience[locale].manageHistory} className="history-tools">
      <button type="button" className="secondary-button" disabled={busy} onClick={refresh}>{messages.common.refresh}</button>
      <button type="button" className="secondary-button" disabled={busy || clearDisabled} onClick={clear}>{clearLabel}</button>
      {protection && <p className="workspace-secondary">{protection}</p>}
    </UtilityPopover>}
  </div>;
}
