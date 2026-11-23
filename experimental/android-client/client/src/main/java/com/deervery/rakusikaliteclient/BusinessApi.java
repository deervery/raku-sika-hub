package com.deervery.rakusikaliteclient;
import org.json.*;
import java.net.*;
import java.io.*;
import java.nio.charset.StandardCharsets;

/** Fixed durable endpoints only; tokens and facility selection never come from the model. */
final class BusinessApi {
    static String path(String kind,String id)throws Exception {
        switch(kind){
            case "receiving":return "/api/individuals/durable";
            case "meat":return "/api/meat-lots/durable";
            case "processed":return "/api/processed-lots/durable";
            case "pet":return "/api/pet-food-lots/durable";
            case "daily":case "weekly":case "monthly":case "employee-checks":return "/api/periodic-records/"+kind+"/durable";
            case "outbound":return "/api/meat-lots/"+CatalogApi.id(id)+"/status/durable";
            default:throw new Exception("対応していない登録です。");
        }
    }
    static JSONObject execute(String origin,String token,String facility,JSONObject args)throws Exception {
        String kind=args.optString("kind"),key=args.optString("requestId");
        String path=path(kind,args.optString("id"));
        if(!key.matches("[A-Za-z0-9:_-]{16,128}")||!args.optBoolean("confirmed"))throw new Exception("内容を確認してから登録してください。");
        JSONObject data=args.getJSONObject("data");
        for(String forbidden:new String[]{"facilityId","createdBy","updatedBy","archived","createdAt","updatedAt"})if(data.has(forbidden))throw new Exception("登録できない項目があります。");
        if(kind.equals("outbound")&&!data.optString("status").equals("shipped"))throw new Exception("出庫内容を確認してください。");
        byte[] body=new JSONObject().put("data",data).toString().getBytes(StandardCharsets.UTF_8);
        if(body.length>65536)throw new Exception("登録内容が大きすぎます。");
        HttpURLConnection c=(HttpURLConnection)new URL(origin+path).openConnection();
        try{
            c.setRequestMethod(kind.equals("outbound")?"PATCH":"POST");c.setInstanceFollowRedirects(false);c.setConnectTimeout(4000);c.setReadTimeout(20000);
            c.setRequestProperty("Authorization","Bearer "+token);c.setRequestProperty("Content-Type","application/json");c.setRequestProperty("Idempotency-Key",key);c.setRequestProperty("X-Expected-Facility-Id",facility);c.setRequestProperty("Cookie","active_facility_id="+facility);
            c.setDoOutput(true);c.setFixedLengthStreamingMode(body.length);try(OutputStream out=c.getOutputStream()){out.write(body);}
            int status=c.getResponseCode();InputStream stream=status>=200&&status<300?c.getInputStream():c.getErrorStream();
            JSONObject result;
            if(stream==null)throw new IOException();
            try(InputStream in=stream;ByteArrayOutputStream out=new ByteArrayOutputStream()){byte[] bytes=new byte[4096];int n;while((n=in.read(bytes))!=-1){if(out.size()+n>262144)throw new IOException();out.write(bytes,0,n);}result=new JSONObject(out.toString("UTF-8"));}
            if(status<200||status>=300)throw new Exception(result.optString("message","受付結果を確認できません。同じ受付番号で確認してください。"));
            if(!result.optBoolean("ok")||result.optJSONObject("data")==null)throw new Exception("受付結果を読み取れません。同じ受付番号で確認してください。");
            return result;
        }finally{c.disconnect();}
    }
}
