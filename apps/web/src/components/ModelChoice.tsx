import { useId, useLayoutEffect, useRef, useState } from 'react';
import { useI18n } from '../i18n';
import { modelChoiceText } from '../i18n/model-choice';
import { SearchField } from './SearchField';
import './model-choice.css';

export type ModelChoiceOption = { id: string; name: string; details?: string; disabled?: boolean };

/** Inline disclosure, safe inside an existing utility popover or form. */
export function ModelChoice({ label, value, options, placeholder, missing, summary, disabled = false, defaultOpen = false, onChange }: {
  label: string; value: string; options: ModelChoiceOption[]; placeholder: string; missing?: string;
  summary?: string; disabled?: boolean; defaultOpen?: boolean; onChange: (id: string) => void;
}) {
  const id = useId(), { locale, language, messages } = useI18n(), copy = modelChoiceText[locale];
  const [open, setOpen] = useState(defaultOpen), [query, setQuery] = useState('');
  const trigger = useRef<HTMLButtonElement>(null), root = useRef<HTMLDivElement>(null);
  const returnFocus = useRef(false);
  useLayoutEffect(() => {
    if (!returnFocus.current || disabled || open) return;
    returnFocus.current = false;
    // A resolution can temporarily disable the trigger. Restore only if the
    // user has not moved to another control while waiting for its response.
    if (document.activeElement === document.body || root.current?.contains(document.activeElement)) trigger.current?.focus();
  }, [disabled, open]);
  const selected = options.find(option => option.id === value);
  const matches = options.filter(option => `${option.name} ${option.id} ${option.details ?? ''}`.toLocaleLowerCase(language).includes(query.trim().toLocaleLowerCase(language)));
  const choose = (option: ModelChoiceOption) => {
    if (disabled || option.disabled) return;
    returnFocus.current = true; onChange(option.id); setOpen(false);
  };
  return <div className="model-choice" ref={root} onKeyDown={event => {
    if (!open || event.nativeEvent.isComposing || event.nativeEvent.keyCode === 229) return;
    if (event.key === 'Escape') { event.preventDefault(); event.stopPropagation(); setOpen(false); trigger.current?.focus(); }
    if (event.key === 'Enter' && event.target instanceof HTMLInputElement && event.target.type === 'search') {
      event.preventDefault(); root.current?.querySelector<HTMLInputElement>('input[type="radio"]:not(:disabled)')?.focus();
    }
    if (event.key === 'Enter' && event.target instanceof HTMLInputElement && event.target.type === 'radio') {
      event.preventDefault(); const option = options.find(option => option.id === (event.target as HTMLInputElement).value); if (option) choose(option);
    }
  }}>
    <span className="model-choice-label">{label}</span>
    <button ref={trigger} type="button" className="secondary-button model-choice-trigger" aria-label={label} aria-expanded={open} aria-controls={`${id}-choices`} disabled={disabled} onClick={() => { setOpen(!open); setQuery(''); }}>
      <span>{selected?.name ?? (value ? missing ?? value : placeholder)}</span><span aria-hidden="true">{open ? '−' : '+'}</span>
    </button>
    {value && !selected && <small className="model-choice-missing" dir="auto">{value} · {missing}</small>}
    {!open && !disabled && summary && <small className="model-choice-summary">{summary}</small>}
    {open && <div className="model-choice-panel" id={`${id}-choices`}>
      <SearchField label={copy.filter} value={query} onChange={setQuery} disabled={disabled} />
      <div className="model-choice-options" role="radiogroup" aria-label={label}>
        {matches.map(option => <label key={option.id} className={`model-choice-option${option.disabled ? ' unavailable' : ''}`}>
          <input type="radio" name={id} value={option.id} checked={value === option.id} disabled={disabled || option.disabled} onChange={() => choose(option)} onClick={() => { if (value === option.id) { setOpen(false); trigger.current?.focus(); } }} />
          <span><strong>{option.name}</strong>{option.id && <code dir="auto">{option.id}</code>}{option.details && <small>{option.details}</small>}</span>
        </label>)}
      </div>
      {!matches.length && <p role="status">{copy.empty}</p>}
    </div>}
  </div>;
}
