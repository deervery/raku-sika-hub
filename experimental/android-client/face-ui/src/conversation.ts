import type {startRealtime} from './lib/voice-guidance/realtimeClient';
import {interpretationInput} from './interpretationInput';
import {localIntent} from './localIntent';
import {labelIntent} from './lib/voice-guidance/labelEditor';
export type VoiceEvent={session:string;type:'ready'|'speech'|'recognizing'|'text'|'retry'|'error';turn:number;text:string};
type Subscribe=(receive:(event:VoiceEvent)=>void)=>()=>void;
let sequence=0;
/** Native capture remains open between turns and during TTS. Never resumes after stop/pause. */
export function createConversation(native:(op:string,args?:Record<string,unknown>)=>Promise<any>,cloudAllowed:()=>boolean,subscribe:Subscribe){
  const conversation:Array<{role:'user'|'assistant';text:string}>=[];
  const connect:typeof startRealtime=(values,options,c)=>{
    const session=Date.now().toString(36)+'-'+(++sequence);
    let closed=false,turn=0,order=0,validTurn=false,continuousMode=true;
    let currentValues={...values};
    const live=(id:number)=>!closed&&validTurn&&turn===id;
    let unsubscribe=()=>{};
    const stop=()=>{if(closed)return;closed=true;unsubscribe();void native('stop').catch(()=>{});c.state('off');};
    const fail=(message:string)=>{if(closed)return;stop();c.error(message);};
    const process=async(id:number,text:string)=>{
      if(!live(id))return;
      c.message('user-'+id,'user',text,++order);c.state('connecting');
      try{
        const route=localIntent(text);
        let result=c.context?.().operation==='labels'&&!route?.operation&&route?.navigate===undefined?labelIntent(text):route;
        if(!result&&cloudAllowed())result=await native('interpret',{...interpretationInput(text,currentValues,options,conversation),...(continuousMode?{session,turn:id}:{})});
        if(!live(id))return;
        result??={question:'項目と値を一つずつ教えてください。顔を長押しすると、画面でも入力できます。'};
        if(result.label){const response:any=c.label?.(result.label);if(response?.missing?.length)result.question=response.missing[0];}
        if(result.patch&&Object.keys(result.patch).length){const response:any=c.propose(result.patch);if(response?.question)result.question=response.question;Object.assign(currentValues,result.patch);}
        if(result.review)c.review({mode:result.review});
        if(result.operation)c.operation?.({operation:result.operation});
        if(result.navigate!==undefined){await native('work',{path:result.navigate});stop();return;}
        conversation.push({role:'user',text:text.slice(0,400)});
        if(result.question){
          conversation.push({role:'assistant',text:result.question});
          c.message('reply-'+id,'assistant',result.question,++order);c.state('speaking');
          await native('speak',{text:result.question,...(continuousMode?{session,turn:id}:{})});
        }
        if(conversation.length>6)conversation.splice(0,conversation.length-6);
        if(live(id))c.state('listening');
      }catch(error){if(live(id)){c.message('error-'+id,'assistant',error instanceof Error?error.message:'内容を確認してください。',++order);c.state('listening');}}
    };
    unsubscribe=subscribe(event=>{
      if(closed||event.session!==session)return;
      if(event.type==='error'){fail(event.text);return;}
      if(event.type==='ready'){if(event.text==='echo-unavailable')c.message('audio-route','assistant','この端末ではエコー抑制を利用できないため、返答を画面に表示します。マイクはオンのままです。',++order);if(turn===0)c.state('listening');return;}
      if(event.type==='speech'){
        if(event.turn<=turn)return;
        turn=event.turn;validTurn=true;c.state('listening'); // Native stops playback before sending this event.
      }else if(live(event.turn)){
        if(event.type==='recognizing')c.state('connecting');
        else if(event.type==='text')void process(event.turn,event.text);
        else if(event.type==='retry'){c.message('retry-'+turn,'assistant',event.text,++order);c.state('listening');}
      }
    });
    c.state('connecting');
    void native('voiceStart',{session}).then(async mode=>{
      if(closed||mode.continuous!==false)return;
      // Preserve the existing tap-to-talk path on Android 12. Do not claim full duplex there.
      continuousMode=false;c.message('compatibility','assistant','このAndroidでは自動割り込みを利用できません。今回は1回ずつ聞き取ります。',++order);
      c.state('listening');turn=1;validTurn=true;const result=await native('listen');if(closed)return;
      await process(turn,result.text);stop();
    }).catch(error=>fail(error instanceof Error?error.message:'マイクを開始できません。'));
    return {stop,inform:(patch)=>{if(closed)return;Object.assign(currentValues,patch);validTurn=false;void native('voiceDiscard',{session}).catch(()=>{});c.state('listening');}};
  };
  return connect;
}
