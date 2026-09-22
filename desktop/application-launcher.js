'use strict';

const crypto = require('node:crypto');
const fsDefault = require('node:fs');
const path = require('node:path');
const childProcess = require('node:child_process');

const digest = value => crypto.createHash('sha256').update(value).digest('hex').slice(0, 32);
const titleFrom = file => path.basename(file).replace(/\.(lnk|app|desktop)$/i, '').replace(/[-_]+/g, ' ').trim() || file;

function walk(root, fs, depth = 0, out = []) {
  if (depth > 2 || out.length >= 200) return out;
  let entries;
  try { entries = fs.readdirSync(root, { withFileTypes: true }); } catch { return out; }
  for (const entry of entries) {
    if (entry.name.startsWith('.')) continue;
    const full = path.join(root, entry.name);
    if (entry.isDirectory()) walk(full, fs, depth + 1, out);
    else out.push(full);
    if (out.length >= 200) break;
  }
  return out;
}

function parseDesktopFile(file, fs) {
  let text;
  try { text = fs.readFileSync(file, 'utf8'); } catch { return null; }
  const section = text.match(/(?:^|\n)\[Desktop Entry\]([\s\S]*?)(?:\n\[|$)/)?.[1] ?? '';
  const value = key => section.match(new RegExp(`^${key}=([^\\r\\n]*)`, 'm'))?.[1]?.trim();
  if (value('Type') !== 'Application' || value('NoDisplay') === 'true' || value('Hidden') === 'true') return null;
  const name = value('Name');
  const exec = value('Exec');
  // The command is only used as an eligibility check. Launching uses the
  // desktop ID through gtk-launch, never this untrusted command string.
  if (!name || !exec) return null;
  return { name, exec };
}

function createApplicationLauncher({ platform = process.platform, env = process.env, fs = fsDefault, shell, spawn = childProcess.spawn, applicationRoots } = {}) {
  let catalog = new Map();
  const remember = (id, item) => { catalog.set(id, item); return { id, title: item.title, driver: item.driver }; };

  function discover() {
    catalog = new Map();
    const result = [];
    if (platform === 'win32') {
      const roots = applicationRoots?.windows ?? [env.ProgramData && path.join(env.ProgramData, 'Microsoft', 'Windows', 'Start Menu', 'Programs'), env.APPDATA && path.join(env.APPDATA, 'Microsoft', 'Windows', 'Start Menu', 'Programs')].filter(Boolean);
      for (const file of roots.flatMap(root => walk(root, fs)).filter(file => /\.lnk$/i.test(file))) {
        const item = { title: titleFrom(file), path: file, driver: 'windows-uia' };
        result.push(remember(digest(file), item));
      }
    } else if (platform === 'darwin') {
      const roots = applicationRoots?.macos ?? ['/Applications', '/System/Applications', env.HOME && path.join(env.HOME, 'Applications')].filter(Boolean);
      for (const root of roots) {
        let entries;
        try { entries = fs.readdirSync(root, { withFileTypes: true }); } catch { continue; }
        for (const entry of entries.filter(entry => entry.isDirectory() && /\.app$/i.test(entry.name))) {
          const file = path.join(root, entry.name);
          result.push(remember(digest(file), { title: titleFrom(file), path: file, driver: 'macos-ax' }));
        }
      }
    } else if (platform === 'linux') {
      const roots = applicationRoots?.linux ?? ['/usr/share/applications', env.HOME && path.join(env.HOME, '.local', 'share', 'applications')].filter(Boolean);
      for (const file of roots.flatMap(root => walk(root, fs)).filter(file => /\.desktop$/i.test(file))) {
        const parsed = parseDesktopFile(file, fs);
        if (!parsed) continue;
        result.push(remember(digest(file), { title: parsed.name, path: file, desktopId: path.basename(file), driver: 'linux-atspi' }));
      }
    }
    return result.sort((a, b) => a.title.localeCompare(b.title));
  }

  async function launch(id) {
    const item = catalog.get(id);
    if (!item) throw new Error('application_not_selected');
    if (item.driver === 'linux-atspi') {
      await new Promise((resolve, reject) => {
        let settled = false;
        const done = error => { if (settled) return; settled = true; error ? reject(error) : resolve(); };
        let child;
        try { child = spawn('gtk-launch', [item.desktopId], { detached: true, stdio: 'ignore' }); }
        catch (error) { done(error); return; }
        child.once('error', done); child.once('spawn', () => { child.unref?.(); done(); });
      });
      return { id, title: item.title, driver: item.driver };
    }
    if (!shell?.openPath) throw new Error('application_launcher_unavailable');
    const error = await shell.openPath(item.path);
    if (error) throw new Error('application_launch_failed');
    return { id, title: item.title, driver: item.driver };
  }

  return { discover, launch, has: id => catalog.has(id) };
}

module.exports = { createApplicationLauncher, parseDesktopFile, titleFrom };
