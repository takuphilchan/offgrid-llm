import { expect, test } from '@playwright/test';
import { craftFixture } from './helpers/craft-fixture';
import { workspaceManagement } from '../src/i18n/workspace-management';
import { workspaceOrder } from '../src/i18n/workspace-order';
import { workspaceAudit } from '../src/i18n/workspace-audit';
import { agentSchedulesText } from '../src/i18n/agent-schedules';

for (const locale of ['en', 'ar'] as const) for (const width of [390, 1440]) test(`management hierarchy ${locale} ${width}`, async ({ page }, info) => {
  const state = await craftFixture(page, locale, 'dark');
  await page.setViewportSize({ width, height: 900 });
  const management = workspaceManagement(locale), order = workspaceOrder(locale);
  for (const route of ['models', 'knowledge', 'settings', 'activity']) {
    await page.goto(`/ui/#/${route}`);
    await expect(page.locator('.models-workspace, .knowledge-workspace, .settings-page, .activity-page')).toBeVisible();
    const layout = await page.locator('.page-content').evaluate(el => {
      const child = el.querySelector('.models-workspace, .knowledge-workspace, .settings-page, .activity-page')!;
      const outer = el.getBoundingClientRect(), inner = child.getBoundingClientRect(), style = getComputedStyle(el);
      return getComputedStyle(el).direction === 'rtl' ? outer.right - parseFloat(style.paddingRight) - inner.right : inner.left - outer.left - parseFloat(style.paddingLeft);
    });
    expect(Math.abs(layout), route).toBeLessThanOrEqual(1);
    expect(await page.evaluate(() => document.documentElement.scrollWidth)).toBeLessThanOrEqual(width);
    await page.screenshot({ path: info.outputPath(`${route}-${locale}-${width}.png`), fullPage: true, animations: 'disabled' });
  }
  await page.goto('/ui/#/knowledge');
  const toolbar = page.locator('.knowledge-library-toolbar');
  await expect(toolbar.getByRole('searchbox', { name: management.searchDocuments })).toBeVisible();
  await expect(toolbar.getByRole('button', { name: management.askKnowledge, exact: true })).toBeVisible();
  if (width === 1440) {
    const heading = await page.locator('.workspace-section-heading').boundingBox();
    expect(heading!.height).toBeLessThan(140);
  }
  await page.goto('/ui/#/models');
  await expect(page.getByRole('button', { name: workspaceAudit(locale).selected, exact: true })).toBeVisible();
  await expect(page.locator('.model-maintenance > .utility-popover-panel')).toBeHidden();
  await page.getByRole('button', { name: workspaceAudit(locale).selected, exact: true }).click();
  await expect(page.getByRole('button', { name: workspaceAudit(locale).selected, exact: true })).toHaveAttribute('aria-pressed', 'true');
  await page.goto('/ui/#/activity');
  const views=page.locator('.model-library-tabs');
  const selectedFill=await views.locator('[aria-pressed=true]').evaluate(el=>getComputedStyle(el).borderBottomColor);
  const idleFill=await views.locator('[aria-pressed=false]').evaluate(el=>getComputedStyle(el).borderBottomColor);
  expect(selectedFill).not.toBe(idleFill);
  const aligned=await page.locator('.activity-page').evaluate(el=>{
    const list=el.querySelector('.task-list')!.getBoundingClientRect(),parent=el.getBoundingClientRect();
    return getComputedStyle(el).direction==='rtl'?parent.right-list.right:list.left-parent.left;
  });
  expect(Math.abs(aligned)).toBeLessThanOrEqual(1);
  await expect(page.locator('.agent-schedules')).toHaveCount(0);
  await page.getByRole('button', { name: agentSchedulesText[locale].title, exact: true }).click();
  await expect(page.getByText(agentSchedulesText[locale].unsupported, { exact: true })).toBeVisible();
  expect(state.writes).toEqual([]);
});
