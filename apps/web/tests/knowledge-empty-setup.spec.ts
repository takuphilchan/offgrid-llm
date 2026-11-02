import {test,expect} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';

test('empty search-model catalog provides a working next step without Unknown or zero-byte promises',async({page})=>{
  const state=await craftFixture(page,'en','light');
  await page.route('**/v1/rag/status',r=>r.fulfill({json:{enabled:false,stats:{}}}));
  await page.route('**/v1/models',r=>r.fulfill({json:{data:[]}}));
  await page.route('**/v1/catalog',r=>r.fulfill({json:{models:[]}}));
  await page.goto('/ui/#/knowledge');
  const setup=page.locator('.knowledge-setup');
  await expect(setup).toBeVisible();
  await expect(setup).not.toContainText('0 B');
  await expect(setup.locator('.knowledge-setup-controls > p')).toHaveCount(0);
  const action=setup.getByRole('button',{name:'Open model manager',exact:true}).filter({visible:true});
  await expect(action).toBeEnabled();await action.click();
  await expect(page.locator('.models-workspace')).toBeVisible();
  expect(state.writes).toEqual([]);
});
