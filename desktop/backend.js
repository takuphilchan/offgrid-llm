const http = require('node:http');
const { URL } = require('node:url');
const { createHash } = require('node:crypto');

function fingerprintUI(index) {
  return createHash('sha256').update(index.toString('utf8').replace(/\r\n/g, '\n')).digest('hex');
}

const requiredCapabilities = ['sessions-v1', 'chat-streaming-v1', 'durable-agent-runs-v1'];

function assessIdentity(value, expectedVersion, expectedUIBuild) {
  if (!value || value.product !== 'offgrid' || value.api_version !== 2 ||
      !Array.isArray(value.capabilities) || !requiredCapabilities.every(item => value.capabilities.includes(item))) {
    return 'This service does not implement the required OffGrid client contract.';
  }
  if (value.version !== expectedVersion) return `Desktop ${expectedVersion} requires the matching OffGrid service. Update the service or use its web UI.`;
  if (typeof value.ui_build_id !== 'string' || !/^[a-f0-9]{64}$/.test(value.ui_build_id)) return 'The service has no identifiable web UI build installed.';
  if (expectedUIBuild && value.ui_build_id !== expectedUIBuild) return 'The running service contains a different UI build. Rebuild or update it before connecting this desktop app.';
  return null;
}

function inspectBackend(serverURL, expectedVersion, expectedUIBuild, timeoutMs = 2000) {
  return new Promise(resolve => {
    let settled = false;
    let timer;
    const finish = result => {
      if (settled) return;
      settled = true;
      clearTimeout(timer);
      resolve({ url: serverURL, ...result });
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
          const reason = assessIdentity(value, expectedVersion, expectedUIBuild);
          if (reason) finish({ state: 'incompatible', reason });
          else finish({ state: 'ready', version: value.version, revision: String(value.revision || 'unknown'), uiBuildID: value.ui_build_id, apiVersion: value.api_version });
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
