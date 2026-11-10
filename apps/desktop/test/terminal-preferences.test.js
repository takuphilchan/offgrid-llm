const test=require('node:test'),assert=require('node:assert/strict');
const preferences=require('../terminal-preferences');
const luminance=hex=>{const components=hex.slice(1).match(/../g).map(n=>parseInt(n,16)/255).map(n=>n<=.04045?n/12.92:((n+.055)/1.055)**2.4);return components[0]*.2126+components[1]*.7152+components[2]*.0722;};
const contrast=(a,b)=>{const x=luminance(a),y=luminance(b);return (Math.max(x,y)+.05)/(Math.min(x,y)+.05);};
test('complete ANSI palettes remain legible on both terminal backgrounds',()=>{
 for(const dark of [false,true]){const theme=preferences.theme(dark);for(const key of [...preferences.colors,'foreground'])assert.ok(contrast(theme[key],theme.background)>=4.5,`${dark?'dark':'light'} ${key}`);assert.ok(contrast(theme.selectionForeground,theme.selectionBackground)>=4.5);}
});
test('settings are bounded data, never commands or arbitrary terminal options',()=>{
 const value=preferences.normalize({fontSize:999,lineHeight:NaN,cursorStyle:'<script>',cursorBlink:'true',scrollback:-100,command:'unsafe',allowProposedApi:true});
 assert.deepEqual(value,{fontSize:24,lineHeight:1.25,cursorStyle:'block',cursorBlink:true,scrollback:1000});
 assert.equal(preferences.options(value,true).cursorBlink,false);
 assert.equal(preferences.options(value,false).cursorBlink,true);
});
test('storage failures and unknown versions use defaults; valid preferences round trip',()=>{
 for(const payload of ['bad','null','{"version":2,"settings":{"fontSize":24}}'])assert.deepEqual(preferences.load({getItem:()=>payload}),preferences.defaults);
 const broken={getItem(){throw Error('denied');},setItem(){throw Error('full');}};
 assert.deepEqual(preferences.load(broken),preferences.defaults);assert.equal(preferences.save(broken,{}),false);
 let saved;const storage={getItem:()=>saved,setItem:(_,v)=>saved=v};
 assert.equal(preferences.save(storage,{fontSize:18,cursorBlink:false}),true);
 assert.equal(preferences.load(storage).fontSize,18);assert.equal(preferences.load(storage).cursorBlink,false);
});
test('forced colors override ANSI palette and selection with system colors',()=>{
 const theme=preferences.theme(true,{background:'#ffffff',foreground:'#000000'});
 for(const key of preferences.colors)assert.equal(theme[key],'#000000');
 assert.equal(theme.selectionForeground,'#ffffff');assert.equal(theme.selectionBackground,'#000000');
});
