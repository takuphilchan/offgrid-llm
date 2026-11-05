'use strict';

// Explain the existing tray lifecycle once per launch, without interrupting work.
function createTrayFeedback({ platform, tray, copy }) {
  let reported = false;
  return () => {
    const current = tray();
    if (reported || platform !== 'win32' || !current?.displayBalloon) return;
    reported = true;
    try {
      current.displayBalloon({
        title: copy('trayBackgroundTitle'), content: copy('trayBackgroundBody'),
        iconType: 'info', noSound: true, respectQuietTime: true,
      });
    } catch { /* Closing still works when OS notifications are unavailable. */ }
  };
}

module.exports = { createTrayFeedback };
