package com.deervery.rakusikaliteclient;

import android.Manifest;
import android.app.AlertDialog;
import android.app.DownloadManager;
import android.content.Intent;
import android.content.SharedPreferences;
import android.content.BroadcastReceiver;
import android.content.Context;
import android.content.IntentFilter;
import android.app.PendingIntent;
import android.hardware.usb.UsbDevice;
import android.hardware.usb.UsbManager;
import androidx.core.content.ContextCompat;
import android.content.pm.PackageManager;
import android.net.Uri;
import android.net.http.SslError;
import android.os.Bundle;
import android.os.Environment;
import android.view.WindowManager;
import android.view.InputDevice;
import android.view.KeyEvent;
import android.hardware.input.InputManager;
import android.webkit.CookieManager;
import android.webkit.PermissionRequest;
import android.webkit.SslErrorHandler;
import android.webkit.URLUtil;
import android.webkit.ValueCallback;
import android.webkit.WebChromeClient;
import android.webkit.WebResourceRequest;
import android.webkit.WebResourceError;
import android.webkit.WebView;
import android.webkit.WebViewClient;
import android.widget.Button;
import android.widget.EditText;
import android.widget.LinearLayout;
import android.widget.TextView;
import android.widget.Toast;
import androidx.webkit.WebViewCompat;
import androidx.webkit.WebViewFeature;
import androidx.activity.ComponentActivity;
import androidx.activity.OnBackPressedCallback;
import org.json.JSONObject;
import java.net.HttpURLConnection;
import java.net.URL;
import java.util.ArrayList;
import java.util.List;
import java.util.Collections;
import java.util.concurrent.ExecutorService;
import java.util.concurrent.Executors;

/** Existing Lite UI + facility Hub client. No Gemini key or native device bridge. */
public final class MainActivity extends ComponentActivity {
    private static final int MEDIA_PERMISSION = 71, FILE_PICKER = 72;
    private SharedPreferences settings;
    private WebView web;
    private PermissionRequest pendingMedia;
    private ValueCallback<Uri[]> pendingFile;
    private String webUrl, hubUrl;
    private TextView connection;
    private volatile boolean visible,usbMode,trustedPage;
    private UsbScale usbScale;
    private UsbScaleRoutes usbRoutes;
    private UsbDevice selectedScale;
    private final String usbPermission="com.deervery.rakusikaliteclient.USB_PERMISSION";
    private final BroadcastReceiver usbReceiver=new BroadcastReceiver(){
        @Override public void onReceive(Context context,Intent intent){
            UsbDevice device=intent.getParcelableExtra(UsbManager.EXTRA_DEVICE);
            if(UsbManager.ACTION_USB_DEVICE_DETACHED.equals(intent.getAction())){
                if(selectedScale!=null&&device!=null&&device.getDeviceId()==selectedScale.getDeviceId()){
                    selectedScale=null;usbScale.disconnect();showUsbStatus();
                }
            }else if(usbPermission.equals(intent.getAction())&&selectedScale!=null&&device!=null&&device.getDeviceId()==selectedScale.getDeviceId()){
                if(intent.getBooleanExtra(UsbManager.EXTRA_PERMISSION_GRANTED,false)&&visible&&usbMode)connectUsb();
                else showUsbStatus();
            }
        }
    };
    private final ScannerBuffer scannerBuffer=new ScannerBuffer();
    private String scannerDescriptor="";
    private final InputManager.InputDeviceListener inputListener=new InputManager.InputDeviceListener(){
        @Override public void onInputDeviceAdded(int id){scannerBuffer.clear();}
        @Override public void onInputDeviceRemoved(int id){scannerBuffer.clear();}
        @Override public void onInputDeviceChanged(int id){scannerBuffer.clear();}
    };
    private final ExecutorService checks = Executors.newSingleThreadExecutor();

