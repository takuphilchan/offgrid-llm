export const APPROVAL_MODES = Object.freeze(['ask_every_time','scoped_changes','full_task']);
export const ACTION_CLASSES = Object.freeze(['read_only','reversible','consequential','forbidden']);

export function validApprovalMode(value) { return APPROVAL_MODES.includes(value); }
export function validActionClass(value) { return ACTION_CLASSES.includes(value); }
export function policyAllows(mode, actionClass) {
  if (!validApprovalMode(mode) || !validActionClass(actionClass) || actionClass === 'forbidden') return false;
  if (mode === 'full_task') return true;
  if (mode === 'scoped_changes') return actionClass === 'read_only' || actionClass === 'reversible';
  return actionClass === 'read_only';
}

const blocked = /\b(password|passcode|credential|secret|token|one[ -]?time|verification code|credit card|card number|cvv|cvc|payment|pay now|purchase|buy now|checkout|bank|wire|transfer money|install|update software|administrator|admin access|security setting|firewall|antivirus|permanent(?:ly)? delete|delete forever|erase)\b/i;
const reversible = /\b(save draft|save|apply|add|create draft|update draft)\b/i;

export function classifyBrowserAction(kind, control = {}) {
  const context = `${control.label ?? ''} ${control.type ?? ''} ${control.tag ?? ''} ${control.riskText ?? ''}`;
  if (blocked.test(context)) return 'forbidden';
  if (['browser_observe','browser_capture','browser_verify'].includes(kind)) return 'read_only';
  if (['browser_navigate','browser_fill','browser_select','browser_set_checked'].includes(kind)) return 'reversible';
  if (['browser_download','browser_upload'].includes(kind)) return 'consequential';
  if (kind === 'browser_click') return reversible.test(context) ? 'reversible' : 'consequential';
  return 'forbidden';
}
