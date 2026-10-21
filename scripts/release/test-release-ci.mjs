import assert from 'node:assert/strict';
import { verifyCI } from '../../.github/scripts/verify-release-ci.mjs';
const sha = 'a'.repeat(40);
const run = (value = {}) => ({id:1, head_sha:sha, event:'push', run_attempt:1, status:'completed', conclusion:'success', ...value});
const check = runs => verifyCI({sha, load:async()=>({workflow_runs:runs}), delay:async()=>{}, attempts:2});
assert.equal(await check([run()]), 1);
for (const runs of [[], [run({head_sha:'b'.repeat(40)})], [run({event:'pull_request'})],
  [run({conclusion:'failure'})], [run({conclusion:'cancelled'})], [run({conclusion:'skipped'})],
  [run(), run({id:2, conclusion:'failure'})], [run(), run({run_attempt:2, conclusion:'failure'})]]) {
  await assert.rejects(check(runs));
}
let calls = 0;
assert.equal(await verifyCI({sha, attempts:2, delay:async()=>{}, load:async()=>({workflow_runs:[run(++calls === 1 ? {status:'in_progress',conclusion:null} : {})]})}), 1);
await assert.rejects(check([run({status:'queued',conclusion:null})]), /deadline/);
await assert.rejects(verifyCI({sha:'main', load:async()=>{}, delay:async()=>{}}), /immutable/);
await assert.rejects(verifyCI({sha, load:async()=>({}), delay:async()=>{}}), /Invalid/);
console.log('Release CI gate rejects missing, wrong-source, failed, cancelled, skipped and superseded checks; waits for pending checks');
