package com.deervery.rakusikaliteclient;
import org.junit.Test;
import static org.junit.Assert.*;
public class ScannerBufferTest {
    @Test public void oneCompleteScanThenNoReplay(){ScannerBuffer b=new ScannerBuffer();b.accept('0',false,10);b.accept('7',false,20);assertEquals("07",b.accept(0,true,30));assertNull(b.accept(0,true,40));}
    @Test public void timeoutDropsPartial(){ScannerBuffer b=new ScannerBuffer();b.accept('1',false,10);assertNull(b.accept(0,true,1000));}
    @Test public void timeoutNeverEmitsSuffix(){ScannerBuffer b=new ScannerBuffer();b.accept('1',false,10);b.accept('2',false,1000);assertNull(b.accept(0,true,1010));}
    @Test public void overflowNeverEmitsSuffix(){ScannerBuffer b=new ScannerBuffer();for(int i=0;i<2050;i++)b.accept('1',false,i);assertNull(b.accept(0,true,2051));}
    @Test public void backgroundClearPreventsStaleScan(){ScannerBuffer b=new ScannerBuffer();b.accept('1',false,10);b.clear();assertNull(b.accept(0,true,20));}
}
