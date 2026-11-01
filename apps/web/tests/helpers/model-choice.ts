import { expect, type Locator, type Page } from '@playwright/test';

export async function chooseModel(root: Page | Locator, label: string, id: string) {
  const trigger = root.getByRole('button', { name: label, exact: true });
  await expect(trigger).toBeEnabled();
  if (await trigger.getAttribute('aria-expanded') !== 'true') await trigger.click();
  const group = root.getByRole('radiogroup', { name: label, exact: true });
  const radios = group.getByRole('radio');
  const index = await radios.evaluateAll((nodes, value) => nodes.findIndex(node => (node as HTMLInputElement).value === value), id);
  expect(index).toBeGreaterThanOrEqual(0);
  // Selection closes the inline list, so do not wait for a removed radio node.
  await radios.nth(index).click();
  await expect(trigger).toBeFocused();
}
