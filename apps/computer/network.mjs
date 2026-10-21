import {lookup} from 'node:dns/promises';
import {createServer} from 'node:http';
import {connect, isIP} from 'node:net';

export function publicIPv4(address) {
  const p = address.split('.').map(Number);
  return isIP(address) === 4 && ![0,10,127].includes(p[0]) && p[0] < 224 &&
    !(p[0] === 169 && p[1] === 254) && !(p[0] === 172 && p[1] >= 16 && p[1] <= 31) &&
    !(p[0] === 192 && (p[1] === 168 || p[1] === 0 || p[1] === 2)) &&
    !(p[0] === 100 && p[1] >= 64 && p[1] <= 127) && !(p[0] === 198 && [18,19,51].includes(p[1])) &&
    !(p[0] === 203 && p[1] === 0);
}

export function publicPage(value) {
  if (typeof value !== 'string' || Buffer.byteLength(value) > 2048) throw Error('target_invalid');
  const url = new URL(value);
  if (url.protocol !== 'https:' || isIP(url.hostname) || url.hostname.startsWith('[') ||
      !url.hostname.includes('.') || /\.(localhost|local|internal|test|invalid)$/.test(url.hostname) ||
      url.username || url.password) throw Error('target_invalid');
  return url;
}

export async function pinnedDestination(value, mode = 'direct', resolve = lookup) {
  const url = publicPage(value);
  if (!['direct','trusted-vpn'].includes(mode)) throw Error('network_mode_invalid');
  let addresses;
  try { addresses = await resolve(url.hostname, {all:true, family:4}); }
  catch { throw Error('network_unavailable'); }
  const trustedFake = address => mode === 'trusted-vpn' && /^198\.(18|19)\./.test(address) && isIP(address) === 4;
  if (!addresses.length || addresses.some(({address}) => !publicIPv4(address) && !trustedFake(address))) throw Error('network_blocked');
  return {origin:url.origin, hostname:url.hostname, port:Number(url.port || 443), address:addresses[0].address};
}

// A loopback-only HTTPS CONNECT relay pins one locally consented destination.
// This also blocks redirect escapes: route callbacks alone do not intercept
// every redirected request. TLS remains end-to-end with Chromium verification.
// Trusted VPN mode permits only the disclosed 198.18/15 fake-DNS mapping; it
// does not grant arbitrary private IPs, origins, plaintext HTTP or proxy auth.
export async function startEgress(destination) {
  const sockets = new Set();
  let closed = false;
  const track = socket => {
    sockets.add(socket);
    socket.on('error', () => {});
    socket.once('close', () => sockets.delete(socket));
    socket.setTimeout(120000, () => socket.destroy());
    return socket;
  };
  const server = createServer((request, response) => {response.writeHead(403);response.end();});
  server.headersTimeout = 10000;
  server.on('connection', socket => {track(socket); if (closed || sockets.size > 128) socket.destroy();});
  server.on('connect', (request, client, head) => {
    if (closed || sockets.size > 128 || request.url !== `${destination.hostname}:${destination.port}` || head.length) {
      client.end('HTTP/1.1 403 Forbidden\r\nConnection: close\r\n\r\n'); return;
    }
    const upstream = track(connect({host:destination.address, port:destination.port}));
    const timer = setTimeout(() => upstream.destroy(), 10000);
    upstream.once('connect', () => {
      clearTimeout(timer);
      if (closed || client.destroyed) {upstream.destroy();return;}
      client.write('HTTP/1.1 200 Connection Established\r\n\r\n');
      client.pipe(upstream); upstream.pipe(client);
    });
    upstream.once('close', () => {clearTimeout(timer);client.destroy();});
    client.once('close', () => upstream.destroy());
  });
  await new Promise((resolve,reject) => {server.once('error',reject);server.listen(0,'127.0.0.1',resolve);});
  return {server:`http://127.0.0.1:${server.address().port}`, close: async () => {
    closed = true;
    for (const socket of sockets) socket.destroy();
    await new Promise(resolve => server.close(resolve));
  }};
}
