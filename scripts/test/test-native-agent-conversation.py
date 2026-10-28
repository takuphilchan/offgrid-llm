import base64
import hashlib
import json
import os
from pathlib import Path
import pty
import re
import select
import signal
import subprocess
import tempfile
import time
import urllib.request

from native_agent_fixture import configure, verified_input_write, valid_conversation_workspace_transition

cli,model,api,require_source,check_abstention=configure('Real-model conversation listing/read/edit/test qualification in a disposable folder', conversation=True)
health=api('/health')['inference']
if health['active'] or health['queued'] or health['switching']:raise SystemExit('Inference busy; no fixture created')
evidence=Path(tempfile.mkdtemp(prefix='offgrid-native-conversation-'))
root=evidence/'workspace';root.mkdir()
source='Fixture value \u2014 caf\u00e9 \u65e5\u672c\u8a9e 12345.\n'
wanted='OffGrid native test \u2014 caf\u00e9 \u65e5\u672c\u8a9e 12345'
(root/'alpha-cafe.txt').write_text(source)
(root/'notes-\u65e5\u672c\u8a9e.md').write_text('Read-only neighbor.\n')
(root/'reports').mkdir()
(root/'go.mod').write_text('module offgrid-native-fixture\n\ngo 1.24\n')
(root/'workflow_test.go').write_text('package fixture\nimport "testing"\nfunc TestFixture(t *testing.T) { if 2+2!=4 { t.Fatal("fixture arithmetic") } }\n')
before={p.name:hashlib.sha256(p.read_bytes()).hexdigest() for p in root.iterdir() if p.is_file()}
original_paths={str(p.relative_to(root)) for p in root.rglob('*')}
profile=api('/api/v2/agent-profiles',{'settings':{'name':'Disposable qualification '+evidence.name[-8:],'model':model,'max_iterations':30}})
master,slave=pty.openpty()
process=subprocess.Popen([cli,'agent','chat','--model',model,'--profile',profile['id'],'--profile-revision',str(profile['revision']),'--workspace-folder','.','--workspace-access','trusted'],cwd=root,stdin=slave,stdout=slave,stderr=slave,start_new_session=True)
os.close(slave)
output=b'';approved=False;approval_end=0;approval_levels=set();runs=[];results=[]
def until_prompt(start,timeout=180):
    global output,approved,approval_end
    deadline=time.monotonic()+timeout
    while time.monotonic()<deadline:
        ready,_,_=select.select([master],[],[],.2)
        if ready:
            try:chunk=os.read(master,65536)
            except OSError:raise RuntimeError('CLI exited before next conversation prompt')
            if not chunk:raise RuntimeError('CLI closed before next conversation prompt')
            output+=chunk
            print(chunk.decode('utf-8',errors='replace'),end='',flush=True)
            pending=output[approval_end:]
            approval=re.search(rb'Allow OffGrid to ([^\r\n]+?) for this host session\? \[y(?:/remember)?/N\]',pending)
            if approval:
                if str(root).encode() not in pending:raise RuntimeError('Approval targeted another root')
                level=approval.group(1)
                if level in approval_levels or len(approval_levels)>=3:raise RuntimeError('Repeated same-scope approval')
                approval_levels.add(level);approval_end=len(output)
                os.write(master,b'y\n');approved=True
            if re.search(rb'(?:^|\r?\n)> $',output[start:]):return
        if process.poll() is not None:raise RuntimeError('CLI exited before conversation finished')
    raise RuntimeError('Bounded turn timed out; the same CLI will be stopped, never resubmitted')
