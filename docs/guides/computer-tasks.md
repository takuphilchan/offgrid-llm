# Use applications and browsers in agent tasks

Computer Tasks connects a saved agent task to a locally consented application or managed browser. It is a preview: the implemented controls and tests do not establish universal desktop automation or reliability on every model and OS.

## Start with an outcome

1. Open **Agents** in the matching desktop app, describe the result, and choose **Start task**.
2. When **Access needed** appears, review the requested application or site. Select or launch the actual local target.
3. Review the action policy and approve local access. OffGrid continues the same task using fresh observations.
4. Inspect activity and any requested approvals. Verify the actual saved result, not just the model's completion message.

Web users can choose **Continue in desktop** for the saved task. The desktop must use the same workspace; a link does not grant permissions or switch services silently. Normal packaged setup does not require Node, terminal commands, manual pairing codes, or an expected-page-text field.

A Linux service in Docker/WSL can coordinate work, but it cannot directly control your Windows desktop. The installed host components supply that boundary. Updating the container does not update those components; see [installation and upgrades](../setup/installation.md#update-an-existing-installation).

## Choose the right target

| Target | Implemented boundary | Important limit |
| --- | --- | --- |
| Existing application window | Structured accessibility observations and supported typed edits, selections, activations, and fixed shortcuts | Only controls the driver can address and verify; not unrestricted pointer/keyboard input |
| Existing browser window selected as an app | The native application's exposed accessibility controls | Not attachment to a personal browser profile through Playwright; support varies by browser and page |
| Managed browser | Separate bundled Chromium session, constrained navigation, structured controls, and governed transfers | No personal profile import; cross-origin/authentication workflows remain limited |
| Native vision or whole desktop | Not implemented as general native control in this checkout | Do not infer availability from a vision model or browser screenshot feature |

Native workers exist for Windows UI Automation, macOS Accessibility, and Linux AT-SPI. Linux X11 shortcuts and Wayland portal input are not interchangeable; general Wayland capture/input remains unfinished. All native profiles still need the recorded workflow qualification gates. See the [driver documentation](../../computer/README.md) and [reliability evidence](../advanced/product-reliability-plan.md).

## Approve changes at the right level

| Policy | Behavior inside the selected scope |
| --- | --- |
| Ask every time | Require exact-action approval for changes |
| Approve scoped changes | Automatically permit only supported, locally classified reversible changes; ask for consequential or ambiguous actions |
| Full task access | Permit available typed actions, including supported consequential operations, within the approved scope |

Full task access is not unrestricted OS access. Credential/payment entry, financial transactions, privilege changes, installation, arbitrary scripts, permanent deletion, and replay of uncertain actions remain blocked. The selected policy is fixed for that session; a model cannot broaden it. Unknown-effect controls are not automatically classified as harmless.

The default access panel uses **Approve scoped changes**. Permissions do not make unsupported controls work, and changing reasoning style does not repair invalid tool calls.

## Try a bounded task

Use a disposable document or draft, not valuable unsaved work.

**Application example:**

> In the selected text editor, replace the draft with a short grocery list containing bread, rice, and tomatoes. Read it back and confirm all three items are present. Do not save over an existing file.

**Public browser example:**

> Open the Writing tests page from the selected Playwright documentation page. Summarize the basic test structure and give the source URL. Do not submit forms or download files.

Start the browser at `https://playwright.dev/docs/intro`. Site and model behavior can change; these are suggested tests, not guarantees. A driver test on the local practice page isolates control problems but does not establish usefulness or planning quality on arbitrary websites.

## Files and vision

Managed-browser uploads require a locally selected file and approved destination. Downloads use verified staging and explicit host saving; they are never automatically executed. Native application control does not supply general folder operations or automatic file transfers.

Managed-browser viewport images can be used only with compatible installed model/projector components and applicable preflight. This does not implement native-window screenshots, general visual grounding, or a qualified vision profile. Structured observations remain the first choice. Do not use this preview on sensitive pages or assume redaction detects every secret.

## Stop and recover

Use **Pause**, **Take over**, or **Stop** in the task as appropriate. Take over revokes that task's computer session; the local emergency stop also works when the service cannot be reached. Stop is not undo: already dispatched effects may finish and must be inspected.

Reloading a page restores the saved task, not a new execution. Service restart invalidates computer access. Reconnect with fresh local consent rather than replaying old grants. Sessions are bounded; finish or renew access when the allowed budget expires.

If the result is **Outcome unknown**, inspect the affected target before reconciliation. Never retry a possibly submitted form just to clear the status. See [task recovery and deletion](agents.md#restart-and-uncertain-outcomes).

## Diagnose access problems

- **Desktop/service mismatch:** update both to the same product version and UI build; retry afterward.
- **Runtime unavailable:** repair the matching desktop package; replacing the container alone cannot repair host files.
- **Model check failed:** choose a suitable tool-calling model/runtime. A synthetic preflight pass still does not guarantee task planning.
- **Stale observation or changed window:** inspect again; do not bypass target or focus validation.
- **VPN fake DNS blocked:** Direct mode rejects reserved addresses. Explicit **Trusted VPN routing** supports its documented fake-DNS case while retaining selected-origin/TLS restrictions; it is not unrestricted proxy support. See [network limits](../../computer/README.md).

For internal protocol and native-worker development, use [computer/README.md](../../computer/README.md). Do not interpret developer pairing commands as the normal user setup flow.
