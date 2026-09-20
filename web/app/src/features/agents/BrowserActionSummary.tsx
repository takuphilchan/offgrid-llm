import type {AgentStep} from '../../api/client';
import {useI18n} from '../../i18n';
import {browserActionLabel,computerExperience} from '../../i18n/computer-experience';

export function BrowserActionSummary({tool,args,steps}:{tool:string;args:Record<string,unknown>;steps:AgentStep[]}) {
 const {locale}=useI18n(); const text=computerExperience(locale);
 let target=typeof args.url==='string'?args.url:typeof args.element==='string'?args.element:'';
 let option=typeof args.option==='string'?args.option:'';
 for(const step of [...steps].reverse()) {
  try {
   const view=JSON.parse(step.tool_result??'{}');
   if(view.observation_id!==args.observation_id || !Array.isArray(view.elements)) continue;
   const element=view.elements.find((entry:{id:string})=>entry.id===args.element);
   if(typeof element?.label==='string' && element.label) target=element.label;
   const choice=Array.isArray(element?.options)?element.options.find((entry:{id:string})=>entry.id===args.option):undefined;
   if(typeof choice?.label==='string' && choice.label) option=choice.label;
   break;
  }catch{ /* Missing evidence must not become an invented control label. */ }
 }
 return <div className="browser-action-summary"><h2>{browserActionLabel(locale,tool)}</h2><dl>
  {target && <><dt>{text.target}</dt><dd>{target}</dd></>}
  {typeof args.text==='string' && <><dt>{text.value}</dt><dd>{args.text}</dd></>}
  {option && <><dt>{text.change}</dt><dd>{option}</dd></>}
  {typeof args.checked==='boolean' && <><dt>{text.change}</dt><dd>{text.checkboxStates[args.checked?0:1]}</dd></>}
 </dl></div>;
}

export function BrowserActivity({steps}:{steps:AgentStep[]}) {
 const {locale}=useI18n(); const text=computerExperience(locale);
 return <ol className="browser-task-timeline">{steps.filter(step=>step.tool_name?.startsWith('browser_')).map((step,index)=>{
   let result:Record<string,unknown>={};
   try {result=JSON.parse(step.tool_result??'{}');}catch{}
   return <li key={step.id??index}><strong>{browserActionLabel(locale,step.tool_name!)}</strong>
    {step.tool_name==='browser_verify' && <p>{result.verified===true?text.verified:text.notVerified}{typeof result.text==='string'?`: ${result.text}`:''}</p>}
    <details><summary>{text.details}</summary><pre>{step.tool_result}</pre></details>
   </li>;
 })}</ol>;
}
