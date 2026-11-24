// Session-only device choice. Device identities and permission are never saved.
let selected = '';
export const selectedMicrophone = () => selected;
export const selectMicrophone = (deviceID: string) => { selected = deviceID; };
export function microphoneConstraints(): MediaStreamConstraints {
  return {audio: {echoCancellation: {ideal: true}, noiseSuppression: {ideal: true},
    autoGainControl: {ideal: true}, channelCount: {ideal: 1},
    ...(selected ? {deviceId: {exact: selected}} : {})}};
}

// Called only after an explicitly requested capture has received permission.
export async function microphoneChoices(): Promise<MediaDeviceInfo[]> {
  if (!navigator.mediaDevices?.enumerateDevices) return [];
  return (await navigator.mediaDevices.enumerateDevices()).filter(device => device.kind === 'audioinput' && device.deviceId);
}
