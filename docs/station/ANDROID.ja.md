# 既存ライトを使うAndroidクライアント（2026-11-22）

既存のラクシカライトをAndroid WebViewで開き、画面・メール/パスワード認証・在庫/販売/ラベル操作を再利用する検証アプリ。
独自のフォームや在庫DBを複製しない。Go/Pipecatの新しい音声試作を本番UIへ統合したものではない。
本番Webコード、現場施設の設定、Firestoreのデータは変更していない。

## 接続構成

```text
Androidの既存ライト ─ HTTPS/Tailscale ─ 施設Goハブ ─ USB/LAN ─ 印刷機・スキャナ・計量器
        │
        └ USB/Bluetooth HIDスキャナ（明示的に機器を選択）
```

- アプリID：`com.deervery.rakusikaliteclient`。既存の会話PoC/直接接続PoCとは別アプリ。
- 初回にライトURLとハブURLを確認して保存。既存ライトの`rakuSikaHubUrl`設定を、信頼したオリジンの文書開始時に適用する。
- HTTPSのみ。証明書エラーを無視せず、HTTP混在コンテンツと任意サイトへのWebView内遷移を許可しない。
- Geminiキー/サービスアカウントをAPKに含めない。任意のネイティブ操作を呼べるJavaScriptInterfaceも設けない。
- カメラ/マイクは信頼した画面から要求された時だけOS許可を取得。ファイル選択、同一オリジンHTTPSファイルの保存をサポート。
- Android 16の戻るジェスチャーはAndroidXのOnBackPressedDispatcherを使う。
- Bluetooth/USB HIDスキャナはAndroidで接続後、アプリの「スキャナ」で**対象機器を選ぶ**。その機器の入力だけをまとめ、既存ライトのHID経路へ1回渡す。ソフトキーボードや他の外部機器は捕捉しない。
- 途中切れ・500ms超の間隔・2048文字超・背景移行・デバイス変更で未完スキャンを破棄。先頭ゼロを保持する。
- ハブ側のスキャナが接続中の場合、既存ライトはハブ経路を優先する。Android直結とハブ接続を同時に有効にせず、利用する側を選ぶ。

## office検証ハブ

運用手順で検証用とされているofficeだけに、Tailscale Serveを追加した。
追加前のServe設定は空。既存Hubのバイナリ・CUPSキュー・サービス設定は変更していない。

URL：`https://raku-sika-hub-office.zapus-opah.ts.net`

既存HubのLANOnlyは100.64.0.0/10を許可せず、Tailscale IPへの直接HTTPは403になった。
LANOnlyを一括変更せず、Tailscale Serve→127.0.0.1:19800経由にした。HTTPSの`/health`が200になることを確認済み。

```sh
# officeで実施済み（本番施設には実施していない）
sudo tailscale serve --bg http://127.0.0.1:19800
# この検証公開だけを取り消す場合
sudo tailscale serve --https=443 off
```

初回確認ではプリンタ・スキャナ・計量器すべて未接続。
その後officeのUSBにNewtologic 4010E（VID 060e / PID 0960）が存在し、Hubの識別設定が未設定と判明。
`/etc/systemd/system/raku-sika-hub.service.d/60-android-office-scanner.conf` にSCANNER_VID/SCANNER_PIDを追加し、officeのHubのみ再起動した。
`/health`のscanner.connected=true、device=/dev/input/event4を確認済み。実バーコードの読取りは未実施。
取消時はこのdrop-inファイルだけを削除してdaemon-reloadとHub再起動を行う。既存設定ファイルは変更していない。
プリンタの既存キューは`rakuql://`のUSB構成で、指定シリアルのUSB機器が見つからないとの応答だった。
別途、既知の試験プリンタ192.168.40.123:9100へのTCP接続は成功したが、USBキューをWi-Fi対応に切り替えたわけではない。
**このAPKでの物理印刷・スキャナ実機読取りは未検証。** 過去の別PoCのWi-Fi/BT印刷成功とは区別する。
officeの試験印刷は利用者が現物を確認できるときに短いラベル数枚まで。今回自動印刷はしていない。

## ビルドと検証

ソース：`experimental/android-client`。Android SDK 36、Build Tools 36.0.0、JDK17以上、Gradle8.13。

```sh
cd experimental/android-client
./gradlew :clientCore:test :client:assembleDebug :client:lintDebug :client:assembleDebugAndroidTest
adb -s emulator-5554 install -r client/build/outputs/apk/debug/client-debug.apk
adb -s emulator-5554 install -r client/build/outputs/apk/androidTest/debug/client-debug-androidTest.apk
adb -s emulator-5554 shell am instrument -w com.deervery.rakusikaliteclient.test/androidx.test.runner.AndroidJUnitRunner
```

エミュレーター試験はローカルHTMLのみ。業務データの登録、実機印刷、利用者アカウントでのログインを行わない。
WebViewへのキーイベントの直接転送では読取りを再現できなかったため、選択済み物理HID機器→バッファ→信頼したWebViewのHIDイベントという経路を実装した。
コードからのスキャン配送試験と、物理スキャナのBluetooth/USB互換性試験は別である。

