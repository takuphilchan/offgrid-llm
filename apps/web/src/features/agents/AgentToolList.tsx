import type { AgentTool } from '../../api/client';
import { Icon, type IconName } from '../../components/Icon';
import { useI18n } from '../../i18n';
import { presentation } from '../../i18n/presentation';
import { browserActionLabel } from '../../i18n/computer-experience';

/** Presentation only. An icon never classifies a tool's risk or grants access. */
export function toolIcon(tool: Pick<AgentTool, 'name' | 'source'>): IconName {
  if (tool.source.startsWith('mcp')) return 'plug';
  const name = tool.name;
  if (name.startsWith('browser_')) return 'browser';
  if (name.startsWith('computer_')) return 'quick-window';
  if (name.startsWith('workspace_process_') || name === 'shell') return 'terminal';
  if (name.startsWith('workspace_') || name.includes('file')) return 'folder';
  if (name.startsWith('document_') || name.includes('search')) return 'file-search';
  if (name.includes('calculate') || name === 'calculator') return 'calculator';
  if (name.includes('schedule') || name.includes('time')) return 'activity';
  if (name.includes('memory')) return 'knowledge';
  if (name.includes('delegate')) return 'agents';
  return 'tool';
}

export function AgentToolList({ tools, busy, toggle }: {
  tools: AgentTool[]; busy: boolean; toggle: (tool: AgentTool) => void;
}) {
  const { locale, messages: text } = useI18n();
  return <div className="tool-list">{tools.map(tool => <article key={tool.name}>
    <span className="tool-glyph"><Icon name={toolIcon(tool)} size={20} /></span>
    <div>
      <h3>{tool.source.startsWith('mcp') ? <bdi>{tool.name}</bdi> : browserActionLabel(locale, tool.name)}</h3>
      <p dir="auto">{tool.description}</p>
      <details><summary>{presentation[locale].details}</summary>
        <small><bdi>{tool.name}</bdi></small>
        <small><bdi>{tool.source}</bdi>{tool.capability ? ` · ${tool.capability.risk} ${text.agentRuntime.risk}` : ''}</small>
      </details>
    </div>
    <label className="switch"><input type="checkbox" aria-label={tool.name} checked={tool.enabled} disabled={busy} onChange={() => toggle(tool)} /><span /></label>
  </article>)}</div>;
}
