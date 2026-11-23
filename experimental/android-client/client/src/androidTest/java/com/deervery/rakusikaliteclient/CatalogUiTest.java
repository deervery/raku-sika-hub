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
 private void capture(String name)throws Exception{
  android.graphics.Bitmap bitmap=androidx.test.platform.app.InstrumentationRegistry.getInstrumentation().getUiAutomation().takeScreenshot();
  java.io.File file=new java.io.File(androidx.test.platform.app.InstrumentationRegistry.getInstrumentation().getTargetContext().getExternalFilesDir("screenshots"),name);
  try(java.io.FileOutputStream out=new java.io.FileOutputStream(file)){bitmap.compress(android.graphics.Bitmap.CompressFormat.PNG,100,out);}bitmap.recycle();
 }
 @Test public void selectionHydratesCardsAndCameraUsesSameLookup()throws Exception{
  AtomicReference<FaceAgentView> holder=new AtomicReference<>();AtomicInteger reads=new AtomicInteger();AtomicReference<String> destination=new AtomicReference<>("");
  try(ActivityScenario<MainActivity> s=ActivityScenario.launch(MainActivity.class)){
   s.onActivity(a->{FaceAgentView v=new FaceAgentView(a,new FaceAgentView.Host(){
    public void microphonePermission(){}public void openWork(String p){destination.set(p);}public void weigh(java.util.function.Consumer<JSONObject> cb){try{cb.accept(new JSONObject("{\"status\":\"ok\",\"stable\":true,\"value\":0.07,\"unit\":\"kg\"}"));}catch(Exception e){throw new AssertionError(e);}}
    public JSONObject session(){try{return new JSONObject().put("scope","catalog-fixture:"+hashCode());}catch(Exception e){throw new AssertionError(e);}}
    public void camera(java.util.function.Consumer<JSONObject> cb){try{cb.accept(new JSONObject().put("value","https://rakusika.com/t/fixture-individual"));}catch(Exception e){throw new AssertionError(e);}}
    public void catalog(JSONObject args,java.util.function.Consumer<JSONObject> cb){reads.incrementAndGet();try{
     JSONObject row=new JSONObject("{\"id\":\"fixture-individual\",\"individualNumber\":\"0001\",\"species\":\"シカ\",\"sex\":\"オス\",\"captureLocation\":\"検証場所\",\"receivingDate\":\"2026-11-23\"}");
     if(args.optString("kind").equals("products"))row=new JSONObject("{\"id\":\"fixture-product\",\"name\":\"ロース\",\"defaultContentQuantity\":70,\"unit\":\"g\"}");
     JSONArray candidates=new JSONArray().put(row);
     if(args.optString("kind").equals("products"))for(int i=1;i<12;i++)candidates.put(new JSONObject().put("id","product-"+i).put("name","候補"+i).put("isTraceable",true));
     cb.accept(args.optString("kind").equals("individual")||args.optString("kind").equals("trace")?new JSONObject().put("item",row):new JSONObject().put("items",candidates).put("cursor",""));
    }catch(Exception e){throw new AssertionError(e);}}
   });holder.set(v);a.setContentView(v);v.activate();});
   FaceAgentView v=holder.get();waitFor(s,v,"!!document.querySelector('[aria-label=\"マイクをオン・オフ\"]')");
   js(s,v,"(()=>{const b=Array.from(document.querySelectorAll('button')).find(b=>b.textContent==='ホーム');if(b)b.click();})()");Thread.sleep(200);
   assertEquals("false",js(s,v,"document.body.innerText.includes('機器状態')"));
   js(s,v,"Array.from(document.querySelectorAll('button')).find(b=>b.textContent.trim()==='ラベル印刷').click()");Thread.sleep(300);
   assertEquals("true",js(s,v,"document.body.innerText.includes('精肉・個体番号あり')"));
   js(s,v,"Array.from(document.querySelectorAll('[aria-label=\"ラベルの印刷内容\"] button')).find(b=>b.textContent.includes('製品名')).click()");
   waitFor(s,v,"!!Array.from(document.querySelectorAll('[role=dialog] button')).find(b=>b.textContent==='ロース')");
   assertEquals("true",js(s,v,"Array.from(document.querySelectorAll('[role=dialog] button')).filter(b=>b.textContent.startsWith('候補')).length===11"));
   js(s,v,"Array.from(document.querySelectorAll('[role=dialog] button')).find(b=>b.textContent==='ロース').click()");Thread.sleep(200);
   assertEquals("true",js(s,v,"document.body.innerText.includes('ロース')&&document.body.innerText.includes('70g')&&!document.querySelector('[role=dialog]')"));
   js(s,v,"document.querySelector('[aria-label=\"カメラスキャン\"]').click()");
   waitFor(s,v,"document.body.innerText.includes('0001')&&document.body.innerText.includes('シカ')");
   assertTrue(reads.get()>=2);
   String before=js(s,v,"document.querySelector('[aria-label=\"ラベルの印刷内容\"]').textContent");
   js(s,v,"(()=>{const c=document.querySelector('[aria-label=\"ラベルの印刷内容\"]');for(const [type,x] of [['touchstart',50],['touchend',300]]){const e=new Event(type,{bubbles:true});Object.defineProperty(e,type==='touchstart'?'touches':'changedTouches',{value:[{clientX:x,clientY:200}]});c.dispatchEvent(e);}})()");Thread.sleep(500);
   assertNotEquals(before,js(s,v,"document.querySelector('[aria-label=\"ラベルの印刷内容\"]').textContent"));
   js(s,v,"(()=>{const b=Array.from(document.querySelectorAll('button')).find(b=>b.textContent==='ホーム');if(b)b.click();})()");Thread.sleep(200);
   assertEquals("true",js(s,v,"document.querySelectorAll('[data-operation=menu] .not-a-class').length===0&&Array.from(document.querySelectorAll('button')).filter(b=>b.textContent==='スキャン').length===1"));
   js(s,v,"Array.from(document.querySelectorAll('button')).find(b=>b.textContent==='在庫登録').click()");Thread.sleep(200);
   assertEquals("true",js(s,v,"['精肉','加工品','ペットフード'].every(t=>Array.from(document.querySelectorAll('button')).some(b=>b.textContent===t))"));
   js(s,v,"Array.from(document.querySelectorAll('button')).find(b=>b.textContent==='精肉').click()");Thread.sleep(300);
   assertEquals("true",js(s,v,"document.body.innerText.includes('計量器から重量を取得')"));assertEquals("",destination.get());capture("v035-inventory.png");
   js(s,v,"Array.from(document.querySelectorAll('button')).find(b=>b.textContent==='戻る').click()");Thread.sleep(200);
   js(s,v,"Array.from(document.querySelectorAll('button')).find(b=>b.textContent==='ホーム').click()");Thread.sleep(200);
   js(s,v,"Array.from(document.querySelectorAll('button')).find(b=>b.textContent==='定期報告').click()");Thread.sleep(200);
   js(s,v,"Array.from(document.querySelectorAll('button')).find(b=>b.textContent==='週報').click()");Thread.sleep(300);
   assertEquals("true",js(s,v,"document.body.innerText.includes('対象週')&&document.body.innerText.includes('点検日')"));assertEquals("",destination.get());
   capture("v035-weekly.png");
   s.onActivity(a->v.destroy());
  }
 }
}