    @Override public void onCreate(Bundle state) {
        super.onCreate(state);
        getOnBackPressedDispatcher().addCallback(this,new OnBackPressedCallback(true){
            @Override public void handleOnBackPressed(){
                if(web!=null&&web.canGoBack())web.goBack();
                else {setEnabled(false);getOnBackPressedDispatcher().onBackPressed();setEnabled(true);}
            }
        });
        getWindow().addFlags(WindowManager.LayoutParams.FLAG_KEEP_SCREEN_ON);
        settings = getSharedPreferences("lite-client", MODE_PRIVATE);
        usbMode=settings.getBoolean("usbScale",false);
        usbScale=new UsbScale(this);
        usbRoutes=new UsbScaleRoutes(new UsbScaleRoutes.Device(){
            public boolean connected(){return usbScale.connected();}
            public JSONObject command(String command){return usbScale.command(command);}
        });
        IntentFilter usbFilter=new IntentFilter(usbPermission);usbFilter.addAction(UsbManager.ACTION_USB_DEVICE_DETACHED);
        ContextCompat.registerReceiver(this,usbReceiver,usbFilter,ContextCompat.RECEIVER_NOT_EXPORTED);
        webUrl = settings.getString("web", "https://rakusika.com/home");
        hubUrl = settings.getString("hub", "");
        scannerDescriptor=settings.getString("scanner","");
        ((InputManager)getSystemService(INPUT_SERVICE)).registerInputDeviceListener(inputListener,null);
        if (settings.getBoolean("configured", false)) openLite(); else showSetup();
    }

    private LinearLayout layout() {
        LinearLayout root = new LinearLayout(this) {
            @Override public boolean dispatchKeyEvent(KeyEvent event) {
                return handleScannerKey(event) || super.dispatchKeyEvent(event);
            }
        };
        root.setOrientation(LinearLayout.VERTICAL);
        root.setBackgroundColor(0xfff4f8f5);
        root.setOnApplyWindowInsetsListener((view, insets) -> {
            if (android.os.Build.VERSION.SDK_INT >= 30) {
                android.graphics.Insets bars = insets.getInsets(android.view.WindowInsets.Type.systemBars());
                view.setPadding(bars.left, bars.top, bars.right, bars.bottom);
            } else view.setPadding(insets.getSystemWindowInsetLeft(), insets.getSystemWindowInsetTop(), insets.getSystemWindowInsetRight(), insets.getSystemWindowInsetBottom());
            return insets;
        });
        setContentView(root); return root;
    }
    private Button button(LinearLayout parent, String text, Runnable action) {
        Button b = new Button(this); b.setText(text); b.setAllCaps(false); b.setMinHeight(52);
        b.setOnClickListener(v -> action.run()); parent.addView(b); return b;
    }
    private TextView label(LinearLayout parent, String text, int size) {
        TextView view = new TextView(this); view.setText(text); view.setTextSize(size); view.setPadding(20,16,20,12);parent.addView(view);return view;
    }
    private EditText input(LinearLayout parent, String value) {
        EditText input = new EditText(this); input.setSingleLine(true); input.setText(value);
        input.setInputType(android.text.InputType.TYPE_CLASS_TEXT | android.text.InputType.TYPE_TEXT_VARIATION_URI);
        input.setTextSize(18);parent.addView(input);return input;
    }
    private void disposeWeb() {
        trustedPage=false;
        if(usbScale!=null)usbScale.disconnect();
        denyPendingMedia();
        if (pendingFile != null) { pendingFile.onReceiveValue(null); pendingFile=null; }
        if (web != null) { web.stopLoading(); web.destroy(); web=null; }
    }
    private void showSetup() {
        disposeWeb(); LinearLayout frame=layout();
        android.widget.ScrollView scroll=new android.widget.ScrollView(this);
        frame.addView(scroll,new LinearLayout.LayoutParams(-1,-1));
        LinearLayout root=new LinearLayout(this);root.setOrientation(LinearLayout.VERTICAL);scroll.addView(root);
        label(root,"ラクシカライト Android",28);
        label(root,"既存の画面・アカウントを利用します。ハブのプリンタ・スキャナ・計量器を施設単位で接続します。",17);
        label(root,"ライトのURL",18); EditText site=input(root,webUrl);
        label(root,"施設ハブのHTTPS URL（未設定でも画面は開けます）",18); EditText hub=input(root,hubUrl);
        button(root,"検証用officeハブを選ぶ",()->hub.setText("https://raku-sika-hub-office.zapus-opah.ts.net"));
        label(root,"Tailscaleをオンにしてください。印刷先が変わるため、施設のハブURLを確認して保存します。",16);
        TextView result=label(root,"",16);
        button(root,"接続を確認（印刷しません）",()-> {
            try { String target=EndpointPolicy.hub(hub.getText().toString()); if(target.isEmpty())throw new IllegalArgumentException("ハブURLを入力してください。");probe(target,result); }
            catch(IllegalArgumentException e){result.setText(e.getMessage());}
        });
        button(root,"この接続先でライトを開く",()-> {
            try {
                String nextWeb=EndpointPolicy.web(site.getText().toString());
                String nextHub=EndpointPolicy.hub(hub.getText().toString());
                new AlertDialog.Builder(this).setTitle("接続先の確認").setMessage("画面: "+nextWeb+"\n施設ハブ: "+(nextHub.isEmpty()?"未設定":nextHub)+"\n\n登録・印刷は既存ライトの操作で実行されます。")
                    .setNegativeButton("戻る",null).setPositiveButton("開く",(d,w)-> {
                        webUrl=nextWeb;hubUrl=nextHub;
                        settings.edit().putString("web",webUrl).putString("hub",hubUrl).putBoolean("configured",true).apply();openLite();
                    }).show();
            }catch(IllegalArgumentException e){result.setText(e.getMessage());}
        });
    }

