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

from native_agent_fixture import configure

cli,model,api=configure('Real-model owned-process Stop qualification in a disposable folder')
health=api('/health')['inference']
if health['active'] or health['queued'] or health['switching']:raise SystemExit('Inference busy; no Stop fixture started')
evidence=Path(tempfile.mkdtemp(prefix='offgrid-native-stop-'));root=evidence/'workspace';root.mkdir()
prior_ids={job['id'] for job in api('/api/v2/jobs')}

def owned_runs():
    # The foreground UI intentionally keeps run IDs in the final inspection
    # footer. Before cancellation, identify only new canonical jobs whose
    # service-confirmed workspace is this unique disposable directory.
    candidates=[job['id'] for job in api('/api/v2/jobs') if job['id'] not in prior_ids]
    if len(candidates)>100:raise RuntimeError('Ambiguous fixture activity; no job control attempted')
    observed=set(re.findall(r'(?:Run: |Inspect: offgrid agent status )(run-[a-f0-9]{32})',output.decode('utf-8',errors='replace')))
    owned=set()
    for run in candidates:
        job=api('/api/v2/jobs/'+run)
        selected=(job.get('execution_workspace') or {}).get('root')
        # Before attachment, the own PTY's canonical footer still identifies a
        # submitted task. Never infer ownership from prompt text alone.
        if selected==str(root) or selected is None and run in observed and job.get('prompt')==prompt:
            owned.add(run)
    return owned
(root/'go.mod').write_text('module offgrid-stop-fixture\n\ngo 1.24\n')
(root/'stop_test.go').write_text('package fixture\nimport("testing";"os";"strconv";"time")\nfunc TestWaiting(t *testing.T){os.WriteFile("started.pid",[]byte(strconv.Itoa(os.Getpid())),0600);time.Sleep(45*time.Second);os.WriteFile("late-result.txt",[]byte("should not complete"),0600)}\n')
master,slave=pty.openpty()
prompt='Run go test ./... in this directory and report the actual exit result. Do not edit files.'
process=subprocess.Popen([cli,'agent','run',prompt,'--model',model,'--workspace-access','trusted','--max-steps','12'],cwd=root,stdin=slave,stdout=slave,stderr=slave,start_new_session=True)
os.close(slave);output=b'';approved=False;pid=None;generation=None
def identity(pid):
    try:return Path(f'/proc/{pid}/stat').read_text().rsplit(') ',1)[1].split()[19]
    except (OSError,IndexError):return None
try:
    deadline=time.monotonic()+150
    while time.monotonic()<deadline:
        ready,_,_=select.select([master],[],[],.02)
        if ready:
            try:chunk=os.read(master,65536)
            except OSError:raise RuntimeError('CLI stopped before the fixture process started')
            output+=chunk;print(chunk.decode('utf-8',errors='replace'),end='',flush=True)
            if not approved and b'Allow OffGrid to' in output and (b'[y/N]' in output or b'[y/remember/N]' in output):
                if str(root).encode() not in output:raise RuntimeError('Wrong local approval target')
                os.write(master,b'y\n');approved=True
            if b'Status: waiting_for_approval' in output:raise RuntimeError('Routine project command unexpectedly needed exact-call approval')
        marker=root/'started.pid'
        if marker.exists():
            pid=int(marker.read_text());generation=identity(pid)
            if generation is None or Path(f'/proc/{pid}/cwd').resolve()!=root:raise RuntimeError('Fixture PID identity not established')
            break
        if process.poll() is not None:raise RuntimeError('CLI exited before fixture process started')
    else:raise RuntimeError('Bounded fixture did not start; no retry')
    started=time.monotonic();os.killpg(process.pid,signal.SIGINT)
    while time.monotonic()-started<5 and identity(pid)==generation:time.sleep(.02)
    elapsed=time.monotonic()-started
    if identity(pid)==generation:raise RuntimeError('Native Stop did not terminate its owned test process')
    process.wait(timeout=15)
    if (root/'late-result.txt').exists():raise RuntimeError('Owned command completed after Stop')
    ids=owned_runs()
    if len(ids)!=1 or not approved or output.count(b'Allow OffGrid to')!=1:raise RuntimeError('Missing canonical root or consent scope')
    run=next(iter(ids));job=api('/api/v2/jobs/'+run)
    (evidence/'job.json').write_text(json.dumps(job,ensure_ascii=False,indent=2))
    if job.get('execution_workspace',{}).get('root')!=str(root) or job.get('status') not in ['cancelled','interrupted','uncertain','failed']:raise RuntimeError('Stop outcome was falsely reported as completion')
    print(json.dumps({'passed':True,'run':run,'model':model,'evidence':str(evidence),'owned_process_gone_ms':round(elapsed*1000,2),'status':job['status'],'late_write':False}))
finally:
    if process.poll() is None:
        os.killpg(process.pid,signal.SIGINT)
        try:process.wait(timeout=15)
        except subprocess.TimeoutExpired:os.killpg(process.pid,signal.SIGTERM);process.wait(timeout=5)
    # Failure cleanup is separate from, and cannot make, a passing verdict.
    os.close(master);(evidence/'transcript.txt').write_bytes(output)
    for run in owned_runs():
        job=api('/api/v2/jobs/'+run)
        if job.get('status') in ['running','pending','waiting_for_approval','waiting_for_input','waiting_for_children']:
            (evidence/'cleanup.json').write_text(json.dumps(api('/api/v2/jobs/'+run+'/cancel',{}),indent=2))
