import {useState} from 'react';
import {QUANTITY_UNITS,parseQuantity,validQuantity,validDate} from '../../lib/voice-guidance/labelEditor';
import css from './VoiceWorkspace.module.css';

export function LabelValueEditor({kind,label,value,onSave,onClose}:{kind:'date'|'quantity';label:string;value:string;onSave:(value:string)=>void;onClose:()=>void}){
  const parsed=parseQuantity(value);
  const [amount,setAmount]=useState(parsed?.amount||'');const [unit,setUnit]=useState(parsed?.unit||'');
  const [parts,setParts]=useState(validDate(value)?value.split('-'):['','','']);
  const [error,setError]=useState('');
  const year=new Date().getFullYear();
  const years=Array.from({length:Math.max(year+6,Number(parts[0])||year)-Math.min(year-6,Number(parts[0])||year)+1},(_,i)=>Math.min(year-6,Number(parts[0])||year)+i);
  const days=parts[0]&&parts[1]?new Date(Number(parts[0]),Number(parts[1]),0).getDate():31;
  const updatePart=(index:number,next:string)=>setParts(previous=>{const changed=[...previous];changed[index]=next;if(changed[0]&&changed[1]&&Number(changed[2])>new Date(Number(changed[0]),Number(changed[1]),0).getDate())changed[2]='';return changed;});
  const result=kind==='quantity'?amount+unit:parts.join('-');
  const valid=kind==='quantity'?validQuantity(result):validDate(result);
  return <div className={css.editor} role="dialog" aria-modal="true" aria-label={`${label}を編集`}>
    <header><h2>{label}</h2><button type="button" onClick={onClose} aria-label="編集を閉じる">×</button></header>
    {kind==='date'?<>
      <div className={css.dateSelects}>{['年','月','日'].map((title,index)=><label key={title}>{title}<select aria-label={title} value={parts[index]} onChange={e=>updatePart(index,e.target.value)}><option value="">選択</option>{(index===0?years:Array.from({length:index===1?12:days},(_,i)=>i+1)).map(n=><option key={n} value={index===0?String(n):String(n).padStart(2,'0')}>{n}{title}</option>)}</select></label>)}</div>
      <div className={css.choices}><button type="button" onClick={()=>{const now=new Date();setParts([String(now.getFullYear()),String(now.getMonth()+1).padStart(2,'0'),String(now.getDate()).padStart(2,'0')]);}}>今日</button><button type="button" onClick={()=>setParts(['','',''])}>クリア</button></div>
    </>:<>
      <div className={css.quantityEntry}><output aria-label="内容量の数値" className={css.number}>{amount||'—'}</output><label>単位<select aria-label="内容量の単位" value={unit} onChange={e=>setUnit(e.target.value)}><option value="">選択</option>{QUANTITY_UNITS.map(unit=><option key={unit}>{unit}</option>)}</select></label></div>
      <div className={css.keypad}>{['1','2','3','4','5','6','7','8','9','.','0','⌫'].map(key=><button type="button" key={key} onClick={()=>setAmount(previous=>key==='⌫'?previous.slice(0,-1):key==='.'&&previous.includes('.')?previous:(previous+key).slice(0,9))}>{key}</button>)}</div>
    </>}
    <button type="button" className={css.primary} disabled={!valid} onClick={()=>{try{onSave(result);}catch(e){setError(e instanceof Error?e.message:'入力を確認してください。');}}}>この内容にする</button>
    {error&&<p role="alert">{error}</p>}
  </div>;
}
