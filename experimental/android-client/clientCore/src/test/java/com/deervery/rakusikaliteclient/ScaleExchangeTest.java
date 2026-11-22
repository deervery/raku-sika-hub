package com.deervery.rakusikaliteclient;
import org.junit.Test;
import static org.junit.Assert.*;
import java.io.IOException;
import java.nio.charset.StandardCharsets;
import java.util.ArrayDeque;
import java.util.concurrent.atomic.AtomicBoolean;

public class ScaleExchangeTest {
    static class Wire implements ScaleExchange.Wire {
        long now;int writes;boolean sent;String stale="ST,+999.99 kg\r\n",written;
        ArrayDeque<String> responses=new ArrayDeque<>();Runnable onRead=()->{};
        Wire(String... lines){for(String line:lines)responses.add(line);}
        public int read(byte[] buffer,int timeout){
            now+=timeout;String data;
            if(!sent){data=stale;stale="";}else {data=responses.isEmpty()?"":responses.remove();onRead.run();}
            byte[] bytes=data.getBytes(StandardCharsets.US_ASCII);System.arraycopy(bytes,0,buffer,0,bytes.length);return bytes.length;
        }
        public void write(byte[] data,int timeout){sent=true;writes++;written=new String(data,StandardCharsets.US_ASCII);}
        ScaleProtocol.Reading run(String command)throws IOException{return ScaleExchange.run(this,command,()->now,()->true);}
    }
    @Test public void drainsOldWeightAndReassemblesFreshReading()throws Exception{
        Wire wire=new Wire("ST,+0000","0.07 kg\r","\n");
        ScaleProtocol.Reading r=wire.run("Q");assertEquals("0.07",r.value.toPlainString());assertTrue(r.stable);assertEquals("Q\r\n",wire.written);
    }
    @Test public void unstableIsNotConfirmed()throws Exception{assertFalse(new Wire("US,+0.07 kg\r\n").run("Q").stable);}
    @Test public void weightDoesNotAcknowledgeTare()throws Exception{
        Wire wire=new Wire("ST,+0.00 kg\r\n");
        assertThrows(IOException.class,()->wire.run("T"));assertEquals(1,wire.writes);
    }
    @Test public void tareAndZeroRequireMatchingAcknowledgement()throws Exception{
        assertNull(new Wire("ZR\r\n","QT\r\n").run("T"));
        assertNull(new Wire("TA\r\n","ZR\r\n").run("R"));
    }
    @Test public void detachDuringReadDiscardsResult(){
        Wire wire=new Wire("ST,+0.07 kg\r\n");AtomicBoolean connected=new AtomicBoolean(true);wire.onRead=()->connected.set(false);
        assertThrows(IOException.class,()->ScaleExchange.run(wire,"Q",()->wire.now,connected::get));
    }
    @Test public void overloadAndBrokenDataNeverBecomeZero(){
        assertThrows(IllegalArgumentException.class,()->new Wire("OL,+999.99 kg\r\n").run("Q"));
        assertThrows(IllegalArgumentException.class,()->new Wire("ST,not-a-weight kg\r\n").run("Q"));
        assertThrows(IllegalArgumentException.class,()->new Wire("\u0000\r\n").run("Q"));
    }
    @Test public void oversizedFrameFailsClosed(){assertThrows(IllegalArgumentException.class,()->new Wire(new String(new char[130]).replace('\0','1')+"\n").run("Q"));}
    @Test public void preservesNegativeAndGramUnits()throws Exception{
        ScaleProtocol.Reading r=new Wire("ST,-001.20 g\r\n").run("Q");assertEquals("-1.20",r.value.toPlainString());assertEquals("g",r.unit);
    }
}
