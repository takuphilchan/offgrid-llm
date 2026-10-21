import test from 'node:test';
import assert from 'node:assert/strict';
import { execFileSync } from 'node:child_process';
import { existsSync, mkdirSync, mkdtempSync, readFileSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { isAbsolute, join, relative, sep } from 'node:path';
import { inventory, outputTargets, quarantine, validateQuarantine } from './workspace-cleanup.mjs';

function fixture(t, ignored = true) {
  const base = mkdtempSync(join(tmpdir(), 'offgrid-cleanup-fixture-'));
  const root = join(base, 'repo');
  mkdirSync(root);
  execFileSync('git', ['init', '--quiet'], { cwd: root });
  writeFileSync(join(root, '.gitignore'), ignored ? 'node_modules/\n' : '');
  writeFileSync(join(root, 'README.md'), '# fixture\n');
  execFileSync('git', ['add', '.gitignore', 'README.md'], { cwd: root });
  const output = join(root, 'apps', 'web', 'node_modules');
  mkdirSync(output, { recursive: true });
  writeFileSync(join(output, 'module.js'), 'export default 1;\n');
  t.after(() => {
    const rel = relative(tmpdir(), base);
    assert.ok(rel.startsWith('offgrid-cleanup-fixture-') && !rel.includes(sep) && isAbsolute(base));
    rmSync(base, { recursive: true, force: true });
  });
  return { base, root, output, destination: join(base, 'quarantine') };
}

test('inventory hashes nonignored source and enumerates only known outputs', t => {
  const { root } = fixture(t);
  const result = inventory(root);
  assert.ok(result.source.some(item => item.path === 'README.md' && item.sha256.length === 64));
  assert.ok(!result.source.some(item => item.path.includes('node_modules')));
  assert.deepEqual(outputTargets(root).map(item => item.path), ['apps/web/node_modules']);
  assert.equal(result.outputs[0].files, 1);
});

test('quarantine writes a recoverable manifest and preserves source digest', t => {
  const { base, root, output, destination } = fixture(t);
  const before = inventory(root);
  const result = quarantine(root, before, destination, base);
  assert.equal(result.entries[0].state, 'moved');
  assert.equal(existsSync(output), false);
  assert.equal(readFileSync(join(destination, 'apps/web/node_modules/module.js'), 'utf8'), 'export default 1;\n');
  assert.equal(JSON.parse(readFileSync(join(destination, 'quarantine-manifest.json'), 'utf8')).entries[0].state, 'moved');
  assert.equal(inventory(root).sourceDigest, before.sourceDigest);
});

test('rejects changed source and occupied destinations before moving', t => {
  const { base, root, output, destination } = fixture(t);
  const before = inventory(root);
  writeFileSync(join(root, 'README.md'), '# changed\n');
  assert.throws(() => validateQuarantine(root, before, destination, base), /Source changed/);
  writeFileSync(join(root, 'README.md'), '# fixture\n');
  mkdirSync(destination);
  assert.throws(() => validateQuarantine(root, before, destination, base), /already exists/);
  assert.equal(existsSync(output), true);
});

test('rejects tracked or nonignored material inside an output target', t => {
  const { base, root, output, destination } = fixture(t);
  execFileSync('git', ['add', '-f', 'apps/web/node_modules/module.js'], { cwd: root });
  assert.throws(() => validateQuarantine(root, inventory(root), destination, base), /Source inside output target/);
  assert.equal(existsSync(output), true);
});

test('rejects an output target that is not ignored', t => {
  const { base, root, output, destination } = fixture(t, false);
  rmSync(join(output, 'module.js'));
  assert.throws(() => validateQuarantine(root, inventory(root), destination, base), /Output target is not ignored/);
  assert.equal(existsSync(output), true);
});

test('rejects destination under the repository or another volume', t => {
  const { root, base } = fixture(t);
  const before = inventory(root);
  assert.throws(() => validateQuarantine(root, before, join(root, 'backup'), base), /outside the repository/);
  mkdirSync(join(base, 'another'));
  assert.throws(() => validateQuarantine(root, before, join(base, 'another', 'quarantine'), base), /approved backup directory/);
});
