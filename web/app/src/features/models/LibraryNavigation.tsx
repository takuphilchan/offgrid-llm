import { useRef } from 'react';
import { useI18n } from '../../i18n';

export type LibraryView = 'installed' | 'discover';
export function LibraryNavigation({ view, change }: { view: LibraryView; change: (view: LibraryView) => void }) {
  const { messages: text } = useI18n();
  const tabs = useRef<HTMLDivElement>(null);
  return <div className="model-library-tabs" role="tablist" aria-label={text.models.title} ref={tabs} onKeyDown={event => {
    if (!['ArrowLeft', 'ArrowRight', 'Home', 'End'].includes(event.key)) return;
    event.preventDefault();
    const next = event.key === 'Home' ? 'installed' : event.key === 'End' ? 'discover' : view === 'installed' ? 'discover' : 'installed';
    change(next); tabs.current?.querySelector<HTMLButtonElement>(`#model-view-${next}`)?.focus();
  }}>
    {(['installed', 'discover'] as const).map(value => <button key={value} id={`model-view-${value}`} className="secondary-button" role="tab" aria-selected={view === value} aria-controls="model-library-panel" tabIndex={view === value ? 0 : -1} onClick={() => change(value)}>{text.models[value]}</button>)}
  </div>;
}
