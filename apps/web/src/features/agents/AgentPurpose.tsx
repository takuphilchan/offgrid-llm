import { useId, useRef } from 'react';
import { useI18n } from '../../i18n';
import { agentProfilesText } from '../../i18n/agent-profiles';
import { agentPurpose } from '../../i18n/agent-purpose';

/** Examples edit only the unsaved instruction field, never resources or grants. */
export function AgentPurpose({ value, onChange }: { value: string; onChange: (instructions: string) => void }) {
  const { locale } = useI18n(), copy = agentPurpose[locale], profile = agentProfilesText[locale];
  const id = useId(), input = useRef<HTMLTextAreaElement>(null);
  return <section className="agent-purpose" aria-labelledby={`${id}-title`}>
    <h4 id={`${id}-title`}>{copy.title}</h4>
    <p className="profile-hint">{copy.hint}</p>
    {!value.trim() && <div className="agent-purpose-examples" role="group" aria-label={copy.examples}>
      {copy.options.map(([label, instructions]) => <button key={label} className="secondary-button" type="button" onClick={() => { onChange(instructions); input.current?.focus(); }}>{label}</button>)}
    </div>}
    <label>{profile.instructions}<textarea ref={input} rows={6} value={value} maxLength={8192} onChange={event => onChange(event.target.value)} aria-describedby={`${id}-hint`} /></label>
    <p id={`${id}-hint`} className="profile-hint">{profile.instructionHint}</p>
  </section>;
}
