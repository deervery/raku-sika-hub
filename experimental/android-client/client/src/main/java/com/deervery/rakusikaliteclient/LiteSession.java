package com.deervery.rakusikaliteclient;
import org.json.JSONObject;
import java.net.*;
import java.nio.charset.StandardCharsets;
import android.util.Base64;

/** Ephemeral ID token is checked by the existing read-only Lite API, never by a UI cookie alone. */
final class LiteSession {
    final String scope;
    final long expiresAt;
    LiteSession(String scope,long expiresAt){this.scope=scope;this.expiresAt=expiresAt;}
    boolean valid(){return !scope.isEmpty()&&System.currentTimeMillis()<expiresAt;}
    static LiteSession verify(String origin,String token,String uid,String facility){
        HttpURLConnection c=null;
        try{
            if(!origin.equals("https://rakusika.com")&&!origin.equals("https://www.rakusika.com")&&!origin.equals("https://raku-sika-lite.web.app")&&!origin.equals("https://raku-sika-lite.firebaseapp.com"))return null;
            if(!token.matches("[A-Za-z0-9_-]+\\.[A-Za-z0-9_-]+\\.[A-Za-z0-9_-]+")||token.length()>12000||uid.isEmpty()||uid.length()>128||(!facility.isEmpty()&&!facility.matches("[A-Za-z0-9_-]{1,128}")))return null;
            c=(HttpURLConnection)new URL(origin+"/api/users/company").openConnection();c.setRequestMethod("GET");c.setConnectTimeout(4000);c.setReadTimeout(4000);c.setInstanceFollowRedirects(false);c.setRequestProperty("Authorization","Bearer "+token);
            if(!facility.isEmpty()&&facility.matches("[A-Za-z0-9_-]{1,128}"))c.setRequestProperty("Cookie","active_facility_id="+facility);
            if(c.getResponseCode()!=200)return null;
            // Claims are used only after the production API has verified the signed token.
            JSONObject claims=new JSONObject(new String(Base64.decode(token.split("\\.")[1],Base64.URL_SAFE|Base64.NO_WRAP|Base64.NO_PADDING),StandardCharsets.UTF_8));
            if(!uid.equals(claims.optString("sub")))return null;
            long expiry=Math.min(claims.getLong("exp")*1000,System.currentTimeMillis()+65000);
            if(expiry<=System.currentTimeMillis())return null;
            return new LiteSession(uid+":"+facility,expiry);
        }catch(Exception ignored){return null;}
        finally{if(c!=null)c.disconnect();}
    }
    static String fingerprint(String token){try{return Base64.encodeToString(java.security.MessageDigest.getInstance("SHA-256").digest(token.getBytes(StandardCharsets.UTF_8)),Base64.NO_WRAP);}catch(Exception e){return "";}}
    static String snapshotScript(int ticket){return snapshotScript(ticket,false);}
    static String snapshotScript(int ticket,boolean confirm){return "(async()=>{let result={ticket:"+ticket+",confirm:"+confirm+"};try{"+
        "if(!/(?:^|;\\s*)__session_exists=true(?:;|$)/.test(document.cookie))throw 0;"+
        "if(indexedDB.databases&&!(await indexedDB.databases()).some(x=>x.name==='firebaseLocalStorageDb'))throw 0;"+
        "const db=await new Promise((ok,no)=>{const r=indexedDB.open('firebaseLocalStorageDb');r.onsuccess=()=>ok(r.result);r.onerror=no;r.onupgradeneeded=()=>{r.transaction.abort();no(0)}});"+
        "let rows;try{rows=await new Promise((ok,no)=>{const r=db.transaction('firebaseLocalStorage','readonly').objectStore('firebaseLocalStorage').getAll();r.onsuccess=()=>ok(r.result);r.onerror=no})}finally{db.close()}"+
        "const users=rows.filter(x=>String(x.fbase_key).startsWith('firebase:authUser:')&&x.value?.uid&&x.value?.stsTokenManager?.accessToken);if(users.length!==1)throw 0;"+
        "result.token=users[0].value.stsTokenManager.accessToken;result.uid=users[0].value.uid;"+
        "result.facility=decodeURIComponent((document.cookie.match(/(?:^|;\\s*)active_facility_id=([^;]*)/)||[])[1]||'');"+
        "}catch{}StationSession.postMessage(JSON.stringify(result));})()";}
}
