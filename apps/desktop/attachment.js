// A local ownership hint, never permission or proof of service identity.
const fs = require('node:fs/promises');
const path = require('node:path');
const { createHash } = require('node:crypto');

function attachmentStore(root) {
  const filename = url => path.join(root, 'desktop-backends', createHash('sha256').update(url).digest('hex') + '.json');
  return {
    async load(url) {
      let file;
      try { file = await fs.open(filename(url), 'r'); }
      catch (error) { if (error.code === 'ENOENT') return false; throw error; }
      try {
        const buffer = Buffer.alloc(4097);
        const { bytesRead } = await file.read(buffer, 0, buffer.length, 0);
        if (bytesRead > 4096) throw Error('Invalid desktop backend record');
        const value = JSON.parse(buffer.subarray(0, bytesRead).toString('utf8'));
        if (value.version !== 1 || value.url !== url || value.external !== true) throw Error('Invalid desktop backend record');
        return true;
      } finally { await file.close(); }
    },
    async save(url) {
      const target = filename(url);
      await fs.mkdir(path.dirname(target), { recursive: true });
      // Exclusive creation means existing/corrupt records are never overwritten.
      // Partial writes fail closed on subsequent startup, rather than authorizing
      // a bundled replacement. No credentials, prompts or access grants stored.
      try { await fs.writeFile(target, JSON.stringify({ version: 1, url, external: true }), { flag: 'wx', mode: 0o600 }); }
      catch (error) { if (error.code !== 'EEXIST') throw error; await this.load(url); }
    }
  };
}

module.exports = { attachmentStore };
