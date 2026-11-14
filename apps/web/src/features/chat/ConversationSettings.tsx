import {useEffect, useRef, useState} from 'react';
import {api} from '../../api/client';
import type {components} from '../../api/schema.generated';
import {useI18n} from '../../i18n';
import {agentProfilesText} from '../../i18n/agent-profiles';

type Configuration = components['schemas']['EffectiveConversationPreferences'];
type Project = components['schemas']['ConversationProject'];

// Inspection is intentionally read-only: opening settings cannot apply a
// profile revision, reconnect a host or convert project membership into access.
export function ConversationSettings({id}: {id: string}) {
  const {messages, locale} = useI18n();
  const [current, setCurrent] = useState<{configuration: Configuration; project: Project} | null>(null);
  const [error, setError] = useState(''), [busy, setBusy] = useState(false);
  const controller = useRef<AbortController | null>(null);
  useEffect(() => () => controller.current?.abort(), [id]);
  const refresh = async () => {
    controller.current?.abort();
    const pending = new AbortController(); controller.current = pending;
    setBusy(true); setError(''); setCurrent(null);
    try {
      const configuration = await api.conversations.configuration(id, pending.signal);
      const project = await api.conversations.project(configuration.project_id, configuration.project_revision, pending.signal);
      if (!pending.signal.aborted) setCurrent({configuration, project});
    } catch (reason) {if (!pending.signal.aborted) setError(reason instanceof Error ? reason.message : messages.common.error);}
    finally {if (!pending.signal.aborted) setBusy(false);}
  };
  return <details className="conversation-settings" onToggle={event => {if (event.currentTarget.open) void refresh(); else {controller.current?.abort(); setCurrent(null);}}}>
    <summary>{messages.agentRuntime.context}</summary>
    {busy && <p role="status">{messages.common.loading}</p>}
    {error && <p role="alert">{error}</p>}
    {current && <><p className="conversation-project-name">{current.project.name}</p><details className="conversation-configuration-details"><summary>{agentProfilesText[locale].advanced}</summary><pre>{JSON.stringify(current.configuration, null, 2)}</pre></details></>}
    <button type="button" className="text-button" disabled={busy} onClick={() => void refresh()}>{messages.common.refresh}</button>
  </details>;
}
