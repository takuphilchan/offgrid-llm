import { useEffect, useMemo, useRef, useState, type KeyboardEvent } from 'react';
import { Icon, type IconName } from './Icon';
import { useFocusScope } from '../lib/focus-scope';

export type CommandAction = {
  id: string;
  label: string;
  group: string;
  icon: IconName;
  keywords?: string;
  run: () => void;
};

export function CommandPalette({ actions, title, placeholder, empty, closeLabel, onClose }: {
  actions: CommandAction[];
  title: string;
  placeholder: string;
  empty: string;
  closeLabel: string;
  onClose: () => void;
}) {
  const [query, setQuery] = useState('');
  const dialog = useRef<HTMLDialogElement>(null);
  const list = useRef<HTMLDivElement>(null);
  const keyboardSelection = useRef(false);
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

  useEffect(() => { const node = dialog.current!; node.showModal(); return () => { node.close(); previousFocus?.focus(); }; }, [previousFocus]);
  useFocusScope(dialog, true, onClose);
  useEffect(() => {
    const selected = results[active];
    if (!keyboardSelection.current || !selected || !list.current) return;
    keyboardSelection.current = false;
    const option = document.getElementById(`command-${selected.id}`);
    if (!option) return;
    // Scroll only the results viewport, never the dialog or underlying workspace.
    const box = option.getBoundingClientRect(), viewport = list.current.getBoundingClientRect();
    if (box.top < viewport.top + 8) list.current.scrollTop += box.top - viewport.top - 8;
    else if (box.bottom > viewport.bottom - 8) list.current.scrollTop += box.bottom - viewport.bottom + 8;
  }, [active, results]);

  const choose = (action?: CommandAction) => {
    if (!action) return;
    action.run();
    onClose();
  };

  const keyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    if (event.nativeEvent.isComposing || event.nativeEvent.keyCode === 229) return;
    if (event.key === 'Escape') onClose();
    else if (event.key === 'ArrowDown') { event.preventDefault(); keyboardSelection.current = true; setActive(current => results.length ? (current + 1) % results.length : 0); }
    else if (event.key === 'ArrowUp') { event.preventDefault(); keyboardSelection.current = true; setActive(current => results.length ? (current - 1 + results.length) % results.length : 0); }
    else if (event.key === 'Enter') { event.preventDefault(); choose(results[Math.min(active, results.length - 1)]); }
  };

  return <dialog ref={dialog} className="command-palette" aria-label={title} onCancel={event => { event.preventDefault(); onClose(); }} onClick={event => { if (event.target === event.currentTarget) { const box = event.currentTarget.getBoundingClientRect(); if (event.clientX < box.left || event.clientX > box.right || event.clientY < box.top || event.clientY > box.bottom) onClose(); } }}>
      <div className="command-search"><Icon name="search" size={18} /><input role="combobox" aria-expanded="true" aria-autocomplete="list" autoFocus value={query} onChange={event => { setQuery(event.target.value); setActive(0); if (list.current) list.current.scrollTop = 0; }} onKeyDown={keyDown} placeholder={placeholder} aria-label={placeholder} aria-controls="command-results" aria-activedescendant={results[active] ? `command-${results[active].id}` : undefined} /><button type="button" className="command-dismiss" aria-label={closeLabel} title={`${closeLabel} (Esc)`} onClick={onClose}><kbd aria-hidden="true">Esc</kbd></button></div>
      <div ref={list} className="command-results" id="command-results" role="listbox">{results.length === 0 ? <p>{empty}</p> : groups.map(group => <div className="command-group" role="group" aria-label={group.name} key={group.name}>
        <span>{group.name}</span>
        {group.actions.map(({ action, index }) => <button id={`command-${action.id}`} role="option" aria-selected={index === active} onMouseEnter={() => setActive(index)} onFocus={() => setActive(index)} onClick={() => choose(action)} key={action.id}><Icon name={action.icon} size={17} /><strong>{action.label}</strong></button>)}
      </div>)}</div>
  </dialog>;
}
