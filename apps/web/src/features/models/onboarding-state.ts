import { readPreference, writePreference } from '../../lib/preferences';

const legacyComplete = 'offgrid.onboarding.complete', legacyStage = 'offgrid.onboarding.stage';
const legacyOwner = 'offgrid.onboarding.legacyOwner';
export const onboardingKey = (identity: string) => `offgrid.onboarding.workspace:${identity}`;

export function readOnboarding(identity: string) {
  let state = identity ? readPreference(onboardingKey(identity)) : null;
  // Bind old browser-wide preferences once. They cannot complete a second
  // workspace or another account's setup. No conversation data is migrated.
  if (identity && !readPreference(legacyOwner)) {
    writePreference(legacyOwner, identity);
    if (!state) {
      state = readPreference(legacyComplete) === 'true' ? 'complete' : readPreference(legacyStage);
      if (state) writePreference(onboardingKey(identity), state);
    }
  }
  if (!identity) state = readPreference(legacyComplete) === 'true' ? 'complete' : readPreference(legacyStage);
  return { pending: state !== 'complete', visible: state !== 'complete' && state !== 'working' };
}

export function saveOnboarding(identity: string, state: 'complete' | 'working') {
  if (identity) writePreference(onboardingKey(identity), state);
  // Preserve the old presentation preference for older clients; current
  // clients use the workspace key after the one-time migration above.
  writePreference(legacyComplete, state === 'complete' ? 'true' : null);
  writePreference(legacyStage, state === 'working' ? 'working' : null);
}
