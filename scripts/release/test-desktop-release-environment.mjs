import assert from 'node:assert/strict';
import { readFileSync } from 'node:fs';
import { spawnSync } from 'node:child_process';

// Exercise the workflow's actual shell body without invoking a build or using
// real credentials. Empty CSC_LINK is significant to macOS Electron Builder.
const workflow = readFileSync('.github/workflows/release-unified.yml', 'utf8').replace(/\r\n/g, '\n');
const step = workflow.split('      - name: Build desktop app\n')[1]?.split('\n      - name:')[0];
assert.ok(step, 'Desktop build step must exist');
const body = step.match(/        run: \|\n([\s\S]*?)        shell: bash/)[1].replace(/^          /gm, '');
const keys = ['CSC_LINK', 'CSC_KEY_PASSWORD', 'CSC_INSTALLER_LINK', 'CSC_INSTALLER_KEY_PASSWORD', 'CSC_KEYCHAIN',
  'WIN_CSC_LINK', 'WIN_CSC_KEY_PASSWORD', 'APPLE_ID', 'APPLE_APP_SPECIFIC_PASSWORD', 'APPLE_TEAM_ID',
  'APPLE_API_KEY', 'APPLE_API_KEY_ID', 'APPLE_API_ISSUER', 'APPLE_KEYCHAIN', 'APPLE_KEYCHAIN_PROFILE'];
for (const platform of ['macos', 'windows', 'linux']) {
  const build = { macos: 'build:mac', windows: 'build:win', linux: 'build:linux' }[platform];
  for (const channel of ['unsigned-preview', 'signed']) for (const value of ['', 'fixture-only']) {
    const script = body.replaceAll('${{ needs.release-context.outputs.channel }}', channel)
      .replaceAll('${{ matrix.os }}', platform).replaceAll('${{ matrix.build_script }}', build);
    assert.ok(!script.includes('${{'), 'Every workflow expression must be resolved by the fixture');
    const expected = channel === 'unsigned-preview'
      ? keys.map(key => `[[ ! -v ${key} ]] || exit 21`).join('\n') + '\n[[ "$CSC_IDENTITY_AUTO_DISCOVERY" == false ]] || exit 22'
      : keys.map(key => `[[ -v ${key} && "\${${key}}" == "$EXPECTED_VALUE" ]] || exit 23`).join('\n') + '\n[[ "$CSC_IDENTITY_AUTO_DISCOVERY" == true ]] || exit 24';
    const mock = `set -euo pipefail\nnpm() {\n[[ "$*" == "run ${build} -- --publish never" ]] || exit 20\n${expected}\nprintf 'environment-verified\\n'\n}\n`;
    const result = spawnSync('bash', ['-c', mock + script], { encoding: 'utf8',
      env: { ...process.env, ...Object.fromEntries(keys.map(key => [key, value])), EXPECTED_VALUE: value, CSC_IDENTITY_AUTO_DISCOVERY: 'true' } });
    assert.equal(result.status, 0, `${platform}/${channel}/${value || 'empty'}: ${result.stderr || result.stdout}`);
    assert.match(result.stdout, /environment-verified/);
  }
}
console.log('Desktop release environment: 12 platform/channel/credential fixtures passed; builder publication disabled.');
