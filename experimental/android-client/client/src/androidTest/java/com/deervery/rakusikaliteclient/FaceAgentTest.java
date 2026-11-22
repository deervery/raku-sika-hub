package com.deervery.rakusikaliteclient;
import androidx.test.core.app.ActivityScenario;
import androidx.test.ext.junit.runners.AndroidJUnit4;
import android.webkit.WebView;
import org.json.JSONObject;
import org.junit.Test;
import org.junit.runner.RunWith;
import java.util.concurrent.*;
import java.util.concurrent.atomic.*;
import static org.junit.Assert.*;

@RunWith(AndroidJUnit4.class)
public class FaceAgentTest {
    private String js(ActivityScenario<MainActivity> s,WebView w,String script)throws Exception{
        CountDownLatch done=new CountDownLatch(1);AtomicReference<String> value=new AtomicReference<>();
        s.onActivity(a->w.evaluateJavascript(script,r->{value.set(r);done.countDown();}));assertTrue(done.await(5,TimeUnit.SECONDS));return value.get();
    }
    @Test public void bundledFaceLoadsAndUsesNarrowNativeScaleBridge()throws Exception{
        AtomicReference<FaceAgentView> holder=new AtomicReference<>();AtomicInteger reads=new AtomicInteger();
        try(ActivityScenario<MainActivity> s=ActivityScenario.launch(MainActivity.class)){
            s.onActivity(a->{FaceAgentView v=new FaceAgentView(a,new FaceAgentView.Host(){
                public void microphonePermission(){}public void openWork(String path){}
                public void weigh(java.util.function.Consumer<JSONObject> callback){reads.incrementAndGet();try{callback.accept(new JSONObject("{\"status\":\"ok\",\"stable\":true,\"value\":0.07,\"unit\":\"kg\"}"));}catch(Exception e){throw new AssertionError(e);}}
            });holder.set(v);a.setContentView(v);v.activate();});
            FaceAgentView v=holder.get();boolean loaded=false;
            for(int i=0;i<60;i++){if("true".equals(js(s,v,"document.body.innerText.includes('分かっていることから')"))){loaded=true;break;}Thread.sleep(150);}
            assertTrue("Bundled original face must render without a Next server",loaded);
            js(s,v,"window.stationReply=(id,value)=>window.testWeight=value;StationNative.postMessage(JSON.stringify({id:'99',op:'weigh',args:{}}))");
            String answer="";for(int i=0;i<30;i++){answer=js(s,v,"window.testWeight?.value");if(answer.equals("0.07"))break;Thread.sleep(100);}
            assertEquals("0.07",answer);assertEquals(1,reads.get());
            s.onActivity(a->v.suspend());js(s,v,"StationNative.postMessage(JSON.stringify({id:'100',op:'weigh',args:{}}))");
            assertEquals(1,reads.get());
            s.onActivity(a->{v.activate();v.loadDataWithBaseURL("https://untrusted.invalid/","<body>untrusted</body>","text/html","UTF-8",null);});
            String available="";for(int i=0;i<30;i++){available=js(s,v,"location.origin+':'+typeof StationNative");if(available.contains("untrusted.invalid"))break;Thread.sleep(100);}
            assertTrue(available.endsWith(":undefined\""));assertFalse(available.contains(FaceAgentView.ORIGIN));
            s.onActivity(a->v.destroy());
        }
    }
}
