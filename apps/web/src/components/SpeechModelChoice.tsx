import type { SpeechProfile } from '../api/client';
import { useI18n } from '../i18n';
import { voiceSettingsText } from '../i18n/voice-settings';
import { speechRuntimeText } from '../i18n/voice';
import { ModelChoice } from './ModelChoice';

export function SpeechModelChoice({ label, profiles, value, disabled, onChange }: {
  label: string; profiles: SpeechProfile[]; value: string; disabled: boolean; onChange: (id: string) => void;
}) {
  const { locale } = useI18n(), copy = voiceSettingsText(locale), runtime = speechRuntimeText(locale);
  const automatic = profiles.find(profile => profile.available);
  const selected = value ? profiles.find(profile => `${profile.id}@${profile.revision}` === value) : automatic;
  const readiness = selected ? selected.available ? runtime[selected.smoke_tested ? 'tested' : 'available'] : selected.issue || copy.unavailable : value ? undefined : copy.notReady;
  return <ModelChoice label={label} value={value} disabled={disabled} onChange={onChange} placeholder={copy.automatic} missing={copy.selectedUnavailable}
    summary={readiness}
    options={[
      { id: '', name: `${copy.automatic}${automatic ? ` · ${automatic.name}` : ''}`, details: automatic ? `${automatic.id}@${automatic.revision}` : copy.notReady },
      ...profiles.map(profile => ({ id: `${profile.id}@${profile.revision}`, name: profile.name, disabled: !profile.available,
        details: [profile.available ? runtime[profile.smoke_tested ? 'tested' : 'available'] : profile.issue || copy.unavailable, profile.architecture].filter(Boolean).join(' · ') })),
    ]} />;
}
