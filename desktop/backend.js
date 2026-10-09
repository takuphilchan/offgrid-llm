const http = require('node:http');
const { URL } = require('node:url');
const { fingerprintUI, BRIDGE_PROTOCOL } = require('./compatibility.cjs');
const legacy = require('./legacy-compatibility.json').entries;

const requiredCapabilities = ['sessions-v1', 'chat-streaming-v1', 'durable-agent-runs-v1'];

function assessIdentity(value, expectedVersion, expectedUIBuild, mode = 'external') {
  const fail = (code, reason) => ({ state: 'incompatible', code, reason });
  if (!value || value.product !== 'offgrid') return fail('service_not_offgrid', 'The occupied address did not identify an OffGrid workspace.');
  if (value.api_version !== 2 ||
      !Array.isArray(value.capabilities) || !requiredCapabilities.every(item => value.capabilities.includes(item))) {
    const missing = requiredCapabilities.filter(item => !Array.isArray(value.capabilities) || !value.capabilities.includes(item));
    return fail('service_contract_unsupported', `Required OffGrid API contract unavailable (API 2; missing: ${missing.join(', ') || 'supported API version'}).`);
  }
  if (typeof value.version !== 'string' || !value.version || value.version.length > 80 ||
      typeof value.ui_build_id !== 'string' || !/^[a-f0-9]{64}$/.test(value.ui_build_id)) return fail('service_metadata_invalid', 'The service has no valid version or renderer identity.');
  if (!['owned', 'external'].includes(mode)) return fail('bundle_inconsistent', 'Unknown desktop attachment context.');
  if (mode === 'owned' && (value.version !== expectedVersion || !expectedUIBuild || value.ui_build_id !== expectedUIBuild)) {
    return fail('bundle_inconsistent', 'The bundled service version or renderer differs from this desktop package. Repair the desktop application.');
  }
  const bridge = value.desktop_bridge;
  const missing = !Object.hasOwn(value, 'desktop_bridge') || (bridge?.status === 'missing' && Object.keys(bridge).length === 1);
  if (missing) {
    const match = mode === 'external' && legacy.find(item => item.versions.includes(value.version) && item.ui_build_id === value.ui_build_id && item.protocol === BRIDGE_PROTOCOL);
    if (!match) return fail(mode === 'owned' ? 'bundle_inconsistent' : 'service_legacy_unreviewed', 'This renderer has no reviewed desktop compatibility contract. Update the workspace service or open it in your browser.');
    return { state: 'ready', bridgeProtocol: match.protocol, compatibilityBasis: 'reviewed-legacy' };
  }
  if (bridge?.status !== 'ready' || bridge.schema_version !== 1 || !Number.isSafeInteger(bridge.protocol) || bridge.protocol < 1 || bridge.ui_build_id !== value.ui_build_id) {
    return fail(mode === 'owned' ? 'bundle_inconsistent' : 'service_metadata_invalid', 'The renderer compatibility declaration is invalid or refers to different UI files.');
  }
  if (bridge.protocol !== BRIDGE_PROTOCOL) return fail(mode === 'owned' ? 'bundle_inconsistent' : 'service_contract_unsupported', 'This renderer requires an unsupported desktop bridge protocol. Update the desktop and service to compatible builds.');
  return { state: 'ready', bridgeProtocol: bridge.protocol, compatibilityBasis: 'declared-contract' };
}

function inspectBackend(serverURL, expectedVersion, expectedUIBuild, timeoutMs = 2000, signal, mode = 'external') {
  return new Promise(resolve => {
    let settled = false;
    let timer;
    const finish = result => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      signal?.removeEventListener('abort', abort);
      resolve({ url: serverURL, code: result.state === 'unavailable' ? 'service_unavailable' : result.state === 'incompatible' ? 'service_metadata_invalid' : undefined, ...result });
    };
    const abort = () => {
      finish({ state: 'unavailable', reason: 'Connection check cancelled.' });
      request.destroy();
    };
    const request = http.get(`${serverURL}/api/v2/system`, response => {
      if (response.statusCode !== 200) {
        finish({ state: 'incompatible', reason: `The occupied service port returned HTTP ${response.statusCode} instead of an OffGrid compatibility handshake.` });
        response.destroy();
        return;
      }
      let size = 0;
      const chunks = [];
      response.on('data', chunk => {
        size += chunk.length;
        if (size > 16384) {
          finish({ state: 'incompatible', reason: 'Service identity response exceeds the size limit.' });
          response.destroy();
        } else chunks.push(chunk);
      });
      response.on('error', () => finish({ state: 'unavailable', reason: 'The compatibility handshake was interrupted.' }));
      response.on('end', () => {
        try {
          const value = JSON.parse(Buffer.concat(chunks).toString('utf8'));
          const assessment = assessIdentity(value, expectedVersion, expectedUIBuild, mode);
          if (assessment.state !== 'ready') finish({ ...assessment,
            version: typeof value?.version === 'string' ? value.version.slice(0, 80) : undefined,
            canOpenBrowser: value?.product === 'offgrid' && typeof value?.ui_build_id === 'string' && /^[a-f0-9]{64}$/.test(value.ui_build_id) });
          else finish({ ...assessment, version: value.version, revision: String(value.revision || 'unknown').slice(0, 128), uiBuildID: value.ui_build_id, apiVersion: value.api_version, workspaceID: typeof value.workspace_id === 'string' ? value.workspace_id.slice(0,128) : undefined });
        } catch {
          finish({ state: 'incompatible', reason: 'The service returned invalid identity metadata.' });
        }
      });
    });
    request.on('error', error => finish({ state: error.code === 'ECONNREFUSED' ? 'offline' : 'unavailable', reason: 'Cannot connect to the configured OffGrid service.' }));
    timer = setTimeout(() => {
      finish({ state: 'unavailable', reason: 'The compatibility handshake timed out. The occupied port will not be replaced.' });
      request.destroy();
    }, timeoutMs);
    signal?.addEventListener('abort', abort, { once: true });
    if (signal?.aborted) abort();
  });
}

function isTrustedPage(value, serverURL, loadingURL) {
  try {
    const parsed = new URL(value);
    if (parsed.username || parsed.password) return false;
    if (parsed.protocol === 'file:') { parsed.hash = ''; return parsed.href === loadingURL; }
    return parsed.origin === new URL(serverURL).origin && parsed.pathname.startsWith('/ui/');
  } catch { return false; }
}

function isTrustedSender(event, contents, serverURL, loadingURL) {
  return Boolean(contents && event.sender === contents && event.senderFrame === contents.mainFrame &&
    isTrustedPage(event.senderFrame.url, serverURL, loadingURL));
}

module.exports = { assessIdentity, inspectBackend, isTrustedPage, isTrustedSender, fingerprintUI };
