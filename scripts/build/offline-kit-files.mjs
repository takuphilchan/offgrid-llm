import fs from 'node:fs/promises';
import { createReadStream, constants } from 'node:fs';
import { createHash } from 'node:crypto';
import path from 'node:path';

export function safePath(value) {
  if (typeof value !== 'string' || !value || /[\\:*?"<>|\x00-\x1f\x7f]/.test(value)) throw Error('Unsafe kit path');
  if (value.split('/').some(p => !p || p === '.' || p === '..' || /[. ]$/.test(p) || /^(con|prn|aux|nul|com[1-9]|lpt[1-9])(?:\.|$)/i.test(p))) throw Error('Unsafe kit path');
  return value;
}

export async function digest(file) {
  const hash = createHash('sha256');
  for await (const chunk of createReadStream(file)) hash.update(chunk);
  return hash.digest('hex');
}

export async function inventory(root, prefix = '') {
  const stat = await fs.lstat(path.join(root, prefix));
  if (!stat.isDirectory() || stat.isSymbolicLink()) throw Error('Kit directories must not be links');
  const files = [], names = new Set();
  for (const entry of await fs.readdir(path.join(root, prefix), { withFileTypes: true })) {
    const name = safePath(prefix ? `${prefix}/${entry.name}` : entry.name);
    if (names.has(entry.name.toLowerCase())) throw Error('Case-colliding kit paths');
    names.add(entry.name.toLowerCase());
    if (entry.isDirectory()) files.push(...await inventory(root, name));
    else if (entry.isFile() && !entry.isSymbolicLink()) {
      const file = path.join(root, name), stat = await fs.lstat(file);
      if (!stat.isFile() || stat.isSymbolicLink()) throw Error('Kit file changed during inventory');
      files.push({ path: name, size: stat.size, sha256: await digest(file) });
    } else throw Error(`Unsupported linked or special kit file: ${name}`);
  }
  return files.sort((a, b) => a.path < b.path ? -1 : a.path > b.path ? 1 : 0);
}

export async function readJSON(root, name) {
  const file = path.join(root, safePath(name)), stat = await fs.lstat(file);
  if (!stat.isFile() || stat.isSymbolicLink() || stat.size > 4 * 1024 * 1024) throw Error(`Invalid manifest: ${name}`);
  return JSON.parse(await fs.readFile(file, 'utf8'));
}

export function checkRecord(files, prefix, record) {
  const name = prefix + safePath(record.path);
  const found = files.find(f => f.path === name);
  if (!found || !Number.isSafeInteger(record.size) || record.size < 1 || record.size !== found.size || !/^[a-f0-9]{64}$/.test(record.sha256) || found.sha256 !== record.sha256) throw Error(`Artifact verification failed: ${name}`);
}

export async function copyInventory(source, target, files) {
  for (const file of files) {
    const src = path.join(source, file.path), dst = path.join(target, file.path);
    const stat = await fs.lstat(src);
    if (!stat.isFile() || stat.isSymbolicLink()) throw Error('Source changed before copying');
    await fs.mkdir(path.dirname(dst), { recursive: true });
    await fs.copyFile(src, dst, constants.COPYFILE_EXCL);
    if ((await fs.stat(dst)).size !== file.size || await digest(dst) !== file.sha256) throw Error(`Copy verification failed: ${file.path}`);
  }
}
