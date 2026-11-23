package com.deervery.rakusikaliteclient;
import android.view.View;
import android.webkit.CookieManager;
import androidx.test.core.app.ActivityScenario;
import androidx.test.ext.junit.runners.AndroidJUnit4;
import org.junit.Test;
import org.junit.runner.RunWith;
import java.util.concurrent.*;
import static org.junit.Assert.*;

@RunWith(AndroidJUnit4.class)
public class LoginGateTest {
    @Test public void noLoginCannotOpenAssistantEvenWithForgedUiCookie()throws Exception{
        android.content.Context context=androidx.test.platform.app.InstrumentationRegistry.getInstrumentation().getTargetContext();
        context.getSharedPreferences("lite-client",0).edit().putBoolean("configured",true).putString("web","https://rakusika.com/home").putString("hub","").commit();
        CountDownLatch ready=new CountDownLatch(1);
        try(ActivityScenario<MainActivity> s=ActivityScenario.launch(MainActivity.class)){
            s.onActivity(a->{android.webkit.WebView w=a.findViewById(R.id.work_web);w.stopLoading();w.setWebViewClient(new android.webkit.WebViewClient(){@Override public void onPageFinished(android.webkit.WebView v,String url){if(url.contains("fixture"))ready.countDown();}});
                CookieManager.getInstance().setCookie("https://rakusika.com","__session_exists=true; path=/");
                w.loadDataWithBaseURL("https://rakusika.com/fixture","<body>login fixture</body>","text/html","UTF-8",null);});
            boolean fixtureReady=false;
            for(int i=0;i<40;i++){
                java.util.concurrent.atomic.AtomicBoolean loaded=new java.util.concurrent.atomic.AtomicBoolean();CountDownLatch checked=new CountDownLatch(1);
                s.onActivity(a->((android.webkit.WebView)a.findViewById(R.id.work_web)).evaluateJavascript("document.body&&document.body.textContent==='login fixture'",value->{loaded.set("true".equals(value));checked.countDown();}));
                assertTrue(checked.await(5,TimeUnit.SECONDS));if(loaded.get()){fixtureReady=true;break;}Thread.sleep(100);
            }
            assertTrue("Login fixture DOM must load",fixtureReady);s.onActivity(a->a.findViewById(R.id.tab_agent).performClick());
            Thread.sleep(2800);
            s.onActivity(a->{assertNotEquals(View.VISIBLE,a.findViewById(R.id.agent_panel).getVisibility());assertEquals(View.VISIBLE,a.findViewById(R.id.work_web).getVisibility());CookieManager.getInstance().setCookie("https://rakusika.com","__session_exists=; path=/; max-age=0");});
        }
    }
    @Test public void expiredAndUntrustedSessionsFailClosed(){
        assertFalse(new LiteSession("user:facility",System.currentTimeMillis()-1).valid());
        assertFalse(new LiteSession("",System.currentTimeMillis()+60000).valid());
        assertNull(LiteSession.verify("https://untrusted.invalid","abc.def.ghi","uid",""));
        assertNull(LiteSession.verify("https://rakusika.com","not-a-token","uid",""));
    }
}
