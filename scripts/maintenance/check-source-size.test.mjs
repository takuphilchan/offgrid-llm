import test from 'node:test';
import assert from 'node:assert/strict';
import { isHandwrittenProduction, lineCount, sourceSizeIssues } from './check-source-size.mjs';

const lines = count => `${'x\n'.repeat(count)}`;
const exception = (path, baselineLines = 601) => ({
  path, owner: 'Inference maintainers', responsibility: 'Reviewed cohesive runtime logic',
  reviewDate: '2027-04-21', baselineLines,
});

test('hard entrypoint limit and new production file limit', () => {
  const issues = sourceSizeIssues([
    ['cmd/offgrid/main.go', lines(151)], ['internal/new.go', lines(601)],
  ], [], '2026-10-21');
  assert.match(issues.join('\n'), /150-line entrypoint limit/);
  assert.match(issues.join('\n'), /without a reviewed exception/);
});

test('reviewed exception is a no-growth baseline', () => {
  assert.deepEqual(sourceSizeIssues([['internal/model.go', lines(601)]], [exception('internal/model.go')], '2026-10-21'), []);
  assert.match(sourceSizeIssues([['internal/model.go', lines(602)]], [exception('internal/model.go')], '2026-10-21').join('\n'), /exceeds the reviewed/);
});

test('exceptions require current ownership and real paths', () => {
  const bad = { ...exception('internal/missing.go'), owner: '', reviewDate: '2020-01-01' };
  const issues = sourceSizeIssues([], [bad], '2026-10-21').join('\n');
  assert.match(issues, /stale or non-production/);
  assert.match(issues, /owner and responsibility/);
  assert.match(issues, /current ISO review date/);
});

test('generated and historical content is excluded', () => {
  assert.equal(isHandwrittenProduction('apps/web/src/api/schema.generated.ts'), false);
  assert.equal(isHandwrittenProduction('apps/web/src/workspace.css'), true);
  assert.equal(isHandwrittenProduction('internal/agents/agent_test.go'), false);
  assert.equal(isHandwrittenProduction('openspec/changes/old/code.go'), false);
  assert.deepEqual(sourceSizeIssues([['apps/web/src/api/schema.generated.ts', lines(5000)]], []), []);
  assert.equal(lineCount('one\ntwo\n'), 2);
});
