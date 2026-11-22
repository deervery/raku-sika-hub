import './theme.css';
import React,{useState,useEffect,useMemo,useRef} from 'react';
import {createRoot} from 'react-dom/client';
import {VoiceStation} from './components/voice/VoiceStation';
import {RECEIVING_FORM} from './lib/constants/form-definitions';
import {native,subscribeVoice} from './bridge';
import {createConversation} from './conversation';
import {draftValues,defaultFieldKeys} from './draftDefaults';
import type {StationDevices} from './lib/voice-guidance/stationOperations';

let cloudEnabled=false;
function App({scope}:{scope:string}){
  const storageKey='receiving-draft:v031:'+encodeURIComponent(scope);
  const [overrides,setOverrides]=useState<Record<string,unknown>>(()=>{try{const stored=JSON.parse(localStorage.getItem(storageKey)||'{}');return stored&&typeof stored==='object'&&!Array.isArray(stored)?stored:{};}catch{return {};}});
  const values=useMemo(()=>draftValues(RECEIVING_FORM,overrides),[overrides]);
  const defaults=useMemo(()=>defaultFieldKeys(RECEIVING_FORM,overrides),[overrides]);
  const connect=useMemo(()=>createConversation(native,()=>cloudEnabled,subscribeVoice),[scope]);
  const save=(patch:Record<string,unknown>)=>setOverrides(previous=>{const next={...previous,...patch};localStorage.setItem(storageKey,JSON.stringify(next));return next;});
  const unavailable=async()=>({state:'error' as const,message:'この操作は「業務」画面から行ってください。'});
  const devices:StationDevices={identity:'android-native:'+scope,draftStorageKey:'label-draft:v032:'+encodeURIComponent(scope),printer:'施設のプリンタ',labelUnavailable:'業務ラベルは「業務」画面で、ログインと施設設定を確認して印刷してください。',printLabel:unavailable,testPrint:unavailable,tare:unavailable,zero:unavailable,
    weigh:async()=>{const r=await native('weigh');if(r.status!=='ok'||!r.stable)return {state:'error',message:r.message||'重量が安定していません。'};const v=Number(r.value)/(r.unit==='g'?1000:1);if(!['kg','g'].includes(r.unit)||!Number.isFinite(v)||v<0)return {state:'error',message:'重量を確認してください。'};return {state:'done',message:'計量器から読み取りました。',weightKg:v,cards:[{label:'重量',value:v+' kg'}]};},
    scan:async()=>{const {value}=await native('scan');return {state:'done',message:'読み取りました。',scanValue:value,cards:[{label:'コード',value}]};},health:unavailable};
  return <VoiceStation form={RECEIVING_FORM} values={values} enteredKeys={Object.keys(values)} defaultKeys={defaults} connect={connect} devices={devices} onApply={save} onManualPatch={save} onReview={()=>{}}/>;
}
function SessionRoot(){
  const [session,setSession]=useState<{scope:string;geminiEnabled:boolean}|null>(null);
  const generation=useRef(0);
  useEffect(()=>{
    const refresh=()=>{const token=++generation.current;void native('session').then(value=>{if(token!==generation.current)return;cloudEnabled=!!value.geminiEnabled;setSession(typeof value.scope==='string'&&value.scope?value:null);}).catch(()=>{if(token===generation.current){cloudEnabled=false;setSession(null);}});};
    const clear=()=>{generation.current++;cloudEnabled=false;setSession(null);};
    const pause=()=>{generation.current++;};
    window.addEventListener('station-pause',pause);window.addEventListener('station-resume',refresh);window.addEventListener('station-session-refresh',refresh);window.addEventListener('station-session-cleared',clear);refresh();
    return ()=>{generation.current++;window.removeEventListener('station-pause',pause);window.removeEventListener('station-resume',refresh);window.removeEventListener('station-session-refresh',refresh);window.removeEventListener('station-session-cleared',clear);};
  },[]);
  return session?<App key={session.scope} scope={session.scope}/>:<div style={{padding:32,color:'var(--station-ink)',font:'700 20px system-ui'}}>業務画面でログインしてください。</div>;
}
createRoot(document.getElementById('root')!).render(<SessionRoot/>);
