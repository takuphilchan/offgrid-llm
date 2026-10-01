import {test} from 'node:test';
import assert from 'node:assert/strict';
import {checkDocument, maintained, prose} from './check-docs.mjs';

const paths = new Set(['README.md', 'docs/guide.md', 'docs/setup/quickstart.md', 'docs/with space.md']);
test('valid relative, root, encoded, directory and external links', () => {
  const content = '[Read](../README.md) [Start](setup/quickstart.md#first) [Root](/README.md) [Space](<with space.md>) [Encoded](with%20space.md) [Dir](setup/) [Web](https://example.com) [Local](#here)';
  assert.deepEqual(checkDocument('docs/guide.md', content, paths), []);
});
test('case-sensitive file identity on all operating systems', () => {
  assert.match(checkDocument('docs/guide.md', '[Start](setup/Quickstart.md)', paths)[0].message, /case mismatch/);
});
test('missing images and reference targets have line numbers', () => {
  const errors = checkDocument('docs/guide.md', 'Title\n![image](missing.png)\n[ref]: missing.md "Title"', paths);
  assert.deepEqual(errors.map(e => e.line), [2,3]);
});
test('code examples do not become live links; variable fences close correctly', () => {
  assert.deepEqual(checkDocument('docs/guide.md', '````md\n```\n[bad](missing.md)\n```\n````\n`[bad](missing.md)`', paths), []);
  assert.equal(prose('~~~md\nexample\n~~~').unclosedFence, undefined);
  assert.equal(prose('````md\n```').unclosedFence, 1);
});
test('archives and templates are historical or illustrative, not current instructions', () => {
  assert.equal(maintained('docs/releases/release-notes-v0.1.0.md'), false);
  assert.equal(maintained('docs/templates/guide-template.md'), false);
  assert.equal(maintained('docs/setup/quickstart.md'), true);
});
test('link titles and query fragments are not filenames', () => {
  assert.deepEqual(checkDocument('docs/guide.md', '[Start](setup/quickstart.md?view=1#first "Start here")', paths), []);
  assert.match(checkDocument('docs/guide.md', '[Bad](%zz.md)', paths)[0].message, /encoding/);
});
