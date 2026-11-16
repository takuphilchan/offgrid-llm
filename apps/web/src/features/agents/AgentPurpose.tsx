import { useId, useRef } from 'react';
import { useI18n } from '../../i18n';
import { agentProfilesText } from '../../i18n/agent-profiles';
import { agentPurpose } from '../../i18n/agent-purpose';
import { Icon } from '../../components/Icon';

/** Examples edit only the unsaved instruction field, never resources or grants. */
export function AgentPurpose({ value, onChange }: { value: string; onChange: (instructions: string) => void }) {
  const { locale } = useI18n(), copy = agentPurpose[locale], profile = agentProfilesText[locale];
  const id = useId(), input = useRef<HTMLTextAreaElement>(null);
  return <section className="agent-purpose" aria-labelledby={`${id}-title`}>
    <h4 id={`${id}-title`}>{copy.title}</h4>
    <p className="profile-hint">{copy.hint}</p>
    {!value.trim() && <div className="agent-purpose-examples" role="group" aria-label={copy.examples}>
      {copy.options.map(([label, instructions], index) => <button key={label} className="agent-example" type="button" onClick={() => { onChange(instructions); input.current?.focus(); }}><Icon name={(['edit', 'compare', 'knowledge'] as const)[index] ?? 'agents'} size={18}/><span>{label}</span><Icon name="chevron-right" size={14}/></button>)}
    </div>}
    <label>{profile.instructions}<textarea ref={input} rows={6} value={value} maxLength={8192} onChange={event => onChange(event.target.value)} aria-describedby={`${id}-hint`} /></label>
    <p id={`${id}-hint`} className="profile-hint">{profile.instructionHint}</p>
  </section>;
}
