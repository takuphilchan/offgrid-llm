import {createContext,useContext,useState,type ReactNode} from 'react';

export type ReaderPosition={page:number;zoom:number;query:string;textView:boolean;compare:boolean;top:number;left:number};
const Context=createContext<Map<string,ReaderPosition>|null>(null);

// Owned by the account/workspace/permission boundary. Never persist document
// text or search queries to browser storage, and never reuse cached authority.
export function ReaderMemoryProvider({children}:{children:ReactNode}){
 const [positions]=useState(()=>new Map<string,ReaderPosition>());
 return <Context.Provider value={positions}>{children}</Context.Provider>;
}
export function useReaderMemory(){return useContext(Context);}
export function rememberReader(memory:Map<string,ReaderPosition>|null,key:string,position:ReaderPosition){
 if(!memory||!key)return;
 memory.delete(key);memory.set(key,{...position});
 while(memory.size>40)memory.delete(memory.keys().next().value!);
}