    private void probe(String target,TextView output) {
        output.setText("ハブに接続しています…");
        checks.submit(()-> {
            String message;
            HttpURLConnection conn=null;
            try {
                conn=(HttpURLConnection)new URL(target+"/health").openConnection();conn.setConnectTimeout(5000);conn.setReadTimeout(5000);conn.setInstanceFollowRedirects(false);
                int status=conn.getResponseCode();if(status!=200)throw new IllegalStateException("HTTP "+status);
                java.io.ByteArrayOutputStream buffer=new java.io.ByteArrayOutputStream();
                try(java.io.InputStream input=conn.getInputStream()){
                    byte[] chunk=new byte[4096];int count;
                    while((count=input.read(chunk))!=-1){if(buffer.size()+count>32768)throw new IllegalStateException("応答が大きすぎます");buffer.write(chunk,0,count);}
                }
                byte[] data=buffer.toByteArray();
                JSONObject health=new JSONObject(new String(data,java.nio.charset.StandardCharsets.UTF_8));
                message="ハブ接続OK / 印刷機: "+connected(health,"printer")+" / スキャナ: "+connected(health,"scanner")+" / 計量器: "+connected(health,"scale");
            } catch(Exception e) { message="ハブに接続できません。Tailscale、HTTPS URL、施設ハブの起動を確認してください。"; }
            finally { if(conn!=null)conn.disconnect(); }
            String text=message;runOnUiThread(()->{if(!isFinishing()&&!isDestroyed())output.setText(text);});
        });
    }
    private static String connected(JSONObject object,String key) { JSONObject v=object.optJSONObject(key);return v!=null&&v.optBoolean("connected")?"接続":"未接続"; }

