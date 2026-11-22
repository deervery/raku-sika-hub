package com.deervery.rakusikaliteclient;
import android.net.Uri;
import android.webkit.*;
import androidx.test.core.app.ActivityScenario;
import androidx.test.ext.junit.runners.AndroidJUnit4;
import org.junit.Test;
import org.junit.runner.RunWith;
import org.json.JSONObject;
import java.util.*;
import java.util.concurrent.*;
import java.util.concurrent.atomic.*;
import static org.junit.Assert.*;

@RunWith(AndroidJUnit4.class)
public class UsbScaleRoutesTest {
    private UsbScaleRoutes routes(AtomicInteger calls){return new UsbScaleRoutes(new UsbScaleRoutes.Device(){
        public boolean connected(){return true;}
        public JSONObject command(String command){calls.incrementAndGet();try{return new JSONObject("{\"status\":\"ok\",\"value\":0.07,\"unit\":\"kg\",\"stable\":true}");}catch(Exception e){throw new AssertionError(e);}}
    });}
    @Test public void existingFetchContractReadsLocalUsbWithoutHub()throws Exception{
        AtomicInteger calls=new AtomicInteger();UsbScaleRoutes router=routes(calls);
        CountDownLatch done=new CountDownLatch(1);AtomicReference<String> value=new AtomicReference<>();AtomicReference<WebView> holder=new AtomicReference<>();
        try(ActivityScenario<MainActivity> activity=ActivityScenario.launch(MainActivity.class)){
            activity.onActivity(a->{
                WebView web=new WebView(a);holder.set(web);a.setContentView(web);web.getSettings().setJavaScriptEnabled(true);
                web.setWebViewClient(new WebViewClient(){
                    @Override public WebResourceResponse shouldInterceptRequest(WebView v,WebResourceRequest r){return router.intercept(r,"","https://rakusika.com",true);}
                    @Override public void onPageFinished(WebView v,String url){
                        v.evaluateJavascript("(async()=>{try{const h=await(await fetch('https://android-scale.invalid/health')).json();const w=await(await fetch('https://android-scale.invalid/scale/weigh',{method:'POST'})).json();window.result=[h.scale.connected,h.printer.connected,w.value,w.unit,w.stable].join(':')}catch(e){window.result='error:'+e}})()",null);
                        v.postDelayed(()->v.evaluateJavascript("window.result",s->{value.set(s);done.countDown();}),2000);
                    }
                });
                web.loadDataWithBaseURL("https://rakusika.com/fixture","<body>USB contract fixture</body>","text/html","UTF-8","https://rakusika.com/fixture");
            });
            assertTrue(done.await(15,TimeUnit.SECONDS));assertEquals("\"true:false:0.07:kg:true\"",value.get());assertEquals(1,calls.get());
            activity.onActivity(a->holder.get().destroy());
        }
    }
    @Test public void untrustedPausedOrWrongMethodCannotOperateUsb(){
        AtomicInteger calls=new AtomicInteger();UsbScaleRoutes router=routes(calls);
        assertEquals(403,router.intercept(request("/scale/tare","POST","https://evil.invalid"),"","https://rakusika.com",true).getStatusCode());
        assertEquals(403,router.intercept(request("/scale/zero","POST","https://rakusika.com"),"","https://rakusika.com",false).getStatusCode());
        assertEquals(405,router.intercept(request("/scale/weigh","GET","https://rakusika.com"),"","https://rakusika.com",true).getStatusCode());
        assertNull(router.intercept(request("/printer/print","POST","https://rakusika.com"),"","https://rakusika.com",true));
        assertEquals(0,calls.get());
    }
    private WebResourceRequest request(String path,String method,String origin){return new WebResourceRequest(){
        public Uri getUrl(){return Uri.parse(UsbScaleRoutes.LOCAL+path);}
        public boolean isForMainFrame(){return false;}public boolean isRedirect(){return false;}public boolean hasGesture(){return false;}
        public String getMethod(){return method;}public Map<String,String> getRequestHeaders(){return Collections.singletonMap("Origin",origin);}
    };}
}
