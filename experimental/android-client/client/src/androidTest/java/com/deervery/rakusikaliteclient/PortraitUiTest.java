package com.deervery.rakusikaliteclient;
import android.webkit.WebView;
import androidx.test.core.app.ActivityScenario;
import androidx.test.ext.junit.runners.AndroidJUnit4;
import androidx.test.platform.app.InstrumentationRegistry;
import org.junit.Test;
import org.junit.runner.RunWith;
import java.util.concurrent.*;
import java.util.concurrent.atomic.*;
import static org.junit.Assert.*;

/** UI fixture only: no real account, business write, speech recording or hardware action. */
@RunWith(AndroidJUnit4.class)
public class PortraitUiTest {
    private String js(ActivityScenario<MainActivity> s,WebView v,String source)throws Exception{
        AtomicReference<String> result=new AtomicReference<>();CountDownLatch done=new CountDownLatch(1);
        s.onActivity(a->v.evaluateJavascript(source,r->{result.set(r);done.countDown();}));assertTrue(done.await(5,TimeUnit.SECONDS));return result.get();
    }
    private void capture(String name)throws Exception{
        android.graphics.Bitmap bitmap=InstrumentationRegistry.getInstrumentation().getUiAutomation().takeScreenshot();
        java.io.File target=new java.io.File(InstrumentationRegistry.getInstrumentation().getTargetContext().getExternalFilesDir("screenshots"),name);
        try(java.io.FileOutputStream out=new java.io.FileOutputStream(target)){assertTrue(bitmap.compress(android.graphics.Bitmap.CompressFormat.PNG,100,out));}bitmap.recycle();
    }
    @Test public void portraitHomeMenuAndDefaultsFitViewport()throws Exception{
        android.content.Context context=InstrumentationRegistry.getInstrumentation().getTargetContext();
        context.getSharedPreferences("lite-client",0).edit().putBoolean("configured",true).putString("web","https://rakusika.com/home").putString("hub","").commit();
        AtomicReference<WebView> face=new AtomicReference<>();
        try(ActivityScenario<MainActivity> s=ActivityScenario.launch(MainActivity.class)){
            s.onActivity(a->{try{
                WebView work=a.findViewById(R.id.work_web);work.stopLoading();work.setWebViewClient(new android.webkit.WebViewClient());
                java.lang.reflect.Field auth=MainActivity.class.getDeclaredField("liteSession");auth.setAccessible(true);auth.set(a,new LiteSession("portrait-fixture:office",System.currentTimeMillis()+60000));
                java.lang.reflect.Method show=MainActivity.class.getDeclaredMethod("showAgent",boolean.class);show.setAccessible(true);show.invoke(a,true);face.set(a.findViewById(R.id.agent_panel));
            }catch(Exception e){throw new AssertionError(e);}});
            WebView v=face.get();boolean ready=false;
            for(int i=0;i<60;i++){if("true".equals(js(s,v,"document.body.innerText.includes('私の顔を押したり、長押ししてね')"))){ready=true;break;}Thread.sleep(100);}
            assertTrue(ready);
            s.onActivity(a->{int[] tab=new int[2],panel=new int[2];a.findViewById(R.id.tab_work).getLocationOnScreen(tab);a.findViewById(R.id.agent_panel).getLocationOnScreen(panel);assertTrue(tab[1]<panel[1]);});
            assertEquals("true",js(s,v,"(()=>{const m=document.querySelector('[aria-label=\"マイクで話す\"]');return !!m&&Number(getComputedStyle(m).fontWeight)>=700&&m.getBoundingClientRect().bottom<=innerHeight})()"));
            Thread.sleep(300);capture("v031-home.png");
            js(s,v,"document.querySelector('[aria-label=\"顔を押すと会話、長押しで作業を選ぶ\"]').dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true}))");
            Thread.sleep(250);
            assertEquals("true",js(s,v,"document.body.innerText.includes('作業を選ぶ')&&!document.body.innerText.includes('会話と作業')&&!document.body.innerText.includes('音声・AI設定')"));
            js(s,v,"Array.from(document.querySelectorAll('button')).find(b=>b.textContent.includes('搬入入力')).click()");
            Thread.sleep(700);
            assertEquals("true",js(s,v,"document.body.innerText.includes('シカ')&&document.body.innerText.includes('初期値')&&document.body.innerText.includes('ホームに戻る')"));
            assertEquals("true",js(s,v,"(()=>{const c=document.querySelector('[aria-label=\"下書きの内容\"]');const m=document.querySelector('[aria-label=\"マイクで話す\"]');return c.getBoundingClientRect().bottom<m.getBoundingClientRect().top&&m.getBoundingClientRect().bottom<=innerHeight})()"));
            capture("v031-defaults.png");
        }
    }
}
