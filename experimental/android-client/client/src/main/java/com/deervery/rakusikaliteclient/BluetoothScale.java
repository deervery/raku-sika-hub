package com.deervery.rakusikaliteclient;
import android.bluetooth.*;
import android.os.SystemClock;
import org.json.JSONObject;
import java.io.*;
import java.util.UUID;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.locks.ReentrantLock;

/** Explicitly paired SPP serial adapters only. RS-232 side must be configured to 2400/7E1. */
final class BluetoothScale {
 private volatile BluetoothSocket socket;
 private final AtomicInteger generation=new AtomicInteger();
 private final ReentrantLock lock=new ReentrantLock();
 private final ScheduledExecutorService timer=Executors.newSingleThreadScheduledExecutor();
 JSONObject weigh(BluetoothDevice device){
  if(!lock.tryLock())return UsbScale.error("BUSY","計量処理中です。");
  int token=generation.get();BluetoothSocket current=null;ScheduledFuture<?> deadline=null;
  try{
   if(device==null||device.getBondState()!=BluetoothDevice.BOND_BONDED)throw new IOException("ペアリング済みの計量アダプタを選んでください。");
   current=device.createRfcommSocketToServiceRecord(UUID.fromString("00001101-0000-1000-8000-00805F9B34FB"));socket=current;
   final BluetoothSocket active=current;
   deadline=timer.schedule(()->{try{active.close();}catch(IOException ignored){}},10,TimeUnit.SECONDS);
   current.connect();InputStream in=current.getInputStream();OutputStream out=current.getOutputStream();
   ScaleProtocol.Reading r=ScaleExchange.run(new ScaleExchange.Wire(){
    public int read(byte[] bytes,int timeout)throws IOException{long end=SystemClock.elapsedRealtime()+timeout;while(SystemClock.elapsedRealtime()<end){if(token!=generation.get())throw new IOException("接続が変わりました。");int n=in.available();if(n>0)return in.read(bytes,0,Math.min(bytes.length,n));SystemClock.sleep(8);}return 0;}
    public void write(byte[] bytes,int timeout)throws IOException{out.write(bytes);out.flush();}
   },"Q",SystemClock::elapsedRealtime,()->token==generation.get());
   if(token!=generation.get())throw new IOException("接続が変わりました。");
   return new JSONObject().put("status",r.stable?"ok":"weighing").put("stable",r.stable).put("value",r.value).put("unit",r.unit);
  }catch(SecurityException e){return UsbScale.error("BLUETOOTH_PERMISSION","近くのデバイスへの接続を設定で許可してください。");}
  catch(Exception e){return UsbScale.error("BLUETOOTH_SCALE",e.getMessage()==null?"Bluetooth計量器に接続できません。":e.getMessage());}
  finally{if(deadline!=null)deadline.cancel(false);if(current!=null)try{current.close();}catch(IOException ignored){}socket=null;lock.unlock();}
 }
 void disconnect(){generation.incrementAndGet();BluetoothSocket current=socket;if(current!=null)try{current.close();}catch(IOException ignored){}}
 void destroy(){disconnect();timer.shutdownNow();}
}
