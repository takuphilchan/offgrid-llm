import type { AgentStep, AgentRun } from '../../api/client';
import { useI18n } from '../../i18n';
import { executionWorkspaceText,executionPolicyText } from '../../i18n/execution-workspace';
import { conversationWorkText } from '../../i18n/conversation-work';
import { useTaskStatus } from './useTaskStatus';

export function ExecutionActivity({step}:{step:AgentStep}){
  const {locale}=useI18n(),copy=executionWorkspaceText[locale],policy=executionPolicyText[locale],work=conversationWorkText[locale],status=useTaskStatus();
  let args:Record<string,unknown>={},envelope:Record<string,any>={};
  try{const parsed=JSON.parse(step.tool_args??'{}');if(parsed&&typeof parsed==='object'&&!Array.isArray(parsed))args=parsed;}catch{/* Legacy arbitrary text remains in details. */}
  try{const parsed=JSON.parse(step.tool_result??'{}');if(parsed&&typeof parsed==='object'&&!Array.isArray(parsed))envelope=parsed;}catch{/* Never infer success from prose. */}
  const result=envelope.receipt?.result??{},receipt=envelope.receipt?.state;
  const process=step.tool_name?.startsWith('workspace_process_');
  const title=process?copy.commands:['workspace_write','workspace_write_from_input'].includes(step.tool_name??'')?copy.edit:['workspace_info','workspace_cd'].includes(step.tool_name??'')?copy.label:copy.read;
  const command=typeof args.program==='string'?[args.program,...(Array.isArray(args.arguments)?args.arguments.filter(value=>typeof value==='string'):[])].join(' '):'';
  const terminal=['exited','failed','cancelled','timed_out'].includes(result.state)&&Number.isInteger(result.exit_code);
  const observed=process?(result.state==='running'?'running':terminal?result.state==='cancelled'?'cancelled':result.state==='timed_out'||result.exit_code!==0?'failed':'completed':'uncertain'):receipt==='rejected'?'failed':receipt==='complete'?'completed':'uncertain';
  return <div className="execution-step">
    <strong>{title} · {status(observed)}</strong>
    {(command||typeof args.path==='string')&&<p dir="auto">{(command||(args.path as string)).slice(0,4096)}</p>}
    {typeof result.directory==='string'&&<p dir="auto">{result.directory}</p>}
    {process&&Number.isInteger(result.exit_code)&&<p>{policy.exit}: {result.exit_code}</p>}
    {(result.output_truncated===true||result.complete===false)&&<p role="status">{policy.incomplete}</p>}
    {typeof envelope.text==='string'&&envelope.text&&<pre dir="auto">{envelope.text.slice(0,16384)}</pre>}
    <details><summary>{work.details} · {step.tool_name}</summary><pre>{step.tool_result||step.content}</pre></details>
  </div>;
}

export function ExecutionLocation({run}:{run:AgentRun}){
  const {locale}=useI18n(),copy=executionWorkspaceText[locale],workspace=run.execution_workspace;
  if(!workspace?.reference)return null;
  return <p className="workspace-secondary execution-location" dir="auto">{copy.label}: {workspace.reference.host} · {workspace.root} · {workspace.backend==='isolated_linux'?copy.isolated:copy.local}</p>;
}
