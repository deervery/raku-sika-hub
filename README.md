# raku-sika-hub

Raspberry Pi 上で常駐する機器制御 WebSocket ゲートウェイ。
A&D 計量器（HV-60KCWP-K）と Brother ラベルプリンタ（QL-800）を制御し、フロントエンド（[raku-sika-lite](https://github.com/deervery/raku-sika-lite)）に対して WSA 互換の WebSocket API を提供する。

## システム構成

```
┌─────────────────┐     WebSocket      ┌──────────────────┐     Serial/USB     ┌─────────────┐
│  raku-sika-lite │ ◄────────────────► │  raku-sika-hub   │ ◄────────────────► │  A&D 計量器  │
│  (Next.js)      │   ws://pi:19800    │  (Node.js / TS)  │   2400bps 7E1     │  HV-60KCWP-K│
│  ブラウザ上で動作 │                    │  Raspberry Pi    │                    │  FTDI USB    │
└─────────────────┘                    │                  │     CUPS/lp       ┌─────────────┐
                                       │                  │ ◄────────────────► │  Brother     │
                                       └──────────────────┘                    │  QL-800      │
                                                                               └─────────────┘
```

### 関連リポジトリ

| リポジトリ | 役割 |
|-----------|------|
| [raku-sika-lite](https://github.com/deervery/raku-sika-lite) | Next.js フロントエンド。ブラウザから hub へ WebSocket 接続して計量・印刷を操作 |
| [raku-sika-wsa](https://github.com/deervery/raku-sika-wsa) | Windows 版の Go 製サービス（リファレンス実装）。hub はこれの Raspberry Pi 移植版 |

## 現在の開発状況

### 実装済み

| 機能 | 状態 | 備考 |
|------|------|------|
| WebSocket サーバー (port 19800) | **動作確認済** | WSA 互換プロトコル、CORS 制御、単一クライアント制限あり |
| MockScaleDriver | **動作確認済** | 実機なしでの開発・テスト用。ランダムなジッターつき重量を返す |
| RealScaleDriver (A&D シリアル通信) | **実装済・未テスト** | FTDI 自動検出、コマンド送受信、パース処理まで完了。実機到着待ち |
| FTDI ポート自動検出 | **実装済・未テスト** | VID:0403 / PID:6015 でデバイスを自動検出 |
| スケール再接続ループ | **実装済・動作確認済** | 3秒間隔で接続試行、クラッシュしない |
| print_test ハンドラ | **実装済・動作確認済** | プリンタ未設定時は `print_test_error` を返す |
| BrotherPrinterDriver | **基本実装のみ** | `testPrint()` は `lp` コマンド経由。ラベル印刷は未実装 |
| DeviceStateManager | **動作確認済** | 接続状態変化を全クライアントにブロードキャスト |
| HTTP ヘルスチェック | **動作確認済** | `GET http://pi:19800/` → `{"status":"ok","service":"raku-sika-hub"}` |

### 未実装・TODO

- **A&D 実機テスト**: デバイス到着後にシリアル通信の実機検証が必要
- **ラベル印刷**: テンプレートシステムと `brother_ql` or CUPS 連携
- **Pi 向け systemd サービスファイル**: 作成が必要
- **CUPS プリンタセットアップ**: Pi 上での Brother QL-800 ドライバ設定

## セットアップ

### 前提条件

- Node.js 18+ (Raspberry Pi OS には `nvm` 経由でインストール推奨)
- npm

### インストール

```bash
git clone https://github.com/deervery/raku-sika-hub.git
cd raku-sika-hub
npm install
```

### ビルド

```bash
npm run build    # TypeScript → dist/ にコンパイル
```

## 起動方法

### 開発モード（Mock スケール）

実機が手元にない状態での開発・テスト用。ランダムな重量値を返すモックドライバで動作する。

```bash
npm run dev
```

### 開発モード（Real スケール）

A&D 計量器を USB 接続した状態で起動する。FTDI チップを VID/PID で自動検出して接続する。
デバイスが見つからない場合は 3 秒間隔で再接続を試行し続ける（クラッシュしない）。

```bash
# FTDI 自動検出で起動（推奨）
SCALE_DRIVER=real npm run dev

# ポートを明示的に指定する場合
SCALE_DRIVER=real SERIAL_PORT=/dev/ttyUSB0 npm run dev

# npm script のショートカット
npm run dev:real
```

### プロダクション

```bash
npm run build
npm start

# または環境変数つき
SCALE_DRIVER=real npm start
```

## 環境変数

| 変数 | デフォルト | 説明 |
|------|-----------|------|
| `HUB_PORT` | `19800` | WebSocket サーバーポート |
| `SCALE_DRIVER` | `mock` | スケールドライバ。`mock`（開発用）または `real`（実機） |
| `SERIAL_PORT` | （空 = 自動検出） | シリアルポートパス。空の場合 FTDI VID/PID で自動検出 |
| `BAUD_RATE` | `2400` | シリアル通信ボーレート（A&D デフォルト: 2400） |
| `DATA_BITS` | `7` | データビット（A&D デフォルト: 7） |
| `PARITY` | `even` | パリティ（A&D デフォルト: even） |
| `STOP_BITS` | `1` | ストップビット |
| `FTDI_VID` | `0403` | FTDI ベンダー ID（自動検出用） |
| `FTDI_PID` | `6015` | FTDI プロダクト ID（自動検出用） |
| `PRINTER_NAME` | `Brother QL-800` | CUPS 上のプリンタ名 |
| `MAX_CLIENTS` | `1` | WebSocket 同時接続数上限（v1: 1台） |
| `LOG_LEVEL` | `INFO` | ログレベル（`ERROR` / `WARN` / `INFO` / `DEBUG`） |

## WebSocket API（WSA 互換）

### 接続

```
ws://<hub-ip>:19800
```

接続直後にサーバーから `connection_status` メッセージが自動送信される。
スケールの接続状態が変化した場合も、全クライアントにブロードキャストされる。

### CORS

以下のオリジンからの接続を許可:
- `localhost` / `127.0.0.1`（任意ポート）
- `*.rakusika.com`
- `preview.rakusika.com`
- オリジンなし（非ブラウザクライアント）

### Client → Server メッセージ

すべてのリクエストは JSON 形式。`requestId` はレスポンスの紐付けに使用する任意の文字列。

| type | fields | 説明 |
|------|--------|------|
| `weigh` | `requestId` | 安定重量を取得。不安定時は最大10回リトライ（500ms 間隔） |
| `tare` | `requestId` | 風袋引き |
| `zero` | `requestId` | ゼロリセット |
| `health` | `requestId` | スケール接続状態のヘルスチェック |
| `status` | — | 現在の接続状態を取得 |
| `print_test` | `requestId` | テスト印刷を実行 |

### Server → Client メッセージ

| type | fields | 説明 |
|------|--------|------|
| `weight` | `requestId`, `value`, `unit`, `stable` | 安定重量の取得結果 |
| `weighing` | `requestId`, `retry`, `maxRetry` | 計量中の進捗（不安定時のリトライ回数） |
| `tare_ok` | `requestId` | 風袋引き成功 |
| `zero_ok` | `requestId` | ゼロリセット成功 |
| `health_ok` | `requestId`, `connected`, `port` | ヘルスチェック応答 |
| `connection_status` | `connected`, `port` | 接続状態（接続時自動送信 + 状態変化時ブロードキャスト） |
| `print_test_ok` | `requestId`, `message?` | テスト印刷成功 |
| `print_test_error` | `requestId`, `message` | テスト印刷失敗 |
| `error` | `requestId`, `code`, `message` | エラー |

### エラーコード

v1 では現場での即時対応がしやすいよう、全エラーに日本語の原因・対処法メッセージを含めている。

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
| `PRINTER_PERMISSION_DENIED` | プリンタアクセス権限なし | `sudo usermod -aG lpadmin pi` 実行 |
| `PRINTER_DISABLED` | プリンタが無効 | CUPS管理画面でプリンタを有効化 |
| `PRINTER_PAPER_ERROR` | 用紙エラー | ラベル用紙の補充またはジャム除去 |
| `PRINTER_ERROR` | その他の印刷エラー | エラーメッセージを確認 |

#### 通信関連

| code | 意味 | 現場での対処 |
|------|------|-------------|
| `INVALID_REQUEST` | JSON不正・typeなし・空メッセージ | リクエスト形式を確認 |
| `UNKNOWN_TYPE` | 不明なリクエストタイプ | 使用可能: weigh, tare, zero, health, status, print_test |
| `UNKNOWN_ERROR` | 未分類エラー | エラーメッセージで原因特定 |

#### 接続制限（HTTP 429）

v1 は単一クライアント接続のみ対応。2台目以降の接続は HTTP 429 で拒否される。
拒否メッセージ: `"Too Many Connections: Another client is already connected to raku-sika-hub. Disconnect the existing client first."`

`MAX_CLIENTS` 環境変数で接続上限を変更可能（デフォルト: 1）。

## A&D 計量器（HV-60KCWP-K）シリアルプロトコル

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
| `Z\r\n` | ゼロリセット |

### レスポンス形式

```
HD,±NNNNN.NN  UU
```

- `HD`: ヘッダー（`ST`=安定, `US`=不安定, `OL`=過負荷, `QT`/`TA`=風袋完了, `ZR`=ゼロ完了）
- `±NNNNN.NN`: 符号付き数値
- `UU`: 単位（`g`, `kg` 等）

### FTDI 自動検出

`SERIAL_PORT` 環境変数が空の場合、接続されている USB デバイスの VID/PID を走査して FTDI デバイスを自動検出する。
A&D の FTDI チップはデフォルトで VID: `0403`, PID: `6015` を使用する。

## 検証手順

### 1. Mock モードでの基本動作確認

```bash
# ターミナル 1: hub 起動
npm run dev

# ターミナル 2: WebSocket クライアントで接続
node -e "
const ws = new (require('ws'))('ws://127.0.0.1:19800');
ws.on('message', d => console.log(JSON.parse(d.toString())));
ws.on('open', () => {
  ws.send(JSON.stringify({ type: 'weigh', requestId: 'w1' }));
  setTimeout(() => ws.send(JSON.stringify({ type: 'tare', requestId: 't1' })), 3000);
  setTimeout(() => ws.send(JSON.stringify({ type: 'health', requestId: 'h1' })), 4000);
});
"
```

期待される出力:
```json
{ "type": "connection_status", "connected": true, "port": "mock" }
{ "type": "weighing", "requestId": "w1", "retry": 1, "maxRetry": 10 }
{ "type": "weight", "requestId": "w1", "value": 12.38, "unit": "kg", "stable": true }
{ "type": "tare_ok", "requestId": "t1" }
{ "type": "health_ok", "requestId": "h1", "connected": true, "port": "mock" }
```

### 2. print_test の確認

```bash
# ターミナル 1: hub 起動
npm run dev

# ターミナル 2:
node -e "
const ws = new (require('ws'))('ws://127.0.0.1:19800');
ws.on('message', d => console.log(JSON.parse(d.toString())));
ws.on('open', () => ws.send(JSON.stringify({ type: 'print_test', requestId: 'p1' })));
"
```

期待される出力（CUPS 未設定の場合）:
```json
{ "type": "connection_status", "connected": true, "port": "mock" }
{ "type": "print_test_error", "requestId": "p1", "message": "Printer \"Brother QL-800\" not available. ..." }
```

### 3. Real スケール再接続ループの確認（実機なし）

```bash
SCALE_DRIVER=real npm run dev
```

期待されるログ:
```
[Hub] Starting raku-sika-hub...
[Hub] Scale driver: real
[Hub] Port: 19800
[Hub] Scale connection failed: FTDI device not found (VID:0403 PID:6015). Available ports: none
[Hub] Printer available: false
[WS] Server listening on 0.0.0.0:19800
[Hub] Ready
[Hub] Scale reconnect failed: FTDI device not found (VID:0403 PID:6015). Available ports: none
[Hub] Scale reconnect failed: FTDI device not found (VID:0403 PID:6015). Available ports: none
...（3秒間隔で繰り返し、クラッシュしない）
```

### 4. シリアルポート一覧の確認

```bash
npm run ports
```

FTDI デバイスが接続されている場合:
```
[ '/dev/ttyUSB0 [VID:0403 PID:6015 FTDI]' ]
```

### 5. A&D 実機到着時の動作確認

```bash
# 1. デバイスを USB 接続
# 2. ポート確認
npm run ports

# 3. Real モードで起動（自動検出）
SCALE_DRIVER=real npm run dev

# 4. lite から接続して計量
# raku-sika-lite の useScale.tsx で WS URL を ws://<pi-ip>:19800 に変更
```

## ディレクトリ構成

```
src/
├── index.ts                    # エントリポイント。ドライバ初期化・再接続ループ・サーバー起動
├── config.ts                   # 環境変数ベースの設定管理
├── transport/
│   ├── server.ts               # WebSocket サーバー（ws ライブラリ）。CORS・ブロードキャスト
│   └── handler.ts              # WSA 互換メッセージハンドラ。リクエスト→レスポンス変換
├── drivers/
│   ├── scale/
│   │   ├── types.ts            # ScaleDriver インターフェース
│   │   ├── mock.ts             # MockScaleDriver（開発用）
│   │   └── real.ts             # RealScaleDriver（A&D シリアル通信）
│   └── printer/
│       ├── types.ts            # PrinterDriver インターフェース
│       └── brother.ts          # BrotherPrinterDriver（CUPS/lp 経由）
└── state/
    └── device.ts               # DeviceStateManager（接続状態管理・変更通知）
```

## Raspberry Pi デプロイ

### systemd サービス登録

```ini
# /etc/systemd/system/raku-sika-hub.service
[Unit]
Description=RakuSika Hub - Scale & Printer Gateway
After=network.target

[Service]
Type=simple
User=pi
WorkingDirectory=/home/pi/raku-sika-hub
ExecStart=/usr/bin/node dist/index.js
Environment=SCALE_DRIVER=real
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

```bash
sudo cp raku-sika-hub.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable raku-sika-hub
sudo systemctl start raku-sika-hub

# ログ確認
sudo journalctl -u raku-sika-hub -f
```

### Brother QL-800 プリンタセットアップ（未検証）

```bash
# CUPS ドライバインストール
sudo apt-get install printer-driver-ptouch cups

# ユーザーを lpadmin グループに追加
sudo usermod -aG lpadmin pi

# CUPS Web UI でプリンタ追加
# http://localhost:631/admin → Add Printer → Brother QL-800

# または brother_ql (Python) を使う場合
pip3 install brother_ql
```

## lite 側の接続設定

`raku-sika-lite` の `lib/hooks/useScale.tsx`（19行目付近）で WebSocket URL を変更する:

```typescript
// 開発時（ローカル）
const SCALE_WS_URL = 'ws://localhost:19800';

// Raspberry Pi に接続する場合
const SCALE_WS_URL = 'ws://192.168.x.x:19800';
```

## npm scripts

| コマンド | 説明 |
|---------|------|
| `npm run dev` | Mock スケールで開発サーバー起動 |
| `npm run dev:real` | Real スケールで開発サーバー起動 |
| `npm run build` | TypeScript コンパイル |
| `npm start` | ビルド済み JS で起動 |
| `npm run ports` | 接続されているシリアルポート一覧を表示 |

## 技術スタック

- **Runtime**: Node.js 18+
- **Language**: TypeScript（ES2020 target, strict mode）
- **WebSocket**: [ws](https://github.com/websockets/ws) ライブラリ
- **Serial**: [serialport](https://serialport.io/) ライブラリ（FTDI USB-Serial 通信）
- **Printer**: CUPS `lp` コマンド経由
