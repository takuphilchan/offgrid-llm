import type { SessionMessage } from '../../api/client';

export type Selection = NonNullable<SessionMessage['attachment_selections']>[number];
export type PassageRef = { source: Selection; start: number; end: number; label: string };

// Bind model prose to this answer's persisted selection, never to a path or URL
// supplied by the model. Ambiguous legacy filenames remain ordinary text.
export function resolveAttachmentCitation(token: string, selections: Selection[]): PassageRef | undefined {
  const compact = /^\[S([1-9]\d*)(?:\.([1-9]\d*))?\]$/.exec(token);
  let source: Selection | undefined, range: number[] | undefined, index = 0;
  if (compact) {
    source = selections[Number(compact[1]) - 1];
    index = compact[2] ? Number(compact[2]) - 1 : 0;
    if (!source || !Array.isArray(source.ranges) || (!compact[2] && source.mode !== 'full_text')) return;
    range = source.ranges[index];
  } else {
    const legacy = /^\[([^\]\n]{1,1024}):\s*(\d+)\s*[-–]\s*(\d+)\]$/.exec(token);
    if (!legacy) return;
    const matches = selections.filter(item => item?.name === legacy[1]);
    if (matches.length !== 1) return;
    source = matches[0];
    if (!Array.isArray(source.ranges)) return;
    index = source.ranges.findIndex(item => Array.isArray(item) && item[0] === Number(legacy[2]) && item[1] === Number(legacy[3]));
    range = source.ranges[index];
  }
  if (!source || typeof source.name !== 'string' || !['full_text','excerpts'].includes(source.mode) || !Number.isSafeInteger(source.total_bytes) || source.total_bytes <= 0 || !/^file-[a-f0-9]{32}$/.test(source.id) || !/^[a-f0-9]{64}$/.test(source.revision) || !Array.isArray(range) || range.length !== 2) return;
  const [start, end] = range;
  if (!Number.isSafeInteger(start) || !Number.isSafeInteger(end) || start < 0 || end <= start || end > source.total_bytes || end-start > 256*1024) return;
  return {source, start, end, label: source.mode === 'excerpts' ? `${source.name} · ${index + 1}` : source.name};
}
