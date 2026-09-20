import copy from './computer-experience.json';
import type {LocaleCode} from './index';
export const computerExperience = (locale: LocaleCode) => copy[locale];
export function browserActionLabel(locale: LocaleCode, name: string) {
 const index = ['browser_observe','browser_navigate','browser_fill','browser_click','browser_verify','browser_select','browser_set_checked'].indexOf(name);
 return index < 0 ? name : copy[locale].actions[index];
}
