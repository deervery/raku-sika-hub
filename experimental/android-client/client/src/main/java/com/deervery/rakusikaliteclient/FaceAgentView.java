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
    interface Host {void microphonePermission();void openWork(String path);void weigh(Consumer<JSONObject> callback);default void catalog(JSONObject args,Consumer<JSONObject> callback){callback.accept(error("候補を取得できません。"));}default void camera(Consumer<JSONObject> callback){callback.accept(error("カメラを利用できません。"));}default JSONObject session(){return new JSONObject();}}
    static final String ORIGIN="https://appassets.androidplatform.net";
    private final Host host;
    private boolean active,localTts;
    private int generation;
    private SpeechRecognizer speech;
    private TextToSpeech tts;
    private Consumer<JSONObject> listening,scanning,speaking;
    private AssistantApi.Call interpretation;
    private Future<?> inference;
    private ContinuousVoice continuous;
    private String voiceSession="";
    private long voiceTurn;
    private boolean duplexPlayback;
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
                    final String op=request.getString("op");final int token=generation;Consumer<JSONObject> reply=value->{if(token!=generation||(!op.equals("session")&&(!active||host.session().optString("scope").isEmpty())))return;try{proxy.postMessage(new JSONObject().put("id",id).put("value",value).toString());}catch(Exception ignored){}};
                    if(op.equals("session")){reply.accept(host.session());return;}
                    if(!active)return;
                    if(host.session().optString("scope").isEmpty()){reply.accept(error("業務画面でログインしてください。"));clearSession();host.openWork("");return;}
                    dispatch(op,request.optJSONObject("args"),reply);
                }catch(Exception ignored){}
            });
        }
        tts=new TextToSpeech(context,status->{if(status!=TextToSpeech.SUCCESS||tts==null)return;Set<Voice> voices=tts.getVoices();if(voices==null)return;
            for(Voice voice:voices)if("ja".equals(voice.getLocale().getLanguage())&&!voice.isNetworkConnectionRequired()){localTts=tts.setVoice(voice)==TextToSpeech.SUCCESS;break;}
        });
        tts.setOnUtteranceProgressListener(new UtteranceProgressListener(){public void onStart(String id){}public void onDone(String id){post(()->finishSpeaking(id));}public void onError(String id){post(()->finishSpeaking(id));}});
        tts.setAudioAttributes(new android.media.AudioAttributes.Builder().setUsage(android.media.AudioAttributes.USAGE_VOICE_COMMUNICATION).setContentType(android.media.AudioAttributes.CONTENT_TYPE_SPEECH).build());
        loadUrl(ORIGIN+"/assets/face/index.html");
    }
    private String utterance="";
    private void finishSpeaking(String id){if(!utterance.equals(id))return;Consumer<JSONObject> callback=speaking;speaking=null;if(callback!=null)callback.accept(new JSONObject());}
    private static JSONObject object(String key,Object value){try{return new JSONObject().put(key,value);}catch(Exception e){return new JSONObject();}}
    private static JSONObject error(String message){return object("error",message);}
    private void dispatch(String op,JSONObject args,Consumer<JSONObject> reply){
        if(args==null)args=new JSONObject();
        // Even a delayed bridge message must not restart an old answer after speech interruption.
        if((op.equals("speak")||op.equals("interpret"))&&args.has("session")&&(!args.optString("session").equals(voiceSession)||args.optLong("turn",-1)!=voiceTurn)){reply.accept(error("古い応答を破棄しました。"));return;}
        switch(op){
            case "voiceStart":startContinuous(args.optString("session"),reply);break;
            case "voiceDiscard":if(!args.optString("session").equals(voiceSession)){reply.accept(new JSONObject());break;}voiceTurn=-1;stopPlayback();cancelInterpretation();reply.accept(new JSONObject());break;
            case "listen":listen(reply);break;
            case "stop":stopAudio();cancelInterpretation();reply.accept(new JSONObject());break;
            case "speak":
                String text=args.optString("text");if(text.length()>400){reply.accept(error("応答が長すぎます。"));break;}
                stopPlayback();if(!localTts||(continuous!=null&&!duplexPlayback)){reply.accept(new JSONObject());break;}
                speaking=reply;utterance=UUID.randomUUID().toString();tts.speak(text,TextToSpeech.QUEUE_FLUSH,null,utterance);break;
            case "catalog":host.catalog(args,reply);break;
            case "cameraScan":stopAudio();cancelInterpretation();host.camera(reply);break;
            case "weigh":host.weigh(reply);break;
            case "scan":
                if(scanning!=null)scanning.accept(error("新しい読取りを開始しました。"));scanning=reply;
                postDelayed(()->{if(scanning==reply){scanning=null;reply.accept(error("スキャナを操作してください。メニューの機器設定で読取り機を選べます。"));}},20000);break;
            case "work":
                String path=args.optString("path");if(Arrays.asList("","/inventory/history","/reports/labels","/input/sales/new","/input/meat-inbound").contains(path)){reply.accept(new JSONObject());host.openWork(path);}else reply.accept(error("対応していない画面です。"));break;
            case "interpret":
                if(!host.session().optBoolean("geminiEnabled")){reply.accept(error("音声・AI設定でGeminiの利用をオンにしてください。"));break;}
                cancelInterpretation();final JSONObject input=new JSONObject();
                for(String key:new String[]{"text","values","options","history"})try{if(args.has(key))input.put(key,args.get(key));}catch(JSONException ignored){}
                final int token=generation;final AssistantApi.Call call=new AssistantApi.Call();interpretation=call;
                inference=api.submit(()->{JSONObject result=call.execute(input);post(()->{if(active&&generation==token&&interpretation==call&&!host.session().optString("scope").isEmpty()){interpretation=null;reply.accept(result);}});});break;
            default:reply.accept(error("対応していない操作です。"));
        }
    }
    private void voiceEvent(String session,String type,long turn,String text){
        if(!active||!voiceSession.equals(session))return;
        if(host.session().optString("scope").isEmpty()){stopAudio();cancelInterpretation();clearSession();host.openWork("");return;}
        try{JSONObject event=new JSONObject().put("session",session).put("type",type).put("turn",turn).put("text",text);
            evaluateJavascript("window.dispatchEvent(new CustomEvent('station-voice',{detail:"+event+"}))",null);
        }catch(Exception ignored){}
    }
    private void startContinuous(String session,Consumer<JSONObject> reply){
        stopAudio();cancelInterpretation();
        if(!session.matches("[a-zA-Z0-9-]{1,80}")){reply.accept(error("会話を開始し直してください。"));return;}
        if(getContext().checkSelfPermission(Manifest.permission.RECORD_AUDIO)!=PackageManager.PERMISSION_GRANTED){reply.accept(error("マイクを許可したら、顔をタップしてください。"));host.microphonePermission();return;}
        if(Build.VERSION.SDK_INT>=31&&Build.VERSION.SDK_INT<33&&SpeechRecognizer.isOnDeviceRecognitionAvailable(getContext())){reply.accept(object("continuous",false));return;}
        if(!ContinuousVoice.available(getContext())){reply.accept(error("自動割り込みにはAndroid 13以降と端末内の日本語音声モデルが必要です。"));return;}
        voiceSession=session;voiceTurn=0;
        continuous=new ContinuousVoice(getContext(),new ContinuousVoice.Listener(){
            public void ready(boolean aec){if(!voiceSession.equals(session))return;duplexPlayback=aec;voiceEvent(session,"ready",0,aec?"echo-cancelled":"echo-unavailable");}
            public void speech(long id){if(!voiceSession.equals(session))return;voiceTurn=id;stopPlayback();cancelInterpretation();voiceEvent(session,"speech",id,"");}
            public void recognizing(long id){voiceEvent(session,"recognizing",id,"");}
            public void text(long id,String text){if(voiceTurn==id)voiceEvent(session,"text",id,text);}
            public void retry(long id,String message){if(voiceTurn==id)voiceEvent(session,"retry",id,message);}
            public void failed(String message){if(!voiceSession.equals(session))return;voiceEvent(session,"error",voiceTurn,message);stopAudio();cancelInterpretation();}
        });
        continuous.start();reply.accept(object("continuous",true));
    }
    private void listen(Consumer<JSONObject> reply){
        stopAudio();cancelInterpretation();
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
    private void stopPlayback(){Consumer<JSONObject> b=speaking;speaking=null;utterance="";if(tts!=null)tts.stop();if(b!=null)b.accept(new JSONObject());}
    private void stopAudio(){voiceSession="";if(continuous!=null){continuous.close();continuous=null;}Consumer<JSONObject> a=listening;listening=null;releaseRecognizer();stopPlayback();if(a!=null)a.accept(error("音声入力を停止しました。"));}
    void scan(String value){if(!active||scanning==null)return;Consumer<JSONObject> r=scanning;scanning=null;r.accept(object("value",value));}
    void activate(){if(host.session().optString("scope").isEmpty())return;active=true;onResume();evaluateJavascript("window.dispatchEvent(new Event('station-resume'))",null);}
    void refreshSession(){evaluateJavascript("window.dispatchEvent(new Event('station-session-refresh'))",null);}
    void clearSession(){stopConversation();evaluateJavascript("window.dispatchEvent(new Event('station-session-cleared'))",null);}
    void stopConversation(){stopAudio();cancelInterpretation();evaluateJavascript("window.dispatchEvent(new Event('station-pause'))",null);}
    private void cancelInterpretation(){if(interpretation!=null){interpretation.cancel();interpretation=null;}if(inference!=null){inference.cancel(true);inference=null;}}
    void suspend(){evaluateJavascript("window.dispatchEvent(new Event('station-pause'))",null);generation++;active=false;stopAudio();cancelInterpretation();scanning=null;onPause();}
    @Override public void destroy(){suspend();if(tts!=null)tts.shutdown();api.shutdownNow();super.destroy();}
}
