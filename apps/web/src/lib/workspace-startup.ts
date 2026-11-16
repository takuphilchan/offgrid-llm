/** Reveal only once; later refreshes and running jobs must not reopen startup. */
export function updateWorkspaceStartup(state: { stage?: string; locale?: string; startupOverrides?: Record<string,string>; retry?: string; ready?: boolean }) {
  window.dispatchEvent(new CustomEvent('offgrid-workspace-startup', { detail: state }));
}
