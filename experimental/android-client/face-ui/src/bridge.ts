type Pending={resolve:(value:any)=>void;reject:(error:Error)=>void;timer:ReturnType<typeof setTimeout>};
const pending=new Map<string,Pending>();let sequence=0;
declare global {interface Window {StationNative?:{postMessage:(message:string)=>void;onmessage?:(event:{data:string})=>void};stationReply:(id:string,value:any)=>void;}}
window.stationReply=(id,value)=>{const p=pending.get(id);if(!p)return;clearTimeout(p.timer);pending.delete(id);value?.error?p.reject(new Error(value.error)):p.resolve(value);};
if(window.StationNative)window.StationNative.onmessage=event=>{try{const r=JSON.parse(event.data);window.stationReply(r.id,r.value);}catch{}};
export function native(op:string,args:Record<string,unknown>={}) {
  return new Promise<any>((resolve,reject)=>{
    if(!window.StationNative){reject(new Error('Androidアプリから開いてください。'));return;}
    const id=String(++sequence),timer=setTimeout(()=>{pending.delete(id);reject(new Error('応答を確認できませんでした。自動再送はしません。'));},30000);
    pending.set(id,{resolve,reject,timer});window.StationNative.postMessage(JSON.stringify({id,op,args}));
  });
}
window.addEventListener('station-pause',()=>{for(const p of pending.values()){clearTimeout(p.timer);p.reject(new Error('画面を切り替えました。'));}pending.clear();});

export function subscribeVoice(receive:(event:import('./conversation').VoiceEvent)=>void){const listener=(event:Event)=>receive((event as CustomEvent).detail);window.addEventListener('station-voice',listener);return ()=>window.removeEventListener('station-voice',listener);}
