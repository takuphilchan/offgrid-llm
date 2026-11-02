import { useEffect, useRef, useState } from 'react';
import { api } from '../../api/client';
import { FilePicker } from '../../components/FilePicker';
import { useI18n } from '../../i18n';
import { ocrSetup } from '../../i18n/ocr-setup';

export function openOCRLanguageSetup() {
  const panel = document.getElementById('knowledge-ocr-setup');
  if (panel instanceof HTMLDetailsElement) {
    // Recovery opens the relevant controls directly, including their optional
    // settings disclosure, instead of making a user hunt through configuration.
    for (let parent = panel.parentElement; parent; parent = parent.parentElement) {
      if (parent instanceof HTMLDetailsElement) parent.open = true;
    }
    panel.open = true;
    panel.querySelector('summary')?.focus();
  }
}

export function OCRLanguageSetup({ disabled, refresh }: { disabled: boolean; refresh: () => Promise<void> }) {
  const { locale, messages } = useI18n(); const copy = ocrSetup(locale);
  const [open, setOpen] = useState(false), [busy, setBusy] = useState(false), [notice, setNotice] = useState('');
  const [installing, setInstalling] = useState(false);
  const [languages, setLanguages] = useState<Awaited<ReturnType<typeof api.ocrLanguages>>['languages']>([]);
  const operation = useRef<AbortController | null>(null), mounted = useRef(true), locked = useRef(false);
  useEffect(() => { mounted.current = true; return () => { mounted.current = false; operation.current?.abort(); }; }, []);
  const check = async () => {
    if (locked.current) return;
    locked.current = true; setBusy(true); setInstalling(false); setNotice('');
    const abort = new AbortController(); operation.current = abort;
    try { const value = await api.ocrLanguages(abort.signal); if (mounted.current) setLanguages(value.languages); }
    catch (error) { if (mounted.current) setNotice(error instanceof Error ? error.message : copy.unknown); }
    finally { locked.current = false; if (mounted.current) setBusy(false); }
  };
  const install = async (language: string, file: File) => {
    if (disabled || locked.current) return;
    if (file.size > 32 * 1024 * 1024) { setNotice(copy.tooLarge); return; }
    locked.current = true; setBusy(true); setInstalling(true); setNotice('');
    const abort = new AbortController(); operation.current = abort;
    try {
      const value = await api.importOCRLanguage(language, file, abort.signal);
      if (!value.languages.some(item => item.language === language && item.state === 'installed_verified')) throw new Error(copy.unknown);
      if (mounted.current) { setLanguages(value.languages); await refresh(); }
    } catch (error) { if (mounted.current) setNotice(error instanceof Error ? error.message : copy.unknown); }
    finally { locked.current = false; if (mounted.current) setBusy(false); }
  };
  return <details id="knowledge-ocr-setup" className="knowledge-ocr-setup" onToggle={event => { const next = event.currentTarget.open; setOpen(next); if (next && !languages.length) void check(); }}>
    <summary>{copy.title}</summary>
    {open && <div>
      <p className="workspace-secondary">{copy.guidance}</p>
      {languages.map(language => <div key={language.language}>
        <strong><bdi>{language.name}</bdi></strong>
        {language.state === 'installed_verified' ? <p role="status">{copy.installed}</p> : <FilePicker label={copy.importFile} accept=".traineddata" disabled={disabled || busy} onFiles={files => { if (files[0]) void install(language.language, files[0]); }} />}
      </div>)}
      {busy && <p role="status">{installing ? copy.installing : messages.common.loading}</p>}
      {notice && <p role="alert">{notice}</p>}
      <button type="button" className="text-button" disabled={busy} onClick={() => void check()}>{copy.check}</button>
    </div>}
  </details>;
}
