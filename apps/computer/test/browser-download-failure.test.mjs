import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, readdir, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import { BrowserDriver } from '../browser.mjs';

for (const first of ['click', 'download']) {
  test(`download failure handles both rejections when ${first} fails first, without replay`, async () => {
    const directory = await mkdtemp(join(tmpdir(), 'offgrid-download-failure-'));
    let clicks = 0;
    const fail = async (name) => {
      await delay(name === first ? 5 : 30);
      throw new Error(`${name} failed`);
    };
    const page = {
      isClosed: () => false,
      url: () => 'https://example.com/',
      evaluate: async () => 0,
      waitForEvent: () => fail('download'),
    };
    const driver = new BrowserDriver({}, page, 'https://example.com', { downloadDir: directory });
    Object.assign(driver, { observation: 'observed', observedAt: Date.now(), observedURL: page.url(), revision: 0, formDigest: 'unchanged' });
    driver.stateDigest = async () => 'unchanged';
    driver.elements.set('1', {
      isVisible: async () => true,
      isEnabled: async () => true,
      evaluate: async () => ({ sensitive: false }),
      getAttribute: async () => 'report.txt',
      click: () => { clicks++; return fail('click'); },
    });
    try {
      await assert.rejects(driver.execute('browser_download', { observation_id: 'observed', element: '1' }), new RegExp(`${first} failed`));
      await delay(50); // The later rejection must also be consumed.
      assert.equal(clicks, 1);
      assert.equal(driver.observation, '');
      assert.deepEqual(await readdir(directory), []);
      await assert.rejects(driver.execute('browser_download', { observation_id: 'observed', element: '1' }), /Stale observation/);
      assert.equal(clicks, 1);
    } finally {
      await rm(directory, { recursive: true, force: true });
    }
  });
}