残確認：Pixel/LAVIEの実機ログイン、実際の登録/印刷/再印刷、HIDスキャナ型番別の動作、カメラ撮影/添付、再接続、背景復帰、長時間稼働。
Google認証ポップアップ、blob URLのダウンロード、Web側でNEXT_PUBLIC_HUB_URLが固定されている構成、全フォームのタッチ操作は互換性確認が必要。
既存ライトのHubスキャナポーリングは背景移行後の復帰経路も追加検証する。未検証を本番同等動作の保証としない。

## 計量器：HV-60KC-KのBluetooth化

第一候補は**HVW-03CJA（RS-232C）＋AD-8541-SCALE-JA**。
メーカーの2026年11月接続表ではHV-C/CP-Kの対応が示され、内蔵HVW-27CJAは検定付きモデルで使用不可とされている。
HV-C（末尾-Kなし）と同じオプション選定にしない。[メーカー接続表・5ページ](https://www.aandd.co.jp/pdf_storage/tech_doc/balance/t_bluetooth_comm_connection.pdf)

| 部品 | メーカー標準価格・税別 | 意味 |
|---|---:|---|
| HVW-03CJA | 8,000円 | D-Sub9P RS-232C出力 |
| AD-8541-SCALE-JA | 9,000円 | 計量器側Bluetoothコンバーター |
| 合計 | 17,000円 | 取付費・送料等を除く。RS-232C装着済みならコンバーター追加分のみ |

価格・仕様：[HV-C-Kのオプション](https://www.aandd.co.jp/products/weighing/balance/bal-approved/hv-c-k/)、[AD-8541-SCALE](https://www.aandd.co.jp/products/weighing/balance/bal-peripherals/ad8541-scale/)。
利用者から「現在USBモジュールを取り付け済み、元はRS端子だった記憶がある」と確認した。
元のRS-232C部品が保管されていれば、型番・欠品・再装着条件を確認して再利用を検討する。
現物がUSBインタフェースHVW-02CBJAの場合、それをRS-232Cコネクターとして扱うことはできない。購入前に実装済みオプションと取付条件を確認する。

HID方式はスマートフォンへキーボード入力できるが、端末からコマンドを送れない。
ラクシカで重量・単位・安定状態・受信時刻を分けて扱い、Q/T/Rなどの双方向通信も行うなら、A&Dの通信仕様/SDKを確認して実装する。
メーカーは開発会社向けに仕様書/SDKの申込み窓口を用意している。[メーカーのBluetooth接続説明](https://www.aandd.co.jp/products/weighing/balance/bal-peripherals/bluetooth_communications/)
接続表には、この系列は?SN/?TN未対応でWeiVからの照会に「?」等を返す場合があるとの注記もある。機種識別コマンド失敗を計量データ断と混同しない。

AD-8541-SCALEはBLEであり、一般的なBluetooth Classic SPPアダプターとして実装しない。
公称最大10mは工場内の金属・水・機器配置での保証距離ではない。実際の配置で切断・再接続・データ欠損を試験する。
またHIDの重量入力とHIDスキャナを同じ入力列へ混ぜず、機器ごとの識別と業務状態で受け先を分ける。

自作するなら外部RS-232C側での方式変換を検討する。RS-232Cと3.3V UARTを直結せず、電圧変換、電源、筐体、通信エラー処理が必要。
USB出力から作るならUSBホストと該当USBシリアルドライバーまで必要で、単なるBluetooth USBドングルの差込みでは代替できない。
今回の第一段階では純正構成との互換性検証を優先し、分解や内部改造は行わない。

## Geminiキー入力カード

`experimental/station/secret_card.py`で、tailnet HTTPS・ワンタイムアクセス・秘密値非表示の入力画面を作成した。
保存先はリポジトリ外の所有者専用領域（ファイル0600、ディレクトリ0700）。前後の空白/改行と長い形式に対応。
Geminiのモデル一覧APIで200を確認した。キーの値はログ・Git・APK・Driveへ保存しない。
保存確認後に入力用HTTPS公開を停止し、アクセス用トークンを削除した。
再利用呼び出し名は「Tailscaleキー入力カード」。1行メモを共有DriveとCodexのAGENTS.mdに残した。

## 今回の確認結果

- Android 16エミュレーター：既存Liteの/homeからログイン画面へ遷移。縦1080×1920で主要項目が収まることを確認。利用者の認証情報は入力していない。
- clientCore：URL検証4件、スキャン分割/タイムアウト/オーバーフロー等5件、計9件成功。
- Android instrumentation：文書開始時のハブ設定、スキャン1回配送（先頭ゼロ保持）の2件成功。
- 初回のスキャン配送試験はloadDataWithBaseURLのhistoryUrl未設定によるabout:blank判定で拒否された。試験のURLを明示して修正し、安全な同一オリジンチェックは維持した。
- assembleDebug / lintDebug成功。GoのHTTP API・プリンタ・スキャナの既存テストも成功。
- office USBスキャナ認識成功。プリンタは指定USB機器が見つからない状態。Wi-Fiへの自動切替・実印刷は行っていない。
- Android直結用はBluetooth HIDスキャナを想定（利用者確認済み）。現在のUSB機器自体にBluetooth機能があるとは扱わない。

## Pixelからインストール

okyohe.githubのTailscaleを有効にして `https://glwin.zapus-opah.ts.net:8445/index.html` を開く。
APKをダウンロードして「ラクシカライト 検証」をインストール。必要に応じ、このブラウザーからのインストールをAndroid設定で許可する。
初回は「検証用officeハブを選ぶ」→「接続を確認」→「この接続先でライトを開く」。既存のメール/パスワードでログインする。
BluetoothスキャナはOS設定でHID接続し、アプリ上部の「スキャナ」でその機器を選ぶ。施設ハブ側スキャナと同時併用しない。
本アプリは既存の業務データに接続する。検証施設/対象を選び、登録・印刷は内容確認後に操作する。
このAPKは検証用debug署名で、WebViewデバッグが有効。本番配布では署名・バージョン管理・debug無効化を別途行う。
配布ディレクトリにはAPKと説明ページだけを置く。秘密ファイル・ソースツリーは公開しない。公開取消はglwinで `sudo tailscale serve --https=8445 off`。

## v0.2.0：既存USBモジュールからAndroidへ直結

Bluetooth部品を買い足す前に、現在のUSBモジュールを利用する構成を優先する。
購入メールでHVW-02CBJAを確認：2026-03-03発送分2個、2026-06-16注文分4個（税抜単価7,941円）。
2月にはHVW-02BCWPJA（mini-B）とA–mini-Bケーブルの注文記録もあるため、混同しない。
購入履歴は手元・officeへの割当を保証するものではないが、officeではFTDI VID0403/PID6015のUSB機器も確認されている。

- HVW-02CBJA：通常のUSB Type-B。USB-C–Bデータケーブル、またはUSB-C OTG（Aメス）＋既存A–Bケーブル。
- HVW-02BCWPJA：USB mini-B。ケーブル端子が異なる。
- USB-C端子の存在だけでホスト動作を保証しない。AndroidのUSBホスト対応と、実機での列挙・許可・通信を確認する。
- 充電専用ケーブルは不可。常設で充電を続ける場合はPD給電とUSBデータを同時利用できるドックを別途実機確認する。

根拠：[A&D補足説明書](https://www.aandd.co.jp/pdf_storage/tech_doc/balance/t_hvw-02cbja_hvw-02bcwpja_hosoku.pdf)、[Android USBホスト](https://developer.android.com/develop/connectivity/usb/host)。
メーカー説明はPC向けで、Android動作保証の表明ではない。

実装：usb-serial-for-android 3.11.0のFTDIドライバーを利用し、VID0403/PID6015を明示選択する。MITライセンスをAPKのassetsへ同梱。
USB許可はOSのダイアログで取得。通信は既存ハブと同じ2400 baud・7ビット・even parity・1 stop bit。
Qで計量、Tで風袋、Rでゼロ。風袋・ゼロは対応するACKを受けたときだけ成功とし、応答不明時の自動再送はしない。
各問い合わせ前に残データを破棄し、分割フレームを再構成する。USはweighing/未確定、OL・破損・タイムアウトはエラー。
抜線・背景移行・接続変更で処理世代を無効化し、処理中の古い応答を破棄する。USB選択中に別のHub計量器へ自動フォールバックしない。

既存Liteのfetch契約（/health、/scale/weigh、/scale/tare、/scale/zero）をAndroid WebViewのリクエスト処理で接続する。
対象は設定済みHubのURLに限定し、信頼したLiteのOriginと前景画面を確認する。印刷・スキャナのリクエストは従来のHubへ送る。
Hub未設定でもUSB計量の模擬テストは成功。USBモードでもWeb側NEXT_PUBLIC_HUB_URLが固定されている配布物は互換性確認が必要。
任意のJavaScriptInterfaceは追加していない。本番Webのコードと既存施設の設定は変更していない。

操作：アプリ上部「計量器」→「AndroidのUSBへ直結する」→画面再読込みの確認→機器選択→USB利用を許可。
「USBで試しに計量する」は読取りだけで業務登録しない。確認後は既存フォームの計量操作を使う。
通信エラー後はケーブル・計量器設定を確認して機器を選び直す。アプリ再起動後も対象機器を明示選択する。

検証：Java単体17件（USB関連8件追加）、Android連携4件（USB関連2件追加）成功。
WebViewから既存fetch契約で模擬デバイスの0.07kg・安定を取得し、未許可Origin/背景状態/誤メソッドを拒否することを確認。
旧重量の破棄、分割受信、不安定値、過負荷、破損、長大フレーム、抜線途中の応答破棄、ACK照合と再送しないことをテスト。
assembleDebug/lintDebug成功。これは実USBの電気接続・ドライバー実通信成功を意味しない。Pixel/LAVIE＋実計量器での検証は未実施。
