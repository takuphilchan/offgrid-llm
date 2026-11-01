import { useId, useState } from 'react';
import type { AgentSkill, AgentSkillReference } from '../../api/client';
import { useI18n } from '../../i18n';
import { agentContextText } from '../../i18n/agent-context';
import { skillLibraryText } from '../../i18n/skill-library';
import './skill-library.css';

export function sameSkill(a: AgentSkillReference, b: AgentSkillReference) {
  return a.id === b.id && a.revision === b.revision && a.digest === b.digest;
}

export function SkillLibrary({ skills, selected, onChange, inspect }: {
  skills: AgentSkill[]; selected: AgentSkillReference[];
  onChange: (refs: AgentSkillReference[]) => void; inspect: (ref: AgentSkillReference) => void;
}) {
  const { locale } = useI18n(), copy = skillLibraryText[locale], context = agentContextText[locale];
  const [query, setQuery] = useState(''), searchId = useId();
  const visible = skills.filter(skill => `${skill.name} ${skill.description}`.toLocaleLowerCase(locale).includes(query.trim().toLocaleLowerCase(locale)));
  const missing = selected.filter(ref => !skills.some(skill => sameSkill(ref, skill)));
  return <div className="skill-library">
    {!!skills.length && <div className="skill-search"><label htmlFor={searchId}>{copy.search}</label><input id={searchId} type="search" value={query} onChange={event => setQuery(event.target.value)} /></div>}
    {!skills.length && <p className="profile-hint">{copy.empty}</p>}
    {!!skills.length && !visible.length && <p role="status">{copy.noMatch}</p>}
    <div className="skill-library-list">{visible.map(skill => {
      const chosen = selected.some(ref => sameSkill(ref, skill));
      return <article className="skill-card" key={`${skill.id}:${skill.revision}:${skill.digest}`}>
        <label className="profile-option"><input type="checkbox" checked={chosen} disabled={skill.draft || !chosen && selected.length >= 16}
          onChange={event => onChange(event.target.checked ? [...selected.filter(ref => ref.id !== skill.id), { id: skill.id, revision: skill.revision, digest: skill.digest }] : selected.filter(ref => ref.id !== skill.id))} />
          <span><strong>{skill.name}</strong><small>{skill.description}</small><small>{skill.draft ? context.draft : `${copy.reviewed} · v${skill.revision}`}</small></span></label>
        <button type="button" className="utility-button" onClick={() => inspect(skill)}>{context.inspect}: {skill.name}</button>
      </article>;
    })}</div>
    {missing.map(ref => <article className="skill-card skill-missing" key={`${ref.id}:${ref.revision}:${ref.digest}`}>
      <strong>{copy.details}</strong><p>{copy.unavailable}</p><small>{ref.id} · v{ref.revision}</small>
      <div className="profile-actions"><button type="button" className="utility-button" onClick={() => inspect(ref)}>{context.inspect}</button>
        <button type="button" className="utility-button" onClick={() => onChange(selected.filter(item => !sameSkill(item, ref)))}>{copy.remove}</button></div>
    </article>)}
  </div>;
}