try:
    until_prompt(0,60)
    prompts=[
        'List files in this directory.',
        'Read alpha-cafe.txt from this same folder and return its complete exact text.',
        'Create a new result.txt containing exactly the text between these markers, excluding the markers: <content>'+wanted+'</content>. Read it back to verify exact equality. Leave all other files unchanged.',
        'Run go test ./... in this workspace and report the actual exit result. Do not edit any files.',
    ]
    prefix=['hello', 'Explain the difference between a relative path and an absolute path.'] if check_abstention else []
    prompts=prefix+prompts
    for index,prompt in enumerate(prompts):
        start=len(output);started=time.monotonic();os.write(master,prompt.encode()+b'\n');until_prompt(start)
        text=output[start:].decode('utf-8',errors='replace')
        ids=set(re.findall(r'(?:Run: |Inspect: offgrid agent status )(run-[a-f0-9]{32})',text))
        if len(ids)!=1:raise RuntimeError('Missing or duplicated root identity')
        job=api('/api/v2/jobs/'+next(iter(ids)));runs.append(job)
        (evidence/f'turn-{index+1}.json').write_text(json.dumps(job,ensure_ascii=False,indent=2))
        if job.get('status')!='completed':raise RuntimeError('Turn failed: '+str(job.get('error')))
        steps=job.get('steps',[])
        if any(s.get('tool_name') in ['list_files','read_file','write_file','shell'] for s in steps):raise RuntimeError('Service filesystem/legacy shell fallback')
        actions=[s for s in steps if s.get('type')=='action']
        phase=index-len(prefix)
        if phase>=0 and job.get('execution_workspace',{}).get('root')!=str(root):raise RuntimeError('Local work lost its selected folder')
        quality_error=None
        if phase<0:
            if actions:quality_error='Conversational request invoked workspace actions'
            if not job.get('output','').strip():quality_error='Conversational request did not answer'
            if phase==-1 and not all(word in job['output'].lower() for word in ['relative','absolute']):quality_error='Explanation did not address the actual question'
        if phase==0:
            if any(name not in job.get('output','') for name in [*before,'reports']):raise RuntimeError('Assistant omitted observed entries')
        if phase==1:
            if not any(s.get('tool_name')=='workspace_read' and json.loads(s['tool_result']).get('text')==source for s in actions) or source.strip() not in job.get('output',''):raise RuntimeError('Follow-up read did not return exact source')
        if phase==2:
            if (root/'result.txt').read_bytes()!=wanted.encode():raise RuntimeError('Incorrect written bytes')
            writes=[s for s in actions if s.get('tool_name') in ['workspace_write','workspace_write_from_input']]
            if require_source:
                if not writes or any(s.get('tool_name')!='workspace_write_from_input' for s in writes):raise RuntimeError('Write did not use the required typed input source')
                for step in writes:
                    if not verified_input_write(step,prompt,wanted,job['execution'],root/'result.txt'):raise RuntimeError('Missing or mismatched native read-back/source provenance')
            explicit_read=any(s.get('tool_name')=='workspace_read' and json.loads(s['tool_result']).get('text')==wanted for s in actions)
            native_read=require_source and writes and all(verified_input_write(s,prompt,wanted,job['execution'],root/'result.txt') for s in writes)
            if not explicit_read and not native_read:raise RuntimeError('No independent exact read-back')
        if phase==3:
            receipts=[json.loads(s['tool_result']).get('receipt',{}).get('result',{}) for s in actions if s.get('tool_name','').startswith('workspace_process_')]
            if not any(p.get('state')=='exited' and p.get('exit_code')==0 and b'offgrid-native-fixture' in base64.b64decode(p.get('output') or '') for p in receipts):raise RuntimeError('No independently recorded successful process exit/output')
            answer=job.get('output','').lower()
            if not any(word in answer for word in ['passed','successful','success','exit code: 0','exit code 0','exit status 0','exited with code 0']):raise RuntimeError('Assistant failed to report the actual successful test result')
            if sum(s.get('tool_name')=='workspace_process_start' for s in actions)!=1:raise RuntimeError('Requested command was unnecessarily restarted')
        for name,digest in before.items():
            if hashlib.sha256((root/name).read_bytes()).hexdigest()!=digest:raise RuntimeError('Original file changed: '+name)
        expected_paths=original_paths|({'result.txt'} if phase>=2 else set())
        if {str(p.relative_to(root)) for p in root.rglob('*')}!=expected_paths:raise RuntimeError('Unexpected fixture files/directories; no further commands authorized')
        attached=[r for r in runs if r.get('execution')]
        if phase>=0 and not attached:raise RuntimeError('Conversation lost execution session')
        if len(attached)>1 and not valid_conversation_workspace_transition(attached[-2],attached[-1]):raise RuntimeError('Conversation changed workspace without a recorded permission upgrade')
        results.append({'turn':index+1,'phase':phase,'run':job['run_id'],'elapsed_seconds':round(time.monotonic()-started,3),'actions':len(actions),'passed':quality_error is None,**({'error':quality_error} if quality_error else {})})
    os.write(master,b'/exit\n');process.wait(timeout=15)
    if process.returncode!=0 or not approved or output.count(b'Allow OffGrid to')!=len(approval_levels):raise RuntimeError('CLI exit or scoped consent failed')
    summary={'passed':all(row['passed'] for row in results),'model':model,'required_input_source':require_source,'checked_abstention':check_abstention,'approval_count':len(approval_levels),'evidence':str(evidence),'turns':results}
    (evidence/'summary.json').write_text(json.dumps(summary,ensure_ascii=False,indent=2))
    print(json.dumps(summary,ensure_ascii=False))
    if not summary['passed']:raise SystemExit(1)
finally:
    if process.poll() is None:
        os.killpg(process.pid,signal.SIGINT)
        try:process.wait(timeout=15)
        except subprocess.TimeoutExpired:os.killpg(process.pid,signal.SIGTERM);process.wait(timeout=5)
    os.close(master)
    (evidence/'transcript.txt').write_bytes(output)
    for run in set(re.findall(r'(?:Run: |Inspect: offgrid agent status )(run-[a-f0-9]{32})',output.decode('utf-8',errors='replace'))):
        saved=api('/api/v2/jobs/'+run)
        (evidence/(run+'.json')).write_text(json.dumps(saved,ensure_ascii=False,indent=2))
        if saved.get('execution_workspace',{}).get('root')==str(root) and saved.get('status') in ['pending','running','waiting_for_approval','waiting_for_input','waiting_for_children']:
            (evidence/(run+'.cleanup.json')).write_text(json.dumps(api('/api/v2/jobs/'+run+'/cancel',{}),ensure_ascii=False,indent=2))
    api('/api/v2/agent-profiles/'+profile['id']+'/archive',{'expected_revision':profile['revision']})
