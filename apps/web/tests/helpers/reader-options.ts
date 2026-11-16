import type {Page} from '@playwright/test';
export async function readerOptions(page:Page){const button=page.locator('.document-reader .document-options > button');if(await button.getAttribute('aria-expanded')!=='true')await button.click();}
