package com.deervery.rakusikaliteclient;

import android.content.Context;
import android.hardware.usb.*;
import android.os.SystemClock;
import com.hoho.android.usbserial.driver.*;
import java.io.IOException;
import java.util.concurrent.*;
import java.util.concurrent.atomic.AtomicInteger;
import java.util.concurrent.locks.ReentrantLock;
import org.json.JSONObject;

/** Only the explicitly selected FTDI device is opened. No automatic Hub fallback. */
final class UsbScale {
    private final UsbManager manager;
    private final ExecutorService worker=Executors.newSingleThreadExecutor();
    private final ReentrantLock lock=new ReentrantLock();
    private final AtomicInteger generation=new AtomicInteger();
    private volatile boolean connected,opening;
    private volatile String status="USB計量器は未接続です";
    private UsbSerialPort port;
    private UsbDeviceConnection connection;
    UsbScale(Context context){manager=(UsbManager)context.getSystemService(Context.USB_SERVICE);}
    static boolean supported(UsbDevice device){return device!=null&&device.getVendorId()==0x0403&&device.getProductId()==0x6015;}
    boolean connected(){return connected;}
    boolean opening(){return opening;}
    String status(){return status;}
    void open(UsbDevice device,Runnable callback){
        if(opening||connected)return;
        int token=generation.incrementAndGet();opening=true;
        worker.execute(()->{
            lock.lock();
            try{
                closePort();
                if(!supported(device)||!manager.hasPermission(device))throw new IOException("対象USB機器の接続許可が必要です");
                UsbSerialDriver driver=UsbSerialProber.getDefaultProber().probeDevice(device);
                if(driver==null)throw new IOException("対応ドライバーがありません");
                connection=manager.openDevice(device);
                if(connection==null)throw new IOException("USBを開けません。接続を確認してください");
                port=driver.getPorts().get(0);port.open(connection);
                port.setParameters(2400,7,UsbSerialPort.STOPBITS_1,UsbSerialPort.PARITY_EVEN);
                if(token!=generation.get()){closePort();return;}
                connected=true;status="USB接続済み · 2400/7E1 · 計量応答は未確認";
            }catch(Exception e){closePort();status="USB接続失敗："+e.getMessage();}
            finally{opening=false;lock.unlock();callback.run();}
        });
    }
    JSONObject command(String command){
        if(!lock.tryLock())return error("SCALE_BUSY","別の計量処理が実行中です。");
        int token=generation.get();
        try{
            if(!connected||port==null)return error("SCALE_NOT_CONNECTED",status);
            final UsbSerialPort current=port;
            ScaleProtocol.Reading reading=ScaleExchange.run(new ScaleExchange.Wire(){
                public int read(byte[] bytes,int timeout)throws IOException{return current.read(bytes,timeout);}
                public void write(byte[] bytes,int timeout)throws IOException{current.write(bytes,timeout);}
            },command,SystemClock::elapsedRealtime,()->connected&&token==generation.get());
            if(token!=generation.get()||!connected)throw new IOException("接続が変わったため結果を破棄しました。");
            if(reading==null){status="USB操作の応答を確認しました";return new JSONObject().put("status","ok");}
            status=reading.value.toPlainString()+" "+reading.unit+(reading.stable?"（安定）":"（不安定・未確定）");
            return new JSONObject().put("status",reading.stable?"ok":"weighing").put("value",reading.value)
                .put("unit",reading.unit).put("stable",reading.stable);
        }catch(Exception e){closePort();status=e.getMessage();return error("SCALE_USB_ERROR",status);}
        finally{lock.unlock();}
    }
    static JSONObject error(String code,String message){
        try{return new JSONObject().put("status","error").put("code",code).put("message",message);}
        catch(org.json.JSONException impossible){throw new IllegalStateException(impossible);}
    }
    void disconnect(){
        generation.incrementAndGet();connected=false;status="USB未接続・前の値は無効です";
        worker.execute(()->{lock.lock();try{closePort();}finally{lock.unlock();}});
    }
    void destroy(){disconnect();worker.shutdown();}
    private void closePort(){
        connected=false;
        if(port!=null)try{port.close();}catch(IOException ignored){}
        if(connection!=null)connection.close();port=null;connection=null;
    }
}
