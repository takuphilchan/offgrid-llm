// Closed, versioned negotiation. This is not a permission or a claim that the
// selected application supports editing. Never forward unknown feature keys.
function nativeFeatures(value = {}) {
  if (!value || typeof value !== 'object' || Array.isArray(value) || Object.keys(value).some(key => key !== 'native_documents') ||
      (Object.hasOwn(value,'native_documents') && value.native_documents !== 1)) throw Error('computer_protocol_invalid');
  return Object.hasOwn(value,'native_documents') ? {native_documents:1} : {};
}
function requestedNativeFeatures(identity, worker) {
  const supported = nativeFeatures(worker.features);
  return identity.capabilities?.includes('native-document-negotiation-v1') ? supported : {};
}
function verifyNativeFeatures(proposed, accepted) {
  const expected=nativeFeatures(proposed),actual=nativeFeatures(accepted);
  if(expected.native_documents !== actual.native_documents)throw Error('computer_protocol_invalid');
  return actual;
}
module.exports={nativeFeatures,requestedNativeFeatures,verifyNativeFeatures};
