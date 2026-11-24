import { useId, useState } from 'react';
import type { EndpointOwnerEntry, EndpointOwnerInput, EndpointOwnerSnapshot } from '../../api/endpoint-owner';
import { useI18n } from '../../i18n';
import { endpointOwner } from '../../i18n/endpoint-owner';
import { inferenceEndpoints } from '../../i18n/inference-endpoints';
import { EndpointOwnerDetails } from './EndpointOwnerDetails';

export function EndpointOwnerForm({ snapshot, previous, busy, prepare }: { snapshot: EndpointOwnerSnapshot; previous?: EndpointOwnerEntry; busy: boolean; prepare: (input: EndpointOwnerInput) => Promise<void> }) {
 const { locale, messages } = useI18n(); const copy = endpointOwner[locale];
 const prefix = useId();
 const [input, setInput] = useState<EndpointOwnerInput>({ id: previous?.id ?? '', revision: '', provider: previous?.provider ?? 'ollama', url: previous?.url ?? '', allowed_ips: previous?.allowed_ips ?? [], model: previous?.model ?? '', model_digest: previous?.model_digest ?? '', runtime_version: previous?.runtime_version ?? '', format: previous?.format ?? 'gguf', context_length: previous?.context_length ?? 4096, actors: previous?.actors ?? [snapshot.actor], credential_reference: previous?.credential_reference ?? '', deployment_attested: false, expected_revision: snapshot.file_revision });
 const [managed, setManaged] = useState(previous?.managed ?? false), [template, setTemplate] = useState(previous?.managed_template ?? '');
 const [ips, setIPs] = useState((previous?.allowed_ips ?? []).join('\n')), [actors, setActors] = useState((previous?.actors ?? [snapshot.actor]).join('\n'));
 const [removeCredential, setRemoveCredential] = useState(false);
 const templateEntry = snapshot.managed_templates.find(item => item.id === template)?.entry;
 const credentials = snapshot.credential_references.filter(item => item.origins.includes(input.url ?? ''));
 const credential = credentials.some(item => item.id === input.credential_reference) ? input.credential_reference : '';
 const needsCredentialRemoval = Boolean(previous?.credential_configured && (managed || !credential));
 const lines = (value: string) => value.split(/\r?\n/).map(value => value.trim()).filter(Boolean);
 const field = (key: 'id'|'revision'|'url'|'model'|'model_digest'|'runtime_version', label: string, pattern?: string) => <label><span>{label}</span><input required dir="ltr" disabled={key === 'id' && Boolean(previous)} value={input[key] ?? ''} maxLength={key === 'id' || key === 'revision' ? 128 : 2048} pattern={pattern} onChange={event => setInput(old => ({ ...old, [key]: event.target.value }))}/></label>;
 return <form onSubmit={event => {
  event.preventDefault(); if (busy || needsCredentialRemoval && !removeCredential || managed && !templateEntry) return;
  const common = { id: input.id, revision: input.revision, actors: lines(actors), deployment_attested: input.deployment_attested, expected_revision: snapshot.file_revision, ...(previous ? { replace: `${previous.id}@${previous.revision}` } : {}) };
  void prepare(managed ? { ...common, managed_template: template } : { ...input, ...common, allowed_ips: lines(ips), credential_reference: credential || undefined });
 }}>
  <fieldset disabled={busy} className="endpoint-owner-fields">
   {field('id', copy.id, '[a-zA-Z0-9][a-zA-Z0-9._\\-]{0,127}')}{field('revision', copy.revision, '[a-zA-Z0-9][a-zA-Z0-9._\\-]{0,127}')}
   <label><span id={`${prefix}-transport`}>{messages.agentRuntime.transport}</span><select aria-labelledby={`${prefix}-transport`} value={managed ? 'managed' : 'remote'} onChange={event => setManaged(event.target.value === 'managed')}><option value="remote">{copy.remote}</option><option value="managed" disabled={!snapshot.managed_templates.length}>{copy.template}</option></select></label>
   {managed ? <><label><span id={`${prefix}-template`}>{copy.template}</span><select aria-labelledby={`${prefix}-template`} required value={template} onChange={event => setTemplate(event.target.value)}><option value="">{copy.none}</option>{snapshot.managed_templates.map(item => <option key={item.id} value={item.id}>{item.id}</option>)}</select></label>{!templateEntry && <p role="status">{copy.missingTemplate}</p>}{templateEntry && <EndpointOwnerDetails entry={templateEntry}/>}</> : <>
    <label><span id={`${prefix}-provider`}>{messages.agentRuntime.provider}</span><select aria-labelledby={`${prefix}-provider`} value={input.provider} onChange={event => setInput(old => ({ ...old, provider: event.target.value as 'ollama'|'vllm' }))}>{['ollama','vllm'].map(provider => <option key={provider}>{provider}</option>)}</select></label>
    {field('url', copy.origin)}<label><span>{copy.ips}</span><textarea dir="ltr" value={ips} maxLength={4096} onChange={event => setIPs(event.target.value)}/></label>
    {field('model', messages.agentRuntime.model)}{field('model_digest', copy.digest, 'sha256:[a-f0-9]{64}')}{field('runtime_version', copy.runtime)}
    <label><span id={`${prefix}-format`}>{copy.format}</span><select aria-labelledby={`${prefix}-format`} value={input.format} onChange={event => setInput(old => ({ ...old, format: event.target.value as EndpointOwnerInput['format'] }))}>{['gguf','safetensors','mlx_safetensors'].map(format => <option key={format}>{format}</option>)}</select></label>
    <label><span>{messages.agentRuntime.context}</span><input required type="number" min={512} max={1048576} step={1} value={input.context_length} onChange={event => setInput(old => ({ ...old, context_length: Number(event.target.value) }))}/></label>
    <label><span id={`${prefix}-credential`}>{copy.credential}</span><select aria-labelledby={`${prefix}-credential`} value={credential} onChange={event => setInput(old => ({ ...old, credential_reference: event.target.value }))}><option value="">{copy.none}</option>{credentials.map(item => <option key={item.id}>{item.id}</option>)}</select></label>
   </>}
   <label><span>{inferenceEndpoints[locale].actors}</span><textarea required dir="ltr" maxLength={32768} value={actors} onChange={event => setActors(event.target.value)}/></label>
   {needsCredentialRemoval && <label className="endpoint-owner-check"><input required type="checkbox" checked={removeCredential} onChange={event => setRemoveCredential(event.target.checked)}/><span>{copy.removeCredential}</span></label>}
   <label className="endpoint-owner-check"><input required type="checkbox" checked={input.deployment_attested} onChange={event => setInput(old => ({ ...old, deployment_attested: event.target.checked }))}/><span>{copy.attest}</span></label>
  </fieldset>
  <button className="primary-button" disabled={busy || managed && !templateEntry}>{busy ? messages.common.loading : copy.review}</button>
 </form>;
}
