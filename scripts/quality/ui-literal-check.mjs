export function validateUiLiterals(literals,allowlist){
 const key=row=>JSON.stringify([row.file,row.text]),allowed=new Map(),errors=[];
 for(const row of allowlist){if(!row.reason?.trim()||allowed.has(key(row)))errors.push('Invalid or duplicate UI literal allowance: '+key(row));allowed.set(key(row),row);}
 const actual=new Set(literals.map(key));
 for(const row of literals)if(!allowed.has(key(row)))errors.push(`${row.file}:${row.line}: uncatalogued UI text: ${row.text}`);
 for(const [id] of allowed)if(!actual.has(id))errors.push('Stale UI literal allowance: '+id);
 return errors;
}
