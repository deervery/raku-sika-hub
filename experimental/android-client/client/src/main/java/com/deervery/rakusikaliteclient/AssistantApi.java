package com.deervery.rakusikaliteclient;
import org.json.JSONObject;
import java.net.*;
import java.io.*;
/** Text-only interpretation broker. No model keys or arbitrary URLs cross the UI bridge. */
final class AssistantApi {
    static final class Call {
    private volatile HttpURLConnection connection;
    private volatile boolean cancelled;
    void cancel(){cancelled=true;HttpURLConnection current=connection;if(current!=null)current.disconnect();}
    JSONObject execute(JSONObject input){
        HttpURLConnection c=null;
        try{
            c=(HttpURLConnection)new URL("https://glwin.zapus-opah.ts.net:8446/interpret").openConnection();
            connection=c;if(cancelled)throw new IOException();
            c.setRequestMethod("POST");c.setConnectTimeout(5000);c.setReadTimeout(20000);c.setInstanceFollowRedirects(false);c.setDoOutput(true);c.setRequestProperty("Content-Type","application/json");
            byte[] body=input.toString().getBytes(java.nio.charset.StandardCharsets.UTF_8);if(body.length>16000)throw new IOException();
            try(OutputStream out=c.getOutputStream()){out.write(body);}
            int status=c.getResponseCode();
            ByteArrayOutputStream bytes=new ByteArrayOutputStream();try(InputStream in=status==200?c.getInputStream():c.getErrorStream()){if(in==null)throw new IOException();byte[] part=new byte[2048];int n;while((n=in.read(part))!=-1){if(bytes.size()+n>16000)throw new IOException();bytes.write(part,0,n);}}
            JSONObject result=new JSONObject(bytes.toString("UTF-8"));
            if(status==403)return new JSONObject().put("error","このTailscaleアカウントでは会話サービスを利用できません。接続アカウントを確認してください。");
            if(status!=200){String error=result.optString("error");if(error.isEmpty()||error.length()>400)throw new IOException();return new JSONObject().put("error",error);}
            return result;
        }catch(Exception e){try{return new JSONObject().put("error","会話サービスに接続できません。Tailscaleを確認してください。端末内の入力と画面操作は使えます。");}catch(Exception ignored){return new JSONObject();}}
        finally{connection=null;if(c!=null)c.disconnect();}
    }
    }
}
