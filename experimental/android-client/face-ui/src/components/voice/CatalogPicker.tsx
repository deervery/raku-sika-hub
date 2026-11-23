import {ClearField} from './ClearField';
import {useEffect,useRef,useState} from 'react';
import {native} from '../../bridge';
import {type CatalogItem,scanQuery,swipeDirection,productFamilyMatches} from '../../catalog';
import css from './VoiceWorkspace.module.css';
export function CatalogPicker({kind,initial='',onSelect,onClose,onNumber,family}:{kind:'individuals'|'products';family?:string;initial?:string;onSelect:(row:CatalogItem)=>void;onClose:()=>void;onNumber?:(value:string)=>void}){
 const [query,setQuery]=useState(initial),[cursor,setCursor]=useState(''),[next,setNext]=useState(''),[items,setItems]=useState<CatalogItem[]>([]),[busy,setBusy]=useState(false),[error,setError]=useState('');
 const [page,setPage]=useState(0);const swipe=useRef<{x:number;y:number}|null>(null);
 const ticket=useRef(0);const selectRef=useRef(onSelect);selectRef.current=onSelect;
 useEffect(()=>{const t=++ticket.current;setBusy(true);setItems([]);setPage(0);setNext('');setError('');const timer=setTimeout(()=>{native('catalog',{kind,query,cursor}).then(result=>{if(t!==ticket.current)return;setItems((result.items||[]).filter((row:CatalogItem)=>kind!=='products'||productFamilyMatches(row,family)));setNext(result.cursor||'');}).catch(e=>{if(t===ticket.current)setError(e.message);}).finally(()=>{if(t===ticket.current)setBusy(false);});},250);return()=>{clearTimeout(timer);ticket.current++;};},[kind,query,cursor,family]);
 async function choose(row:CatalogItem){const t=++ticket.current;setBusy(true);setError('');try{const item=kind==='individuals'?(await native('catalog',{kind:'individual',id:row.id})).item:row;if(t===ticket.current)selectRef.current(item);}catch(e){if(t===ticket.current)setError(e instanceof Error?e.message:'読み取れませんでした');}finally{if(t===ticket.current)setBusy(false);}}
 async function scan(op:'scan'|'cameraScan'){const t=++ticket.current;setBusy(true);setError('');try{await native('stop');const result=scanQuery((await native(op)).value);if(t!==ticket.current)return;if('query'in result){setQuery(result.query);setCursor('');onNumber?.(result.query);const found=await native('catalog',{kind:'individuals',query:result.query});if(t!==ticket.current)return;setItems(found.items||[]);setNext(found.cursor||'');}else {const resultRow=await native('catalog',{kind:result.trace?'trace':'individual',id:result.id});if(t===ticket.current)selectRef.current(resultRow.item);}}catch(e){if(t===ticket.current)setError(e instanceof Error?e.message:'読み取れませんでした');}finally{if(t===ticket.current)setBusy(false);}}
 const size=kind==='products'?12:4;
 const advance=(direction:number)=>{if(busy)return;if(direction<0)setPage(Math.max(0,page-1));else if(direction>0){if((page+1)*size<items.length)setPage(page+1);else if(next)setCursor(next);}};
 return <div className={css.editor} role="dialog" aria-modal="true" aria-label={kind==='individuals'?'個体を選ぶ':'製品を選ぶ'}>
 <header><h2>{kind==='individuals'?'個体番号':'製品名'}</h2><button type="button" aria-label="編集を閉じる" onClick={onClose}>×</button></header>
 <ClearField label={kind==='individuals'?'個体番号を検索':'製品名を検索'} empty={!query} onClear={()=>{setQuery('');setCursor('');onNumber?.('');}}><input autoFocus={kind==='individuals'} aria-label={kind==='individuals'?'個体番号を検索':'製品名を検索'} value={query} maxLength={kind==='individuals'?15:100} onChange={e=>{setQuery(e.target.value);setCursor('');onNumber?.(e.target.value);}}/></ClearField>
 {kind==='individuals'&&<div className={css.operationToolbar}><button type="button" disabled={busy} onClick={()=>void scan('cameraScan')}>カメラスキャン</button><button type="button" disabled={busy} onClick={()=>void scan('scan')}>スキャナで読み取る</button></div>}
 <div className={css.choices} onTouchStart={e=>{const t=e.touches[0];swipe.current={x:t.clientX,y:t.clientY};}} onTouchEnd={e=>{const start=swipe.current;swipe.current=null;if(start){const t=e.changedTouches[0];advance(swipeDirection(t.clientX-start.x,t.clientY-start.y));}}}>{items.slice(page*size,page*size+size).map(row=><button type="button" key={row.id} disabled={busy} onClick={()=>void choose(row)}><strong>{kind==='individuals'?row.individualNumber:row.name}</strong>{kind==='individuals'&&<small>{[row.species,row.sex,String(row.receivingDate||row.timeToFactory||'').slice(0,10)].filter(Boolean).join(' · ')}</small>}</button>)}</div>
 <p role="status">{busy?'読み取り中…':items.length?'候補を選ぶとカードに反映します。':'候補がありません。検索条件を確認してください。'}</p>
 <nav className={css.pagination} aria-label="候補のページ">{cursor&&<button type="button" onClick={()=>setCursor('')}>最初へ</button>}{page>0&&<button type="button" disabled={busy} onClick={()=>advance(-1)}>← 前の候補</button>}{((page+1)*size<items.length||next)&&<button type="button" disabled={busy} onClick={()=>advance(1)}>次の候補 →</button>}</nav>
 {error&&<p role="alert">{error}</p>}
 </div>;
}
