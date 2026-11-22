package com.deervery.rakusikaliteclient;
import android.webkit.WebView;
import org.json.JSONObject;

final class ScannerDelivery {
    private ScannerDelivery(){}
    static void deliver(WebView web,String trustedUrl,String scan){
        if(web==null||!EndpointPolicy.sameOrigin(trustedUrl,web.getUrl()))return;
        web.evaluateJavascript("(()=>{for(const key of Array.from("+JSONObject.quote(scan)+"))window.dispatchEvent(new KeyboardEvent('keydown',{key,bubbles:true}));window.dispatchEvent(new KeyboardEvent('keydown',{key:'Enter',bubbles:true}));})()",null);
    }
}