    private void openLite() {
        disposeWeb(); LinearLayout root=layout();
        LinearLayout toolbar=new LinearLayout(this);root.addView(toolbar);
        button(toolbar,"ホーム",()-> web.loadUrl(webUrl));
        button(toolbar,"接続設定",()->new AlertDialog.Builder(this).setMessage("編集中の内容があれば保存してから接続設定へ進んでください。")
            .setNegativeButton("戻る",null).setPositiveButton("設定へ",(d,w)->showSetup()).show());
        button(toolbar,"ブラウザー",()->external(webUrl));
        button(toolbar,"スキャナ",this::chooseScanner);
        button(toolbar,"計量器",this::chooseScale);
        for(int i=0;i<toolbar.getChildCount();i++){
            Button item=(Button)toolbar.getChildAt(i);
            item.setTextSize(12);item.setMinWidth(0);item.setMinimumWidth(0);
            item.setLayoutParams(new LinearLayout.LayoutParams(0,-2,1));
        }
        connection=label(root,hubUrl.isEmpty()?"ハブ未設定 · 印刷/計量には接続設定が必要です":"施設ハブ: "+hubUrl,13);
        connection.setOnClickListener(v->{if(!hubUrl.isEmpty())probe(hubUrl,connection);});
        web=new WebView(this);root.addView(web,new LinearLayout.LayoutParams(-1,0,1));
        web.getSettings().setJavaScriptEnabled(true);
        web.getSettings().setDomStorageEnabled(true);
        web.getSettings().setAllowFileAccess(false);
        web.getSettings().setAllowContentAccess(false);
        web.getSettings().setMixedContentMode(android.webkit.WebSettings.MIXED_CONTENT_NEVER_ALLOW);
        web.getSettings().setMediaPlaybackRequiresUserGesture(true);
        CookieManager.getInstance().setAcceptCookie(true);
        CookieManager.getInstance().setAcceptThirdPartyCookies(web,false);
        WebView.setWebContentsDebuggingEnabled(BuildConfig.DEBUG);
        String resolvedHub=hubUrl.isEmpty()&&usbMode?UsbScaleRoutes.LOCAL:hubUrl;
        String setupScript="if(window===window.top){"+(resolvedHub.isEmpty()?"localStorage.removeItem('rakuSikaHubUrl');":"localStorage.setItem('rakuSikaHubUrl',"+JSONObject.quote(resolvedHub)+");")+"}";
        if(WebViewFeature.isFeatureSupported(WebViewFeature.DOCUMENT_START_SCRIPT)) {
            WebViewCompat.addDocumentStartJavaScript(web,setupScript,Collections.singleton(EndpointPolicy.origin(webUrl)));
        } else {
            connection.setText("Android System WebViewを更新してください。設定適用前の誤接続を避けるため画面を開いていません。");return;
        }
        web.setWebViewClient(new WebViewClient(){
            @Override public void onPageStarted(WebView view,String url,android.graphics.Bitmap icon){trustedPage=EndpointPolicy.sameOrigin(webUrl,url);}
            @Override public android.webkit.WebResourceResponse shouldInterceptRequest(WebView view,WebResourceRequest request){
                return usbMode?usbRoutes.intercept(request,hubUrl,webUrl,visible&&trustedPage):null;
            }
            @Override public boolean shouldOverrideUrlLoading(WebView view,WebResourceRequest request){
                if(!request.isForMainFrame())return false;
                if(EndpointPolicy.sameOrigin(webUrl,request.getUrl().toString()))return false;
                if("https".equals(request.getUrl().getScheme()))external(request.getUrl().toString());
                return true;
            }
            @Override public void onReceivedSslError(WebView view,SslErrorHandler handler,SslError error){handler.cancel();connection.setText("HTTPS証明書を確認できないため接続を停止しました。");}
            @Override public void onReceivedError(WebView view,WebResourceRequest request,WebResourceError error){if(request.isForMainFrame())connection.setText("画面に接続できません。Tailscaleと通信状態を確認してください。自動再送は行いません。");}
        });
        web.setWebChromeClient(new WebChromeClient(){
            @Override public void onPermissionRequest(PermissionRequest request){runOnUiThread(()->requestMedia(request));}
            @Override public void onPermissionRequestCanceled(PermissionRequest request){if(pendingMedia==request)pendingMedia=null;}
            @Override public boolean onShowFileChooser(WebView view,ValueCallback<Uri[]> callback,FileChooserParams params){
                if(!EndpointPolicy.sameOrigin(webUrl,view.getUrl()))return false;
                if(pendingFile!=null)pendingFile.onReceiveValue(null);pendingFile=callback;
                try{startActivityForResult(params.createIntent(),FILE_PICKER);}catch(Exception e){pendingFile.onReceiveValue(null);pendingFile=null;}
                return true;
            }
        });
        web.setDownloadListener((url,userAgent,disposition,mime,length)->download(url,userAgent,disposition,mime));
        web.loadUrl(webUrl);
        if(usbMode){connectUsb();showUsbStatus();}
    }

