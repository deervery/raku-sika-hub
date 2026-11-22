package com.deervery.rakusikaliteclient;

import android.Manifest;
import android.content.*;
import android.content.pm.PackageManager;
import android.os.*;
import android.speech.*;
import android.speech.tts.*;
import android.webkit.*;
import androidx.webkit.*;
import org.json.*;
import java.util.*;
import java.util.concurrent.*;
import java.util.function.Consumer;

/** Bundled original face UI. Only the APK-owned main frame receives the narrow native API. */
final class FaceAgentView extends WebView {
    interface Host {void microphonePermission();void openWork(String path);void weigh(Consumer<JSONObject> callback);}
    static final String ORIGIN="https://appassets.androidplatform.net";
    private final Host host;
    private boolean active,localTts;
    private int generation;
    private SpeechRecognizer speech;
    private TextToSpeech tts;
    private Consumer<JSONObject> listening,scanning,speaking;
    private final ExecutorService api=Executors.newSingleThreadExecutor();
    FaceAgentView(Context context,Host host){
        super(context);this.host=host;setBackgroundColor(StationStyle.PAPER);
        getSettings().setJavaScriptEnabled(true);getSettings().setDomStorageEnabled(true);
        getSettings().setAllowFileAccess(false);getSettings().setAllowContentAccess(false);
        getSettings().setMixedContentMode(WebSettings.MIXED_CONTENT_NEVER_ALLOW);
        WebViewAssetLoader assets=new WebViewAssetLoader.Builder().addPathHandler("/assets/",new WebViewAssetLoader.AssetsPathHandler(context)).build();
        setWebViewClient(new WebViewClient(){
            @Override public WebResourceResponse shouldInterceptRequest(WebView v,WebResourceRequest r){
                WebResourceResponse result=assets.shouldInterceptRequest(r.getUrl());
                return result!=null?result:new WebResourceResponse("text/plain","UTF-8",403,"Blocked",Collections.emptyMap(),new java.io.ByteArrayInputStream(new byte[0]));
            }
            @Override public boolean shouldOverrideUrlLoading(WebView v,WebResourceRequest r){return true;}
            @Override public void onReceivedSslError(WebView v,SslErrorHandler handler,android.net.http.SslError error){handler.cancel();}
        });
        if(WebViewFeature.isFeatureSupported(WebViewFeature.WEB_MESSAGE_LISTENER)){
            WebViewCompat.addWebMessageListener(this,"StationNative",Collections.singleton(ORIGIN),(view,message,origin,main,proxy)->{
                if(!main||!ORIGIN.equals(origin.toString())||message.getData()==null||message.getData().length()>16000)return;
                try{
                    JSONObject request=new JSONObject(message.getData());String id=request.getString("id");if(!id.matches("[0-9]{1,12}"))return;
                    final int token=generation;Consumer<JSONObject> reply=value->{if(token!=generation||!active)return;try{proxy.postMessage(new JSONObject().put("id",id).put("value",value).toString());}catch(Exception ignored){}};
                    if(!active)return;
                    dispatch(request.getString("op"),request.optJSONObject("args"),reply);
                }catch(Exception ignored){}
            });
        }
        tts=new TextToSpeech(context,status->{if(status!=TextToSpeech.SUCCESS||tts==null)return;Set<Voice> voices=tts.getVoices();if(voices==null)return;
            for(Voice voice:voices)if("ja".equals(voice.getLocale().getLanguage())&&!voice.isNetworkConnectionRequired()){localTts=tts.setVoice(voice)==TextToSpeech.SUCCESS;break;}
        });
        tts.setOnUtteranceProgressListener(new UtteranceProgressListener(){public void onStart(String id){}public void onDone(String id){post(()->finishSpeaking(id));}public void onError(String id){post(()->finishSpeaking(id));}});
        loadUrl(ORIGIN+"/assets/face/index.html");
    }
    private String utterance="";
    private void finishSpeaking(String id){if(!utterance.equals(id))return;Consumer<JSONObject> callback=speaking;speaking=null;if(callback!=null)callback.accept(new JSONObject());}
    private static JSONObject object(String key,Object value){try{return new JSONObject().put(key,value);}catch(Exception e){return new JSONObject();}}
    private static JSONObject error(String message){return object("error",message);}
    private void dispatch(String op,JSONObject args,Consumer<JSONObject> reply){
        if(args==null)args=new JSONObject();
        switch(op){
            case "listen":listen(reply);break;
            case "stop":stopAudio();reply.accept(new JSONObject());break;
            case "speak":
                String text=args.optString("text");if(text.length()>400){reply.accept(error("応答が長すぎます。"));break;}
                stopAudio();if(!localTts){reply.accept(new JSONObject());break;}
                speaking=reply;utterance=UUID.randomUUID().toString();tts.speak(text,TextToSpeech.QUEUE_FLUSH,null,utterance);break;
            case "weigh":host.weigh(reply);break;
            case "scan":
                if(scanning!=null)scanning.accept(error("新しい読取りを開始しました。"));scanning=reply;
                postDelayed(()->{if(scanning==reply){scanning=null;reply.accept(error("スキャナを操作してください。機器・設定で読取り機を選べます。"));}},20000);break;
            case "work":
                String path=args.optString("path");if(Arrays.asList("","/inventory/history","/reports/labels","/input/sales/new","/input/meat-inbound").contains(path)){reply.accept(new JSONObject());host.openWork(path);}else reply.accept(error("対応していない画面です。"));break;
            case "interpret":
                final JSONObject input=args;final int token=generation;
                api.submit(()->{JSONObject result=AssistantApi.interpret(input);post(()->{if(active&&generation==token)reply.accept(result);});});break;
            default:reply.accept(error("対応していない操作です。"));
        }
    }
    private void listen(Consumer<JSONObject> reply){
        stopAudio();
        if(getContext().checkSelfPermission(Manifest.permission.RECORD_AUDIO)!=PackageManager.PERMISSION_GRANTED){reply.accept(error("マイクを許可したら、顔をタップしてください。"));host.microphonePermission();return;}
        if(Build.VERSION.SDK_INT<31||!SpeechRecognizer.isOnDeviceRecognitionAvailable(getContext())){reply.accept(error("端末内の日本語音声モデルが必要です。顔を長押しすると画面で入力できます。"));return;}
        listening=reply;final Consumer<JSONObject> current=reply;
        try{
            speech=SpeechRecognizer.createOnDeviceSpeechRecognizer(getContext());speech.setRecognitionListener(new RecognitionListener(){
                public void onReadyForSpeech(Bundle b){}public void onBeginningOfSpeech(){}public void onRmsChanged(float v){}public void onBufferReceived(byte[] b){}public void onEndOfSpeech(){}public void onPartialResults(Bundle b){}public void onEvent(int t,Bundle b){}
                public void onResults(Bundle b){if(listening!=current||!active)return;ArrayList<String> text=b.getStringArrayList(SpeechRecognizer.RESULTS_RECOGNITION);listening=null;releaseRecognizer();current.accept(text!=null&&!text.isEmpty()?object("text",text.get(0)):error("もう一度教えてください。"));}
                public void onError(int e){if(listening!=current||!active)return;listening=null;releaseRecognizer();current.accept(error("聞き取れませんでした。日本語音声モデルとマイク設定を確認してください。"));}
            });
            speech.startListening(new Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH).putExtra(RecognizerIntent.EXTRA_LANGUAGE,"ja-JP").putExtra(RecognizerIntent.EXTRA_LANGUAGE_MODEL,RecognizerIntent.LANGUAGE_MODEL_FREE_FORM));
        }catch(RuntimeException e){listening=null;releaseRecognizer();reply.accept(error("音声認識を開始できませんでした。"));}
    }
    private void releaseRecognizer(){if(speech!=null){speech.cancel();speech.destroy();speech=null;}}
    private void stopAudio(){Consumer<JSONObject> a=listening,b=speaking;listening=null;speaking=null;utterance="";releaseRecognizer();if(tts!=null)tts.stop();if(a!=null)a.accept(error("音声入力を停止しました。"));if(b!=null)b.accept(new JSONObject());}
    void scan(String value){if(!active||scanning==null)return;Consumer<JSONObject> r=scanning;scanning=null;r.accept(object("value",value));}
    void activate(){active=true;onResume();evaluateJavascript("window.dispatchEvent(new Event('station-resume'))",null);}
    void suspend(){evaluateJavascript("window.dispatchEvent(new Event('station-pause'))",null);generation++;active=false;stopAudio();scanning=null;onPause();}
    @Override public void destroy(){suspend();if(tts!=null)tts.shutdown();api.shutdownNow();super.destroy();}
}
