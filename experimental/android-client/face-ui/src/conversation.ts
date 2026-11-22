import type {startRealtime} from './lib/voice-guidance/realtimeClient';
import {localIntent} from './localIntent';
export function createConversation(native:(op:string,args?:Record<string,unknown>)=>Promise<any>,cloudAllowed:()=>boolean){
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
      if(!result&&cloudAllowed())result=await native('interpret',{text,values,options,history:conversation.slice(-6)});
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

return connect;
}