    private void showUsbStatus(){if(connection!=null&&usbMode)connection.setText("USB計量器: "+usbScale.status());}
    private void connectUsb(){
        if(usbMode&&visible&&selectedScale!=null&&((UsbManager)getSystemService(USB_SERVICE)).hasPermission(selectedScale))
            usbScale.open(selectedScale,()->runOnUiThread(()->{if(!isDestroyed())showUsbStatus();}));
    }
    private void chooseScale(){
        new AlertDialog.Builder(this).setTitle("計量器の接続")
            .setItems(new String[]{"AndroidのUSBへ直結する","施設ハブの計量器を使う","USBで試しに計量する（登録しません）"},(d,index)->{
                if(index==2){
                    if(!usbMode){Toast.makeText(this,"先にUSB直結を選んでください",Toast.LENGTH_LONG).show();return;}
                    checks.submit(()->{JSONObject result=usbScale.command("Q");runOnUiThread(()->{
                        if(isDestroyed())return;showUsbStatus();new AlertDialog.Builder(this).setTitle("USB計量結果")
                            .setMessage(result.optString("status").equals("error")?result.optString("message"):usbScale.status()).setPositiveButton("閉じる",null).show();
                    });});return;
                }
                new AlertDialog.Builder(this).setTitle("計量器を切り替えますか？")
                    .setMessage("画面を読み直します。編集中の内容を保存してから切り替えてください。USBが切れても、別の計量器へ自動切替はしません。")
                    .setNegativeButton("戻る",null).setPositiveButton("切り替える",(dialog,which)->{
                        usbScale.disconnect();selectedScale=null;usbMode=index==0;
                        settings.edit().putBoolean("usbScale",usbMode).apply();openLite();
                        if(usbMode)selectUsbDevice();
                    }).show();
            }).setNegativeButton("閉じる",null).show();
    }
    private void selectUsbDevice(){
        UsbManager manager=(UsbManager)getSystemService(USB_SERVICE);
        List<UsbDevice> devices=new ArrayList<>();List<String> names=new ArrayList<>();
        for(UsbDevice device:manager.getDeviceList().values())if(UsbScale.supported(device)){
            devices.add(device);names.add((device.getProductName()==null?"A&D USB / FTDI":device.getProductName())+" · "+device.getDeviceName());
        }
        if(devices.isEmpty()){
            new AlertDialog.Builder(this).setTitle("USB計量器が見つかりません")
                .setMessage("計量器の電源を入れ、USB-C–Bのデータケーブル、またはUSB-C OTG変換＋既存A–Bケーブルで接続してください。接続後もう一度USB直結を選んでください。")
                .setPositiveButton("閉じる",null).show();return;
        }
        new AlertDialog.Builder(this).setTitle("接続した計量器を選択")
            .setItems(names.toArray(new String[0]),(d,index)->{
                selectedScale=devices.get(index);
                if(manager.hasPermission(selectedScale))connectUsb();
                else manager.requestPermission(selectedScale,PendingIntent.getBroadcast(this,0,new Intent(usbPermission).setPackage(getPackageName()),PendingIntent.FLAG_UPDATE_CURRENT|PendingIntent.FLAG_MUTABLE));
            }).setNegativeButton("戻る",null).show();
    }

