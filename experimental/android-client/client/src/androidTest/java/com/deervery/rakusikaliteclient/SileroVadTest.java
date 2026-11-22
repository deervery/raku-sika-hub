package com.deervery.rakusikaliteclient;
import androidx.test.ext.junit.runners.AndroidJUnit4;
import androidx.test.platform.app.InstrumentationRegistry;
import org.junit.Test;
import org.junit.runner.RunWith;
import static org.junit.Assert.*;
@RunWith(AndroidJUnit4.class)
public class SileroVadTest {
    @Test public void bundledModelDetectsReferenceSpeech()throws Exception{
        try(SileroVad vad=new SileroVad(InstrumentationRegistry.getInstrumentation().getTargetContext());
            java.io.InputStream pcm=InstrumentationRegistry.getInstrumentation().getContext().getAssets().open("silero-speech.pcm")){
            byte[] frame=new byte[1024];short[] samples=new short[512];int speechFrames=0;
            while(pcm.read(frame)==frame.length){for(int i=0;i<512;i++)samples[i]=(short)((frame[2*i]&255)|(frame[2*i+1]<<8));if(vad.probability(samples)>.6f)speechFrames++;}
            assertTrue("Upstream reference speech must be detected",speechFrames>10);
        }
    }
    @Test public void bundledModelRunsOfflineAndRejectsSilence()throws Exception{
        try(SileroVad vad=new SileroVad(InstrumentationRegistry.getInstrumentation().getTargetContext())){
            short[] silence=new short[512];long start=System.nanoTime();
            for(int i=0;i<100;i++){float probability=vad.probability(silence);assertTrue(Float.isFinite(probability));assertTrue(probability<.6f);}
            assertTrue("VAD must process audio faster than real time",(System.nanoTime()-start)/1e6<3200);
        }
    }
}
