import { extractCard } from './api.js';
export function parseRows(raw) {
  if(!Array.isArray(raw) || raw.length<2 || raw.length>101)throw new Error('表格需要表头和 1–100 行数据。');
  if(String(raw[0][0]||'').trim()!=='卡密' || String(raw[0][1]||'').trim().toLowerCase()!=='session')throw new Error('请使用模板，前两列表头必须是“卡密”和“Session”。');
  const seen=new Set();
  return raw.slice(1).map((row,index)=>{
    const line=index+2;
    if(!row || !String(row[0]||'').trim() || !String(row[1]||'').trim())throw new Error(`第 ${line} 行缺少卡密或 Session，请补齐该行。`);
    let code;try{code=extractCard(row[0]);}catch{throw new Error(`第 ${line} 行卡密格式不正确。`);}
    if(seen.has(code))throw new Error(`第 ${line} 行卡密重复。`);seen.add(code);
    const session=String(row[1]);if(new TextEncoder().encode(session).length>131072)throw new Error(`第 ${line} 行 Session 超过 128 KiB。`);
    return {row:line,code,session,state:'pending',attempt:null,error:''};
  });
}
