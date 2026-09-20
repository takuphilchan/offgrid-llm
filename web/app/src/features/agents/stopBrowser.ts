import {api} from '../../api/client';

// Revoke remotely even if local IPC fails, and stop locally even when offline.
// Neither failure may prevent the other independent safety path from running.
export async function stopBrowserAssistance() {
 const managed=!!window.electron?.stopComputerBrowser;
 const [local,service]=await Promise.allSettled([
  managed?window.electron!.stopComputerBrowser!():Promise.resolve(undefined),
  api.emergencyStop(),
 ]);
 const locallyStopped=managed && local.status==='fulfilled' && local.value?.state==='stopped';
 return {
  revoked:service.status==='fulfilled',
  error:managed && !locallyStopped?'stopUnconfirmed':service.status==='rejected'?(locallyStopped?'localStoppedDisconnected':'lost'):undefined,
 } as const;
}
