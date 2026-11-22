package com.deervery.rakusikaliteclient;
import androidx.test.core.app.ActivityScenario;
import android.webkit.WebView;
import org.json.*;
import org.junit.Test;
import java.util.concurrent.*;
import java.util.concurrent.atomic.*;
import static org.junit.Assert.*;
/** Fixture catalog only. Exercises the packaged UI and native bridge without production writes. */
public class CatalogUiTest {
 private String js(ActivityScenario<MainActivity> s,WebView v,String source)throws Exception{AtomicReference<String> value=new AtomicReference<>();CountDownLatch latch=new CountDownLatch(1);s.onActivity(a->v.evaluateJavascript(source,r->{value.set(r);latch.countDown();}));assertTrue(latch.await(5,TimeUnit.SECONDS));return value.get();}
 private void waitFor(ActivityScenario<MainActivity> s,WebView v,String source)throws Exception{for(int i=0;i<50;i++){if("true".equals(js(s,v,source)))return;Thread.sleep(100);}fail(source);}
 @Test public void selectionHydratesCardsAndCameraUsesSameLookup()throws Exception{
  AtomicReference<FaceAgentView> holder=new AtomicReference<>();AtomicInteger reads=new AtomicInteger();
  try(ActivityScenario<MainActivity> s=ActivityScenario.launch(MainActivity.class)){
   s.onActivity(a->{FaceAgentView v=new FaceAgentView(a,new FaceAgentView.Host(){
    public void microphonePermission(){}public void openWork(String p){}public void weigh(java.util.function.Consumer<JSONObject> cb){}
    public JSONObject session(){try{return new JSONObject().put("scope","catalog-fixture:"+hashCode());}catch(Exception e){throw new AssertionError(e);}}
    public void camera(java.util.function.Consumer<JSONObject> cb){try{cb.accept(new JSONObject().put("value","https://rakusika.com/t/fixture-individual"));}catch(Exception e){throw new AssertionError(e);}}
    public void catalog(JSONObject args,java.util.function.Consumer<JSONObject> cb){reads.incrementAndGet();try{
     JSONObject row=new JSONObject("{\"id\":\"fixture-individual\",\"individualNumber\":\"0001\",\"species\":\"シカ\",\"sex\":\"オス\",\"captureLocation\":\"検証場所\",\"receivingDate\":\"2026-11-23\"}");
     if(args.optString("kind").equals("products"))row=new JSONObject("{\"id\":\"fixture-product\",\"name\":\"ロース\",\"defaultContentQuantity\":70,\"unit\":\"g\"}");
     cb.accept(args.optString("kind").equals("individual")||args.optString("kind").equals("trace")?new JSONObject().put("item",row):new JSONObject().put("items",new JSONArray().put(row)).put("cursor",""));
    }catch(Exception e){throw new AssertionError(e);}}
   });holder.set(v);a.setContentView(v);v.activate();});
   FaceAgentView v=holder.get();waitFor(s,v,"!!document.querySelector('[aria-label=\"顔を押すと会話、長押しで作業を選ぶ\"]')");
   js(s,v,"document.querySelector('[aria-label=\"顔を押すと会話、長押しで作業を選ぶ\"]').dispatchEvent(new MouseEvent('contextmenu',{bubbles:true,cancelable:true}))");Thread.sleep(200);
   assertEquals("false",js(s,v,"document.body.innerText.includes('機器状態')"));
   js(s,v,"Array.from(document.querySelectorAll('button')).find(b=>b.textContent.trim()==='ラベル印刷').click()");Thread.sleep(300);
   assertEquals("true",js(s,v,"document.body.innerText.includes('精肉・個体番号あり')"));
   js(s,v,"Array.from(document.querySelectorAll('[aria-label=\"ラベルの印刷内容\"] button')).find(b=>b.textContent.includes('製品名')).click()");
   waitFor(s,v,"!!Array.from(document.querySelectorAll('[role=dialog] button')).find(b=>b.textContent==='ロース')");
   js(s,v,"Array.from(document.querySelectorAll('[role=dialog] button')).find(b=>b.textContent==='ロース').click()");Thread.sleep(200);
   assertEquals("true",js(s,v,"document.body.innerText.includes('ロース')&&document.body.innerText.includes('70g')&&!document.querySelector('[role=dialog]')"));
   js(s,v,"document.querySelector('[aria-label=\"カメラスキャン\"]').click()");
   waitFor(s,v,"document.body.innerText.includes('0001')&&document.body.innerText.includes('シカ')");
   assertTrue(reads.get()>=2);
   String before=js(s,v,"document.querySelector('[aria-label=\"ラベルの印刷内容\"]').textContent");
   js(s,v,"(()=>{const c=document.querySelector('[aria-label=\"ラベルの印刷内容\"]');for(const [type,x] of [['touchstart',300],['touchend',50]]){const e=new Event(type,{bubbles:true});Object.defineProperty(e,type==='touchstart'?'touches':'changedTouches',{value:[{clientX:x,clientY:200}]});c.dispatchEvent(e);}})()");Thread.sleep(500);
   assertNotEquals(before,js(s,v,"document.querySelector('[aria-label=\"ラベルの印刷内容\"]').textContent"));
   s.onActivity(a->v.destroy());
  }
 }
}
