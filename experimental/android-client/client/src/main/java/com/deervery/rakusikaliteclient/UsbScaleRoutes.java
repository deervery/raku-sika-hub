package com.deervery.rakusikaliteclient;

import android.webkit.WebResourceRequest;
import android.webkit.WebResourceResponse;
import java.io.*;
import java.net.*;
import java.nio.charset.StandardCharsets;
import java.util.*;
import org.json.JSONObject;

/** Intercepts only the chosen Hub's scale routes and health, from trusted Lite. */
final class UsbScaleRoutes {
    static final String LOCAL="https://android-scale.invalid";
    interface Device {boolean connected();JSONObject command(String command);}
    private final Device device;
    UsbScaleRoutes(Device device){this.device=device;}
    WebResourceResponse intercept(WebResourceRequest request,String hub,String web,boolean trusted){
        String target=request.getUrl().toString(),base=hub.isEmpty()?LOCAL:hub;
        String origin=EndpointPolicy.origin(web),received=request.getRequestHeaders().get("Origin");
        if(received==null)received=request.getRequestHeaders().get("origin");
        String path=route(target,base);
        if(path==null)return null;
        if(!trusted||request.isForMainFrame()||!origin.equals(received))return response(UsbScale.error("FORBIDDEN","許可されていない画面です"),403,origin);
        if(request.getMethod().equals("OPTIONS"))return response(new JSONObject(),200,origin);
        if(path.equals("/health")){
            if(!request.getMethod().equals("GET"))return response(UsbScale.error("METHOD","GET required"),405,origin);
            return response(health(hub),200,origin);
        }
        if(!request.getMethod().equals("POST"))return response(UsbScale.error("METHOD","POST required"),405,origin);
        String command=path.equals("/scale/weigh")?"Q":path.equals("/scale/tare")?"T":path.equals("/scale/zero")?"R":null;
        JSONObject result=command==null?UsbScale.error("UNSUPPORTED","USB直結ではこの操作に対応していません"):device.command(command);
        return response(result,result.optString("status").equals("error")?503:200,origin);
    }
    private static String route(String target,String base){
        if(target.equals(base+"/health"))return "/health";
        if(target.startsWith(base+"/scale/"))return target.substring(base.length());
        return null;
    }
    private JSONObject health(String hub){
        JSONObject result=new JSONObject();
        if(!hub.isEmpty()){
            HttpURLConnection conn=null;
            try{
                conn=(HttpURLConnection)new URL(hub+"/health").openConnection();
                conn.setConnectTimeout(1200);conn.setReadTimeout(1200);conn.setInstanceFollowRedirects(false);
                if(conn.getResponseCode()==200){
                    ByteArrayOutputStream data=new ByteArrayOutputStream();
                    try(InputStream input=conn.getInputStream()){
                        byte[] bytes=new byte[2048];int n;
                        while((n=input.read(bytes))!=-1){if(data.size()+n>32768)throw new IOException();data.write(bytes,0,n);}
                    }
                    result=new JSONObject(data.toString("UTF-8"));
                }
            }catch(Exception ignored){}finally{if(conn!=null)conn.disconnect();}
        }
        try{
            if(!result.has("printer"))result.put("printer",new JSONObject().put("connected",false));
            if(!result.has("scanner"))result.put("scanner",new JSONObject().put("connected",false));
            return result.put("status","ok").put("scale",new JSONObject().put("connected",device.connected()).put("device","android-usb"));
        }catch(org.json.JSONException impossible){throw new IllegalStateException(impossible);}
    }
    private static WebResourceResponse response(JSONObject data,int code,String origin){
        Map<String,String> headers=new HashMap<>();headers.put("Access-Control-Allow-Origin",origin);
        headers.put("Access-Control-Allow-Methods","GET, POST, OPTIONS");headers.put("Access-Control-Allow-Headers","Content-Type");headers.put("Cache-Control","no-store");
        return new WebResourceResponse("application/json","UTF-8",code,code==200?"OK":"Error",headers,new ByteArrayInputStream(data.toString().getBytes(StandardCharsets.UTF_8)));
    }
}
