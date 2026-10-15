import { Icon } from './Icon';

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
  return <label className={`history-search workspace-search-field ${className}`}>
    <Icon name="search" size={16} />
    <span className="sr-only">{label}</span>
    <input id={id} type="search" value={value} placeholder={label} maxLength={maxLength} disabled={disabled} onChange={event => onChange(event.target.value)} />
  </label>;
}
