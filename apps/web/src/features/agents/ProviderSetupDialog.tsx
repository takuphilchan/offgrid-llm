import { useEffect, useRef } from 'react';
import type { IntegrationSetup } from '../../api/client';
import { Icon } from '../../components/Icon';
import { useI18n } from '../../i18n';
import { externalIntegrations, externalSetupLabel } from '../../i18n/external-integrations';
import { sourceClose } from '../../i18n/knowledge-original';
import { restoreFocus } from '../../lib/restore-focus';

/** Instructions are read-only: copying a command never installs a provider. */
export function ProviderSetupDialog({ name, setup, copied, onCopy, close, returnFocus }: {
  name: string; setup: IntegrationSetup; copied: boolean; onCopy: () => void; close: () => void; returnFocus: HTMLElement | null;
}) {
  const { messages: text, locale } = useI18n();
  const dialog = useRef<HTMLDialogElement>(null);
  useEffect(() => {
    const node = dialog.current!;
    const previous = returnFocus;
    node.showModal();
    return () => { node.close(); restoreFocus(previous); };
  }, []);
  return <dialog ref={dialog} className="provider-setup provider-setup-dialog" aria-label={`${name}: ${externalSetupLabel[locale]}`}
    onCancel={event => { event.preventDefault(); close(); }}>
    <header className="provider-setup-header">
      <div><h2>{name}</h2><p>{externalSetupLabel[locale]}</p></div>
      <button autoFocus type="button" className="utility-button icon-button" aria-label={sourceClose[locale]} onClick={close}><Icon name="close" size={18} /></button>
    </header>
    <div className="provider-setup-body">
      <section aria-label={text.agentRuntime.installPlugin}>
        <div className="provider-command-heading"><h3><Icon name="terminal" size={18} />{text.agentRuntime.installPlugin}</h3><button type="button" className="text-button" onClick={onCopy}><Icon name={copied ? 'check' : 'copy'} size={16} />{copied ? text.agentRuntime.copied : text.agentRuntime.copy}</button></div>
        <pre tabIndex={0} dir="ltr"><code>{setup.install_command}</code></pre>
      </section>
      <section aria-label={text.agentRuntime.verification}>
        <h3>{text.agentRuntime.verification}</h3>
        <pre tabIndex={0} dir="ltr"><code>{setup.verify.join('\n')}</code></pre>
      </section>
      {!!setup.notes?.length && <ul className="provider-setup-notes">{setup.notes.map(note => <li key={note}>{note}</li>)}</ul>}
      <details className="provider-manual"><summary>{externalIntegrations[locale].manual}</summary>
        {Object.keys(setup.environment ?? {}).length > 0 && <section><h3>{text.agentRuntime.environment}</h3><pre tabIndex={0} dir="ltr"><code>{Object.entries(setup.environment ?? {}).map(([key, value]) => `export ${key}=${JSON.stringify(value)}`).join('\n')}</code></pre></section>}
        <section><h3>{setup.config_file}</h3><pre tabIndex={0} dir="ltr"><code>{setup.content}</code></pre></section>
      </details>
    </div>
  </dialog>;
}
