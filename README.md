# raku-sika-hub

## Lane 2 申し送り

- Pi 側の systemd override では `PRINTER_NAME=Brother_QL-820NWB` を前提にする
- 開発中に複数接続で切り分ける場合は `MAX_CLIENTS=3` などを設定する
- 開発用 Origin は `ALLOWED_ORIGINS=localhost:*,127.0.0.1:*,192.168.50.*` を使う
- スケール未接続の mock 動作確認では `SCALE_DRIVER=mock` を設定する

```ini
[Service]
Environment=PRINTER_NAME=Brother_QL-820NWB
Environment=PRINTER_DRIVER=ptouch_template
Environment=MAX_CLIENTS=3
Environment=SCALE_DRIVER=mock
Environment=ALLOWED_ORIGINS=localhost:*,127.0.0.1:*,192.168.50.*
```

反映後:

```bash
sudo systemctl daemon-reload
sudo systemctl restart raku-sika-hub
curl http://127.0.0.1:19800/health
wscat -c ws://192.168.50.40:19800
```

Raspberry Pi 上で常駐する機器制御 WebSocket ゲートウェイ（Go 実装）。
A&D 防塵・防水デジタル台はかり HV-C シリーズ（HV-60KCWP-K 等）と Brother ラベルプリンタ（QL-800/QL-820 シリーズ）を制御し、フロントエンド（[raku-sika-lite](https://github.com/deervery/raku-sika-lite)）に対して WSA 互換の WebSocket API を提供する。

Brother ラベル印刷は v1 では `P-touch Template` を採用する。`.lbx` は Linux 上で直接解釈せず、Windows の `P-touch Editor` / `P-touch Transfer Manager` でプリンタへ事前転送し、hub は template key と差し込みデータだけを送る。

[raku-sika-wsa](https://github.com/deervery/raku-sika-wsa)（Windows 版 Go サービス）と同一構成・同一プロトコルの Linux/Raspberry Pi 版。

## システム構成

```
┌─────────────────┐     WebSocket      ┌──────────────────┐     Serial/USB     ┌──────────────┐
│  raku-sika-lite │ ◄────────────────► │  raku-sika-hub   │ ◄────────────────► │  A&D HV-C    │
│  (Next.js)      │  ws://*.local:19800│  (Go)            │   2400bps 7E1     │  シリーズ      │
│  ブラウザ上で動作 │                    │  Raspberry Pi    │                    │  (HV-60KCWP-K)│
└─────────────────┘                    │                  │     CUPS/lp       ┌──────────────┐
                                       │                  │ ◄────────────────► │  Brother      │
                                       └──────────────────┘                    │  QL-800/820   │
                                                                               └──────────────┘
```

### 関連リポジトリ

| リポジトリ | 役割 |
|-----------|------|
| [raku-sika-wsa](https://github.com/deervery/raku-sika-wsa) | Windows 版 Go サービス（リファレンス実装）。hub はこの Linux/Pi 移植版 |
| [raku-sika-lite](https://github.com/deervery/raku-sika-lite) | Next.js フロントエンド。ブラウザから hub へ WebSocket 接続して計量・印刷を操作 |

### wsa との違い

| | wsa (Windows) | hub (Raspberry Pi) |
|---|---|---|
| OS | Windows | Linux (Raspberry Pi OS) |
| サービス管理 | Windows SCM (install/uninstall) | systemd |
| FTDI 検出 | Windows レジストリ (`FTDIBUS`) | Linux sysfs (`/sys/bus/usb-serial/`) |
| Listen | `127.0.0.1:19800` | `0.0.0.0:19800` (LAN 公開) |
| プリンタ | なし | Brother QL-800/QL-820 (CUPS/lp) |
| print_test | なし | あり |
| 接続制限 | なし | `maxClients: 1` (設定変更可) |
| エラーメッセージ | 英語 | 日本語（現場対応用） |
| 設定 | `%ProgramData%\RakushikaScale\config.json` | `./config.json` |

## 現在の開発状況

### 実装済み

| 機能 | 状態 | 備考 |
|------|------|------|
| WebSocket サーバー (port 19800) | **実装済** | WSA 互換プロトコル、CORS 制御、単一クライアント制限 |
| スケール制御 (weigh/tare/zero) | **実装済・未テスト** | wsa と同一の scale.Client。実機到着待ち |
| FTDI ポート自動検出 (Linux sysfs) | **実装済・未テスト** | `/sys/bus/usb-serial/devices/` から VID/PID で検出 |
| 3秒間隔の再接続ループ | **実装済** | wsa と同一パターン |
| print_test ハンドラ | **実装済** | CUPS lp コマンド経由 |
| ラベル印刷 (print ハンドラ) | **実装済** | 5テンプレート対応。P-touch Template 優先、必要時のみ CUPS PNG fallback |
| BrotherPrinterDriver | **実装済** | `ptouch_template` と `cups_png` の2経路 |
| QR コード生成 (traceable) | **実装済** | skip2/go-qrcode。トレサビリティURL自動生成 |
| 包括的エラー分岐（18種） | **実装済** | 日本語メッセージで現場対応しやすく |
| A&D プロトコルパーサ | **テスト済** | wsa と同一。unit test あり |
| scale.Client unit test | **テスト済** | mock port でテスト |
| 月次ログローテーション | **実装済** | wsa と同一。1年で自動クリーンアップ |
| systemd サービスファイル | **実装済** | `deploy/raku-sika-hub.service` + setup.sh で自動配置 |
| CUPS プリンタセットアップ | **実装済** | `deploy/setup-printer.sh` で Brother QL 自動検出・登録 |
| 日本語フォント自動インストール | **実装済** | setup.sh で `fonts-noto-cjk` を自動インストール |

### 未実装・TODO

- **A&D 実機テスト**: デバイス到着後に Pi 上での実機検証が必要
- **ラベル印刷実機テスト**: CUPS + Brother QL-800/QL-820 での印刷品質・レイアウト確認

## セットアップ

### 前提条件

- Go 1.24+
- Raspberry Pi OS (Debian Bookworm)

### ビルド

```bash
git clone https://github.com/deervery/raku-sika-hub.git
cd raku-sika-hub
go mod tidy
go build -o raku-sika-hub .
```

### クロスコンパイル（開発 PC から Pi 向け）

```bash
GOOS=linux GOARCH=arm64 go build -o raku-sika-hub .
# Pi へ転送
scp raku-sika-hub rakusika@raku-sika-hub.local:~/
```

## 起動方法

```bash
# 設定ファイルなし（デフォルト設定で起動）
./raku-sika-hub

# 設定ファイルあり
cat > config.json << 'EOF'
{
  "vid": "0403",
  "pid": "6015",
  "port": "",
  "baudRate": 2400,
  "dataBits": 7,
  "parity": "even",
  "stopBits": 1,
  "printerName": "Brother_QL-820NWB",
  "printerDriver": "ptouch_template",
  "printerAddress": "",
  "templateMapPath": "templates/siknue/template-map.json",
  "maxClients": 1,
  "listenAddr": "0.0.0.0:19800",
  "logLevel": "INFO",
  "scaleDriver": "mock",
  "allowedOrigins": ["localhost:*", "127.0.0.1:*", "192.168.50.*"]
}
EOF
./raku-sika-hub
```

`PRINTER_NAME` 環境変数を設定した場合は、`config.json` の `printerName` より優先される。
`printerName` を空にした場合だけ、hub は起動時と印刷時に CUPS の default destination を参照し、未設定なら `lpstat -p` の先頭プリンタへフォールバックする。
`MAX_CLIENTS`, `SCALE_DRIVER`, `ALLOWED_ORIGINS`, `ALLOW_ALL_ORIGINS`, `PRINTER_DRIVER`, `PRINTER_ADDRESS`, `TEMPLATE_MAP_PATH` も環境変数で上書きできる。

### ポートを明示指定する場合

```json
{
  "port": "/dev/ttyUSB0"
}
```

`port` が空（デフォルト）の場合、FTDI VID/PID で自動検出する。

## 設定 (config.json)

| フィールド | デフォルト | 説明 |
|-----------|-----------|------|
| `vid` | `"0403"` | FTDI ベンダー ID（自動検出用） |
| `pid` | `"6015"` | FTDI プロダクト ID（自動検出用） |
| `port` | `""` (自動検出) | シリアルポートパス（例: `/dev/ttyUSB0`） |
| `baudRate` | `2400` | ボーレート（A&D デフォルト: 2400） |
| `dataBits` | `7` | データビット（A&D デフォルト: 7） |
| `parity` | `"even"` | パリティ（A&D デフォルト: even） |
| `stopBits` | `1` | ストップビット |
| `printerName` | `"Brother_QL-820NWB"` | CUPS raw 経由で P-touch Template を送る場合のプリンタ名 |
| `printerDriver` | `"ptouch_template"` | `ptouch_template` または `cups_png` |
| `printerAddress` | `""` | P-touch Template を TCP 9100 へ直接送る場合のアドレス。未指定時は `lp -o raw` で CUPS キューへ送る |
| `templateMapPath` | `"templates/siknue/template-map.json"` | template key と差し込み順を定義した JSON |
| `fontPath` | `""` (自動検出) | 日本語フォントパス（ラベル画像生成用） |
| `maxClients` | `1` | WebSocket 同時接続数上限（v1: 1台） |
| `listenAddr` | `"0.0.0.0:19800"` | WebSocket サーバーのリッスンアドレス |
| `logLevel` | `"INFO"` | ログレベル（`ERROR` / `WARN` / `INFO`） |
| `scaleDriver` | `""` | 起動ログに出すスケールドライバモード。systemd では `mock` / `real` 等を設定可能 |
| `allowedOrigins` | `["localhost:*", "127.0.0.1:*", "192.168.50.*", ...]` | 開発時に許可する WebSocket Origin パターン |
| `allowAllOrigins` | `false` | `true` で WebSocket Origin 制限を無効化 |

## WebSocket API（WSA 互換）

### 接続

```
ws://raku-sika-hub.local:19800
ws://192.168.50.40:19800
```

接続直後にサーバーから `connection_status` メッセージが自動送信される。
スケールの接続状態が変化した場合も、全クライアントにブロードキャストされる。

HTTP ヘルスチェックは同じ 19800 番ポートの `GET /health` で受ける。`/` は WebSocket Upgrade 用で、通常の `curl /` には 426 が返る。
印刷キュー確認は `GET /printer/queue`、全削除は `DELETE /printer/queue`。どちらも `ALLOWED_ORIGINS` に一致する Origin からの HTTP 呼び出しを許可する。

## P-touch Template 運用

### 手作業テンプレート転送

1. Windows で `P-touch Editor` を開く
2. 既存 `.lbx` を修正する
3. `P-touch Transfer Manager` で QL-820NWB にテンプレートを転送する
4. 各テンプレートに `Key Assign Number` を振る
5. 差し込み順を [template-map.json](/home/okyohe/gibier/raku-sika-hub/templates/siknue/template-map.json) と一致させる
6. プリンタ側の `P-touch Template` 設定では command prefix を `^`、print start command を `^FF`、delimiter を `TAB` に揃える

hub は `.lbx` 自体を読み込まない。実行時に使うのは `template-map.json` の `key` と `fields` だけ。

### template-map.json

`templates/siknue/template-map.json` は lite の `template` 名と、プリンタ内テンプレート key の対応表。

```json
{
  "encoding": "shift_jis",
  "templates": {
    "traceable": {
      "key": 1,
      "fields": ["productName", "productQuantity", "deadlineDate", "storageTemperature", "individualNumber", "captureLocation", "qrCode"]
    }
  }
}
```

`fields` の順番が、そのまま P-touch Template へ流し込む順番になる。Windows 側テンプレート修正時に差し込み順が変わったら、この JSON も更新する。
`encoding` は P-touch Template へ送る文字コード。現在の Siknue テンプレートは日本語文字化けを避けるため `shift_jis` を前提にしている。
hub は各ジョブ送信時に `ESC i a '3'` で P-touch Template モードへ切り替え、続けて `^II` で初期化してから `^TS...^FF` を送る。プリンタ本体に文字化けしたテキストが表示される場合は、まず hub が古いバイナリのまま動いていないか確認する。

`processed` と `pet` は `raku-sika-lite` のテンプレートプリセットと同じ差し込み順に合わせる:
- `processed`: `productName`, `productQuantity`, `deadlineDate`, `storageTemperature`, `productIngredient`, `nutritionUnit`, `caloriesQuantity`, `proteinQuantity`, `fatQuantity`, `carbohydratesQuantity`, `saltEquivalentQuantity`, `isHeatedMeatProducts`, `attentionText`
- `pet`: `productName`, `productQuantity`, `deadlineDate`, `storageTemperature`, `productIngredient`, `nutritionUnit`, `attentionText`

### CORS

以下のオリジンからの接続を許可:
- `localhost:*` / `127.0.0.1:*`
- `192.168.50.*`
- `rakusika.com` / `*.rakusika.com`
- `preview.rakusika.com`

必要なら `ALLOW_ALL_ORIGINS=true` で開発中だけ無効化できる。

### 接続制限（v1: 単一クライアント）

v1 の推奨デフォルトは単一クライアント接続（`maxClients: 1`）。
制限は WebSocket Upgrade リクエストのみに適用される。HTTP `GET /health` は 429 にならない。
2台目以降の WebSocket 接続は HTTP 429 で拒否される:
```
Too Many Connections: 既に別のクライアントが接続中です。既存の接続を切断してから再試行してください。
```

### Client → Server メッセージ

すべてのリクエストは JSON 形式。`requestId` はレスポンスの紐付けに使用する任意の文字列。

| type | fields | 説明 |
|------|--------|------|
| `weigh` | `requestId` | 安定重量を取得。不安定時は最大10回リトライ（500ms 間隔） |
| `tare` | `requestId` | 風袋引き |
| `zero` | `requestId` | ゼロリセット |
| `health` | `requestId` | スケール・プリンタ接続状態のヘルスチェック |
| `status` | — | 現在の接続状態を取得 |
| `print_test` | `requestId` | テスト印刷を実行 |
| `print` | `requestId`, `template`, `copies`, `data` | ラベル印刷を実行（下記参照） |

### Server → Client メッセージ

| type | fields | 説明 |
|------|--------|------|
| `weight` | `requestId`, `value`, `unit`, `stable` | 安定重量の取得結果 |
| `weighing` | `requestId`, `retry`, `maxRetry` | 計量中の進捗（不安定時のリトライ回数） |
| `tare_ok` | `requestId` | 風袋引き成功 |
| `zero_ok` | `requestId` | ゼロリセット成功 |
| `health_ok` | `requestId`, `connected`, `port`, `printerConnected` | ヘルスチェック応答 |
| `connection_status` | `connected`, `port` | 接続状態（接続時自動送信 + 状態変化時ブロードキャスト） |
| `print_test_ok` | `requestId`, `message?` | テスト印刷成功 |
| `print_test_error` | `requestId`, `message` | テスト印刷失敗 |
| `print_ok` | `requestId`, `message`, `copies` | ラベル印刷成功 |
| `print_error` | `requestId`, `code`, `message` | ラベル印刷失敗 |
| `error` | `requestId`, `code`, `message` | エラー |

### エラーコード

全エラーに日本語の「原因 + 現場での対処法」を含む。lite 側でそのまま `message` を表示すれば対応可能。

#### スケール関連

| code | 意味 | 現場での対処 |
|------|------|-------------|
| `SCALE_NOT_CONNECTED` | スケール未接続 | USBケーブル確認。自動再接続が3秒間隔で試行中 |
| `SCALE_BUSY` | 別コマンド処理中 | 少し待って再試行 |
| `TIMEOUT` | 応答タイムアウト（3秒） | スケール電源確認、USBケーブル抜き差し |
| `UNSTABLE` | 計量値不安定（10回リトライ超過） | 計量台の振動・風を確認、物が動いていないか確認 |
| `OVERLOAD` | 過負荷（60kg超） | 荷物を降ろす |
| `SERIAL_WRITE_ERROR` | シリアル書き込み失敗 | USBケーブルが抜けた可能性 |
| `PERMISSION_DENIED` | ポートアクセス権限なし | `sudo usermod -aG dialout $USER` 実行後に再ログイン |
| `PORT_IN_USE` | ポートが別プロセスで使用中 | 他にスケールを使用しているプログラムを終了 |
| `PORT_NOT_FOUND` | シリアルポートが存在しない | USBケーブル接続確認 |
| `FTDI_NOT_FOUND` | FTDI デバイス未検出 | USBケーブル確認、スケール電源ON |
| `SCALE_NO_RESPONSE` | ポートは開いたが応答なし | ボーレート設定（2400bps）がスケール側と一致しているか確認 |
| `UNEXPECTED_RESPONSE` | 予期しないスケール応答 | スケールの通信設定を確認 |
| `TARE_FAILED` | 風袋引き失敗 | スケールの状態確認 |
| `ZERO_FAILED` | ゼロリセット失敗 | 計量台に物が乗っていないか確認 |

#### プリンタ関連

| code | 意味 | 現場での対処 |
|------|------|-------------|
| `PRINTER_NOT_CONFIGURED` | CUPSにプリンタ未登録 | Pi上で `apt-get install printer-driver-ptouch` + CUPS設定 |
| `PRINTER_OFFLINE` | プリンタへ到達不可 | 電源、USB/LAN接続、`ipp-usb` / `9100/tcp` を確認 |
| `PRINTER_PERMISSION_DENIED` | プリンタアクセス権限なし | `sudo usermod -aG lpadmin rakusika` 実行 |
| `PRINTER_DISABLED` | プリンタが無効 | CUPS管理画面でプリンタを有効化 |
| `PRINTER_PAPER_ERROR` | 用紙エラー | ラベル用紙の補充またはジャム除去 |
| `PRINTER_ERROR` | その他の印刷エラー | エラーメッセージを確認 |

#### 通信関連

| code | 意味 |
|------|------|
| `INVALID_REQUEST` | JSON不正・typeなし・空メッセージ |
| `UNKNOWN_TYPE` | 不明なリクエストタイプ |
| `UNKNOWN_ERROR` | 未分類エラー |

## ラベル印刷 API (print)

### リクエスト

```json
{
  "type": "print",
  "requestId": "p1",
  "template": "traceable_deer",
  "copies": 1,
  "data": {
    "productName": "鹿肉（モモ）",
    "productQuantity": "2.35 kg",
    "deadlineDate": "2026年3月18日",
    "storageTemperature": "-18℃以下",
    "individualNumber": "1234-56-78-90",
    "captureLocation": "長野県信濃町",
    "qrCode": "https://rakusika.com/t/abc/def"
  }
}
```

### テンプレート

| template | 名称 | 必須フィールド | 追加フィールド |
|----------|------|---------------|---------------|
| `traceable_deer` | トレーサブル（鹿肉） | productName, productQuantity, deadlineDate, storageTemperature, individualNumber | captureLocation, qrCode, attentionText |
| `traceable_bear` | トレーサブル（クマ肉） | productName, productQuantity, deadlineDate, storageTemperature, individualNumber | captureLocation, qrCode, attentionText |
| `non_traceable_deer` | 非トレーサブル（鹿肉） | productName, productQuantity, deadlineDate, storageTemperature | attentionText |
| `processed` | 加工品 | productName, productQuantity, deadlineDate, storageTemperature | productIngredient, nutritionUnit, caloriesQuantity, proteinQuantity, fatQuantity, carbohydratesQuantity, saltEquivalentQuantity, isHeatedMeatProducts, attentionText |
| `pet` | ペット向け | productName, productQuantity, deadlineDate, storageTemperature | productIngredient, nutritionUnit, attentionText |

### ラベル画像生成

- Go の `image` パッケージ + `golang.org/x/image/font` で PNG 画像を生成
- QR コード: `skip2/go-qrcode`（traceable テンプレート用）
- フォント: システムフォント自動検出（Noto Sans CJK 推奨）
- 出力: 62mm 幅 × 可変高さ（300 DPI, 732px 幅）
- CUPS `lp` コマンドで Brother QL-800/QL-820 に送信

### 前提条件

```bash
# 日本語フォント（ラベル画像生成に必須）
sudo apt-get install fonts-noto-cjk

# Brother プリンタドライバ
sudo apt-get install printer-driver-ptouch cups
```

### プリンタキュー API

```bash
curl http://127.0.0.1:19800/printer/queue
curl -X DELETE http://127.0.0.1:19800/printer/queue
```

`ptouch_template` + CUPS 経由では、hub は `lp` の受理だけで成功扱いせず、投入した job が CUPS キューから流れるまで待ってから `print_ok` を返す。ジョブが停滞した場合は `print_error` を返し、`/printer/queue` で残件確認と削除ができる。

### フォント設定

config.json で明示指定も可能:
```json
{
  "fontPath": "/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc"
}
```

未指定の場合、以下のパスを順に検索:
1. `/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc`
2. `/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc`
3. `/usr/share/fonts/truetype/fonts-japanese-gothic.ttf`
4. `/usr/share/fonts/truetype/vlgothic/VL-Gothic-Regular.ttf`
5. `/usr/share/fonts/truetype/dejavu/DejaVuSans.ttf`（フォールバック、CJK 非対応）

## A&D 防塵・防水デジタル台はかり HV-C シリーズ（HV-60KCWP-K）シリアルプロトコル

### 通信仕様

- インターフェース: USB（FTDI FT-X シリーズチップ経由）
- ボーレート: 2400 bps
- データビット: 7
- パリティ: Even
- ストップビット: 1
- 行終端: `\r\n`（CR+LF）

### コマンド

| コマンド | 機能 |
|---------|------|
| `Q\r\n` | 重量値リクエスト |
| `T\r\n` | 風袋引き |
| `R\r\n` | ゼロリセット |

### レスポンス形式

```
HD,±NNNNN.NN  UU\r\n
```

- `HD`: ヘッダー（`ST`=安定, `US`=不安定, `OL`=過負荷, `QT`/`TA`=風袋完了, `ZR`=ゼロ完了）
- `±NNNNN.NN`: 符号付き数値
- `UU`: 単位（`g`, `kg` 等）
- `OL` は単独（カンマ・値なし）

### FTDI 自動検出（Linux）

`port` 設定が空の場合、`/sys/bus/usb-serial/devices/` を走査して FTDI デバイスを VID/PID で自動検出する。
A&D の FTDI チップはデフォルトで VID: `0403`, PID: `6015` を使用する。

## 検証手順

### 1. ビルド確認

```bash
go build -o raku-sika-hub .
```

### 2. 起動確認（スケール未接続）

```bash
./raku-sika-hub
```

期待されるログ:
```
raku-sika-hub running (press Ctrl+C to stop)
```
ログファイル (`logs/service-YYYY-MM.log`):
```
[INFO] starting raku-sika-hub (listen=0.0.0.0:19800, maxClients=1)
[INFO] starting raku-sika-hub (listen=0.0.0.0:19800, printer=Brother_QL-820NWB, maxClients=1, scaleDriver=mock)
[INFO] starting raku-sika-hub (listen=0.0.0.0:19800, printer=Brother_QL-820NWB, maxClients=1, scaleDriver=mock, printerDriver=ptouch_template, printerAddress=, templateMap=templates/siknue/template-map.json)
[INFO] WebSocket server starting on 0.0.0.0:19800 (max clients: 1, allowAllOrigins=false, allowedOrigins=[localhost:* 127.0.0.1:* 192.168.50.* preview.rakusika.com rakusika.com *.rakusika.com])
[INFO] port detect: FTDI_NOT_FOUND: デバイスが見つかりません (VID:0403 PID:6015)
```
→ 3秒ごとに port detect ログ、クラッシュしない

### 3. WebSocket 接続テスト

```bash
# 別ターミナルから（wscat がある場合）
wscat -c ws://127.0.0.1:19800
> {"type":"status"}
> {"type":"health","requestId":"h1"}
> {"type":"print_test","requestId":"p1"}
```

または Node.js:
```bash
node -e "
const ws = new (require('ws'))('ws://127.0.0.1:19800');
ws.on('message', d => console.log(JSON.parse(d.toString())));
ws.on('open', () => {
  ws.send(JSON.stringify({ type: 'health', requestId: 'h1' }));
  setTimeout(() => ws.send(JSON.stringify({ type: 'print_test', requestId: 'p1' })), 1000);
});
"
```

### 4. 接続制限テスト

```bash
# 1台目
wscat -c ws://127.0.0.1:19800
# → 接続成功、connection_status 受信

# 2台目（別ターミナル）
wscat -c ws://127.0.0.1:19800
# → HTTP 429: Too Many Connections
```

### 5. A&D 実機到着時

```bash
# 1. デバイスを USB 接続
# 2. dmesg でデバイス確認
dmesg | tail -20

# 3. /dev/ttyUSB* を確認
ls -la /dev/ttyUSB*

# 4. パーミッション設定
sudo usermod -aG dialout $USER
# → 再ログイン

# 5. 起動（自動検出）
./raku-sika-hub

# 6. lite から接続して計量
# raku-sika-lite の useScale.tsx で WS URL を ws://raku-sika-hub.local:19800 に変更
```

## ディレクトリ構成

```
.
├── main.go                         # エントリポイント。シグナルハンドリング
├── go.mod / go.sum
├── config.json                     # 設定ファイル（オプション、なければデフォルト値）
├── logs/                           # 月次ログファイル（自動生成）
│   └── service-YYYY-MM.log
├── templates/                      # Brother P-touch ラベルテンプレート (.lbx)
│   └── siknue/                     # 信濃エリア用テンプレート
├── deploy/
│   ├── setup.sh                    # Pi 一括セットアップ（hostname, mDNS, dialout, fonts, CUPS, systemd）
│   ├── setup-printer.sh            # Brother QL プリンタ自動検出・CUPS 登録
│   ├── raku-sika-hub.service       # systemd ユニットファイル
│   └── avahi/
│       └── raku-sika-hub.service   # mDNS サービス定義（raku-sika-hub.local）
└── internal/
    ├── app/
    │   └── app.go                  # App コンテナ。全コンポーネントの統合
    ├── config/
    │   └── config.go               # config.json の読み込み。デフォルト値管理
    ├── logging/
    │   └── logger.go               # 月次ローテーション付きロガー。1年で自動削除
    ├── scale/
    │   ├── port.go                 # Port インターフェース（io.ReadWriteCloser）
    │   ├── serial.go               # go.bug.st/serial ラッパー
    │   ├── detector.go             # Linux FTDI デバイス検出（/sys/bus/usb-serial）
    │   ├── protocol.go             # A&D コマンド定数・レスポンスパーサ
    │   ├── client.go               # scale.Client: 再接続ループ・weigh/tare/zero
    │   ├── protocol_test.go        # プロトコルパーサのテスト
    │   └── client_test.go          # mock port を使った Client テスト
    ├── printer/
    │   ├── brother.go              # Brother QL-800/QL-820 ドライバ（CUPS lp コマンド）
    │   ├── label.go                # ラベル画像レンダラ（Go image + QR code 生成）
    │   ├── types.go                # LabelData 型定義・テンプレート定数
    │   └── label_test.go           # ラベルレンダリングテスト
    └── ws/
        ├── message.go              # リクエスト・レスポンス JSON 型定義・エラーコード
        ├── server.go               # WebSocket サーバー。Hub + Client + 接続制限
        └── handler.go              # リクエストハンドラ。エラー分岐・日本語メッセージ
```

## Raspberry Pi デプロイ

### ワンライナーセットアップ

```bash
git clone https://github.com/deervery/raku-sika-hub.git
cd raku-sika-hub
# Go ビルド（Pi上）or 開発PCからクロスコンパイルしてバイナリを配置
go build -o raku-sika-hub .

# hostname, avahi(mDNS), dialout, systemd を一括設定
sudo bash deploy/setup.sh
```

これにより:
- ホスト名が `raku-sika-hub` に変更される
- **`raku-sika-hub.local`** で LAN 内から名前解決可能になる（avahi/mDNS）
- シリアルポートのパーミッション設定（dialout グループ）
- systemd サービスが登録・有効化される

```bash
# サービス起動
sudo systemctl start raku-sika-hub

# 動作確認（LAN 内の別マシンから）
curl http://raku-sika-hub.local:19800/health

# WebSocket 接続確認
wscat -c ws://192.168.50.40:19800

# ログ確認
journalctl -u raku-sika-hub -f
tail -f logs/service-*.log
```

### 手動セットアップ

<details>
<summary>deploy/setup.sh を使わない場合</summary>

#### ホスト名 & mDNS 設定

```bash
# ホスト名変更
sudo hostnamectl set-hostname raku-sika-hub
sudo sed -i 's/127\.0\.1\.1.*$/127.0.1.1\traku-sika-hub/' /etc/hosts

# avahi サービス定義を配置
sudo cp deploy/avahi/raku-sika-hub.service /etc/avahi/services/
sudo systemctl restart avahi-daemon

# 確認（別マシンから）
ping raku-sika-hub.local
```

#### systemd サービス登録

```bash
sudo cp deploy/raku-sika-hub.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable raku-sika-hub
sudo systemctl start raku-sika-hub
```

#### シリアルポートパーミッション

```bash
sudo usermod -aG dialout $USER
# 再ログイン必要
```

</details>

### Brother QL-820/QL-820NWB プリンタセットアップ（未検証）

```bash
sudo apt-get install printer-driver-ptouch cups
sudo usermod -aG lpadmin rakusika
# CUPS Web UI: http://localhost:631/admin → Add Printer → Brother QL-820 or QL-820NWB

# systemd で明示したい場合
sudo systemctl edit raku-sika-hub
# [Service]
# Environment=PRINTER_NAME=Brother_QL-820NWB
# Environment=PRINTER_DRIVER=ptouch_template
# Environment=PRINTER_ADDRESS=192.168.50.40:9100
# Environment=TEMPLATE_MAP_PATH=templates/siknue/template-map.json
# Environment=MAX_CLIENTS=3
# Environment=SCALE_DRIVER=mock

# または brother_ql (Python)
pip3 install brother_ql
```

## lite 側の接続設定

`raku-sika-lite` の `lib/hooks/useScale.tsx`（19行目付近）で WebSocket URL を変更:

```typescript
// mDNS で名前解決（推奨）
const SCALE_WS_URL = 'ws://raku-sika-hub.local:19800';

// または IP 直指定（mDNS が使えない環境の場合）
// const SCALE_WS_URL = 'ws://192.168.x.x:19800';
```

## リリース & Raspberry Pi 更新

### リリースの作成

Git タグをプッシュすると GitHub Actions が ARM64 バイナリをビルドし、GitHub Release に添付する。

```bash
git tag v0.3.0
git push origin v0.3.0
```

### Raspberry Pi で最新版に更新

```bash
bash ~/raku-sika-hub/deploy/update.sh
```

`update.sh` は以下を自動で行う:
1. GitHub Releases から最新の `raku-sika-hub-linux-arm64` をダウンロード
2. サービスを停止し、バイナリを差し替え
3. サービスを再起動し、ヘルスチェック
4. 失敗時は自動ロールバック

## テスト実行

```bash
go test ./internal/scale/ -v
```

## 技術スタック

- **Language**: Go 1.24
- **WebSocket**: [coder/websocket](https://github.com/coder/websocket)
- **Serial**: [go.bug.st/serial](https://pkg.go.dev/go.bug.st/serial)
- **Printer**: CUPS `lp` コマンド経由
- **Label Rendering**: [golang.org/x/image](https://pkg.go.dev/golang.org/x/image) (font) + [skip2/go-qrcode](https://github.com/skip2/go-qrcode) (QR)
- **構成**: wsa と同一の `internal/` パッケージ構成
