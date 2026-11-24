import {publicPage} from './network.mjs';

// Only local consent supplies these origins. Page content cannot add a site.
export function browserOrigins(initial, additional = [], testLoopback = false) {
  if (!Array.isArray(additional) || additional.length > 15) throw Error('browser_scope_invalid');
  const parse = value => {
    if (!testLoopback) return publicPage(value);
    const url = new URL(value);
    if (url.protocol !== 'http:' || url.hostname !== '127.0.0.1' || url.username || url.password) throw Error('browser_scope_invalid');
    return url;
  };
  const origins = [parse(initial).origin];
  for (const value of additional) {
    if (typeof value !== 'string' || value.length > 2048) throw Error('browser_scope_invalid');
    const url = parse(value);
    if (url.pathname !== '/' || url.search || url.hash) throw Error('browser_scope_invalid');
    if (!origins.includes(url.origin)) origins.push(url.origin);
  }
  if(origins.some(origin=>Buffer.byteLength(origin)>512))throw Error('browser_scope_invalid');
  return Object.freeze(origins);
}

export function allowsBrowserURL(origins, value) {
  try {
    const url = new URL(value);
    return !url.username && !url.password && origins.includes(url.origin);
  } catch { return false; }
}
