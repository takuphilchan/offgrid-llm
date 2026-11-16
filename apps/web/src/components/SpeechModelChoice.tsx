import type { SpeechProfile } from '../api/client';
import { useI18n } from '../i18n';
import { voiceSettingsText } from '../i18n/voice-settings';
import { speechRuntimeText } from '../i18n/voice';
import { ModelChoice } from './ModelChoice';
import { speechModelName } from '../features/models/model-display';

export function SpeechModelChoice({ label, profiles, value, disabled, onChange }: {
  label: string; profiles: SpeechProfile[]; value: string; disabled: boolean; onChange: (id: string) => void;
}) {
  const { locale } = useI18n(), copy = voiceSettingsText(locale), runtime = speechRuntimeText(locale);
  const automatic = profiles.find(profile => profile.available);
  const selected = value ? profiles.find(profile => `${profile.id}@${profile.revision}` === value) : automatic;
  const readiness = selected ? selected.available ? runtime[selected.smoke_tested ? 'tested' : 'available'] : selected.issue || copy.unavailable : value ? undefined : profiles.find(profile => profile.issue)?.issue || copy.notReady;
  const names = profiles.map(profile => speechModelName(profile.name, profile.architecture));
  const variants=profiles.map((profile,index)=>{
    const peers=profiles.filter((_,peer)=>peer!==index&&names[peer]===names[index]);
    if(!peers.length)return undefined;
    if(peers.some(peer=>peer.revision===profile.revision))return `${profile.id} · ${profile.revision.slice(0,12)}`;
    let length=12;
    while(length<profile.revision.length&&peers.some(peer=>peer.revision.slice(0,length)===profile.revision.slice(0,length)))length++;
    return profile.revision.slice(0,length);
  });
  return <ModelChoice label={label} value={value} disabled={disabled} onChange={onChange} placeholder={copy.automatic} missing={copy.selectedUnavailable}
    triggerName={selected ? `${value ? '' : `${copy.automatic} · `}${speechModelName(selected.name,selected.architecture)}` : undefined}
    summary={readiness}
    options={[
      { id: '', name: copy.automatic, details: automatic ? speechModelName(automatic.name, automatic.architecture) : copy.notReady,
        technical: automatic ? `${automatic.name} · ${automatic.id}@${automatic.revision}` : undefined },
      ...profiles.map((profile,index) => ({ id: `${profile.id}@${profile.revision}`, name: names[index], disabled: !profile.available,
        // Only colliding display names need a revision badge; exact identities
        // remain searchable and inspectable, and are always used for selection.
        variant: variants[index],
        details: !profile.available ? profile.issue || copy.unavailable : undefined,
        technical: [profile.name,profile.available ? runtime[profile.smoke_tested ? 'tested' : 'available'] : profile.issue || copy.unavailable,profile.architecture].filter(Boolean).join(' · ') })),
    ]} />;
}
