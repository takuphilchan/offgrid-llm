import { expect, test, type Page } from '@playwright/test';
import { loginContext } from '../src/i18n/login-context';

async function loginFixture(page: Page, locale='en', theme='light') {
  await page.addInitScript(({locale,theme})=>{
    localStorage.setItem('offgrid.locale',locale);localStorage.setItem('offgrid.theme',theme);
    localStorage.setItem('offgrid.onboarding.complete','true');
  },{locale,theme});
  await page.route('**/v1/users/me',r=>r.fulfill({status:401,json:{error:'Authentication required'}}));
  await page.route('**/v1/models',r=>r.fulfill({status:401,json:{error:'Authentication required'}}));
}

test('sign-in identifies the service, supports password inspection and preserves account details on failure',async({page})=>{
  await loginFixture(page);
  let calls=0, release: (()=>void)|undefined;
  await page.route('**/v1/auth/login',async r=>{
    calls++; expect(r.request().postDataJSON()).toEqual({username:'demo',password:'disposable test password'});
    await new Promise<void>(resolve=>{release=resolve;});
    await r.fulfill({status:401,json:{error:'Invalid account details'}});
  });
  await page.goto('/ui/#/chat');
  await expect(page.locator('.login-service code')).toHaveText(new URL(page.url()).origin);
  await expect(page.getByLabel('Username')).toBeFocused();
  await page.getByLabel('Username').fill('demo');await page.getByLabel('Password',{exact:true}).fill('disposable test password');
  const reveal=page.getByRole('button',{name:'Show password',exact:true});await reveal.click();
  await expect(page.getByLabel('Password',{exact:true})).toHaveAttribute('type','text');
  await expect(page.getByRole('button',{name:'Hide password',exact:true})).toHaveAttribute('aria-pressed','true');
  expect(calls).toBe(0);
  await page.getByRole('button',{name:'Hide password',exact:true}).click();
  await page.getByLabel('Password',{exact:true}).press('Enter');
  await expect.poll(()=>calls).toBe(1);
  await expect(page.getByLabel('Username')).toBeDisabled();await expect(page.getByLabel('Password',{exact:true})).toBeDisabled();
  await page.locator('.login-card').dispatchEvent('submit');expect(calls).toBe(1);
  release!();
  await expect(page.getByRole('alert')).toContainText('Invalid account details');
  await expect(page.getByRole('alert')).toContainText('administrator of this service');
  await expect(page.getByLabel('Username')).toHaveValue('demo');
  await expect(page.getByLabel('Password',{exact:true})).toHaveValue('disposable test password');
  await expect(page.getByRole('button',{name:'Sign in',exact:true})).toBeEnabled();
});

for(const address of ['https://user:secret@workspace.example.test:9443/private?token=hidden','javascript:alert(1)','invalid']) test(`desktop sign-in safely presents backend metadata: ${address.split(':')[0]}`,async({page})=>{
  await loginFixture(page);
  await page.addInitScript(address=>{(window as any).electron={getBackendInfo:async()=>({url:address}),getSystemTheme:async()=>'light',onThemeChange:()=>()=>{}};},address);
  await page.goto('/ui/#/chat');
  await expect(page.locator('.login-service code')).toHaveText(address.startsWith('https:')?'https://workspace.example.test:9443':new URL(page.url()).origin);
  await expect(page.locator('.login-card')).not.toContainText('secret');
  await expect(page.locator('.login-card')).not.toContainText('token=hidden');
});

for(const locale of ['en','fr','de','es','ar','sw','sn','nd','zu'] as const) for(const theme of ['light','dark']) test(`login remains readable and operable at 320px: ${locale} ${theme}`,async({page},info)=>{
  await loginFixture(page,locale,theme);await page.setViewportSize({width:320,height:480});
  await page.goto('/ui/#/chat');
  await expect(page.getByRole('heading',{level:1})).toBeVisible();
  await expect(page.locator('.login-card')).toContainText(loginContext[locale].account);
  const show=page.getByRole('button',{name:loginContext[locale].show,exact:true});
  await show.focus();await page.keyboard.press('Space');
  await expect(page.getByRole('button',{name:loginContext[locale].hide,exact:true})).toBeFocused();
  await page.keyboard.press('Space');
  await expect(page.locator('input[autocomplete="current-password"]')).toHaveAttribute('type','password');
  expect(await page.evaluate(()=>document.documentElement.scrollWidth)).toBeLessThanOrEqual(320);
  expect(await page.locator('.login-card').evaluate(node=>node.scrollWidth<=node.clientWidth+1)).toBe(true);
  if(locale==='en'||locale==='ar') await page.screenshot({path:info.outputPath(`login-${locale}-${theme}.png`),fullPage:true,animations:'disabled'});
});
