import test from 'node:test';
import assert from 'node:assert/strict';
import { mkdtemp, readdir, rm } from 'node:fs/promises';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { setTimeout as delay } from 'node:timers/promises';
import {createServer} from 'node:http';
import { BrowserDriver } from '../browser.mjs';

for (const first of ['click', 'download']) {
  test(`download failure handles both rejections when ${first} fails first, without replay`, async () => {
    const directory = await mkdtemp(join(tmpdir(), 'offgrid-download-failure-'));
    let clicks = 0;
    const fail = async (name) => {
      await delay(name === first ? 5 : 30);
      throw new Error(`${name} failed`);
    };
    const server=createServer((req,res)=>res.end('<a href="/report" download="report.txt">Report</a>'));
    await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));let driver;
    try {
      driver=await BrowserDriver.open(`http://127.0.0.1:${server.address().port}`,{headless:true,testLoopback:true,downloadDir:directory});
      const view=await driver.observe(),element=view.elements[0].id;
      driver.page.waitForEvent=()=>fail('download');
      driver.elements.get(element).click=()=>{clicks++;return fail('click');};
      const action={observation_id:view.observation_id,element};
      await assert.rejects(driver.execute('browser_download', action), new RegExp(`${first} failed`));
      await delay(50); // The later rejection must also be consumed.
      assert.equal(clicks, 1);
      assert.equal(driver.observation, '');
      assert.deepEqual(await readdir(directory), []);
      await assert.rejects(driver.execute('browser_download', action), /Stale observation/);
      assert.equal(clicks, 1);
    } finally {
      await driver?.close();await new Promise(resolve=>server.close(resolve));
      await rm(directory, { recursive: true, force: true });
    }
  });
}
