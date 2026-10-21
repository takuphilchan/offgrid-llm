export async function copyText(value: string): Promise<void> {
  if (navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(value);
      return;
    } catch { /* Fall through for restricted browser and Electron contexts. */ }
  }

  const field = document.createElement('textarea');
  field.value = value;
  field.readOnly = true;
  field.style.position = 'fixed';
  field.style.opacity = '0';
  document.body.appendChild(field);
  field.select();
  try {
    if (!document.execCommand('copy')) throw new Error('Clipboard access is unavailable.');
  } finally {
    field.remove();
  }
}
