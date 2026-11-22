package com.deervery.rakusikaliteclient;

import android.webkit.WebView;
import android.webkit.WebViewClient;
import androidx.test.core.app.ActivityScenario;
import androidx.test.ext.junit.runners.AndroidJUnit4;
import androidx.webkit.WebViewCompat;
import androidx.webkit.WebViewFeature;
import org.junit.Test;
import org.junit.runner.RunWith;
import java.util.Collections;
import java.util.concurrent.CountDownLatch;
import java.util.concurrent.TimeUnit;
import java.util.concurrent.atomic.AtomicReference;
import static org.junit.Assert.*;

/** Local HTML fixtures only: never signs in, registers business data, or prints. */
@RunWith(AndroidJUnit4.class)
public class ClientSmokeTest {
    @Test public void hubSettingExistsBeforePageScript() throws Exception {
        try(ActivityScenario<MainActivity> scenario=ActivityScenario.launch(MainActivity.class)){
            CountDownLatch done=new CountDownLatch(1);
            AtomicReference<String> value=new AtomicReference<>();
            AtomicReference<WebView> holder=new AtomicReference<>();
            scenario.onActivity(activity->{
                assertTrue(WebViewFeature.isFeatureSupported(WebViewFeature.DOCUMENT_START_SCRIPT));
                WebView web=new WebView(activity);holder.set(web);activity.setContentView(web);
                web.getSettings().setJavaScriptEnabled(true);web.getSettings().setDomStorageEnabled(true);
                WebViewCompat.addDocumentStartJavaScript(web,"localStorage.setItem('rakuSikaHubUrl','https://test-hub.zapus-opah.ts.net')",Collections.singleton("https://rakusika.com"));
                web.setWebViewClient(new WebViewClient(){@Override public void onPageFinished(WebView view,String url){view.evaluateJavascript("window.hubAtStart",result->{value.set(result);done.countDown();});}});
                web.loadDataWithBaseURL("https://rakusika.com/fixture","<script>window.hubAtStart=localStorage.getItem('rakuSikaHubUrl')</script>","text/html","UTF-8","https://rakusika.com/fixture");
            });
            assertTrue(done.await(20,TimeUnit.SECONDS));assertEquals("\"https://test-hub.zapus-opah.ts.net\"",value.get());
            scenario.onActivity(activity->holder.get().destroy());
        }
    }
    @Test public void nativeScannerDeliveryReachesWebOnce() throws Exception {
        try(ActivityScenario<MainActivity> scenario=ActivityScenario.launch(MainActivity.class)){
            CountDownLatch loaded=new CountDownLatch(1),done=new CountDownLatch(1);
            AtomicReference<WebView> holder=new AtomicReference<>();AtomicReference<String> value=new AtomicReference<>();
            scenario.onActivity(activity->{
                WebView web=new WebView(activity);holder.set(web);activity.setContentView(web);web.getSettings().setJavaScriptEnabled(true);
                web.setFocusableInTouchMode(true);
                web.setWebViewClient(new WebViewClient(){@Override public void onPageFinished(WebView view,String url){view.requestFocus();view.evaluateJavascript("document.body.focus()",value->loaded.countDown());}});
                web.loadDataWithBaseURL("https://rakusika.com/fixture","<body tabindex='0'><script>window.scan='';window.count=0;window.buffer='';window.addEventListener('keydown',e=>{if(e.key==='Enter'){window.scan=window.buffer;window.buffer='';window.count++}else if(e.key.length===1)window.buffer+=e.key})</script></body>","text/html","UTF-8","https://rakusika.com/fixture");
            });
            assertTrue(loaded.await(20,TimeUnit.SECONDS));
            scenario.onActivity(activity->{
                WebView web=holder.get();
                assertTrue("Fixture origin: "+web.getUrl(),EndpointPolicy.sameOrigin("https://rakusika.com",web.getUrl()));
                ScannerDelivery.deliver(web,"https://rakusika.com","00123456");
                web.postDelayed(()->web.evaluateJavascript("window.scan+':'+window.count",result->{value.set(result);done.countDown();}),500);
            });
            assertTrue(done.await(10,TimeUnit.SECONDS));assertEquals("\"00123456:1\"",value.get());
            scenario.onActivity(activity->holder.get().destroy());
        }
    }
}
