import assert from 'node:assert/strict';
import { spawnSync } from 'node:child_process';
import { existsSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { dirname, join, resolve } from 'node:path';
import { fileURLToPath } from 'node:url';

const repo = resolve(dirname(fileURLToPath(import.meta.url)), '../..');
const workflow = readFileSync(join(repo, '.github/workflows/release-unified.yml'), 'utf8');

function step(name) {
  const section = workflow.split(`- name: ${name}\n`)[1]?.split('\n      - name:')[0];
  assert.ok(section, `Missing workflow step ${name}`);
  const script = section.split('        run: |\n')[1].split('\n')
    .filter((line) => line.startsWith('          ')).map((line) => line.slice(10)).join('\n');
  return script.replace(/\$\{\{\s*([^}]+?)\s*\}\}/g, (_, key) => {
    const values = {
      'steps.version.outputs.version': 'v0.4.2',
      'needs.create-release.outputs.release_id': '123',
      'needs.release-context.outputs.version': 'v0.4.2',
      'github.repository': 'example/offgrid-llm',
      'matrix.platform': 'linux',
      'matrix.os': 'linux',
      'matrix.arch': 'amd64',
      'matrix.variant': 'cpu',
      'matrix.cpu_features': 'avx2',
    };
    assert.ok(key in values, `Unexpected workflow expression ${key}`);
    return values[key];
  });
}

const mock = `
gh() {
  local filter='' arg previous=''
  for arg in "$@"; do
    if [[ "$previous" == '--jq' ]]; then filter="$arg"; fi
    previous="$arg"
  done
  if [[ "$1 $2" == 'release view' && "$*" == *'--json databaseId'* ]]; then
    printf '%s' '{"databaseId":123}' | jq -r "$filter"
  elif [[ "$1" == api && "$2" == 'repos/example/offgrid-llm/releases/123/assets?per_page=100' && "$*" == *'--paginate'* ]]; then
    if [[ "$MOCK_DENIED" == true ]]; then echo 'Forbidden' >&2; return 1; fi
    jq -r "$filter" assets.json
  else
    echo "Unexpected request (draft tag endpoint returns 404): $*" >&2
    return 1
  fi
}
`;

function run(name, assets, denied = false) {
  const root = mkdtempSync(join(tmpdir(), 'offgrid-asset-access-'));
  try {
    writeFileSync(join(root, 'assets.json'), JSON.stringify(assets));
    const output = join(root, 'output');
    const result = spawnSync('bash', ['-euo', 'pipefail', '-s'], {
      input: mock + step(name), cwd: root, encoding: 'utf8',
      env: { ...process.env, GITHUB_OUTPUT: output, GITHUB_REPOSITORY: 'example/offgrid-llm',
        RELEASE_VERSION: 'v0.4.2', MOCK_DENIED: String(denied) },
    });
    return { ...result, output: existsSync(output) ? readFileSync(output, 'utf8') : '' };
  } finally {
    rmSync(root, { recursive: true, force: true });
  }
}

const asset = (name) => ({ name, state: 'uploaded', size: 1024, digest: `sha256:${'a'.repeat(64)}` });
const bundle = asset('offgrid-v0.4.2-linux-amd64-cpu-avx2.tar.gz');
const desktop = ['OffGrid.LLM.Desktop-0.4.2-amd64.deb', 'OffGrid.LLM.Desktop-0.4.2-x86_64.AppImage'].map(asset);
const preflight = 'Verify draft asset access before starting builds';
for (const assets of [[], [bundle, ...desktop], [{ ...bundle, digest: null }]]) {
  const result = run(preflight, assets);
  assert.equal(result.status, 0, result.stdout + result.stderr);
  assert.equal(result.output, 'release_id=123\n');
}
const denied = run(preflight, [], true);
assert.notEqual(denied.status, 0);
assert.equal(denied.output, '', 'An inaccessible draft must block all builders');

for (const [name, assets] of [
  ['Reuse verified release bundle on rerun', [bundle]],
  ['Reuse verified desktop assets on rerun', desktop],
]) {
  const ready = run(name, assets);
  assert.equal(ready.status, 0, ready.stdout + ready.stderr);
  assert.equal(ready.output, 'skip=true\n');
  for (const missing of [[], assets.map((item) => ({ ...item, digest: null })), assets.slice(1)]) {
    const result = run(name, missing);
    assert.equal(result.status, 0, result.stdout + result.stderr);
    assert.equal(result.output, '', 'Missing/unindexed assets must rebuild, not fail or be reused');
  }
}
console.log('Workflow draft preflight and reuse queries pass with empty, complete, unindexed, and inaccessible JSON responses');

const source = 'a'.repeat(40);
for (const [draft, rebuild, succeeds, operations] of [
  [{isDraft:true,body:`<!-- offgrid-source: ${source} -->`},'false',true,''],
  [{isDraft:true,body:'old source'},'false',false,''],
  [{isDraft:true,body:'old source'},'true',true,'delete\ncreate\n'],
  [{isDraft:false,body:'published'},'true',false,''],
  [{isDraft:false,body:'published'},'false',true,''],
]) {
  const root=mkdtempSync(join(tmpdir(),'offgrid-draft-rebuild-'));
  try {
    writeFileSync(join(root,'draft.json'),JSON.stringify(draft));
    const result=spawnSync('bash',['-euo','pipefail','-s'],{cwd:root,encoding:'utf8',
      env:{...process.env,RUNNER_TEMP:root,REBUILD_DRAFT:rebuild,SOURCE_SHA:source},
      input:`gh() {
        case "$1 $2" in
          'release view') cat draft.json ;;
          'release delete') echo delete >> operations ;;
          'release create') echo create >> operations ;;
          *) return 1 ;;
        esac
      }\n`+step('Create draft release if missing')});
    assert.equal(result.status===0,succeeds,result.stdout+result.stderr);
    assert.equal(existsSync(join(root,'operations'))?readFileSync(join(root,'operations'),'utf8'):'',operations);
  } finally {rmSync(root,{recursive:true,force:true});}
}
console.log('Draft rebuild requires explicit approval, rejects published releases, and preserves matching-source retries');
