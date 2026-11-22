package com.deervery.rakusikaliteclient;

import ai.onnxruntime.*;
import android.content.Context;
import java.nio.*;
import java.util.*;

/** Silero v6.2.1, MIT. Recurrent state and 64-sample context belong to one capture session. */
final class SileroVad implements AutoCloseable {
    private final OrtEnvironment environment = OrtEnvironment.getEnvironment();
    private final OrtSession session;
    private float[][][] state = new float[2][1][128];
    private final float[] input = new float[576];
    SileroVad(Context context) throws Exception {
        try (var model = context.getAssets().open("voice/silero_vad.onnx"); var options = new OrtSession.SessionOptions()) {
            options.setIntraOpNumThreads(1); options.setInterOpNumThreads(1);
            java.io.ByteArrayOutputStream bytes=new java.io.ByteArrayOutputStream();
            byte[] buffer=new byte[8192];int n;while((n=model.read(buffer))!=-1)bytes.write(buffer,0,n);
            session = environment.createSession(bytes.toByteArray(), options);
        }
    }
    float probability(short[] frame) throws OrtException {
        if (frame.length != 512) throw new IllegalArgumentException("Expected 512 PCM samples");
        for (int i = 0; i < 512; i++) input[i + 64] = frame[i] / 32768f;
        try (var audio = OnnxTensor.createTensor(environment, FloatBuffer.wrap(input), new long[]{1,576});
             var memory = OnnxTensor.createTensor(environment, state);
             var rate = OnnxTensor.createTensor(environment, LongBuffer.wrap(new long[]{16000}), new long[0]);
             var result = session.run(Map.of("input", audio, "state", memory, "sr", rate))) {
            float value = ((float[][]) result.get(0).getValue())[0][0];
            state = (float[][][]) result.get(1).getValue();
            System.arraycopy(input,512,input,0,64);
            return value;
        }
    }
    @Override public void close() throws OrtException {session.close();}
}
