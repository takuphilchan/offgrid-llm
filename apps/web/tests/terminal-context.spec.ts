import {test,expect,type Page} from '@playwright/test';
import {craftFixture} from './helpers/craft-fixture';

const context={version:1,source:'terminal_selection',command:'Get-ChildItem',cwd:'C:\\temporary',output:'café 日本語\nprivate value',question:'Explain these results',truncated:false};
async function setup(page:Page){
 const state=await craftFixture(page,'en','light');
 await page.addInitScript(()=>{
  (window as any).electron={getSystemTheme:async()=>'light',onThemeChange:()=>()=>{},onTerminalContext(callback:unknown){(window as any).terminalCallback=callback;return ()=>{delete (window as any).terminalCallback;};}};
 });
 await page.goto('/ui/#/chat');await expect(page.locator('.composer textarea')).toBeEnabled();return state;
}
test('terminal context is edited into the chosen draft without sending or replacing existing text',async({page})=>{
 const state=await setup(page);
 await page.locator('.composer textarea').fill('Keep my original draft');
 expect(await page.evaluate(value=>(window as any).terminalCallback(value),context)).toBe(true);
 const dialog=page.getByRole('dialog',{name:'Ask about terminal output'});
 await expect(dialog).toBeVisible();

 const preview=dialog.getByLabel('Review and remove private information');
 await expect(preview).toHaveValue(/private value/);
 await preview.fill((await preview.inputValue()).replace('private value','[removed]'));
 expect(await page.evaluate(value=>(window as any).terminalCallback(value),{...context,output:'do not overwrite'})).toBe(false);
 await dialog.getByRole('button',{name:'Add to draft',exact:true}).click();
 await expect(dialog).toBeHidden();
 await expect(page.locator('.composer textarea')).toHaveValue(/Keep my original draft/);
 await expect(page.locator('.composer textarea')).toHaveValue(/\[removed\]/);
 await expect(page.locator('.composer textarea')).not.toHaveValue(/private value/);
 expect(state.writes).toEqual([]);
 // Selecting a new chat must not overwrite the saved-conversation draft.
 expect(await page.evaluate(value=>(window as any).terminalCallback(value),context)).toBe(true);
 await dialog.getByLabel('Conversation',{exact:true}).selectOption('');
 await dialog.getByRole('button',{name:'Add to draft',exact:true}).click();
 await expect(page.locator('.composer textarea')).toHaveValue(/Explain these results/);
 await expect(page.locator('.composer textarea')).not.toHaveValue(/Keep my original draft/);
 expect(state.writes).toEqual([]);
});
test('malformed context is rejected, cancellation preserves drafts, and narrow preview stays contained',async({page})=>{
 const state=await setup(page);await page.setViewportSize({width:390,height:650});
 await page.locator('.composer textarea').fill('Retained');
 for(const patch of [{output:'\x1b[1m'},{output:'x'.repeat(32769)},{question:''},{source:'trusted_action'}])expect(await page.evaluate(value=>(window as any).terminalCallback(value),{...context,...patch})).toBe(false);
 expect(await page.evaluate(value=>(window as any).terminalCallback(value),context)).toBe(true);
 const dialog=page.getByRole('dialog',{name:'Ask about terminal output'});await expect(dialog).toBeVisible();
 expect(await dialog.evaluate(el=>{const r=el.getBoundingClientRect();return r.left>=0&&r.right<=innerWidth&&el.scrollWidth<=el.clientWidth;})).toBe(true);
 await page.keyboard.press('Escape');await expect(dialog).toBeHidden();await expect(page.locator('.composer textarea')).toHaveValue('Retained');expect(state.writes).toEqual([]);
});
