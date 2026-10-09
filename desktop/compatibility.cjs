'use strict';
// Protocol 1 is the existing bounded window.electron main/preload API.
// This is a compatibility identifier, not a signature or an access grant.
const { createHash } = require('node:crypto');
const fs = require('node:fs');
const path = require('node:path');
const BRIDGE_PROTOCOL = 1;
const MAX_MANIFEST_BYTES = 4096;
const fingerprintUI = index => createHash('sha256').update(index.toString('utf8').replace(/\r\n/g, '\n')).digest('hex');
function createManifest(index) {
  return { schema_version: 1, protocol: BRIDGE_PROTOCOL, ui_build_id: fingerprintUI(index) };
}
function parseManifest(data, index) {
  if (Buffer.byteLength(data) > MAX_MANIFEST_BYTES) throw Error('Desktop compatibility metadata is too large');
  const value = JSON.parse(data.toString('utf8'));
  if (!value || value.schema_version !== 1 || !Number.isSafeInteger(value.protocol) || value.protocol < 1 ||
      value.ui_build_id !== fingerprintUI(index)) throw Error('Invalid or mismatched desktop compatibility metadata');
  return value;
}
function verifyUI(root) {
  const manifest = path.join(root, 'desktop-compatibility.json');
  if (!fs.statSync(manifest).isFile() || fs.statSync(manifest).size > MAX_MANIFEST_BYTES) throw Error('Invalid desktop compatibility file');
  const value = parseManifest(fs.readFileSync(manifest), fs.readFileSync(path.join(root, 'index.html')));
  if (value.protocol !== BRIDGE_PROTOCOL) throw Error('Unsupported packaged desktop bridge protocol');
  return value;
}
module.exports = { BRIDGE_PROTOCOL, MAX_MANIFEST_BYTES, fingerprintUI, createManifest, parseManifest, verifyUI };
