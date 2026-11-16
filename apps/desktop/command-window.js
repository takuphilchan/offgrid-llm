'use strict';

// A presentation of the existing trusted window, never a second renderer or
// executor. Only a bounded shortcut choice crosses IPC; paths/commands do not.
const shortcuts = ['CommandOrControl+Shift+Space', 'Alt+Space', 'CommandOrControl+Alt+Space'];
async function saveShortcut(value, persist, configure) {
  if (value !== '' && !shortcuts.includes(value)) throw Error('invalid_command_shortcut');
  // A failed disk write must leave the active OS binding unchanged. A collision
  // after saving remains an explicit, retryable registration state.
  await persist(value);
  return configure(value);
}
function createCommandWindow({ window, screen, globalShortcut, ready, changed, hideTerminal = () => {}, motion = require('./command-window-motion').createCommandWindowMotion() }) {
  let mode = 'full', previous = null, shortcut = '', registered = false, layout = 'input', lastExpandedBounds = null, owner = null, destinationMode = null;
  const present = (target, destination, update) => {
    destinationMode = destination;
    return motion.run(target, update);
  };
  const sameBounds = (a, b) => a && b && ['x','y','width','height'].every(key => a[key] === b[key]);
  const validWindow = () => {
    const value = window();
    if (!value || value.isDestroyed()) return null;
    if (owner && owner !== value) { motion.cancel(); mode = 'full'; previous = null; lastExpandedBounds = null; layout = 'input'; }
    owner = value;
    return value;
  };
  const snapshot = () => { validWindow(); return { mode, shortcut, registered }; };
  const publish = () => { const value = snapshot(); changed(value); return value; };
  const size = area => ({width:Math.min(layout === 'voice' ? 420 : 680,area.width),height:Math.min(layout === 'voice' ? 540 : layout === 'work' ? 480 : 164,area.height)});
  const setLayout = (value, options = {}) => {
    if (!['input','work','voice'].includes(value)) throw Error('invalid_command_layout');
    if (value === layout) return motion.active ? motion.settled.then(snapshot) : snapshot();
    layout = value;
    const target = validWindow();
    if (target && mode === 'compact') {
      const bounds = target.getNormalBounds(), area = screen.getDisplayMatching(bounds).workArea, next = size(area);
      return motion.run(target, resize => resize({...next,x:Math.round(Math.max(area.x,Math.min(bounds.x+(bounds.width-next.width)/2,area.x+area.width-next.width))),y:Math.max(area.y,Math.min(bounds.y,area.y+area.height-next.height))}), options).then(snapshot);
    }
    return snapshot();
  };
  const dismiss = () => {
    motion.cancel();
    const target = validWindow();
    if (target && mode === 'compact') { target.webContents.send('command-window-dismiss'); target.hide(); }
    return snapshot();
  };
  const expand = () => {
    const target = validWindow();
    if (!target) return snapshot();
    if (mode === 'compact') {
      return present(target, 'full', (resize, after) => {
      mode = 'full'; target.setAlwaysOnTop(false);
      target.setResizable(true); target.setMaximizable(true); target.setFullScreenable(true);
      target.setMaximumSize(0, 0);
      const area = screen.getDisplayMatching(previous?.bounds ?? target.getNormalBounds()).workArea;
      if (previous) {
        const display = screen.getDisplayMatching(previous.bounds).workArea;
        const width = Math.min(previous.bounds.width, display.width), height = Math.min(previous.bounds.height, display.height);
        resize({ width, height, x: Math.max(display.x, Math.min(previous.bounds.x, display.x + display.width - width)), y: Math.max(display.y, Math.min(previous.bounds.y, display.y + display.height - height)) });
      }
      after(() => {
        target.setMinimumSize(Math.min(760, area.width), Math.min(560, area.height));
        lastExpandedBounds = target.getNormalBounds();
        if (previous?.maximized) target.maximize();
        if (previous?.fullScreen) target.setFullScreen(true);
        if (target.isMinimized()) target.restore();
        target.show(); target.focus();
      });
      publish();
      }).then(snapshot);
    }
    // Repeated activation must not freeze an in-progress expansion halfway.
    if (motion.active && destinationMode === 'full') return motion.settled.then(snapshot);
    // Expanding while a summon is still preparing must cancel that summon.
    motion.cancel();
    if (target.isMinimized()) target.restore();
    target.show(); target.focus();
    return snapshot();
  };
  const open = () => {
    const target = validWindow();
    if (!target || !ready()) return expand();
    hideTerminal();
    if (mode !== 'compact') {
      const bounds = target.getNormalBounds();
      // Windows can round setBounds by a DIP on scaled displays. Keep the
      // requested full geometry until the user actually moves/resizes it; using
      // the rounded result each time slowly grows the window on every toggle.
      if (!motion.active || !previous) previous = { bounds: sameBounds(bounds, lastExpandedBounds) && previous ? previous.bounds : bounds, maximized: target.isMaximized(), fullScreen: target.isFullScreen() };
      return present(target, 'compact', (resize, after) => {
      mode = 'compact';
      if (target.isFullScreen()) target.setFullScreen(false);
      if (target.isMaximized()) target.unmaximize();
      target.setMaximizable(false); target.setFullScreenable(false);
      const area = screen.getDisplayNearestPoint(screen.getCursorScreenPoint()).workArea;
      target.setMinimumSize(Math.min(320,area.width), Math.min(144,area.height));
      const {width,height} = size(area);
      resize({ x: Math.round(area.x + (area.width - width) / 2), y: Math.round(area.y + Math.max(0, (area.height - height) * .3)), width, height });
      target.setResizable(false);
      target.setAlwaysOnTop(true);
      publish();
      after(() => {if (target.isMinimized()) target.restore(); target.show(); target.focus();});
      }).then(snapshot);
    }
    return present(target, 'compact', (resize, after) => {
      // Dismissal can interrupt a resize. Reopening always fits the final
      // requested layout instead of exposing an intermediate native rectangle.
      const bounds=target.getNormalBounds(),area=screen.getDisplayMatching(bounds).workArea,next=size(area);
      resize({...next,x:Math.round(Math.max(area.x,Math.min(bounds.x+(bounds.width-next.width)/2,area.x+area.width-next.width))),y:Math.max(area.y,Math.min(bounds.y,area.y+area.height-next.height))});
      publish();
      after(()=>{if(target.isMinimized())target.restore();target.show();target.focus();});
    }).then(snapshot);
  };
  const configure = value => {
    if (value !== '' && !shortcuts.includes(value)) throw Error('invalid_command_shortcut');
    if (value === shortcut && (registered || !value)) return snapshot();
    if (registered) globalShortcut.unregister(shortcut);
    shortcut = value; registered = false;
    if (value) {
      try { registered = globalShortcut.register(value, () => {
        const target = validWindow();
        if (mode === 'compact' && target?.isVisible() && target.isFocused()) dismiss(); else open();
      }); } catch { registered = false; }
    }
    return publish();
  };
  const dispose = () => { motion.cancel(); if (registered) globalShortcut.unregister(shortcut); registered = false; };
  return { open, expand, dismiss, configure, setLayout, snapshot, dispose, fullState: () => mode === 'compact' || motion.active ? previous : null };
}
module.exports = { createCommandWindow, shortcuts, saveShortcut };
