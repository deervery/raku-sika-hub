import React,{useState} from 'react';
import {createRoot} from 'react-dom/client';
import {VoiceStation} from './components/voice/VoiceStation';
import {RECEIVING_FORM} from './lib/constants/form-definitions';
import type {startRealtime} from './lib/voice-guidance/realtimeClient';
import {native} from './bridge';
import {localIntent} from './localIntent';
import type {StationDevices} from './lib/voice-guidance/stationOperations';

const conversation:Array<{role:'user'|'assistant';text:string}>=[];
const connect:typeof startRealtime=(values,options,c)=>{
  let closed=false;
  const stop=()=>{if(closed)return;closed=true;void native('stop').catch(()=>{});c.state('off');};
  c.state('listening');
  void (async()=>{
    try{
      const {text}=await native('listen');if(closed)return;
      c.message('user','user',text,1);c.state('connecting');
      let result=localIntent(text);
      if(!result&&localStorage.getItem('gemini-consent')==='1')result=await native('interpret',{text,values,options,history:conversation.slice(-6)});
      if(closed)return;
      result??={question:'個体番号、重量、種類などを一つずつ教えてください。顔を長押しすると、画面でも入力できます。'};
      if(result.patch&&Object.keys(result.patch).length){const response:any=c.propose(result.patch);if(response?.question)result.question=response.question;}
      if(result.review)c.review({mode:result.review});
      if(result.operation)c.operation?.({operation:result.operation});
      if(result.navigate!==undefined)await native('work',{path:result.navigate});
      conversation.push({role:'user',text:text.slice(0,400)});
      if(result.question){conversation.push({role:'assistant',text:result.question});c.message('reply','assistant',result.question,2);c.state('speaking');await native('speak',{text:result.question});}
      if(conversation.length>6)conversation.splice(0,conversation.length-6);
      if(!closed)c.state('off');
    }catch(error){if(!closed){c.error(error instanceof Error?error.message:'内容を確認してください。');c.state('off');}}
  })();
  return {stop,inform:()=>{stop();}};
};
function App(){
  const [values,setValues]=useState<Record<string,unknown>>(()=>{try{return JSON.parse(localStorage.getItem('receiving-draft')||'{}');}catch{return {};}});
  const save=(patch:Record<string,unknown>)=>setValues(previous=>{const next={...previous,...patch};localStorage.setItem('receiving-draft',JSON.stringify(next));return next;});
  const unavailable=async()=>({state:'error' as const,message:'この操作は「業務」画面から行ってください。'});
  const devices:StationDevices={identity:'android-native',printer:'施設のプリンタ',labelUnavailable:'業務ラベルは「業務」画面で、ログインと施設設定を確認して印刷してください。',printLabel:unavailable,testPrint:unavailable,tare:unavailable,zero:unavailable,
    weigh:async()=>{const r=await native('weigh');if(r.status!=='ok'||!r.stable)return {state:'error',message:r.message||'重量が安定していません。'};const v=Number(r.value)/(r.unit==='g'?1000:1);if(!['kg','g'].includes(r.unit)||!Number.isFinite(v)||v<0)return {state:'error',message:'重量を確認してください。'};return {state:'done',message:'計量器から読み取りました。',weightKg:v,cards:[{label:'重量',value:v+' kg'}]};},
    scan:async()=>{const {value}=await native('scan');return {state:'done',message:'読み取りました。',scanValue:value,cards:[{label:'コード',value}]};},health:unavailable};
  return <VoiceStation form={RECEIVING_FORM} values={values} enteredKeys={Object.keys(values)} connect={connect} devices={devices} onApply={save} onManualPatch={save} onReview={()=>{}}/>;
}
createRoot(document.getElementById('root')!).render(<App/>);
