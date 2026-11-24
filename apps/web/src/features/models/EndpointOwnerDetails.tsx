import type { EndpointOwnerEntry } from '../../api/endpoint-owner';
import { useI18n } from '../../i18n';
import { endpointOwner } from '../../i18n/endpoint-owner';
import { inferenceEndpoints } from '../../i18n/inference-endpoints';

export function EndpointOwnerDetails({ entry }: { entry: EndpointOwnerEntry }) {
 const { locale, messages } = useI18n(); const copy = endpointOwner[locale];
 const rows = [
  [copy.id, entry.id], [copy.revision, entry.revision],
  [messages.agentRuntime.provider, entry.provider], [copy.origin, entry.url],
  [copy.ips, (entry.allowed_ips ?? []).join('\n') || copy.none],
  [messages.agentRuntime.model, entry.model], [copy.digest, entry.model_digest],
  [copy.runtime, entry.runtime_version], [copy.format, entry.format],
  [messages.agentRuntime.context, entry.context_length],
  [inferenceEndpoints[locale].actors, entry.actors.join('\n')],
  [copy.credential, entry.credential_reference || (entry.credential_configured ? inferenceEndpoints[locale].credential : copy.none)],
  [copy.template, entry.managed_template || (entry.managed ? copy.missingTemplate : copy.none)]
 ];
 return <dl className="endpoint-owner-details">{rows.map(([label, value]) => <div key={label}><dt>{label}</dt><dd><bdi>{value}</bdi></dd></div>)}</dl>;
}
