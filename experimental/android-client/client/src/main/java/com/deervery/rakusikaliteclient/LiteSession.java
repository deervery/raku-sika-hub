package com.deervery.rakusikaliteclient;
import org.json.JSONObject;
import java.net.*;
import java.nio.charset.StandardCharsets;
import android.util.Base64;

/** Ephemeral ID token is checked by the existing read-only Lite API, never by a UI cookie alone. */
final class LiteSession {
    final String scope;
    private String origin="",token="",facility="";
    String selectedFacility="";
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
            JSONObject current=CatalogApi.get(origin,token,facility,"/api/facilities/current").getJSONObject("data").getJSONObject("facility");
            String actual=CatalogApi.id(current.optString("id"));
            if(!facility.isEmpty()&&!facility.equals(actual))return null;
            LiteSession session=new LiteSession(uid+":"+actual,expiry);session.origin=origin;session.token=token;session.facility=actual;session.selectedFacility=facility;return session;
        }catch(Exception ignored){return null;}
        finally{if(c!=null)c.disconnect();}
    }
    JSONObject catalog(JSONObject args) throws Exception {
        if(!valid()||token.isEmpty())throw new Exception("ログイン状態を確認してください。");
        return CatalogApi.read(origin,token,facility,args);
    }
    JSONObject printPayload(JSONObject args)throws Exception {
        if(!valid()||token.isEmpty())throw new Exception("ログインを確認してください。");
        JSONObject draft=args.getJSONObject("draft"),data=new JSONObject(draft.getJSONObject("data").toString());
        String template=draft.optString("template");int copies=draft.optInt("copies");
        if(!java.util.Arrays.asList("traceable","non_traceable","carcass","processed","pet").contains(template)||copies<1||copies>30)throw new Exception("ラベル内容を確認してください。");
        if(data.length()>30||data.toString().length()>12000)throw new Exception("ラベル内容が大きすぎます。");
        JSONObject facilityData=catalog(new JSONObject().put("kind","facility")).getJSONObject("item");
        JSONObject shown=new JSONObject(args.optString("facilitySnapshot","{}"));boolean same=shown.length()==facilityData.length();
        for(java.util.Iterator<String> keys=facilityData.keys();keys.hasNext();){String key=keys.next();same&=java.util.Objects.equals(facilityData.opt(key),shown.opt(key));}
        if(!same)throw new Exception("施設情報が変わりました。印刷内容を確認し直してください。");
        String name=facilityData.optString("name",facilityData.optString("facilityName")),address=facilityData.optString("address",facilityData.optString("companyAddress"));
        if(name.isEmpty()||address.isEmpty())throw new Exception("ラベルに必要な施設名・住所が未設定です。");
        // No paths or certification marks are accepted from the WebView/model.
        for(String key:new String[]{"logoFile","certificationMarkFile","ezoshikaCertified","plaMark","companyBlock","facilityBlock","qrCode"})data.remove(key);
        data.put("facilityName",name).put("processorName",name).put("processorLocation",address).put("facilityBlock",name+"\n"+address);
        if(template.equals("traceable")){
            String number=data.optString("individualNumber");org.json.JSONArray rows=catalog(new JSONObject().put("kind","individuals").put("query",number)).getJSONArray("items");
            String found="";for(int i=0;i<rows.length();i++){JSONObject row=rows.getJSONObject(i);if(number.equals(row.optString("individualNumber"))){if(!found.isEmpty())throw new Exception("同じ番号の個体があります。対象を確認してください。");found=row.optString("id");}}
            if(found.isEmpty())throw new Exception("登録済みの個体番号を選んでください。");data.put("qrCode","https://rakusika.com/t/"+CatalogApi.id(found));
        }
        boolean bear=data.optString("species").contains("クマ")||data.optString("productName").contains("熊");
        String hubTemplate=template.equals("traceable")||template.equals("carcass")?template+(bear?"_bear":"_deer"):template.equals("non_traceable")?"non_traceable_deer":template;
        return new JSONObject().put("template",hubTemplate).put("copies",copies).put("data",data);
    }
    JSONObject business(JSONObject args)throws Exception {
        if(!valid()||token.isEmpty())throw new Exception("ログイン状態を確認してください。");
        return BusinessApi.execute(origin,token,facility,args);
    }
    static String fingerprint(String token){try{return Base64.encodeToString(java.security.MessageDigest.getInstance("SHA-256").digest(token.getBytes(StandardCharsets.UTF_8)),Base64.NO_WRAP);}catch(Exception e){return "";}}
    static String snapshotScript(int ticket){return snapshotScript(ticket,false);}
    static String snapshotScript(int ticket,boolean confirm){return "(async()=>{let result={ticket:"+ticket+",confirm:"+confirm+"};try{"+
        "if(indexedDB.databases&&!(await indexedDB.databases()).some(x=>x.name==='firebaseLocalStorageDb'))throw 0;"+
        "const db=await new Promise((ok,no)=>{const r=indexedDB.open('firebaseLocalStorageDb');r.onsuccess=()=>ok(r.result);r.onerror=no;r.onupgradeneeded=()=>{r.transaction.abort();no(0)}});"+
        "let rows;try{rows=await new Promise((ok,no)=>{const r=db.transaction('firebaseLocalStorage','readonly').objectStore('firebaseLocalStorage').getAll();r.onsuccess=()=>ok(r.result);r.onerror=no})}finally{db.close()}"+
        "const users=rows.filter(x=>String(x.fbase_key).startsWith('firebase:authUser:')&&x.value?.uid&&x.value?.stsTokenManager?.accessToken);if(users.length!==1)throw 0;"+
        "result.token=users[0].value.stsTokenManager.accessToken;result.uid=users[0].value.uid;"+
        "result.facility=decodeURIComponent((document.cookie.match(/(?:^|;\\s*)active_facility_id=([^;]*)/)||[])[1]||'');"+
        "}catch{}StationSession.postMessage(JSON.stringify(result));})()";}
}
