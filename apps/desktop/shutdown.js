// A failed preference write must not prevent input/runtime cleanup. Attempt
// every owned cleanup even if another fails; callers must not claim quit until
// all owned workers confirm stopping. External services are never stopped here.
async function shutdownDesktop({ save, computer, execution, runtime, saveTimeoutMs = 5000 }) {
  let timer;
  const saved = Promise.race([
    Promise.resolve().then(save).catch(() => undefined),
    new Promise(resolve => { timer = setTimeout(resolve, saveTimeoutMs); }),
  ]).finally(() => clearTimeout(timer));
  const input = await Promise.allSettled([
    Promise.resolve().then(() => computer.stop()).then(state => {
      if (state?.code === 'stop_unconfirmed') throw Error('computer_stop_unconfirmed');
    }),
    Promise.resolve().then(() => execution.stop()),
  ]);
  const service = await Promise.allSettled([Promise.resolve().then(() => runtime.stop())]);
  await saved;
  const failures = [...input, ...service].filter(result => result.status === 'rejected');
  if (failures.length) throw new AggregateError(failures.map(result => result.reason), 'desktop_stop_unconfirmed');
}

module.exports = { shutdownDesktop };
