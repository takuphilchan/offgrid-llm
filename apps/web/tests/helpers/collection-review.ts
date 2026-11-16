import {expect,type Page} from '@playwright/test';
import type {LocaleCode} from '../../src/i18n';
import {collectionDestination} from '../../src/i18n/collection-destination';

export async function reviewKnowledgeImport(page:Page,locale:LocaleCode='en',id='workspace'){
 const dialog=page.getByRole('dialog'),copy=collectionDestination(locale);
 await expect(dialog).toBeVisible();
 await dialog.getByRole('combobox',{name:copy.destination,exact:true}).selectOption(id);
 expect(await dialog.evaluate(node=>node.scrollWidth<=node.clientWidth+1)).toBe(true);
 await dialog.getByRole('checkbox',{name:copy.confirmFiles,exact:true}).check();
 await dialog.locator('button.primary-button').click();
 await expect(dialog).toHaveCount(0);
}

export const collectionImportReceipt=(id:string,state='accepted')=>({id,state,sha256:'a'.repeat(64),collection_id:'workspace',collection_revision:1,...(state==='ready'?{document_id:`doc-${id}`}:{})});
