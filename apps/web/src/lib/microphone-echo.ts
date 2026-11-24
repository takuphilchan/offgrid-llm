export type MicrophoneEchoMode = boolean | 'all' | 'remote-only';
export type MicrophoneEchoConfiguration = {automatic: boolean; echoCancellation?: MicrophoneEchoMode; noiseSuppression?: boolean};

// This describes applied configuration, not measured acoustic cancellation.
// Noise suppression is independent of AEC. In particular it need not be enabled
// for a headset/driver that already suppresses background noise.
export async function configureMicrophoneEcho(track: MediaStreamTrack): Promise<MicrophoneEchoConfiguration> {
  let modes: unknown;
  try { modes = track.getCapabilities?.().echoCancellation; } catch { /* Older device implementations expose settings only. */ }
  if (Array.isArray(modes) && modes.includes('all') && track.readyState === 'live') {
    try {
      // Preserve the selected device and other requested processing constraints.
      // Older DOM typings omit the standardized string AEC modes.
      await track.applyConstraints({...track.getConstraints(), echoCancellation: {exact: 'all'}} as unknown as MediaTrackConstraints);
    } catch {
      // A rejected optional upgrade must not discard working browser-selected
      // AEC. Read current settings instead of assuming that either mode applied.
    }
  }
  const settings = track.getSettings();
  const applied: unknown = settings.echoCancellation;
  const echoCancellation = typeof applied === 'boolean' || applied === 'all' || applied === 'remote-only' ? applied : undefined;
  return {automatic: track.readyState === 'live' && (echoCancellation === true || echoCancellation === 'all'),
    echoCancellation, noiseSuppression: settings.noiseSuppression};
}
