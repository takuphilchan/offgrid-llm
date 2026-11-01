import type { Model } from '../api/client';
import { supportsChat } from '../api/model-capabilities';
import { useI18n } from '../i18n';
import { useRef, useState, type KeyboardEvent } from 'react';
import { UtilityPopover } from './UtilityPopover';
import { SearchField } from './SearchField';
import { Icon } from './Icon';
import { readableModelName } from '../features/models/model-display';
import { modelLibrary, modelProviderDetails } from '../i18n/model-library';
import './model-select.css';

export function ModelSelect({ models, value, onChange }: { models: Model[]; value: string; onChange: (value: string) => void }) {
  const { messages: text, locale } = useI18n(), copy = modelLibrary(locale), runtime = modelProviderDetails(locale);
  const [query, setQuery] = useState('');
  const root = useRef<HTMLDivElement>(null);
  const eligible = models.filter(supportsChat);
  const matches = eligible.filter(model => `${readableModelName(model.id)} ${model.id}`.toLocaleLowerCase(locale).includes(query.trim().toLocaleLowerCase(locale)));
  const choose = (id: string) => {
    onChange(id);
    root.current?.querySelector<HTMLElement>('[popover]')?.hidePopover();
    root.current?.querySelector<HTMLButtonElement>('.model-select-trigger')?.focus();
  };
  const navigate = (event: KeyboardEvent) => {
    const options = [...(root.current?.querySelectorAll<HTMLButtonElement>('[role="option"]') ?? [])];
    const index = options.indexOf(document.activeElement as HTMLButtonElement);
    if (event.key === 'Enter' && event.target instanceof HTMLInputElement && !event.nativeEvent.isComposing) {
      // Filtering inside the profile form must never submit that form.
      event.preventDefault(); options[0]?.focus(); return;
    }
    const next = event.key === 'ArrowDown' ? Math.min(index + 1, options.length - 1) : event.key === 'ArrowUp' ? Math.max(0, index - 1) : event.key === 'Home' && index >= 0 ? 0 : event.key === 'End' && index >= 0 ? options.length - 1 : -1;
    if (next >= 0 && !event.nativeEvent.isComposing) { event.preventDefault(); options[next]?.focus(); }
  };
  return <div ref={root} className="model-select" onKeyDown={navigate}>
    <UtilityPopover label={text.chat.model} triggerClassName="secondary-button model-select-trigger" panelClassName="model-select-panel" panelTabIndex={-1} preferredHeight={360}
      trigger={<><Icon name="models" size={16} /><span>{value ? readableModelName(value) : text.chat.selectModel}</span></>}
      onOpenChange={open => { if (open) { setQuery(''); root.current?.querySelector<HTMLInputElement>('input[type="search"]')?.focus(); } }}>
      <SearchField label={copy.filterInstalled} value={query} onChange={setQuery} />
      <div role="listbox" aria-label={text.chat.model} className="model-select-options">
        {matches.map((model, index) => <button type="button" role="option" aria-selected={model.id === value} tabIndex={model.id === value || !matches.some(m => m.id === value) && index === 0 ? 0 : -1} key={model.id} onClick={() => choose(model.id)}>
          <span className="model-select-name"><strong>{readableModelName(model.id)}</strong>{model.id === value && <Icon name="check" size={16} />}</span>
          <small className="model-select-id" dir="auto" title={model.id}>{model.id}</small>
          <small>{model.loaded ? copy.loaded : model.runtime_ready === true ? runtime.ready : model.runtime_ready === false ? `${runtime.ready}: ${runtime.no}` : copy.unchecked}{model.context_window ? ` · ${runtime.context}: ${model.context_window.toLocaleString(locale)}` : ''}</small>
        </button>)}
      </div>
      {!matches.length && <p role="status">{text.history.noMatches}</p>}
      {value && !eligible.some(model => model.id === value) && <p role="status" dir="auto">{value} · {copy.unknown}</p>}
      <a className="text-button" href="#/models">{text.nav.models}</a>
    </UtilityPopover>
  </div>;
}
