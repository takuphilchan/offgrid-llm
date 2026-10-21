// Real-service/model smoke test. Use only an isolated service on port 11613.
// This never controls a desktop or accesses public sites. Test jobs are retained
// in the isolated workspace as evidence, not written to the live user's history.
import assert from 'node:assert/strict';
import {createHash, randomUUID} from 'node:crypto';

const base = process.env.OFFGRID_TASK_TEST_URL ?? 'http://127.0.0.1:11613';
const url = new URL(base);
assert.equal(url.hostname, '127.0.0.1'); assert.equal(url.port, '11613');
const model = process.env.OFFGRID_TASK_TEST_MODEL ?? 'Qwen2.5-7B-Instruct-Q4_K_M';
async function request(path, body, method = body ? 'POST' : 'GET') {
  const response = await fetch(base + path, {method, headers: {'Content-Type':'application/json'}, body: body ? JSON.stringify(body) : undefined, signal: AbortSignal.timeout(30_000)});
  const value = await response.json(); assert(response.ok, `${response.status} ${JSON.stringify(value)}`); return value;
}
async function start(prompt) {
  const body = {prompt, model, request_id:randomUUID(), max_iterations:20};
  const job = await request('/api/v2/jobs', body);
  const repeated = await request('/api/v2/jobs', body); assert.equal(repeated.run_id, job.run_id);
  console.log(JSON.stringify({event:'submitted',run_id:job.run_id}));
  return job.run_id;
}
async function finish(id, expected = 'completed') {
  const deadline = Date.now()+300_000; let last;
  while (Date.now()<deadline) {
    const job = await request(`/api/v2/jobs/${id}`);
    if (job.status !== last) {console.log(JSON.stringify({event:'state',run_id:id,status:job.status}));last=job.status;}
    if (!['running','pending','waiting_for_children'].includes(job.status)) {
      assert.equal(job.status,expected,JSON.stringify({status:job.status,error:job.error,steps:job.steps.map(s=>s.tool_name)}));return job;
    }
    await new Promise(resolve=>setTimeout(resolve,1000));
  }
  await request(`/api/v2/jobs/${id}/cancel`,{});throw Error(`Timed out; stopped test job ${id}`);
}
const system = await request('/api/v2/system');
assert(system.capabilities.includes('bounded-readonly-delegation-v1'));
assert(system.capabilities.includes('verified-workspace-artifacts-v1'));
console.log(JSON.stringify({event:'system',version:system.version,revision:system.revision,ui:system.ui_build_id}));

const artifactID = await start('Create a downloadable workspace CSV artifact named comparison.csv with exactly this UTF-8 content, including the final newline: model,score\nA,82\nB,91\n Use task_save_artifact to actually save it. Do not use shell or write to a host path. Report the saved artifact.');
const artifactJob = await finish(artifactID);
assert.equal(artifactJob.artifacts.length,1);const artifact=artifactJob.artifacts[0];
const response=await fetch(`${base}/api/v2/jobs/${artifactID}/artifact?digest=${artifact.sha256}`);assert(response.ok);
const bytes=Buffer.from(await response.arrayBuffer());assert.equal(bytes.toString(),'model,score\nA,82\nB,91\n');assert.equal(createHash('sha256').update(bytes).digest('hex'),artifact.sha256);
assert.equal(artifact.rows,3);assert.equal(artifact.columns,2);
console.log(JSON.stringify({event:'artifact_verified',run_id:artifactID,sha256:artifact.sha256,bytes:bytes.length}));

const graphID = await start('Use delegate_tasks to create exactly two read-only subtasks. Key first: calculate 12 * 7 using calculator; no dependencies; tools [calculator]. Key second: use the first result and calculate that result + 6 using calculator; depends_on [first]; tools [calculator]. Wait for both saved child results, then report the final number. Do not calculate the subtasks yourself and do not request computer access.');
const graph = await finish(graphID);assert.equal(graph.children.length,2);
const children=await Promise.all(graph.children.map(child=>request(`/api/v2/jobs/${child.id}`)));
assert(children.every(child=>child.status==='completed'&&child.parent_id===graphID));
assert(children.every(child=>child.steps.some(step=>step.type==='action'&&step.tool_name==='calculator')));
assert.match(graph.output,/\b90\b/);
console.log(JSON.stringify({event:'graph_verified',run_id:graphID,children:graph.children.map(child=>child.id)}));

const accessID=await start('Read the open Notepad document. Request local access to the existing Notepad application first. Do not use shell or pretend you have read it.');
const access=await finish(accessID,'waiting_for_input');assert.equal(access.pending_input.mode,'app');
assert(!access.steps.some(step=>step.type==='action'));
await request(`/api/v2/jobs/${accessID}/cancel`,{});
console.log(JSON.stringify({event:'access_interruption_verified',run_id:accessID}));
console.log('PASS: real local model, verified CSV, durable dependency graph, idempotent submission and consent interruption. Not a native-driver or model-quality qualification.');
