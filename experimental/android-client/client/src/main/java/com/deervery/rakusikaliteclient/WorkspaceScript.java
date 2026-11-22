package com.deervery.rakusikaliteclient;
import org.json.JSONObject;

/** Do not deliver a late scale response into a pane that has been left or resumed. */
final class WorkspaceScript {
    static String guard(String hub){return "(()=>{let epoch=0,paused=false;"+
        "window.addEventListener('station-pause',()=>{epoch++;paused=true});"+
        "window.addEventListener('station-resume',()=>{paused=false});"+
        "const original=window.fetch.bind(window),base="+JSONObject.quote(hub)+";"+
        "window.fetch=async(...args)=>{const url=new URL(typeof args[0]==='string'?args[0]:args[0].url||args[0],location.href).href;"+
        "const scale=url.startsWith(base+'/scale/'),token=epoch;"+
        "if(scale&&paused)throw new DOMException('画面を切り替えました','AbortError');"+
        "const response=await original(...args);if(scale){await response.clone().text();if(paused||token!==epoch)throw new DOMException('古い計量結果を破棄しました','AbortError');}return response;};})()";}
}
