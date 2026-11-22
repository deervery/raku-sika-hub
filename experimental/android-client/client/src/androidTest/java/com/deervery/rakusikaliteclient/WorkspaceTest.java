package com.deervery.rakusikaliteclient;
import android.view.View;
import android.webkit.*;
import androidx.test.core.app.ActivityScenario;
import androidx.test.ext.junit.runners.AndroidJUnit4;
import org.junit.Test;
import org.junit.runner.RunWith;
import java.util.concurrent.*;
import java.util.concurrent.atomic.*;
import static org.junit.Assert.*;

@RunWith(AndroidJUnit4.class)
public class WorkspaceTest {
    @Test public void lateScaleReplyIsRejectedButPrintResultRemainsVisible()throws Exception{
        CountDownLatch loaded=new CountDownLatch(1),done=new CountDownLatch(1);AtomicReference<String> value=new AtomicReference<>();AtomicReference<WebView> holder=new AtomicReference<>();
        try(ActivityScenario<MainActivity> scenario=ActivityScenario.launch(MainActivity.class)){
            scenario.onActivity(a->{WebView web=new WebView(a);holder.set(web);a.setContentView(web);web.getSettings().setJavaScriptEnabled(true);
                web.setWebViewClient(new WebViewClient(){@Override public void onPageFinished(WebView v,String url){loaded.countDown();}});
                web.loadDataWithBaseURL("https://rakusika.com/fixture","<body>lifecycle fixture</body>","text/html","UTF-8","https://rakusika.com/fixture");});
            assertTrue(loaded.await(15,TimeUnit.SECONDS));
            scenario.onActivity(a->{WebView v=holder.get();v.evaluateJavascript("window.releases=[];window.fetch=()=>new Promise(r=>releases.push(r));"+WorkspaceScript.guard("https://hub.invalid")+
                ";Promise.all([fetch('https://hub.invalid/scale/weigh').then(()=> 'bad',e=>e.name),fetch('https://hub.invalid/printer/print').then(()=> 'print-result')]).then(r=>window.result=r.join(':'));window.dispatchEvent(new Event('station-pause'));window.dispatchEvent(new Event('station-resume'));releases.forEach(r=>r(new Response('{}')))",null);
                v.postDelayed(()->v.evaluateJavascript("window.result",s->{value.set(s);done.countDown();}),600);
            });
            assertTrue(done.await(10,TimeUnit.SECONDS));assertEquals("\"AbortError:print-result\"",value.get());scenario.onActivity(a->holder.get().destroy());
        }
    }
    @Test public void switchingRetainsWebInstanceInputAndNavigation()throws Exception{
        android.content.Context context=androidx.test.platform.app.InstrumentationRegistry.getInstrumentation().getTargetContext();
        context.getSharedPreferences("lite-client",0).edit().putBoolean("configured",true).putBoolean("usbScale",false).putString("web","https://rakusika.com/home").putString("hub","").commit();
        AtomicReference<WebView> holder=new AtomicReference<>();AtomicReference<String> result=new AtomicReference<>();CountDownLatch loaded=new CountDownLatch(1),done=new CountDownLatch(1);
        try(ActivityScenario<MainActivity> scenario=ActivityScenario.launch(MainActivity.class)){
            scenario.onActivity(a->{
                WebView web=a.findViewById(R.id.work_web);holder.set(web);web.stopLoading();
                web.setWebViewClient(new WebViewClient(){@Override public void onPageFinished(WebView v,String url){
                    if(!url.equals("https://rakusika.com/fixture"))return;
                    v.evaluateJavascript("(()=>{const input=document.getElementById('draft');if(!input)return false;input.value='未保存の入力';window.counter=7;return true;})()",r->{if("true".equals(r))loaded.countDown();});
                }});
                web.loadDataWithBaseURL("https://rakusika.com/fixture","<input id='draft' aria-label='draft'>","text/html","UTF-8","https://rakusika.com/fixture");
            });
            assertTrue(loaded.await(20,TimeUnit.SECONDS));
            scenario.onActivity(a->{
                // Verified session fixture; production tab entry must always pass the server check.
                try{java.lang.reflect.Field session=MainActivity.class.getDeclaredField("liteSession");session.setAccessible(true);session.set(a,new LiteSession("fixture:office",System.currentTimeMillis()+60000));
                    java.lang.reflect.Method show=MainActivity.class.getDeclaredMethod("showAgent",boolean.class);show.setAccessible(true);show.invoke(a,true);
                }catch(Exception e){throw new AssertionError(e);}
                assertEquals(View.VISIBLE,a.findViewById(R.id.agent_panel).getVisibility());
            });
            scenario.onActivity(a->{a.findViewById(R.id.tab_work).performClick();assertSame(holder.get(),a.findViewById(R.id.work_web));holder.get().evaluateJavascript("document.getElementById('draft').value+':'+window.counter",r->{result.set(r);done.countDown();});});
            assertTrue(done.await(10,TimeUnit.SECONDS));assertEquals("\"未保存の入力:7\"",result.get());
        }
    }
}
