
const {test}=require('node:test');const assert=require('node:assert/strict');const catalog=require('../theme-palettes.json');
const luminance=hex=>{const rgb=hex.slice(1).match(/../g).map(v=>parseInt(v,16)/255).map(v=>v<=.04045?v/12.92:((v+.055)/1.055)**2.4);return rgb[0]*.2126+rgb[1]*.7152+rgb[2]*.0722;};
const contrast=(a,b)=>{a=luminance(a);b=luminance(b);return (Math.max(a,b)+.05)/(Math.min(a,b)+.05);};
for(const [name,modes] of Object.entries(catalog.themes))for(const [mode,tokens] of Object.entries(modes))test(name+' '+mode+' semantic contrast',()=>{
 for(const surface of ['surface-0','surface-1','surface-2'])for(const text of ['text','muted','faint'])assert.ok(contrast(tokens[surface],tokens[text])>=4.5,surface+' '+text);
 assert.ok(contrast(tokens.accent,tokens['on-accent'])>=4.5);
});
test('catalog stays data-only, bounded, and valid for desktop persistence',()=>{
 const {normalize}=require('../presentation');
 const {gzipSync}=require('node:zlib');
 assert.ok(gzipSync(JSON.stringify(catalog)).length<8192,'catalog gzip budget: 8 KiB');
 assert.equal(Object.keys(catalog.themes).length,12);
 for(const [id,modes] of Object.entries(catalog.themes))for(const [mode,tokens] of Object.entries(modes)){
  for(const value of Object.values(tokens))assert.match(value,/^#[0-9a-f]{6}$/i);
  assert.ok(catalog.labels[id][mode]);
  assert.equal(normalize({appearance:{version:1,[mode]:id}}).appearance[mode],id);
 }
 assert.equal(normalize({appearance:{version:1,light:'nord',dark:'nord'}}).appearance.light,'offgrid');
});
for(const [id,modes] of Object.entries(catalog.terminal))for(const [mode,ansi] of Object.entries(modes))test(id+' '+mode+' terminal and chrome integration',()=>{
 const tokens=catalog.themes[id][mode];
 const prefs=require('../terminal-preferences');
 const actual=prefs.theme(mode==='dark',null,{...tokens,ansi});
 assert.equal(Object.keys(ansi).length,16);
 for(const name of prefs.colors){assert.equal(actual[name],ansi[name]);assert.ok(contrast(actual[name],actual.background)>=4.5,name);}
 assert.ok(contrast(actual.selectionBackground,actual.selectionForeground)>=4.5);
 assert.equal(actual.cursorAccent,actual.background);
 const chrome=require('../branding.cjs').windowChrome('win32',mode,tokens);
 assert.equal(chrome.backgroundColor,tokens['surface-0']);assert.equal(chrome.titleBarOverlay.symbolColor,tokens.text);
 const forced=prefs.theme(mode==='dark',{background:'#ffffff',foreground:'#000000'},{...tokens,ansi});
 assert.equal(forced.background,'#ffffff');assert.equal(forced.blue,'#000000');
});
