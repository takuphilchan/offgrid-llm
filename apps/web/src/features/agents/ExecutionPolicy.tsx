import type { components } from '../../api/schema.generated';
import { useI18n } from '../../i18n';
import { executionWorkspaceText,executionPolicyText } from '../../i18n/execution-workspace';
import './profile-controls.css';
type Permissions=components['schemas']['ExecutionPermissions'];

/** Configuration may only narrow a separately established native grant. */
export function ExecutionPolicy({value,ceiling,onChange,disabled=false}:{value?:Permissions;ceiling?:Permissions;onChange:(value:Permissions|undefined)=>void;disabled?:boolean}){
  const {locale}=useI18n(),copy=executionWorkspaceText[locale],policy=executionPolicyText[locale];
  return <fieldset className="profile-execution-policy" disabled={disabled}><legend className="sr-only">{copy.label}</legend>
    <label className="profile-option"><input type="checkbox" aria-label={policy.limit} checked={!!value} onChange={event=>onChange(event.target.checked?{read:ceiling?.read??true,write:false,processes:false}:undefined)}/><span>{policy.limit}<small>{value ? policy.hint : policy.inherit}</small></span></label>
    {value&&(['read','write','processes'] as const).map(key=><label className="profile-option" key={key}><input type="checkbox" checked={value[key]} disabled={ceiling?.[key]===false} onChange={event=>onChange({...value,[key]:event.target.checked})}/><span>{key==='read'?copy.read:key==='write'?copy.edit:copy.commands}</span></label>)}
    {!value && <p className="profile-hint">{policy.hint}</p>}
  </fieldset>;
}
