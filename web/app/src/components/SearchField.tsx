import { Icon } from './Icon';
import { useRef } from 'react';
import { useI18n } from '../i18n';
import { workspaceRefinement } from '../i18n/workspace-refinement';

/** Shared compact search surface; the visible placeholder never replaces its accessible label. */
export function SearchField({ label, value, onChange, className = '', id, maxLength, disabled }: {
  label: string;
  value: string;
  onChange: (value: string) => void;
  className?: string;
  id?: string;
  maxLength?: number;
  disabled?: boolean;
}) {
  const input = useRef<HTMLInputElement>(null);
  const { locale } = useI18n();
  return <div className={`history-search workspace-search-field ${className}`}>
    <Icon name="search" size={16} />
    <input ref={input} id={id} aria-label={label} type="search" value={value} placeholder={label} maxLength={maxLength} disabled={disabled} onChange={event => onChange(event.target.value)} />
    {value && <button type="button" className="search-clear icon-button" aria-label={workspaceRefinement[locale].clear} disabled={disabled} onClick={() => { onChange(''); input.current?.focus(); }}><Icon name="close" size={16} /></button>}
  </div>;
}
