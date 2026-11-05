const { test } = require('node:test');
const assert = require('node:assert/strict');
const { createTrayFeedback } = require('../tray-feedback');
const catalog = require('../../web/public/desktop-presentation.json');

test('Windows tray explanation is quiet, localized and shown only once per launch', () => {
  for (const row of Object.values(catalog)) {
    const notices = [];
    const report = createTrayFeedback({ platform: 'win32', tray: () => ({ displayBalloon: notice => notices.push(notice) }), copy: key => row[key] });
    report(); report();
    assert.deepEqual(notices, [{ title: row.trayBackgroundTitle, content: row.trayBackgroundBody, iconType: 'info', noSound: true, respectQuietTime: true }]);
    assert.ok(notices[0].title && notices[0].content);
  }
});

test('notification refusal cannot prevent closing or repeat failures on every close', () => {
  let calls = 0;
  const report = createTrayFeedback({ platform: 'win32', tray: () => ({ displayBalloon: () => { calls++; throw Error('notifications unavailable'); } }), copy: key => catalog.en[key] });
  assert.doesNotThrow(report);
  assert.doesNotThrow(report);
  assert.equal(calls, 1);
});

test('missing tray and unsupported platforms never invoke Windows notification APIs', () => {
  for (const platform of ['darwin', 'linux']) {
    createTrayFeedback({ platform, tray: () => ({ displayBalloon: () => assert.fail('Windows-only API') }), copy: () => assert.fail('No notice') })();
  }
  let tray;
  let calls = 0;
  const report = createTrayFeedback({ platform: 'win32', tray: () => tray, copy: key => catalog.en[key] });
  assert.doesNotThrow(report);
  tray = { displayBalloon: () => calls++ };
  report(); report();
  assert.equal(calls, 1);
});
