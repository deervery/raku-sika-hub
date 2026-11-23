import {useState} from 'react';
import type {FormDefinition} from '../../types/forms';
import {healthEntries} from '../../receivingLayout';
import css from './VoiceWorkspace.module.css';
export function HealthEditor({form,values,defaults,onSave,onClose}:{form:FormDefinition;values:Record<string,unknown>;defaults:string[];onSave:(key:string,value:string)=>void;onClose:()=>void}){
 const [error,setError]=useState('');
 return <div className={css.editor} role="dialog" aria-modal="true" aria-label="異常項目を選ぶ"><header><h2>異常の確認</h2><button type="button" onClick={onClose} aria-label="編集を閉じる">×</button></header><p>異常がある項目を選んでください。初期値は確認済みの記録ではありません。</p>
 {healthEntries(form,values).map(([key,field])=><fieldset key={key} className={css.healthRow}><legend>{field.label}</legend><div className={css.choices}>{['異常なし','異常あり'].map(value=><button type="button" key={value} aria-pressed={values[key]===value} data-default={defaults.includes(key)} onClick={()=>{try{onSave(key,value);setError('');}catch(e){setError(e instanceof Error?e.message:'入力を確認してください');}}}>{value}{defaults.includes(key)&&values[key]===value&&<small>初期値</small>}</button>)}</div></fieldset>)}
 {error&&<p role="alert">{error}</p>}</div>;
}
