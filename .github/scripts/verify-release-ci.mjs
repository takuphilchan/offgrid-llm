import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';

export function latestCI(response, sha) {
  if (!Array.isArray(response?.workflow_runs)) throw Error('Invalid CI response');
  return response.workflow_runs.filter(run => run.head_sha === sha &&
    ['push', 'workflow_dispatch'].includes(run.event))
    .sort((a, b) => b.id - a.id || b.run_attempt - a.run_attempt)[0];
}

export async function verifyCI({ sha, load, delay, attempts = 120 }) {
  if (!/^[a-f0-9]{40}$/.test(sha)) throw Error('Expected an immutable commit SHA');
  for (let attempt = 0; attempt < attempts; attempt++) {
    const run = latestCI(await load(), sha);
    if (!run) throw Error(`No CI evidence for ${sha}; run ci.yml on the release tag before retrying`);
    if (run.status === 'completed') {
      if (run.conclusion !== 'success') throw Error(`CI ${run.id} concluded ${run.conclusion}; release remains blocked`);
      return run.id;
    }
    if (attempt + 1 < attempts) await delay();
  }
  throw Error('CI did not finish within the release gate deadline');
}

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  const [repo, sha] = process.argv.slice(2);
  try {
    if (!/^[\w.-]+\/[\w.-]+$/.test(repo ?? '')) throw Error('Expected owner/repository');
    const id = await verifyCI({ sha,
      load: () => JSON.parse(execFileSync('gh', ['api', `repos/${repo}/actions/workflows/ci.yml/runs?head_sha=${sha}&per_page=100`],
        { encoding: 'utf8', timeout: 30000, maxBuffer: 4 * 1024 * 1024 })),
      delay: () => new Promise(resolve => setTimeout(resolve, 15000)) });
    console.log(`Verified CI ${id} for immutable release source ${sha}`);
  } catch (error) {
    console.error(error.message); process.exitCode = 1;
  }
}
