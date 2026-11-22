package com.deervery.rakusikaliteclient;

import org.json.*;
import java.net.*;
import java.io.*;
import java.nio.charset.StandardCharsets;

/** Read-only, bounded, same-facility catalog. ID tokens stay in native memory. */
final class CatalogApi {
    static final String[] FIELDS={"id","individualNumber","species","sex","captureLocation","receivingDate","timeToFactory","name","defaultContentQuantity","unit","expirationDays","productIngredient","ingredient","nutritionUnit","caloriesQuantity","proteinQuantity","fatQuantity","carbohydratesQuantity","saltEquivalentQuantity","attentionText","isHeatedMeatProducts","isTraceable","productType","storageMethod"};
    static String encode(String value){try{return URLEncoder.encode(value,"UTF-8");}catch(java.io.UnsupportedEncodingException e){throw new AssertionError(e);}}
    static String id(String value)throws Exception {if(!value.matches("[A-Za-z0-9_-]{1,128}"))throw new Exception("コードを確認してください。");return value;}
    static JSONObject project(JSONObject source)throws Exception {
        JSONObject out=new JSONObject();for(String key:FIELDS){Object value=source.opt(key);if(value instanceof String&&((String)value).length()<=500||value instanceof Number||value instanceof Boolean)out.put(key,value);}
        return out;
    }
    static JSONObject read(String origin,String token,String facility,JSONObject args)throws Exception {
        String kind=args.optString("kind"),query=args.optString("query"),cursor=args.optString("cursor");
        if(query.length()>100||cursor.length()>2048)throw new Exception("検索条件を短くしてください。");
        if(kind.equals("trace")){
            String code=id(args.optString("id"));JSONObject row;
            try {row=get(origin,token,facility,"/api/individuals/"+code);}
            catch(ApiFailure e){if(e.status!=404)throw e;JSONObject lot=get(origin,token,facility,"/api/meat-lots/"+code).getJSONObject("data").getJSONObject("lot");
                if(!facility.isEmpty()&&!facility.equals(lot.optString("facilityId")))throw new Exception("この施設の個体を選んでください。");
                row=get(origin,token,facility,"/api/individuals/"+id(lot.optString("individualId")));}
            if(!facility.isEmpty()&&!facility.equals(row.optString("facilityId")))throw new Exception("この施設の個体を選んでください。");
            return new JSONObject().put("item",project(row));
        }
        if(kind.equals("individual")){
            JSONObject row=get(origin,token,facility,"/api/individuals/"+id(args.optString("id")));
            if(!facility.isEmpty()&&!facility.equals(row.optString("facilityId")))throw new Exception("この施設の個体を選んでください。");
            return new JSONObject().put("item",project(row));
        }
        String path;
        if(kind.equals("individuals"))path="/api/individuals?limit=24&fields=summary&archived=exclude&individualNumber="+encode(query);
        else if(kind.equals("products"))path="/api/products/query?limit=24&archived=false&name="+encode(query);
        else throw new Exception("対応していない検索です。");
        if(!facility.isEmpty())path+="&facilityId="+encode(facility);
        if(!cursor.isEmpty())path+="&cursor="+encode(cursor);
        JSONObject body=get(origin,token,facility,path),data=body.optJSONObject("data");
        JSONArray rows=data==null?body.optJSONArray("items"):data.optJSONArray("items"),out=new JSONArray();
        if(rows==null)throw new Exception("候補を読み取れませんでした。");
        for(int i=0;i<Math.min(rows.length(),24);i++)out.put(project(rows.getJSONObject(i)));
        JSONObject meta=body.optJSONObject("meta");
        return new JSONObject().put("items",out).put("cursor",meta!=null&&meta.optBoolean("hasNext")?meta.optString("nextCursor"):"");
    }
    private static final class ApiFailure extends Exception{final int status;ApiFailure(int status,String message){super(message);this.status=status;}}
    private static JSONObject get(String origin,String token,String facility,String path)throws Exception {
        HttpURLConnection c=(HttpURLConnection)new URL(origin+path).openConnection();
        try{
            c.setRequestMethod("GET");c.setInstanceFollowRedirects(false);c.setConnectTimeout(4000);c.setReadTimeout(8000);c.setRequestProperty("Authorization","Bearer "+token);
            if(!facility.isEmpty())c.setRequestProperty("Cookie","active_facility_id="+facility);
            int status=c.getResponseCode();if(status!=200)throw new ApiFailure(status,status==401?"ログインし直してください。":status==403?"この施設のデータを選んでください。":status==404?"個体が見つかりません。番号で検索してください。":"候補を取得できません。接続を確認してください。");
            try(InputStream in=c.getInputStream();ByteArrayOutputStream out=new ByteArrayOutputStream()){
                byte[] buffer=new byte[4096];int n;while((n=in.read(buffer))!=-1){if(out.size()+n>524288)throw new Exception("検索結果が大きすぎます。条件を絞ってください。");out.write(buffer,0,n);}return new JSONObject(out.toString(StandardCharsets.UTF_8.name()));
            }
        }finally{c.disconnect();}
    }
}
