package com.deervery.rakusikaliteclient;
import android.content.SharedPreferences;
import org.json.JSONObject;
import java.net.*;
import java.io.*;
import java.nio.charset.StandardCharsets;
/** Hub has no durable deduplication contract: dispatch once, persist unknown across restart, never retry. */
final class HubPrint {
 static synchronized JSONObject execute(SharedPreferences store,String scope,String hub,JSONObject args,JSONObject payload)throws Exception {
  String id=args.optString("requestId");if(!id.matches("[A-Za-z0-9-]{16,80}"))throw new Exception("印刷受付番号を確認してください。");
  if(hub.isEmpty())throw new Exception("設定から施設ハブの接続先を設定してください。");
  String key="print-receipt:"+scope+":"+id,previous=store.getString(key,"");if(!previous.isEmpty())return new JSONObject(previous);
  JSONObject unknown=new JSONObject().put("state","unknown").put("message","印刷結果を確認できません。ラベルとプリンタを確認してください。自動再送はしません。");
  if(!store.edit().putString(key,unknown.toString()).commit())throw new Exception("印刷受付を保存できません。");
  HttpURLConnection c=null;
  try{
   c=(HttpURLConnection)new URL(hub+"/printer/print").openConnection();c.setRequestMethod("POST");c.setInstanceFollowRedirects(false);c.setConnectTimeout(4000);c.setReadTimeout(20000);c.setRequestProperty("Content-Type","application/json");c.setDoOutput(true);
   byte[] bytes=payload.toString().getBytes(StandardCharsets.UTF_8);c.setFixedLengthStreamingMode(bytes.length);try(OutputStream out=c.getOutputStream()){out.write(bytes);}
   if(c.getResponseCode()!=200)return unknown;
   JSONObject response;try(InputStream in=c.getInputStream();ByteArrayOutputStream out=new ByteArrayOutputStream()){byte[] buffer=new byte[1024];int n;while((n=in.read(buffer))!=-1){if(out.size()+n>65536)throw new IOException();out.write(buffer,0,n);}response=new JSONObject(out.toString("UTF-8"));}
   String state=response.optString("printState");if(!java.util.Arrays.asList("completed","printed","done").contains(state))state="pending";else state="done";
   JSONObject result=new JSONObject().put("state",state).put("message",state.equals("done")?"ハブが印刷完了を報告しました。ラベルの印字を確認してください。":"ハブが受け付けました。出力を確認してください。自動再送はしません。").put("jobId",response.optString("jobId"));
   store.edit().putString(key,result.toString()).commit();return result;
  }catch(Exception e){return unknown;}finally{if(c!=null)c.disconnect();}
 }
}