    private void requestMedia(PermissionRequest request) {
        if(!visible||!EndpointPolicy.sameOrigin(webUrl,request.getOrigin().toString())){request.deny();return;}
        List<String> permissions=new ArrayList<>();
        for(String resource:request.getResources()){
            String permission;
            if(PermissionRequest.RESOURCE_AUDIO_CAPTURE.equals(resource))permission=Manifest.permission.RECORD_AUDIO;
            else if(PermissionRequest.RESOURCE_VIDEO_CAPTURE.equals(resource))permission=Manifest.permission.CAMERA;
            else {request.deny();return;}
            if(checkSelfPermission(permission)!=PackageManager.PERMISSION_GRANTED)permissions.add(permission);
        }
        denyPendingMedia();pendingMedia=request;
        if(permissions.isEmpty()){request.grant(request.getResources());pendingMedia=null;}
        else requestPermissions(permissions.toArray(new String[0]),MEDIA_PERMISSION);
    }
    private void chooseScanner(){
        List<String> names=new ArrayList<>(),descriptors=new ArrayList<>();
        names.add("施設ハブのスキャナを使う");descriptors.add("");
        for(int id:InputDevice.getDeviceIds()){
            InputDevice device=InputDevice.getDevice(id);
            if(device!=null&&device.isExternal()&&!device.isVirtual()&&(device.getSources()&InputDevice.SOURCE_KEYBOARD)==InputDevice.SOURCE_KEYBOARD){names.add(device.getName());descriptors.add(device.getDescriptor());}
        }
        new AlertDialog.Builder(this).setTitle("スキャナを選択（USB / Bluetooth HID）")
            .setItems(names.toArray(new String[0]),(dialog,index)->{
                scannerDescriptor=descriptors.get(index);scannerBuffer.clear();settings.edit().putString("scanner",scannerDescriptor).apply();
                connection.setText(index==0?"施設ハブのスキャナを使用します":"Android直結: "+names.get(index)+" · ライトの読取画面を開いてください");
            }).setNegativeButton("戻る",null).show();
    }
    private boolean handleScannerKey(KeyEvent event){
        InputDevice device=event.getDevice();
        if(visible&&web!=null&&!scannerDescriptor.isEmpty()&&device!=null&&device.isExternal()&&!device.isVirtual()&&scannerDescriptor.equals(device.getDescriptor())){
            if(event.getAction()==KeyEvent.ACTION_DOWN&&event.getRepeatCount()==0){
                if(event.isCtrlPressed()||event.isAltPressed()||event.isMetaPressed())scannerBuffer.clear();
                else {
                    String scan=scannerBuffer.accept(event.getUnicodeChar(),event.getKeyCode()==KeyEvent.KEYCODE_ENTER||event.getKeyCode()==KeyEvent.KEYCODE_NUMPAD_ENTER,event.getEventTime());
                    if(scan!=null)ScannerDelivery.deliver(web,webUrl,scan);
                }
            }
            return true;
        }
        return false;
    }
    private void denyPendingMedia(){if(pendingMedia!=null){pendingMedia.deny();pendingMedia=null;}}
    @Override public void onRequestPermissionsResult(int code,String[] permissions,int[] results){
        super.onRequestPermissionsResult(code,permissions,results);
        if(code!=MEDIA_PERMISSION||pendingMedia==null)return;
        boolean all=results.length>0;for(int value:results)all&=value==PackageManager.PERMISSION_GRANTED;
        if(all&&EndpointPolicy.sameOrigin(webUrl,pendingMedia.getOrigin().toString()))pendingMedia.grant(pendingMedia.getResources());else pendingMedia.deny();pendingMedia=null;
    }
    @Override protected void onActivityResult(int request,int result,Intent data){
        super.onActivityResult(request,result,data);
        if(request==FILE_PICKER&&pendingFile!=null){pendingFile.onReceiveValue(WebChromeClient.FileChooserParams.parseResult(result,data));pendingFile=null;}
    }
    private void external(String url){
        if(!"https".equals(Uri.parse(url).getScheme()))return;
        try{startActivity(new Intent(Intent.ACTION_VIEW,Uri.parse(url)));}catch(Exception e){Toast.makeText(this,"ブラウザーを開けません",Toast.LENGTH_SHORT).show();}
    }
    private void download(String url,String agent,String disposition,String mime){
        if(!EndpointPolicy.sameOrigin(webUrl,url)){Toast.makeText(this,"このファイルはブラウザーからダウンロードしてください",Toast.LENGTH_LONG).show();return;}
        new AlertDialog.Builder(this).setTitle("ファイルを保存しますか？").setNegativeButton("戻る",null).setPositiveButton("保存",(d,w)->{
            try{
                DownloadManager.Request request=new DownloadManager.Request(Uri.parse(url));
                String cookie=CookieManager.getInstance().getCookie(url);if(cookie!=null)request.addRequestHeader("Cookie",cookie);
                request.addRequestHeader("User-Agent",agent);request.setMimeType(mime);request.setNotificationVisibility(DownloadManager.Request.VISIBILITY_VISIBLE_NOTIFY_COMPLETED);
                request.setDestinationInExternalPublicDir(Environment.DIRECTORY_DOWNLOADS,URLUtil.guessFileName(url,disposition,mime));
                ((DownloadManager)getSystemService(DOWNLOAD_SERVICE)).enqueue(request);
            }catch(Exception e){Toast.makeText(this,"保存できませんでした",Toast.LENGTH_SHORT).show();}
        }).show();
    }
    @Override protected void onResume(){super.onResume();visible=true;if(web!=null)web.onResume();connectUsb();}
    @Override protected void onPause(){visible=false;usbScale.disconnect();scannerBuffer.clear();if(web!=null&&pendingMedia==null){web.evaluateJavascript("window.dispatchEvent(new Event('station-pause'))",null);web.onPause();}super.onPause();}
    @Override protected void onStop(){denyPendingMedia();super.onStop();}
    @Override protected void onDestroy(){((InputManager)getSystemService(INPUT_SERVICE)).unregisterInputDeviceListener(inputListener);unregisterReceiver(usbReceiver);disposeWeb();usbScale.destroy();checks.shutdownNow();super.onDestroy();}
}
