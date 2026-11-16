import { useI18n } from '../../i18n';
import { agentProfilesText } from '../../i18n/agent-profiles';
import { operationFeedback } from '../../i18n/operation-feedback';
import { SetupLink } from '../../lib/setup-handoff';
import { responseBudgetHint } from '../../i18n/response-budget';
import { useId } from 'react';

import {responseDisplay} from '../../i18n/response-display';
import type {readPreferences} from './chat-presentation';
type Preferences = ReturnType<typeof readPreferences>;

export function ComposerResponseLimits({ preferences, busy, update, expanded = false, presentation = 'disclosure' }: {
  preferences: Preferences; busy: boolean; expanded?: boolean; presentation?: 'disclosure'|'settings'; update: (value: Preferences) => void;
}) {
  const { locale, messages: text , language} = useI18n();
  const { profile, maxTokens } = preferences;
  const id = useId(), display = responseDisplay(locale);
  const profileSelect = <select id={`${id}-profile`} aria-describedby={`${id}-profile-hint`} value={profile} disabled={busy} onChange={event => update({ ...preferences, profile: event.target.value })}>
      <option value="interactive">{text.chatStreaming.interactive}</option><option value="extended">{text.chatStreaming.extended}</option>
    </select>;
  const limitSelect = <select id={`${id}-limit`} aria-describedby={`${id}-limit-hint`} value={maxTokens} disabled={busy} onChange={event => update({ ...preferences, maxTokens: Number(event.target.value) })}>
      {[256, 1024, 4096].map(count => <option value={count} key={count}>{count.toLocaleString(language)}</option>)}
    </select>;
  if (presentation === 'settings') return <div className="settings-card response-settings">
    <div className="response-setting-row"><div><label htmlFor={`${id}-display`}>{display.label}</label><p id={`${id}-display-hint`}>{display.hint}</p></div><select id={`${id}-display`} aria-describedby={`${id}-display-hint`} value={preferences.streamResponses ? 'live' : 'complete'} onChange={event=>update({...preferences,streamResponses:event.target.value==='live'})}><option value="live">{display.live}</option><option value="complete">{display.complete}</option></select></div>
    <div className="response-setting-row"><div><label htmlFor={`${id}-profile`}>{text.chatStreaming.profile}</label><p id={`${id}-profile-hint`}>{text.chatStreaming.profileHint}</p></div>{profileSelect}</div>
    <div className="response-setting-row"><div><label htmlFor={`${id}-limit`}>{text.chatStreaming.responseLimit}</label><p id={`${id}-limit-hint`}>{responseBudgetHint[locale]}</p></div>{limitSelect}</div>
  </div>;
  return <details className="response-advanced" open={expanded || undefined}>
    <summary>{agentProfilesText[locale].advanced}<small>{profile === 'extended' ? text.chatStreaming.extended : text.chatStreaming.interactive}</small></summary>
    <label className="field" htmlFor={`${id}-profile`}>{text.chatStreaming.profile}{profileSelect}</label>
    <label className="field" htmlFor={`${id}-limit`}>{text.chatStreaming.responseLimit}{limitSelect}</label>
    <p id={`${id}-limit-hint`}>{responseBudgetHint[locale]}</p>
    <p id={`${id}-profile-hint`}>{text.chatStreaming.profileHint}</p>
  </details>;
}

export function ComposerKnowledgeReadiness({ state, conversation, refresh }: {
  state: 'checking' | 'ready' | 'unavailable' | 'failed'; conversation: string; refresh: () => Promise<void>;
}) {
  const { locale , language} = useI18n();
  const feedback = operationFeedback(locale);
  return <div className="knowledge-readiness">
    {state === 'checking' ? <span role="status">{feedback.checkingKnowledge}</span>
      : state === 'failed' ? <><span role="status">{feedback.knowledgeCheckFailed}</span> <button className="text-button" onClick={() => void refresh()}>{feedback.retryKnowledge}</button></>
      : <SetupLink destination="knowledge" draft={{ kind: 'chat', id: conversation }}>{feedback.configureKnowledge}</SetupLink>}
  </div>;
}
