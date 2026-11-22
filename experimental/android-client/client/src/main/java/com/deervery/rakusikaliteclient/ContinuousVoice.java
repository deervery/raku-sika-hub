package com.deervery.rakusikaliteclient;

import android.content.*;
import android.media.*;
import android.media.audiofx.*;
import android.os.*;
import android.speech.*;
import com.deervery.rakusikaliteclient.core.VoiceTurnGate;
import java.io.*;
import java.util.*;
import java.util.concurrent.*;

/** One foreground microphone owner. Local VAD + local ASR; raw audio never goes to the broker.
 * ASR consumes an utterance via a pipe, so it never competes for AudioRecord's microphone.
 * EXTRA_AUDIO_SOURCE requires Android 13 and a compatible on-device recognizer.
 */
final class ContinuousVoice implements AutoCloseable {
    interface Listener {
        void ready(boolean echoCancellation);
        void speech(long turn);
        void recognizing(long turn);
        void text(long turn,String text);
        void retry(long turn,String message);
        void failed(String message);
    }
    private final Context context;
    private final Listener listener;
    private final Handler main = new Handler(Looper.getMainLooper());
    private final ExecutorService capture = Executors.newSingleThreadExecutor();
    private final ExecutorService pipeWriter = Executors.newSingleThreadExecutor();
    private volatile boolean closed;
    private final AudioManager audio;
    private AudioFocusRequest focus;
    private int previousMode;
    private boolean modeOwned;
    private SpeechRecognizer recognizer;
    private ParcelFileDescriptor input,output;
    private long turn;
    private Runnable deadline;
    ContinuousVoice(Context context,Listener listener) {this.context=context;this.listener=listener;audio=context.getSystemService(AudioManager.class);}
    static boolean available(Context context) {return Build.VERSION.SDK_INT>=33&&SpeechRecognizer.isOnDeviceRecognitionAvailable(context);}
    void start() {
        if (!available(context)) {fail("自動割り込みにはAndroid 13以降と端末内の日本語音声モデルが必要です。");return;}
        try {
            focus = new AudioFocusRequest.Builder(AudioManager.AUDIOFOCUS_GAIN_TRANSIENT)
                .setAudioAttributes(new AudioAttributes.Builder().setUsage(AudioAttributes.USAGE_VOICE_COMMUNICATION).setContentType(AudioAttributes.CONTENT_TYPE_SPEECH).build())
                .setOnAudioFocusChangeListener(change->{if(change<0)fail("通話などでマイクの利用を中断しました。再開するときはマイクを押してください。");},main).build();
            if(audio.requestAudioFocus(focus)!=AudioManager.AUDIOFOCUS_REQUEST_GRANTED) {fail("音声を開始できません。他の通話を終了してから再開してください。");return;}
            previousMode=audio.getMode();audio.setMode(AudioManager.MODE_IN_COMMUNICATION);modeOwned=true;
            capture.execute(this::record);
        } catch(RuntimeException e) {fail("マイクを開始できません。端末のマイク設定を確認してください。");}
    }
    private void record() {
        AudioRecord recorder=null;AcousticEchoCanceler echo=null;NoiseSuppressor noise=null;
        try(SileroVad vad=new SileroVad(context)) {
            if(closed)return;
            if(context.checkSelfPermission(android.Manifest.permission.RECORD_AUDIO)!=android.content.pm.PackageManager.PERMISSION_GRANTED)throw new SecurityException("Microphone permission denied");
            int minimum=AudioRecord.getMinBufferSize(16000,AudioFormat.CHANNEL_IN_MONO,AudioFormat.ENCODING_PCM_16BIT);
            if(minimum<=0)throw new IOException("Unsupported capture format");
            recorder=new AudioRecord(MediaRecorder.AudioSource.VOICE_COMMUNICATION,16000,AudioFormat.CHANNEL_IN_MONO,AudioFormat.ENCODING_PCM_16BIT,Math.max(minimum,8192));
            if(recorder.getState()!=AudioRecord.STATE_INITIALIZED)throw new IOException("Microphone unavailable");
            if(AcousticEchoCanceler.isAvailable()){echo=AcousticEchoCanceler.create(recorder.getAudioSessionId());if(echo!=null)echo.setEnabled(true);}
            if(NoiseSuppressor.isAvailable()){noise=NoiseSuppressor.create(recorder.getAudioSessionId());if(noise!=null)noise.setEnabled(true);}
            final boolean aec=echo!=null&&echo.getEnabled();
            if(closed)return;
            recorder.startRecording();
            if(recorder.getRecordingState()!=AudioRecord.RECORDSTATE_RECORDING)throw new IOException("Capture did not start");
            main.post(()->{if(!closed)listener.ready(aec);});
            short[] frame=new short[512];ArrayDeque<byte[]> preroll=new ArrayDeque<>();
            ByteArrayOutputStream utterance=null;VoiceTurnGate gate=new VoiceTurnGate();long serial=0;int count=0;
            while(!closed) {
                int offset=0;
                while(offset<512&&!closed){int n=recorder.read(frame,offset,512-offset,AudioRecord.READ_BLOCKING);if(n<=0)throw new IOException("Capture stopped");offset+=n;}
                if(closed)break;
                if(++count%30==0){AudioRecordingConfiguration config=recorder.getActiveRecordingConfiguration();if(config!=null&&config.isClientSilenced())throw new IOException("Microphone silenced");}
                byte[] bytes=new byte[1024];for(int i=0;i<512;i++){bytes[2*i]=(byte)frame[i];bytes[2*i+1]=(byte)(frame[i]>>8);}
                preroll.add(bytes);if(preroll.size()>10)preroll.removeFirst();
                VoiceTurnGate.Event event=gate.accept(vad.probability(frame));
                if(event==VoiceTurnGate.Event.START) {
                    final long id=++serial;
                    utterance=new ByteArrayOutputStream(32768);for(byte[] head:preroll)utterance.write(head);
                    main.post(()->{if(!closed){turn=id;cancelRecognition();listener.speech(id);}});
                } else if(utterance!=null)utterance.write(bytes);
                if(event==VoiceTurnGate.Event.END&&utterance!=null) {
                    final long id=serial;final byte[] pcm=utterance.toByteArray();utterance=null;
                    main.post(()->{if(!closed&&id==turn)recognize(id,pcm);});
                } else if(event==VoiceTurnGate.Event.TOO_LONG) {
                    utterance=null;final long id=serial;
                    main.post(()->{if(!closed&&id==turn)listener.retry(id,"長い発話は一度区切って、もう一度教えてください。");});
                }
            }
        } catch(Exception e) {main.post(()->{if(!closed)fail("マイクを継続できませんでした。マイク権限・他の録音アプリ・音声モデルを確認してください。");});}
        finally {if(recorder!=null){try{recorder.stop();}catch(RuntimeException ignored){}recorder.release();}if(echo!=null)echo.release();if(noise!=null)noise.release();}
    }
    private void recognize(long id,byte[] pcm) {
        if(Build.VERSION.SDK_INT<33){fail("このAndroidでは録音音声からの認識に対応していません。");return;}
        cancelRecognition();listener.recognizing(id);
        try {
            ParcelFileDescriptor[] pipe=ParcelFileDescriptor.createPipe();input=pipe[0];output=pipe[1];
            SpeechRecognizer current=SpeechRecognizer.createOnDeviceSpeechRecognizer(context);recognizer=current;
            current.setRecognitionListener(new RecognitionListener(){
                private boolean valid(){return !closed&&turn==id&&recognizer==current;}
                public void onReadyForSpeech(Bundle b){}public void onBeginningOfSpeech(){}public void onRmsChanged(float v){}public void onBufferReceived(byte[] b){}public void onEndOfSpeech(){}public void onPartialResults(Bundle b){}public void onEvent(int t,Bundle b){}
                public void onResults(Bundle b){if(!valid())return;ArrayList<String> lines=b.getStringArrayList(SpeechRecognizer.RESULTS_RECOGNITION);cancelRecognition();if(lines!=null&&!lines.isEmpty()&&!lines.get(0).isBlank())listener.text(id,lines.get(0));else listener.retry(id,"もう一度教えてください。");}
                public void onError(int error){if(!valid())return;cancelRecognition();if(error==SpeechRecognizer.ERROR_NO_MATCH||error==SpeechRecognizer.ERROR_SPEECH_TIMEOUT)listener.retry(id,"聞き取れませんでした。もう一度教えてください。");else fail("端末内音声認識を利用できません（"+error+"）。日本語モデルと録音音声からの認識対応を確認してください。");}
            });
            Intent intent=new Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH)
                .putExtra(RecognizerIntent.EXTRA_LANGUAGE,"ja-JP")
                .putExtra(RecognizerIntent.EXTRA_LANGUAGE_MODEL,RecognizerIntent.LANGUAGE_MODEL_FREE_FORM)
                .putExtra(RecognizerIntent.EXTRA_PREFER_OFFLINE,true)
                .putExtra(RecognizerIntent.EXTRA_AUDIO_SOURCE,input)
                .putExtra(RecognizerIntent.EXTRA_AUDIO_SOURCE_CHANNEL_COUNT,1)
                .putExtra(RecognizerIntent.EXTRA_AUDIO_SOURCE_ENCODING,AudioFormat.ENCODING_PCM_16BIT)
                .putExtra(RecognizerIntent.EXTRA_AUDIO_SOURCE_SAMPLING_RATE,16000);
            current.startListening(intent);
            final ParcelFileDescriptor writer=output;
            pipeWriter.execute(()->{try(OutputStream stream=new ParcelFileDescriptor.AutoCloseOutputStream(writer)){stream.write(pcm);}catch(IOException ignored){}finally{Arrays.fill(pcm,(byte)0);}});
            deadline=()->{if(!closed&&turn==id&&recognizer==current)fail("端末内音声認識が応答しませんでした。マイクを押して再開してください。");};main.postDelayed(deadline,12000);
        }catch(Exception e){fail("録音音声の認識を開始できません。この端末の音声認識サービスを確認してください。");}
    }
    private void cancelRecognition(){
        if(deadline!=null){main.removeCallbacks(deadline);deadline=null;}
        if(recognizer!=null){SpeechRecognizer old=recognizer;recognizer=null;old.cancel();old.destroy();}
        try{if(input!=null)input.close();}catch(IOException ignored){}input=null;
        try{if(output!=null)output.close();}catch(IOException ignored){}output=null;
    }
    private void fail(String message){if(closed)return;close();listener.failed(message);}
    @Override public void close(){
        if(closed)return;closed=true;cancelRecognition();capture.shutdownNow();pipeWriter.shutdownNow();
        if(modeOwned){modeOwned=false;if(audio.getMode()==AudioManager.MODE_IN_COMMUNICATION)audio.setMode(previousMode);}
        if(focus!=null){audio.abandonAudioFocusRequest(focus);focus=null;}
    }
}
