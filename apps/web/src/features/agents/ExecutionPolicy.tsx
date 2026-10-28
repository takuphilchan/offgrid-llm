import type { components } from '../../api/schema.generated';
import { useI18n } from '../../i18n';
import { executionWorkspaceText,executionPolicyText } from '../../i18n/execution-workspace';
type Permissions=components['schemas']['ExecutionPermissions'];

/** Configuration may only narrow a separately established native grant. */
export function ExecutionPolicy({value,ceiling,onChange,disabled=false}:{value?:Permissions;ceiling?:Permissions;onChange:(value:Permissions|undefined)=>void;disabled?:boolean}){
  const {locale}=useI18n(),copy=executionWorkspaceText[locale],policy=executionPolicyText[locale];
  return <fieldset disabled={disabled}><legend>{copy.label}</legend>
    <label>{copy.label}<select value={value?'limit':'inherit'} onChange={event=>onChange(event.target.value==='inherit'?undefined:{read:ceiling?.read??true,write:false,processes:false})}><option value="inherit">{policy.inherit}</option><option value="limit">{policy.limit}</option></select></label>
    {value&&(['read','write','processes'] as const).map(key=><label className="profile-option" key={key}><input type="checkbox" checked={value[key]} disabled={ceiling?.[key]===false} onChange={event=>onChange({...value,[key]:event.target.checked})}/><span>{key==='read'?copy.read:key==='write'?copy.edit:copy.commands}</span></label>)}
    <p>{policy.hint}</p>
  </fieldset>;
}
