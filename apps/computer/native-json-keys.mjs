// Scan the bounded wire text before JSON.parse can erase conflicting members.
// Strings use JSON's own escape decoding so "name" and "n\u0061me" collide.
export function assertUniqueJSONKeys(text){
  let index=0,nodes=0;
  const invalid=()=>{throw Error('native_message_invalid');};
  const space=()=>{while(index<text.length&&/[\x20\t\r\n]/.test(text[index]))index++;};
  const string=()=>{
    if(text[index]!=='"')invalid();
    const start=index++;
    while(index<text.length){
      const char=text[index++];
      if(char==='"'){
        const value=JSON.parse(text.slice(start,index));
        if(!value.isWellFormed())invalid();
        return value;
      }
      if(char==='\\'){if(index>=text.length)invalid();index++;}
      else if(char.charCodeAt(0)<32)invalid();
    }
    invalid();
  };
  const value=depth=>{
    if(depth>32||++nodes>4096)invalid();
    space();
    if(text[index]==='"'){string();return;}
    if(text[index]==='{'){
      index++;space();const keys=new Set();
      if(text[index]==='}'){index++;return;}
      for(;;){
        const key=string();if(keys.has(key))invalid();keys.add(key);
        space();if(text[index++]!==':')invalid();value(depth+1);space();
        const next=text[index++];if(next==='}')return;if(next!==',')invalid();space();
      }
    }
    if(text[index]==='['){
      index++;space();if(text[index]===']'){index++;return;}
      for(;;){value(depth+1);space();const next=text[index++];if(next===']')return;if(next!==',')invalid();}
    }
    for(const word of ['true','false','null'])if(text.startsWith(word,index)){index+=word.length;return;}
    const number=/^-?(?:0|[1-9]\d*)(?:\.\d+)?(?:[eE][+-]?\d+)?/.exec(text.slice(index));
    if(!number)invalid();index+=number[0].length;
  };
  value(0);space();if(index!==text.length)invalid();
}
