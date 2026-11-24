import type { components } from './schema.generated';
import { clientMessage } from '../i18n/client-errors';

type Snapshot = components['schemas']['InferenceEndpointManagement'];
export function validateInferenceEndpoints(value: Snapshot): Snapshot {
 const invalid=()=>{throw new Error(clientMessage('response'));};
 if(!value||typeof value.configured!=='boolean'||!['not_configured','review_required','restore_review_required','ready'].includes(value.policy_state)||typeof value.policy_revision!=='string'||!Array.isArray(value.items)||value.items.length>128)invalid();
 const seen=new Set<string>();
 for(const item of value.items){
  if(!item||!['ollama','vllm'].includes(item.provider)||typeof item.id!=='string'||typeof item.revision!=='string'||item.model_id!==`endpoint:${item.id}@${item.revision}`||seen.has(item.model_id)||typeof item.url!=='string'||typeof item.model!=='string'||!Array.isArray(item.actors)||item.actors.length>128||item.actors.some(actor=>typeof actor!=='string')||!/^[a-f0-9]{64}$/.test(item.config_pin)||['installed','current','enabled','review_required','credential_ready','managed','probe_allowed'].some(key=>typeof item[key as keyof typeof item]!=='boolean'))invalid();
  seen.add(item.model_id);
 }
 return value;
}
