import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { existsSync, mkdtempSync, readFileSync, realpathSync, rmSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const repo = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const script = readFileSync(join(repo, '.github/scripts/verify-release-containers.sh'), 'utf8');

const cpuComplete = JSON.stringify({
  images: [
    { os: 'linux', architecture: 'amd64' },
    { os: 'linux', architecture: 'arm64' },
  ],
});
const cpuPartial = JSON.stringify({ images: [{ os: 'linux', architecture: 'amd64' }] });
const gpuComplete = JSON.stringify({ images: [{ os: 'linux', architecture: 'amd64' }] });

function run({ initialCpu = cpuComplete, cpu = cpuComplete, gpu = gpuComplete, attempts = '1' } = {}) {
  const root = mkdtempSync(join(tmpdir(), 'offgrid-container-verify-'));
  const mock = `
curl() {
  url="\${*: -1}"
  if [[ "\${url}" == *'-gpu/' ]]; then
    printf '%s' '${gpu}'
    return
  fi
  count_file='.mock-curl-count'
  mock_attempt=$(cat "\${count_file}" 2>/dev/null || echo 0)
  mock_attempt=$((mock_attempt + 1))
  printf '%s' "\${mock_attempt}" > "\${count_file}"
  if [[ "\${mock_attempt}" -eq 1 ]]; then
    printf '%s' '${initialCpu}'
  else
    printf '%s' '${cpu}'
  fi
}
set -- 'v0.4.2'
`;
  try {
    return spawnSync('bash', ['-s'], {
      cwd: root,
      input: mock + script,
      encoding: 'utf8',
      env: {
        ...process.env,
        OFFGRID_CONTAINER_VERIFY_ATTEMPTS: attempts,
        OFFGRID_CONTAINER_VERIFY_RETRY_SECONDS: '0',
      },
    });
  } finally {
    if (dirname(realpathSync(root)) !== realpathSync(tmpdir())) {
      throw new Error(`Refusing to remove unexpected test directory: ${root}`);
    }
    rmSync(root, { recursive: true, force: true });
  }
}

const complete = run();
assert.equal(complete.status, 0, complete.stdout + complete.stderr);
assert.match(complete.stdout, /Verified .*0\.4\.2.*AMD64 and ARM64/);
assert.match(complete.stdout, /Verified .*0\.4\.2-gpu.*AMD64/);

const partial = run({ initialCpu: cpuPartial, cpu: cpuPartial });
assert.notEqual(partial.status, 0, 'A single-platform CPU tag must not pass');
assert.match(partial.stdout, /cpu=false/);

const eventuallyComplete = run({ initialCpu: cpuPartial, attempts: '2' });
assert.equal(eventuallyComplete.status, 0, eventuallyComplete.stdout + eventuallyComplete.stderr);
assert.match(eventuallyComplete.stdout, /not fully indexed/);

console.log('Container release verification checks both CPU platforms, GPU, and registry indexing retries');

for (const path of ['.github/workflows/release-unified.yml', 'docker/Dockerfile', 'docker/Dockerfile.gpu']) {
  const build = readFileSync(join(repo, path), 'utf8');
  assert.match(build, /LLAMA_BUILD_UI=OFF/, `${path} must disable local llama.cpp UI builds`);
  assert.match(build, /LLAMA_USE_PREBUILT_UI=OFF/, `${path} must not download llama.cpp UI assets`);
}
console.log('Release and Docker llama.cpp builds do not depend on optional UI downloads');

// Exercise the workflow's alias decision with an immutable source that would
// otherwise be the latest tag. Preview publication must still disable aliases.
const workflow=readFileSync(join(repo,'.github/workflows/docker-publish.yml'),'utf8').replace(/\r\n/g,'\n');
const versionStep=workflow.split('- name: Resolve build version\n')[1].split('\n      - name:')[0];
const versionScript=versionStep.split('        run: |\n')[1].split('\n')
  .filter(line=>line.startsWith('          ')).map(line=>line.slice(10)).join('\n');
assert.ok(workflow.includes('force_rebuild:\n        description: Rebuild existing versioned image tags'));
assert.ok(workflow.includes('source_ref:\n        description: Immutable vX.Y.Z tag to build from (required for a forced rebuild)'));
assert.ok(!versionScript.includes('${{'),'Version resolver must read dispatch values from environment variables');
function resolveVersion({event='workflow_dispatch',ref='refs/heads/main',version='0.4.32',sourceRef='',forceRebuild=false}={}) {
  const root=mkdtempSync(join(tmpdir(),'offgrid-container-version-'));
  try {
    const output=join(root,'output');
    const result=spawnSync('bash',['-euo','pipefail','-s'],{cwd:root,encoding:'utf8',
      env:{...process.env,GITHUB_OUTPUT:output,GITHUB_EVENT_NAME:event,GITHUB_REF:ref,
        INPUT_VERSION:version,INPUT_SOURCE_REF:sourceRef,FORCE_REBUILD:String(forceRebuild)},
      input:versionScript});
    return {result,output:existsSync(output)?readFileSync(output,'utf8'):''};
  } finally {
    assert.equal(dirname(realpathSync(root)),realpathSync(tmpdir()));
    rmSync(root,{recursive:true,force:true});
  }
}
const forcedTag=resolveVersion({sourceRef:'v0.4.32',forceRebuild:true});
assert.equal(forcedTag.result.status,0,forcedTag.result.stdout+forcedTag.result.stderr);
assert.match(forcedTag.output,/value=0\.4\.32/);
assert.match(forcedTag.output,/source_ref=refs\/tags\/v0\.4\.32/);
for(const invalid of [
  {sourceRef:'v0.4.31',forceRebuild:true},
  {sourceRef:'',forceRebuild:true},
  {sourceRef:'refs/heads/main',forceRebuild:true},
  {version:'0.4.32\nsource_ref=v0.4.31'},
]) {
  const rejected=resolveVersion(invalid);
  assert.notEqual(rejected.result.status,0,'A forced rebuild must require its matching immutable release tag');
}
const ordinaryDispatch=resolveVersion({version:'edge'});
assert.equal(ordinaryDispatch.result.status,0,ordinaryDispatch.result.stdout+ordinaryDispatch.result.stderr);
assert.match(ordinaryDispatch.output,/source_ref=refs\/heads\/main/);
const cpuRepairStep=workflow.split('- name: Apply reviewed CPU packaging from automation revision\n')[1]?.split('\n      - name:')[0];
assert.ok(cpuRepairStep?.includes('git show "${GITHUB_SHA}:docker/Dockerfile" > docker/Dockerfile'),
  'A repair branch or explicit tag dispatch must apply its reviewed CPU Dockerfile while retaining tagged application source');
assert.ok(cpuRepairStep?.includes("github.event_name == 'workflow_dispatch' && inputs.source_ref != ''"));
const gpuRepairStep=workflow.split('- name: Apply reviewed GPU packaging from automation revision\n')[1]?.split('\n      - name:')[0];
assert.ok(gpuRepairStep?.includes('git show "${GITHUB_SHA}:docker/Dockerfile.gpu" > docker/Dockerfile.gpu'),
  'A repair branch or explicit tag dispatch must apply reviewed GPU packaging while retaining tagged application source');
assert.ok(gpuRepairStep?.includes("github.event_name == 'workflow_dispatch' && inputs.source_ref != ''"));
const sourceStep=workflow.split('- name: Record source revision\n')[1].split('\n      - name:')[0];
const sourceScript=sourceStep.split('        run: |\n')[1].split('\n')
  .filter(line=>line.startsWith('          ')).map(line=>line.slice(10)).join('\n');
for(const promote of [true,false]) {
  const root=mkdtempSync(join(tmpdir(),'offgrid-container-channel-'));
  try {
    const input=sourceScript
      .replaceAll('${{ steps.version.outputs.source_ref }}','v0.4.29')
      .replaceAll("${{ toJSON(inputs.promote_latest) != 'false' }}",String(promote));
    assert.ok(!input.includes('${{'),'Every workflow expression must be resolved');
    const result=spawnSync('bash',['-euo','pipefail','-s'],{cwd:root,encoding:'utf8',
      env:{...process.env,GITHUB_OUTPUT:join(root,'output')},
      input:`git() { if [[ "$1" == tag ]]; then echo v0.4.29; else echo ${'a'.repeat(40)}; fi; }\n`+input});
    assert.equal(result.status,0,result.stdout+result.stderr);
    assert.match(readFileSync(join(root,'output'),'utf8'),new RegExp(`is_latest=${promote}`));
  } finally {rmSync(root,{recursive:true,force:true});}
}
const aliasLines=workflow.split('\n').filter(line=>line.includes('type=semver,pattern={{major}}.{{minor}}')||line.includes('type=raw,value=latest,'));
assert.equal(aliasLines.length,4);
for(const line of aliasLines)assert.ok(line.includes("enable=${{ needs.prepare.outputs.is_latest == 'true' }}"));
console.log('Preview CPU/GPU publication cannot promote latest or minor-version aliases');

const reuseStep=workflow.split('- name: Reuse complete published versioned images\n')[1].split('\n  cpu:')[0];
const reuseScript=reuseStep.split('        run: |\n')[1].split('\n')
  .filter(line=>line.startsWith('          ')).map(line=>line.slice(10)).join('\n');
const forcedReuseRoot=mkdtempSync(join(tmpdir(),'offgrid-container-forced-reuse-'));
try {
  const forcedReuseScript=reuseScript
    .replaceAll('${{ steps.version.outputs.value }}','0.4.32')
    .replaceAll('${{ steps.source.outputs.sha }}','a'.repeat(40));
  assert.ok(!forcedReuseScript.includes('${{'));
  const forcedReuse=spawnSync('bash',['-euo','pipefail','-s'],{cwd:forcedReuseRoot,encoding:'utf8',
    env:{...process.env,GITHUB_OUTPUT:join(forcedReuseRoot,'output'),RUNNER_TEMP:forcedReuseRoot,
      IMAGE_NAME:'example/offgrid',FORCE_REBUILD:'true'},
    input:`curl() { echo 'forced rebuild unexpectedly queried Docker Hub' >&2; return 99; }\n`+forcedReuseScript});
  assert.equal(forcedReuse.status,0,forcedReuse.stdout+forcedReuse.stderr);
  assert.match(readFileSync(join(forcedReuseRoot,'output'),'utf8'),/cpu=false\ngpu=false/);
  assert.match(forcedReuse.stdout,/Explicit rebuild/);
} finally {
  assert.equal(dirname(realpathSync(forcedReuseRoot)),realpathSync(tmpdir()));
  rmSync(forcedReuseRoot,{recursive:true,force:true});
}
for(const gpuStatus of [0,2,1]) {
  const root=mkdtempSync(join(tmpdir(),'offgrid-container-reuse-'));
  try {
    const input=reuseScript.replaceAll('${{ steps.version.outputs.value }}','0.4.30')
      .replaceAll('${{ steps.source.outputs.sha }}','a'.repeat(40));
    assert.ok(!input.includes('${{'));
    const result=spawnSync('bash',['-euo','pipefail','-s'],{cwd:root,encoding:'utf8',
      env:{...process.env,GITHUB_OUTPUT:join(root,'output'),RUNNER_TEMP:root,IMAGE_NAME:'example/offgrid',FORCE_REBUILD:'false'},
      input:`curl() { printf '200'; }\nnode() { [[ "$1" == .release-automation/scripts/release/verify-container-source.mjs && "$2" == example/offgrid && "$4" == ${'a'.repeat(40)} ]] || return 99; if [[ "$3" == *-gpu ]]; then return ${gpuStatus}; fi; return 0; }\n`+input});
    assert.equal(result.status,gpuStatus===1?1:0,result.stdout+result.stderr);
    const output=readFileSync(join(root,'output'),'utf8');
    assert.match(output,/cpu=true/);
    assert.equal(output.includes('gpu=true'),gpuStatus===0);
    if(gpuStatus===2)assert.match(result.stdout,/different source identity; rebuild/);
  } finally {
    assert.equal(dirname(realpathSync(root)),realpathSync(tmpdir()));
    rmSync(root,{recursive:true,force:true});
  }
}
const gpuMetadata=workflow.split('- name: Generate NVIDIA image metadata\n')[1].split('\n      - name:')[0];
assert.ok(gpuMetadata.includes('org.opencontainers.image.revision=${{ needs.prepare.outputs.source_sha }}'));
assert.ok(workflow.includes('org.offgrid.packaging.revision=${{ github.sha }}'));
assert.ok(workflow.includes('node scripts/release/verify-container-source.mjs "${IMAGE_NAME}" "${{ needs.prepare.outputs.version }}-gpu" "${{ needs.prepare.outputs.source_sha }}" amd64'));
console.log('Forced rebuilds require matching release tags; image reuse, packaging overlays, and source identity are verified');
