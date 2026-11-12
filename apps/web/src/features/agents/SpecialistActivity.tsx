import type { AgentRun } from '../../api/client';
import { useI18n } from '../../i18n';
import { taskControls } from '../../i18n/task-controls';
import { taskPresentation } from '../../i18n/task-presentation';
import { conversationWorkText } from '../../i18n/conversation-work';
import { specialistActivityText } from '../../i18n/specialist-activity';
import { useTaskRun } from './useTaskRun';
import { useTaskStatus } from './useTaskStatus';
import { TaskResult } from './TaskResult';

type Child = NonNullable<AgentRun['children']>[number];
function Specialist({ parent, child, children, observe }: { parent: string; child: Child; children: Child[]; observe: boolean }) {
  const { locale, messages } = useI18n(), status = useTaskStatus();
  const copy = specialistActivityText(locale), presentation = taskPresentation(locale);
  // Reuse the canonical read-only follower, with at most four automatic
  // subscriptions. Inspecting an older, larger plan remains an explicit choice.
  const { run, connection, error } = useTaskRun(observe ? child.id : '');
  const owned = run?.parent_id === parent ? run : null;
  const currentStatus = owned?.status ?? child.status ?? 'pending';
  const phase = owned?.progress?.phase ?? child.phase;
  const active = currentStatus === 'running';
  const phaseLabel = phase === 'loading' ? messages.chatStreaming.loading
    : phase === 'queued' ? messages.chatStreaming.queued
    : phase === 'generating' ? messages.chatStreaming.generating
    : phase === 'tool' || phase === 'preparing_tool' ? messages.agentLive.tool
    : phase === 'processing' ? messages.chatStreaming.processing : '';
  return <li className={`specialist-row${active ? ' specialist-row-active' : ''}`}>
    <details>
      <summary title={child.spec.goal}><span className="specialist-title"><span className="specialist-state" aria-hidden="true" /><span className="specialist-name">{owned?.profile?.name ?? child.spec.goal}</span></span><small>{owned || child.status ? status(currentStatus, owned?.outcome) : observe && connection === 'connecting' ? messages.common.loading : presentation.unavailable}{active && phaseLabel ? ` · ${phaseLabel}` : ''}</small></summary>
      <div className="specialist-detail">
        {owned?.profile && <p>{child.spec.goal}</p>}
        {!!child.spec.depends_on?.length && <p>{copy.depends}: {child.spec.depends_on.map(key => children.find(item => item.spec.key === key)?.spec.goal ?? key).join(' · ')}</p>}
        {owned && <TaskResult run={owned} compact />}
        {owned?.error && <p>{owned.error}</p>}
        {error && <p role="status">{error}</p>}
        {!owned && <p>{copy.inspect}</p>}
        {owned && connection === 'reconnecting' && <p role="status">{messages.agentLive.reconnectHint}</p>}
        <a className="text-button" href={`#/agents/task/${encodeURIComponent(child.id)}`}>{conversationWorkText[locale].inspect}</a>
      </div>
    </details>
  </li>;
}

export function SpecialistActivity({ run }: { run: AgentRun }) {
  const { locale } = useI18n();
  if (!run.children?.length) return null;
  return <section className="conversation-work-delegates" aria-label={taskControls(locale).children}>
    <strong>{taskControls(locale).children}</strong>
    <ul>{run.children.map((child, index) => <Specialist key={`${run.run_id}:${child.id}`} parent={run.run_id} child={child} children={run.children!} observe={index < 4} />)}</ul>
  </section>;
}
