import { useI18n } from '../../i18n';
import { agentProfilesText } from '../../i18n/agent-profiles';
import { operationFeedback } from '../../i18n/operation-feedback';
import { SetupLink } from '../../lib/setup-handoff';
import { responseBudgetHint } from '../../i18n/response-budget';

type Preferences = { profile: string; maxTokens: number };

export function ComposerResponseLimits({ preferences, busy, update, expanded = false }: {
  preferences: Preferences; busy: boolean; expanded?: boolean; update: (value: Preferences) => void;
}) {
  const { locale, messages: text } = useI18n();
  const { profile, maxTokens } = preferences;
  return <details className="response-advanced" open={expanded || undefined}>
    <summary>{agentProfilesText[locale].advanced}<small>{profile === 'extended' ? text.chatStreaming.extended : text.chatStreaming.interactive}</small></summary>
    <label className="field">{text.chatStreaming.profile}<select value={profile} disabled={busy} onChange={event => update({ ...preferences, profile: event.target.value })}>
      <option value="interactive">{text.chatStreaming.interactive}</option><option value="extended">{text.chatStreaming.extended}</option>
    </select></label>
    <label className="field">{text.chatStreaming.responseLimit}<select value={maxTokens} disabled={busy} onChange={event => update({ ...preferences, maxTokens: Number(event.target.value) })}>
      {[256, 1024, 4096].map(count => <option value={count} key={count}>{count.toLocaleString(locale)}</option>)}
    </select></label>
    <p>{responseBudgetHint[locale]}</p>
    <p>{text.chatStreaming.profileHint}</p>
  </details>;
}

export function ComposerKnowledgeReadiness({ state, conversation, refresh }: {
  state: 'checking' | 'ready' | 'unavailable' | 'failed'; conversation: string; refresh: () => Promise<void>;
}) {
  const { locale } = useI18n();
  const feedback = operationFeedback(locale);
  return <div className="knowledge-readiness">
    {state === 'checking' ? <span role="status">{feedback.checkingKnowledge}</span>
      : state === 'failed' ? <><span role="status">{feedback.knowledgeCheckFailed}</span> <button className="text-button" onClick={() => void refresh()}>{feedback.retryKnowledge}</button></>
      : <SetupLink destination="knowledge" draft={{ kind: 'chat', id: conversation }}>{feedback.configureKnowledge}</SetupLink>}
  </div>;
}
