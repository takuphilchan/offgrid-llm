import { useI18n } from '../i18n';
import { workspaceExperience } from '../i18n/workspace-experience';
import { UtilityPopover } from './UtilityPopover';
import { Icon } from './Icon';

/** No delete authority lives here: the caller keeps eligibility and confirmation. */
export function HistoryItemActions({ title, label, disabled, protection, onDelete }: {
  title: string; label: string; disabled?: boolean; protection?: string; onDelete: () => void;
}) {
  const { locale } = useI18n();
  return <UtilityPopover label={`${workspaceExperience[locale].manageHistory}: ${title}`} trigger={<Icon name="more" size={16} />} triggerClassName="utility-button icon-button history-item-menu" className="history-item-actions" align="end">
    <button type="button" className="danger-button" disabled={disabled} onClick={event => { event.currentTarget.closest<HTMLElement>('[popover]')?.hidePopover(); onDelete(); }}><Icon name="trash" size={16} />{label}</button>
    {disabled && protection && <p className="workspace-secondary">{protection}</p>}
  </UtilityPopover>;
}
