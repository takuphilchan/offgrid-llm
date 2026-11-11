import { useEffect, useId, useMemo, useRef, useState } from 'react';
import { api } from '../../api/client';
import type { components } from '../../api/schema.generated';
import { useI18n } from '../../i18n';
import { scheduleExperience } from '../../i18n/schedule-experience';

export function TimezoneField({ value, onChange }: { value: string; onChange: (value: string) => void }) {
  const { locale , language} = useI18n(), copy = scheduleExperience[locale], id = useId();
  const [open, setOpen] = useState(false), [active, setActive] = useState(-1);
  const input = useRef<HTMLInputElement>(null);
  const zones = useMemo(() => {
    try { return ['UTC', ...Intl.supportedValuesOf('timeZone')]; } catch { return ['UTC']; }
  }, []);
  const matches = zones.filter(zone => zone.replaceAll('_', ' ').toLowerCase().includes(value.replaceAll('_', ' ').toLowerCase())).slice(0, 8);
  const choose = (zone: string) => { onChange(zone); setOpen(false); setActive(-1); input.current?.focus(); };
  return <div className="timezone-field" onBlur={event => { if (!event.currentTarget.contains(event.relatedTarget)) setOpen(false); }}>
    <label htmlFor={id}>{copy.zone}</label>
    <input ref={input} id={id} required maxLength={128} value={value} placeholder={copy.search} autoComplete="off" dir="ltr" role="combobox" aria-autocomplete="list" aria-expanded={open} aria-controls={`${id}-options`} aria-activedescendant={open && active >= 0 ? `${id}-${active}` : undefined}
      onFocus={() => setOpen(true)} onChange={event => { onChange(event.target.value); setActive(-1); setOpen(true); }}
      onKeyDown={event => {
        if (event.nativeEvent.isComposing || event.nativeEvent.keyCode === 229) return;
        if (event.key === 'Escape') { event.preventDefault(); setOpen(false); }
        if (event.key === 'ArrowDown' || event.key === 'ArrowUp') { event.preventDefault(); setOpen(true); setActive(index => matches.length ? Math.max(0, Math.min(matches.length - 1, index + (event.key === 'ArrowDown' ? 1 : -1))) : -1); }
        if (event.key === 'Enter' && open) { event.preventDefault(); if (matches[active] ?? matches[0]) choose(matches[active] ?? matches[0]); }
      }} />
    {open && <div className="timezone-options" role="listbox" id={`${id}-options`} aria-label={copy.zone}>
      {matches.map((zone, index) => <button key={zone} id={`${id}-${index}`} type="button" role="option" tabIndex={-1} aria-selected={index === active} onMouseDown={event => event.preventDefault()} onClick={() => choose(zone)} dir="ltr">{zone.replaceAll('_', ' ')}</button>)}
      {!matches.length && <p>{copy.noMatches}</p>}
    </div>}
  </div>;
}

export function CalendarPreview({ zone, time, days }: { zone: string; time: string; days: number[] }) {
  const { locale, messages , language} = useI18n(), copy = scheduleExperience[locale];
  const [refresh, setRefresh] = useState(0);
  const [result, setResult] = useState<{ key: string; data?: components['schemas']['AgentCalendarPreview']; failed?: boolean }>();
  const key = JSON.stringify([zone, time, days, refresh]);
  useEffect(() => {
    const controller = new AbortController();
    const timer = setTimeout(() => {
      const [hour, minute] = time.split(':').map(Number);
      if (!zone || !/^\d{2}:\d{2}$/.test(time) || !days.length) { setResult({ key, failed: true }); return; }
      void api.previewAgentSchedule({ zone, hour, minute, weekdays: days }, controller.signal).then(data => {
        if (!Array.isArray(data.occurrences) || data.occurrences.length !== 3 || !Number.isFinite(Date.parse(data.as_of)) || data.occurrences.some(value => !Number.isFinite(Date.parse(value)))) throw Error('invalid preview');
        if (!controller.signal.aborted) setResult({ key, data });
      }).catch(() => { if (!controller.signal.aborted) setResult({ key, failed: true }); });
    }, 250);
    return () => { clearTimeout(timer); controller.abort(); };
  }, [key]);
  const current = result?.key === key ? result : undefined;
  const format = (date: string) => {
    try { return new Intl.DateTimeFormat(language, { dateStyle: 'medium', timeStyle: 'short', timeZone: zone }).format(new Date(date)); } catch { return date; }
  };
  return <section className="schedule-calendar-preview" aria-label={copy.preview}>
    <h4>{copy.preview}</h4><p className="workspace-secondary">{copy.hint}</p>
    {!current ? <p role="status">{messages.common.loading}</p> : current.failed ? <p role="status">{copy.failed}</p> : <>
      <ol>{current.data!.occurrences.map(date => <li key={date}><time dateTime={date}>{format(date)}</time></li>)}</ol>
      <small>{copy.asOf}: {format(current.data!.as_of)} · <bdi>{zone}</bdi></small>
    </>}
    <button type="button" className="text-button" onClick={() => setRefresh(value => value + 1)}>{copy.refresh}</button>
  </section>;
}
