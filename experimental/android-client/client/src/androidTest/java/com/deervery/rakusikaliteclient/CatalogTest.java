package com.deervery.rakusikaliteclient;
import org.junit.Test;
import org.json.*;
import static org.junit.Assert.*;
import com.google.zxing.*;
import com.google.zxing.common.*;
public class CatalogTest {
 @Test public void catalogProjectionExcludesPrivateFields()throws Exception{
  JSONObject row=CatalogApi.project(new JSONObject().put("id","fixture").put("individualNumber","0001").put("token","secret").put("address","private").put("facilityId","office"));
  assertEquals("0001",row.getString("individualNumber"));assertFalse(row.has("token"));assertFalse(row.has("address"));assertFalse(row.has("facilityId"));
  try{CatalogApi.id("../secret");fail();}catch(Exception expected){}
  assertEquals("%E3%83%AD%E3%83%BC%E3%82%B9",CatalogApi.encode("ロース"));
 }
 @Test public void qrAndBarcodeDecodeWithoutImageStorage()throws Exception{
  for(BarcodeFormat format:new BarcodeFormat[]{BarcodeFormat.QR_CODE,BarcodeFormat.CODE_128}){
   String value=format==BarcodeFormat.QR_CODE?"https://rakusika.com/t/fixture":"000123";
   BitMatrix matrix=new MultiFormatWriter().encode(value,format,480,320);int[] pixels=new int[480*320];
   for(int y=0;y<320;y++)for(int x=0;x<480;x++)pixels[y*480+x]=matrix.get(x,y)?0xff000000:0xffffffff;
   Result result=new MultiFormatReader().decode(new BinaryBitmap(new HybridBinarizer(new RGBLuminanceSource(480,320,pixels))));assertEquals(value,result.getText());
  }
 }
}
