import { useEffect, useMemo, useState, type KeyboardEvent } from 'react';
import { Icon, type IconName } from './Icon';

export type CommandAction = {
  id: string;
  label: string;
  group: string;
  icon: IconName;
  keywords?: string;
  run: () => void;
};

export function CommandPalette({ actions, title, placeholder, empty, onClose }: {
  actions: CommandAction[];
  title: string;
  placeholder: string;
  empty: string;
  onClose: () => void;
}) {
  const [query, setQuery] = useState('');
  const [active, setActive] = useState(0);
  const [previousFocus] = useState(() => document.activeElement instanceof HTMLElement ? document.activeElement : null);
  const results = useMemo(() => {
    const needle = query.trim().toLocaleLowerCase();
    if (!needle) return actions;
    return actions.filter(action => `${action.label} ${action.group} ${action.keywords ?? ''}`.toLocaleLowerCase().includes(needle));
  }, [actions, query]);
  const groups = useMemo(() => results.reduce<Array<{ name: string; actions: Array<{ action: CommandAction; index: number }> }>>((all, action, index) => {
    const group = all.at(-1);
    if (group?.name === action.group) group.actions.push({ action, index });
    else all.push({ name: action.group, actions: [{ action, index }] });
    return all;
  }, []), [results]);

  useEffect(() => () => previousFocus?.focus(), [previousFocus]);
  useEffect(() => {
    const selected = results[active];
    if (selected) document.getElementById(`command-${selected.id}`)?.scrollIntoView({ block: 'nearest' });
  }, [active, results]);

  const choose = (action?: CommandAction) => {
    if (!action) return;
    action.run();
    onClose();
  };

  const keyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.key === 'Escape') onClose();
    else if (event.key === 'ArrowDown') { event.preventDefault(); setActive(current => results.length ? (current + 1) % results.length : 0); }
    else if (event.key === 'ArrowUp') { event.preventDefault(); setActive(current => results.length ? (current - 1 + results.length) % results.length : 0); }
    else if (event.key === 'Enter') { event.preventDefault(); choose(results[Math.min(active, results.length - 1)]); }
  };

  return <div className="modal-backdrop command-backdrop" role="presentation" onMouseDown={event => { if (event.target === event.currentTarget) onClose(); }}>
    <section className="command-palette" role="dialog" aria-modal="true" aria-label={title}>
      <div className="command-search"><Icon name="search" size={18} /><input autoFocus value={query} onChange={event => { setQuery(event.target.value); setActive(0); }} onKeyDown={keyDown} placeholder={placeholder} aria-label={placeholder} aria-controls="command-results" aria-activedescendant={results[active] ? `command-${results[active].id}` : undefined} /><kbd>Esc</kbd></div>
      <div className="command-results" id="command-results" role="listbox">{results.length === 0 ? <p>{empty}</p> : groups.map(group => <div className="command-group" role="group" aria-label={group.name} key={group.name}>
        <span>{group.name}</span>
        {group.actions.map(({ action, index }) => <button id={`command-${action.id}`} role="option" aria-selected={index === active} onMouseEnter={() => setActive(index)} onClick={() => choose(action)} key={action.id}><Icon name={action.icon} size={17} /><strong>{action.label}</strong></button>)}
      </div>)}</div>
    </section>
  </div>;
}
