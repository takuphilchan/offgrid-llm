// Only the trusted driver creates these failures after establishing that no
// input was invoked, or while performing an allowlisted read. Never classify
// provider exceptions by their message: input transport errors are uncertain.
export class BrowserFailure extends Error {
  constructor(code, message) {
    super(message);
    this.code = code;
  }
}

export const browserRead = kind => ['browser_observe','browser_capture','browser_verify','browser_inspect','browser_tabs','browser_search','browser_frames','browser_frame_select'].includes(kind);
export const safeBrowserFailure = error => error instanceof BrowserFailure &&
  ['computer_browser_precondition_failed','computer_browser_observation_failed'].includes(error.code);
