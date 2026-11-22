package com.deervery.rakusikaliteclient;
import com.deervery.rakusikaliteclient.core.VoiceTurnGate;
import org.junit.Test;
import static org.junit.Assert.*;
import static com.deervery.rakusikaliteclient.core.VoiceTurnGate.Event.*;
public class VoiceTurnGateTest {
    @Test public void noiseSpikeDoesNotInterruptButSustainedSpeechDoes(){
        VoiceTurnGate gate=new VoiceTurnGate();
        for(int i=0;i<100;i++){assertEquals(NONE,gate.accept(.95f));assertEquals(NONE,gate.accept(.1f));}
        for(int i=0;i<3;i++)assertEquals(NONE,gate.accept(.9f));assertEquals(START,gate.accept(.9f));
        for(int i=0;i<23;i++)assertEquals(NONE,gate.accept(.1f));assertEquals(END,gate.accept(.1f));
    }
    @Test public void shortPausesRemainOneTurnAndLongSpeechIsDiscarded(){
        VoiceTurnGate gate=new VoiceTurnGate();for(int i=0;i<4;i++)gate.accept(.9f);
        for(int i=0;i<10;i++)assertEquals(NONE,gate.accept(.1f));
        for(int i=0;i<454;i++)assertEquals(NONE,gate.accept(.8f));
        assertEquals(TOO_LONG,gate.accept(.8f));
        for(int i=0;i<500;i++)assertEquals(NONE,gate.accept(.9f));
        for(int i=0;i<24;i++)gate.accept(.1f);
        for(int i=0;i<3;i++)assertEquals(NONE,gate.accept(.9f));assertEquals(START,gate.accept(.9f));
    }
}
