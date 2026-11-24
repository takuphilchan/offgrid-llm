// Presentation validation only. The desktop and companion independently
// validate/pin every destination before granting or opening network access.
export function additionalBrowserOrigins(text:string, initialURL:string):string[] {
  if(text.length>32768)throw Error('computer_browser_scope_invalid');
  const entries=text.split(/\r?\n/).map(value=>value.trim()).filter(Boolean);
  if(entries.length>15)throw Error('computer_browser_scope_invalid');
  if(!entries.length)return [];
  try {
    const seen=new Set([new URL(initialURL).origin]);
    return entries.map(value=>{
      const url=new URL(value),host=url.hostname;
      if(value.length>2048||new TextEncoder().encode(url.origin).byteLength>512||/[\u0000-\u0020\u007f]/.test(value)||url.protocol!=='https:'||url.username||url.password||url.pathname!=='/'||url.search||url.hash||
        !/^[a-z0-9](?:[a-z0-9-]*[a-z0-9])?(?:\.[a-z0-9](?:[a-z0-9-]*[a-z0-9])?)+$/.test(host)||
        /^\d+(?:\.\d+){3}$/.test(host)||/\.(localhost|local|internal|test|invalid)$/.test(host)||seen.has(url.origin))throw Error();
      seen.add(url.origin);
      return url.origin;
    });
  } catch {throw Error('computer_browser_scope_invalid');}
}
