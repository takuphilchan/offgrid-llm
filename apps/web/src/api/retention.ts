import {clientMessage} from '../i18n/client-errors';
import type { components } from './schema.generated';
type Policy = components['schemas']['DocumentRetention'];

export function validateRetention(value: unknown, sent?: Policy): Policy {
  const policy = value as Policy | null;
  if (!policy || policy.version !== 1 || !Number.isSafeInteger(policy.revision) || policy.revision < 1 ||
      !Number.isSafeInteger(policy.delete_after) || policy.delete_after < 0 || policy.delete_after > 253402300799 || typeof policy.legal_hold !== 'boolean' ||
      (sent && (policy.revision !== sent.revision + 1 || policy.delete_after !== sent.delete_after || policy.legal_hold !== sent.legal_hold))) {
    throw new Error(clientMessage('retention'));
  }
  return policy;
}
