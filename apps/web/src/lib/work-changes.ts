// Acknowledged service mutations only. This event never dispatches a command.
export const workChangedEvent = 'offgrid:work-changed';
export type WorkChange = { id: string; status?: string; deleted?: boolean; archived?: boolean };
export function notifyWorkChanged(change: WorkChange) {
  window.dispatchEvent(new CustomEvent(workChangedEvent, {detail: change}));
}
