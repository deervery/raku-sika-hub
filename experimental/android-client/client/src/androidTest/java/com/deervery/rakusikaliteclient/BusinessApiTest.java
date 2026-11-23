package com.deervery.rakusikaliteclient;
import org.junit.Test;
import org.json.JSONObject;
import static org.junit.Assert.*;
public class BusinessApiTest {
 @Test public void fixedRoutesRejectPathsAndUnknownOperations()throws Exception{
  assertEquals("/api/meat-lots/durable",BusinessApi.path("meat",""));
  assertEquals("/api/periodic-records/daily/durable",BusinessApi.path("daily",""));
  assertEquals("/api/meat-lots/lot-1/status/durable",BusinessApi.path("outbound","lot-1"));
  for(String invalid:new String[]{"../other","x/y","x?role=admin","https://example.test"})try{BusinessApi.path("outbound",invalid);fail();}catch(Exception expected){}
  try{BusinessApi.path("settings","facility");fail();}catch(Exception expected){}
 }
 @Test public void confirmationAndFacilityCannotBeBypassed()throws Exception{
  JSONObject args=new JSONObject().put("kind","meat").put("requestId","12345678-1234-1234").put("data",new JSONObject());
  try{BusinessApi.execute("https://never.invalid","","facility",args);fail();}catch(Exception e){assertTrue(e.getMessage().contains("確認"));}
  args.put("confirmed",true).getJSONObject("data").put("facilityId","another");
  try{BusinessApi.execute("https://never.invalid","","facility",args);fail();}catch(Exception e){assertTrue(e.getMessage().contains("項目"));}
 }
 @Test public void unknownPrintReceiptSurvivesRestartWithoutResending()throws Exception{
  android.content.SharedPreferences store=androidx.test.platform.app.InstrumentationRegistry.getInstrumentation().getTargetContext().getSharedPreferences("print-fixture",0);
  String id="12345678-1234-1234",key="print-receipt:fixture:"+id;
  store.edit().putString(key,"{\"state\":\"unknown\",\"message\":\"verify output\"}").commit();
  try{assertEquals("unknown",HubPrint.execute(store,"fixture","https://never.invalid",new JSONObject().put("requestId",id),new JSONObject()).getString("state"));}finally{store.edit().remove(key).commit();}
 }
 @Test public void identitySnapshotDoesNotDependOnUiCookie(){
  assertFalse(LiteSession.snapshotScript(1).contains("__session_exists"));
  assertTrue(LiteSession.snapshotScript(1).contains("accessToken"));
  assertFalse(LiteSession.snapshotScript(1).contains("refreshToken"));
 }
}
