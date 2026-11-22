package com.deervery.rakusikaliteclient;

import android.Manifest;
import android.app.AlertDialog;
import android.content.*;
import android.content.pm.PackageManager;
import android.os.*;
import android.speech.*;
import android.speech.tts.*;
import android.view.*;
import android.widget.*;
import java.util.*;
import java.util.function.Consumer;
import java.math.BigDecimal;
import org.json.JSONObject;

/** Android-local dialogue and draft. Registration/printing stay in authenticated Lite. */
final class AgentPanel extends LinearLayout {
    interface Host {void microphonePermission();void openWork(String path);void weigh(Consumer<JSONObject> result);}
    private final Host host;
    private final AgentDraft draft=new AgentDraft();
    private AgentDraft.Offer offer;
    private final SharedPreferences saved;
    private final AgentFace face;
    private final LinearLayout bubbles,cards,actions;
    private final TextView user,reply;
    private final Button confirm,cancel;
    private SpeechRecognizer speech;private TextToSpeech tts;private boolean localTts,active,listening,review;
    private int epoch;
    AgentPanel(Context context,Host host){
        super(context);this.host=host;setOrientation(VERTICAL);setBackgroundColor(StationStyle.PAPER);
        int pad=StationStyle.dp(context,20);setPadding(pad,0,pad,pad);
        saved=context.getSharedPreferences("assistant-draft",Context.MODE_PRIVATE);
        Map<String,String> restored=new LinkedHashMap<>();for(String key:new String[]{"individual","part","weight"})restored.put(key,saved.getString(key,""));draft.restore(restored);
        face=new AgentFace(context);face.setOnClickListener(v->{if(listening)stop();else listen();});
        face.setOnLongClickListener(v->{new AlertDialog.Builder(getContext()).setTitle("アシスタント")
            .setItems(new String[]{"計量する","個体番号を入力","部位を選ぶ","重量を入力","入力内容を見る","業務を開く"},(d,i)->{
                if(i==0)requestWeight();else if(i==1)editCard("individual");else if(i==2)editCard("part");else if(i==3)editCard("weight");else if(i==4){review=true;renderCards();}else workMenu();})
            .setNegativeButton("閉じる",null).show();return true;});
        addView(face,new LinearLayout.LayoutParams(-1,0,1));
        bubbles=StationStyle.column(context);bubbles.setGravity(Gravity.CENTER);addView(bubbles);
        user=bubble(saved.getString("lastUser",""),true);reply=bubble(saved.getString("lastReply","何をしましょうか。"),false);
        latest(saved.getBoolean("userLatest",false)?user:reply);
        cards=StationStyle.column(context);cards.setVisibility(GONE);addView(cards,new LinearLayout.LayoutParams(-1,0,1));
        actions=new LinearLayout(context);actions.setBaselineAligned(false);addView(actions);
        addAction("計量",this::requestWeight);
        addAction("入力内容",()->{stop();review=!review;renderCards();});
        addAction("業務を開く",this::workMenu);
        LinearLayout confirmation=new LinearLayout(context);confirmation.setBaselineAligned(false);addView(confirmation);
        confirm=StationStyle.button(context,"この内容を反映",true,()->{
            stop();if(offer==null)return;
            try{draft.accept(offer,SystemClock.elapsedRealtime());offer=null;persist();renderCards();}
            catch(RuntimeException e){offer=null;say(e.getMessage(),false);renderCards();}
        });
        cancel=StationStyle.button(context,"取り消す",false,()->{stop();offer=null;renderCards();});
        LinearLayout.LayoutParams cp=new LinearLayout.LayoutParams(0,-2,1);cp.setMargins(0,StationStyle.dp(context,12),StationStyle.dp(context,8),0);
        confirmation.addView(confirm,cp);LinearLayout.LayoutParams cancelSize=new LinearLayout.LayoutParams(0,-2,1);cancelSize.topMargin=dp(12);confirmation.addView(cancel,cancelSize);
        renderCards();
        tts=new TextToSpeech(context,result->{
            if(result!=TextToSpeech.SUCCESS||tts==null)return;
            Set<Voice> voices=tts.getVoices();if(voices==null)return;
            for(Voice voice:voices)if("ja".equals(voice.getLocale().getLanguage())&&!voice.isNetworkConnectionRequired()){
                localTts=tts.setVoice(voice)==TextToSpeech.SUCCESS;break;
            }
        });
    }
    private TextView bubble(String text,boolean own){
        TextView v=StationStyle.text(getContext(),text,18,false);v.setPadding(dp(18),dp(13),dp(18),dp(13));
        v.setBackground(StationStyle.shape(getContext(),own?StationStyle.SOFT:StationStyle.WHITE,22));
        v.setMaxLines(4);v.setEllipsize(android.text.TextUtils.TruncateAt.END);
        LinearLayout.LayoutParams lp=new LinearLayout.LayoutParams(-2,-2);lp.gravity=own?Gravity.END:Gravity.START;lp.setMargins(own?dp(28):0,0,own?0:dp(28),dp(10));
        bubbles.addView(v,lp);v.setVisibility(text.isEmpty()?GONE:VISIBLE);
        v.setOnClickListener(w->new AlertDialog.Builder(getContext()).setMessage(v.getText()).setPositiveButton("閉じる",null).show());return v;
    }
    private int dp(int n){return StationStyle.dp(getContext(),n);}
    private void latest(TextView view){
        if(bubbles.indexOfChild(view)!=bubbles.getChildCount()-1){android.view.ViewGroup.LayoutParams lp=view.getLayoutParams();bubbles.removeView(view);bubbles.addView(view,lp);}
        if(user!=null)user.setAlpha(view==user?1f:.65f);if(reply!=null)reply.setAlpha(view==reply?1f:.65f);
    }
    private void addAction(String title,Runnable action){
        Button b=StationStyle.button(getContext(),title,false,action);b.setTextSize(14);
        LinearLayout.LayoutParams lp=new LinearLayout.LayoutParams(0,-2,1);lp.setMargins(dp(3),dp(12),dp(3),0);actions.addView(b,lp);
    }
    private void say(String text,boolean speak){
        reply.setText(text);reply.setVisibility(VISIBLE);reply.setAlpha(0);reply.animate().alpha(1).setDuration(180).start();
        latest(reply);saved.edit().putString("lastReply",text).putBoolean("userLatest",false).apply();
        if(speak&&active&&localTts&&tts!=null)tts.speak(text,TextToSpeech.QUEUE_FLUSH,null,"reply-"+epoch);
    }
    void activate(){active=true;}
    void suspend(){active=false;stop();persist();}
    void destroy(){suspend();if(tts!=null)tts.shutdown();}
    private void stop(){epoch++;listening=false;face.listening(false);if(speech!=null){speech.cancel();speech.destroy();speech=null;}if(tts!=null)tts.stop();}
    private void listen(){
        if(!active)return;stop();
        if(getContext().checkSelfPermission(Manifest.permission.RECORD_AUDIO)!=PackageManager.PERMISSION_GRANTED){say("マイクを許可したら、顔をタップして話してください。",false);host.microphonePermission();return;}
        if(Build.VERSION.SDK_INT<31||!SpeechRecognizer.isOnDeviceRecognitionAvailable(getContext())){
            say("この端末では端末内の日本語音声認識を利用できません。業務画面から操作できます。",false);return;
        }
        final int token=++epoch;final long revision=draft.revision();
        try{
            speech=SpeechRecognizer.createOnDeviceSpeechRecognizer(getContext());
            speech.setRecognitionListener(new RecognitionListener(){
                public void onReadyForSpeech(Bundle p){if(token==epoch){listening=true;face.listening(true);}}
                public void onBeginningOfSpeech(){}public void onRmsChanged(float value){}public void onBufferReceived(byte[] bytes){}public void onEndOfSpeech(){}
                public void onPartialResults(Bundle p){if(token!=epoch||!active)return;ArrayList<String> lines=p.getStringArrayList(SpeechRecognizer.RESULTS_RECOGNITION);if(lines!=null&&!lines.isEmpty()){user.setText(lines.get(0));user.setVisibility(VISIBLE);latest(user);}}
                public void onResults(Bundle p){if(token!=epoch||!active)return;ArrayList<String> lines=p.getStringArrayList(SpeechRecognizer.RESULTS_RECOGNITION);stop();if(lines!=null&&!lines.isEmpty())receive(lines.get(0),revision);}
                public void onError(int error){if(token!=epoch||!active)return;stop();say(error==SpeechRecognizer.ERROR_LANGUAGE_UNAVAILABLE?"端末の日本語音声モデルが必要です。音声入力の設定を確認してください。":"聞き取れませんでした。顔をタップして、もう一度どうぞ。",false);}
                public void onEvent(int type,Bundle p){}
            });
            speech.startListening(new Intent(RecognizerIntent.ACTION_RECOGNIZE_SPEECH).putExtra(RecognizerIntent.EXTRA_LANGUAGE,"ja-JP")
                .putExtra(RecognizerIntent.EXTRA_LANGUAGE_MODEL,RecognizerIntent.LANGUAGE_MODEL_FREE_FORM).putExtra(RecognizerIntent.EXTRA_PARTIAL_RESULTS,true));
            listening=true;face.listening(true);
        }catch(RuntimeException error){stop();say("音声入力を開始できませんでした。端末の音声入力設定を確認してください。",false);}
    }
    private void receive(String text,long revision){
        user.setText(text);user.setVisibility(VISIBLE);latest(user);saved.edit().putString("lastUser",text).putBoolean("userLatest",true).apply();
        String s=text.replaceAll("[。！!\\s]","");
        if(s.matches(".*(在庫|ラベル|販売|入力画面).*")){workMenu();return;}
        if(s.matches("(計量|計って|量って|重さを教えて|重量を読んで)")){requestWeight();return;}
        if(s.matches("(取り消し|取り消して|キャンセル)")){offer=null;renderCards();return;}
        if(s.matches("(一つ戻す|ひとつ戻す|元に戻して)")){offer=null;draft.undo();persist();renderCards();return;}
        if(s.matches("(確認|確認して|入力内容を見せて)")){review=true;renderCards();return;}
        if(s.matches("(はい|いいね|確定|もういい)")){say("入力内容の反映、会話の終了、どちらにしますか？",true);return;}
        try{offer=draft.propose(text,revision,SystemClock.elapsedRealtime());review=true;renderCards();}
        catch(RuntimeException error){say(error.getMessage(),true);}
    }
    void scan(String value){
        if(!active)return;stop();
        if(!value.matches("[A-Za-z0-9][A-Za-z0-9-]{0,31}")){say("このコードは業務画面の読取り機能で確認してください。",false);return;}
        receive("個体"+value,draft.revision());
    }
    private void requestWeight(){
        stop();final int token=epoch;final long revision=draft.revision();
        host.weigh(result->{if(active&&epoch==token&&draft.revision()==revision)weight(result);});
    }
    private void weight(JSONObject result){
        if(!active)return;
        if(!result.optString("status").equals("ok")||!result.optBoolean("stable")){
            say(result.optString("message",result.optString("status").equals("weighing")?"重量がまだ安定していません。":"計量器との接続を確認してください。"),false);return;
        }
        try{
            BigDecimal value=new BigDecimal(result.get("value").toString());String unit=result.optString("unit");
            if(unit.equals("g"))value=value.movePointLeft(3);else if(!unit.equals("kg"))throw new IllegalArgumentException();
            if(value.signum()<0)throw new IllegalArgumentException();
            offer=draft.weight(value.stripTrailingZeros().toPlainString(),SystemClock.elapsedRealtime());review=true;renderCards();
        }catch(Exception error){say("重量を確認できませんでした。計量器の表示を確認してください。",false);}
    }
    private void renderCards(){
        if(cards==null)return;cards.removeAllViews();
        Map<String,String> values=new LinkedHashMap<>(draft.values());if(offer!=null)values.putAll(offer.values);
        List<String> entered=new ArrayList<>();for(String key:values.keySet())if(!values.get(key).isEmpty())entered.add(key);
        boolean shown=review&&!entered.isEmpty();cards.setVisibility(shown?VISIBLE:GONE);face.setVisibility(shown?GONE:VISIBLE);
        actions.setVisibility(shown?VISIBLE:GONE);
        bubbles.setVisibility(shown?GONE:VISIBLE);
        if(shown){
            TextView title=StationStyle.text(getContext(),offer==null?"入力内容":"内容を確認",22,true);title.setPadding(0,dp(18),0,dp(8));cards.addView(title);
            TextView note=StationStyle.text(getContext(),"端末の下書き · 業務への登録は業務画面で",13,false);note.setTextColor(StationStyle.MUTED);cards.addView(note);
            for(String key:entered){
                LinearLayout card=StationStyle.column(getContext());card.setGravity(Gravity.CENTER_VERTICAL);card.setPadding(dp(22),dp(12),dp(22),dp(12));
                card.setBackground(StationStyle.shape(getContext(),offer!=null&&offer.values.containsKey(key)?StationStyle.SOFT:StationStyle.WHITE,24));
                TextView label=StationStyle.text(getContext(),key.equals("individual")?"個体番号":key.equals("part")?"部位":"重量",15,false);label.setTextColor(StationStyle.MUTED);card.addView(label);
                TextView value=StationStyle.text(getContext(),values.get(key)+(key.equals("weight")?" kg":""),entered.size()==1?40:28,true);card.addView(value);
                card.setFocusable(true);card.setContentDescription(label.getText()+"、"+value.getText()+"。タップして訂正");card.setOnClickListener(v->editCard(key));
                LinearLayout.LayoutParams lp=new LinearLayout.LayoutParams(-1,0,1);lp.setMargins(0,dp(10),0,0);cards.addView(card,lp);
            }
        }
        if(confirm!=null){confirm.setVisibility(offer==null?GONE:VISIBLE);cancel.setVisibility(offer==null?GONE:VISIBLE);}
    }
    private void editCard(String key){
        stop();
        if(key.equals("part")){
            String[] parts={"モモ","ロース","バラ","スネ"};
            new AlertDialog.Builder(getContext()).setTitle("部位を選ぶ").setItems(parts,(d,i)->receive(parts[i],draft.revision())).setNegativeButton("戻る",null).show();return;
        }
        LinearLayout pad=StationStyle.column(getContext());pad.setPadding(dp(20),dp(12),dp(20),dp(12));
        TextView value=StationStyle.text(getContext(),"",32,true);value.setGravity(Gravity.END);value.setMinHeight(dp(64));pad.addView(value);
        StringBuilder number=new StringBuilder();
        for(String[] row:new String[][]{{"1","2","3"},{"4","5","6"},{"7","8","9"},{key.equals("weight")?".":"消去","0","⌫"}}){
            LinearLayout line=new LinearLayout(getContext());line.setBaselineAligned(false);pad.addView(line);
            for(String digit:row){Button b=StationStyle.button(getContext(),digit,false,()->{
                if(digit.equals("⌫")){if(number.length()>0)number.deleteCharAt(number.length()-1);}
                else if(digit.equals("消去"))number.setLength(0);
                else if(number.length()<12&&(!digit.equals(".")||number.indexOf(".")<0))number.append(digit);
                value.setText(number.toString());
            });LinearLayout.LayoutParams lp=new LinearLayout.LayoutParams(0,-2,1);lp.setMargins(dp(3),dp(3),dp(3),dp(3));line.addView(b,lp);}
        }
        new AlertDialog.Builder(getContext()).setTitle(key.equals("weight")?"重量（kg）":"個体番号")
            .setView(pad).setNegativeButton("戻る",null).setPositiveButton("内容を確認",(d,i)->receive(key.equals("weight")?number+"kg":"個体"+number,draft.revision())).show();
    }
    private void persist(){SharedPreferences.Editor e=saved.edit();for(Map.Entry<String,String> value:draft.values().entrySet())e.putString(value.getKey(),value.getValue());e.apply();}
    private void workMenu(){
        stop();new AlertDialog.Builder(getContext()).setTitle("業務を開く")
            .setItems(new String[]{"現在の業務に戻る","在庫を確認","ラベルを作る","販売を記録","入庫を入力"},(d,i)->host.openWork(new String[]{"","/inventory/history","/reports/labels","/input/sales/new","/input/meat-inbound"}[i]))
            .setNegativeButton("閉じる",null).show();
    }
}
