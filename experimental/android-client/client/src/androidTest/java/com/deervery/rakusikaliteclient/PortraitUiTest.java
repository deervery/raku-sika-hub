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
                java.lang.reflect.Field auth=MainActivity.class.getDeclaredField("liteSession");auth.setAccessible(true);auth.set(a,new LiteSession("portrait-fixture:office-"+System.nanoTime(),System.currentTimeMillis()+60000));
                java.lang.reflect.Method show=MainActivity.class.getDeclaredMethod("showAgent",boolean.class);show.setAccessible(true);show.invoke(a,true);face.set(a.findViewById(R.id.agent_panel));
            }catch(Exception e){throw new AssertionError(e);}});
            WebView v=face.get();boolean ready=false;
            for(int i=0;i<60;i++){if("true".equals(js(s,v,"document.body.innerText.includes('私の顔を押したり、長押ししてね')"))){ready=true;break;}Thread.sleep(100);}
            assertTrue(ready);
            s.onActivity(a->{int[] tab=new int[2],panel=new int[2];a.findViewById(R.id.tab_work).getLocationOnScreen(tab);a.findViewById(R.id.agent_panel).getLocationOnScreen(panel);assertTrue(tab[1]<panel[1]);});
            assertEquals("true",js(s,v,"(()=>{const m=document.querySelector('[aria-label=\"顔を押すと会話、長押しで作業を選ぶ\"]');return !!m&&Number(getComputedStyle(m).fontWeight)>=700&&m.getBoundingClientRect().bottom<=innerHeight})()"));
            Thread.sleep(300);capture("v034-home.png");
            js(s,v,"document.querySelector('[aria-label=\"顔を押すと会話、長押しで作業を選ぶ\"]').dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true}))");
            Thread.sleep(250);
            assertEquals("true",js(s,v,"document.body.innerText.includes('作業を選ぶ')&&!document.body.innerText.includes('会話と作業')&&!document.body.innerText.includes('音声・AI設定')"));
            js(s,v,"Array.from(document.querySelectorAll('button')).find(b=>b.textContent.includes('搬入入力')).click()");
            Thread.sleep(700);
            assertEquals("true",js(s,v,"document.body.innerText.includes('シカ')&&document.body.innerText.includes('初期値')&&document.body.innerText.includes('推定体重')&&document.body.innerText.includes('異常なし')&&document.body.innerText.includes('ホームに戻る')"));
            assertEquals("true",js(s,v,"(()=>{const c=document.querySelector('[aria-label=\"下書きの内容\"]');const m=document.querySelector('[aria-label=\"顔を押すと会話、長押しで作業を選ぶ\"]');return c.getBoundingClientRect().bottom<m.getBoundingClientRect().top&&m.getBoundingClientRect().bottom<=innerHeight})()"));
            capture("v034-defaults.png");
            js(s,v,"Array.from(document.querySelectorAll('[aria-label=\"下書きの内容\"] button')).find(b=>b.textContent.includes('異常の確認')).click()");Thread.sleep(200);
            assertEquals("true",js(s,v,"!!document.querySelector('[aria-label=\"異常項目を選ぶ\"]')"));
            js(s,v,"Array.from(document.querySelectorAll('[role=dialog] button')).find(b=>b.textContent==='異常あり').click()");Thread.sleep(200);
            js(s,v,"document.querySelector('[aria-label=\"編集を閉じる\"]').click()");Thread.sleep(200);
            assertEquals("true",js(s,v,"document.body.innerText.includes('異常あり 1項目')"));
            js(s,v,"document.querySelector('[aria-label=\"顔を押すと会話、長押しで作業を選ぶ\"]').dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true}))");Thread.sleep(200);
            assertEquals("true",js(s,v,"!Array.from(document.querySelectorAll('button')).some(b=>['作業を選ぶ','搬入入力へ'].includes(b.textContent.trim()))"));
            js(s,v,"Array.from(document.querySelectorAll('button')).find(b=>b.textContent.trim()==='ラベル印刷').click()");Thread.sleep(200);
            js(s,v,"Array.from(document.querySelectorAll('button')).find(b=>['個体ラベル','精肉・個体番号あり','加工品'].includes(b.textContent.trim())).click()");Thread.sleep(150);
            js(s,v,"Array.from(document.querySelectorAll('[role=dialog] button')).find(b=>b.textContent.trim()==='精肉・個体番号あり').click()");Thread.sleep(200);
            js(s,v,"Array.from(document.querySelectorAll('[aria-label=\"ラベルの印刷内容\"] button')).find(b=>b.textContent.includes('内容量')).click()");Thread.sleep(150);
            assertEquals("true",js(s,v,"!!document.querySelector('select[aria-label=\"内容量の単位\"]')"));
            js(s,v,"(()=>{const select=document.querySelector('select');select.value='g';select.dispatchEvent(new Event('change',{bubbles:true}));})()");
            js(s,v,"Array.from(document.querySelectorAll('[role=dialog] button')).find(b=>b.textContent==='7').click()");
            js(s,v,"Array.from(document.querySelectorAll('[role=dialog] button')).find(b=>b.textContent==='0').click()");
            Thread.sleep(500);capture("v034-quantity.png");
            js(s,v,"document.querySelector('[aria-label=\"編集を閉じる\"]').click()");Thread.sleep(150);
            assertEquals("true",js(s,v,"document.querySelector('[aria-label=\"ラベルの印刷内容\"]').textContent.includes('70g')"));
            js(s,v,"(()=>{const b=Array.from(document.querySelectorAll('button')).find(b=>b.textContent.trim()==='次へ');if(b)b.click();})()");Thread.sleep(200);
            js(s,v,"Array.from(document.querySelectorAll('[aria-label=\"ラベルの印刷内容\"] button')).find(b=>b.textContent.includes('期限日')).click()");Thread.sleep(150);
            js(s,v,"(()=>{for(const [label,value] of [['年','2028'],['月','02'],['日','29']]){const select=document.querySelector('select[aria-label=\"'+label+'\"]');select.value=value;select.dispatchEvent(new Event('change',{bubbles:true}));}})()");Thread.sleep(150);
            Thread.sleep(500);capture("v034-date.png");
            js(s,v,"document.querySelector('[aria-label=\"編集を閉じる\"]').click()");Thread.sleep(150);
            assertEquals("true",js(s,v,"document.body.innerText.includes('2028-02-29')"));
            js(s,v,"Array.from(document.querySelectorAll('button')).find(b=>b.textContent.trim()==='個体').click()");Thread.sleep(200);
            assertEquals("true",js(s,v,"document.body.innerText.includes('個体識別番号')&&!document.body.innerText.includes('施設・確認')"));
            js(s,v,"(()=>{const b=Array.from(document.querySelectorAll('button')).find(b=>b.textContent.trim()==='次へ');if(b)b.click();})()");Thread.sleep(200);
            assertEquals("true",js(s,v,"document.body.innerText.includes('搬入日')&&document.body.innerText.includes('捕獲場所')"));
            assertEquals("true",js(s,v,"(()=>{const cards=document.querySelector('[aria-label=\"ラベルの印刷内容\"]');const mic=document.querySelector('[aria-label=\"顔を押すと会話、長押しで作業を選ぶ\"]');return cards.getBoundingClientRect().bottom<mic.getBoundingClientRect().top&&mic.getBoundingClientRect().bottom<=innerHeight})()"));
            Thread.sleep(600);capture("v034-labels.png");
            System.out.println("label bounds: "+js(s,v,"(()=>{const c=document.querySelector('[aria-label=\"ラベルの印刷内容\"]');return JSON.stringify([c,c.nextElementSibling,...c.children].map(e=>[e.getBoundingClientRect().top,e.getBoundingClientRect().bottom,e.scrollHeight,e.clientHeight]))})()"));
            assertEquals("true",js(s,v,"(()=>{const cards=document.querySelector('[aria-label=\"ラベルの印刷内容\"]');const footer=cards.nextElementSibling;return cards.getBoundingClientRect().bottom<=footer.getBoundingClientRect().top&&Array.from(cards.children).every(child=>child.getBoundingClientRect().bottom<=cards.getBoundingClientRect().bottom+1)})()"));
            Thread.sleep(500);capture("v034-labels.png");
            js(s,v,"Array.from(document.querySelectorAll('[role=tab]')).find(b=>b.textContent==='基本').click()");Thread.sleep(600);
            assertEquals("true",js(s,v,"document.querySelectorAll('[aria-label=\"ラベルの印刷内容\"] button').length===5&&!Array.from(document.querySelectorAll('button')).some(b=>b.textContent==='次へ')"));
            capture("v034-basic.png");
            js(s,v,"Array.from(document.querySelectorAll('[aria-label=\"ラベルの印刷内容\"] button')).find(b=>b.textContent.includes('製品名')).click()");Thread.sleep(350);
            js(s,v,"document.querySelector('[role=dialog] input').focus()");
            s.onActivity(a->{v.requestFocus();((android.view.inputmethod.InputMethodManager)a.getSystemService(android.content.Context.INPUT_METHOD_SERVICE)).showSoftInput(v,android.view.inputmethod.InputMethodManager.SHOW_IMPLICIT);});
            boolean keyboard=false;
            for(int i=0;i<40;i++){AtomicBoolean shown=new AtomicBoolean();s.onActivity(a->{if(android.os.Build.VERSION.SDK_INT>=30)shown.set(a.getWindow().getDecorView().getRootWindowInsets().isVisible(android.view.WindowInsets.Type.ime()));});if(shown.get()){keyboard=true;break;}Thread.sleep(100);}
            assertTrue("Keyboard must actually be shown for overlap check",keyboard);Thread.sleep(400);
            assertEquals("true",js(s,v,"(()=>{const d=document.querySelector('[role=dialog]'),i=d.querySelector('input'),vp=visualViewport;return d.getBoundingClientRect().bottom<=vp.offsetTop+vp.height+1&&i.getBoundingClientRect().bottom<=vp.offsetTop+vp.height})()"));
            capture("v034-keyboard.png");
            InstrumentationRegistry.getInstrumentation().sendKeyDownUpSync(android.view.KeyEvent.KEYCODE_BACK);Thread.sleep(350);
            js(s,v,"window.stationBack()");Thread.sleep(250);
            s.onActivity(a->assertEquals(android.view.View.VISIBLE,a.findViewById(R.id.agent_panel).getVisibility()));
            assertEquals("true",js(s,v,"!document.querySelector('[role=dialog]')"));

        }
    }
}
