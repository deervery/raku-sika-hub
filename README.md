# raku-sika-hub

Raspberry Pi 上で常駐する機器制御ゲートウェイ（Go 実装）。
A&D 防塵・防水デジタル台はかり HV-C シリーズ、Brother ラベルプリンタ（QL-800/QL-820）、USB バーコードリーダーを制御し、フロントエンド（[raku-sika-lite](https://github.com/deervery/raku-sika-lite)）に対して HTTP REST API を提供する。

ラベル印刷は Hub 内部の Go renderer で行う。`lite/tmp/manoir/*.lbx` は Hub が直接読むわけではないため、テンプレート見た目の変更を Hub 印刷へ反映するには renderer 側の修正が必要。

## システム構成

```
┌─────────────────┐     HTTP REST API    ┌──────────────────┐     Serial/USB     ┌──────────────┐
│  raku-sika-lite │ ◄──────────────────► │  raku-sika-hub   │ ◄────────────────► │  A&D HV-C    │
│  (Next.js)      │  :19800 (LAN only)  │  (Go)            │   2400bps 7E1     │  (HV-60KCWP-K)│
│  ブラウザ上で動作 │                     │  Raspberry Pi    │                    └──────────────┘
└─────────────────┘                     │                  │     CUPS/lp       ┌──────────────┐
                                        │                  │ ◄────────────────► │  Brother      │
                                        │                  │                    │  QL-800/820   │
                                        │                  │     evdev/HID     ├──────────────┤
                                        │                  │ ◄────────────────► │  USB barcode  │
                                        └──────────────────┘                    │  scanner      │
                                                                                └──────────────┘
```

## API エンドポイント

詳細は [docs/hub-v0.2/api.md](docs/hub-v0.2/api.md) を参照。

| メソッド | パス | 説明 |
|---------|------|------|
| GET | `/health` | 全デバイスの接続状態 |
| GET | `/version` | ビルドバージョン情報 |
| POST | `/scale/weigh` | 計量（安定値取得、500ms キャッシュ） |
| POST | `/scale/tare` | 風袋引き |
| POST | `/scale/zero` | ゼロリセット |
| POST | `/printer/print` | ラベル印刷 |
| POST | `/printer/preview` | ラベルプレビュー画像（PNG） |
| POST | `/printer/test` | テスト印刷 |
| GET | `/scanner/scan` | バーコード最新スキャン値（消費型） |

### アクセス制御

- **LAN 内限定**: プライベート IP のみ許可（`192.168.0.0/16`, `10.0.0.0/8`, `172.16.0.0/12`, `127.0.0.0/8`, `::1`, `fe80::/10`）
- **CORS**: `Access-Control-Allow-Origin: *`（LAN 内ブラウザからのクロスオリジンリクエスト対応）

## セットアップ

### 前提条件

- Raspberry Pi OS (Debian Bookworm), ARM64
- Go 1.24+（ビルド時のみ）

### Raspberry Pi 初期化手順

ラズパイを `raku-sika-hub` 端末として使える状態にするまでの大まかな流れ:

1. Raspberry Pi OS を microSD に書き込む
2. ラズパイを起動する
3. `/boot/firmware/config.txt` などに画面表示用の設定を入れる
4. Tailscale をインストールしてリモート接続できるようにする
5. [raku-sika-ops](https://github.com/deervery/raku-sika-ops) を配置して、systemd・キオスク・更新系のセットアップを行う
6. `raku-sika-hub` 本体を配置する
7. Brother プリンタを CUPS に登録する

初期セットアップの責務分担:

- `raku-sika-ops`: OS 初期化後の端末セットアップ、systemd、キオスク、更新導線
- `raku-sika-hub`: Hub アプリ本体、デバイス制御、HTTP/WS API

Pi 全体の初期化フローは [raku-sika-ops](https://github.com/deervery/raku-sika-ops) の README も参照。

### GitHub Releases からインストール（推奨）

Pi 側の更新は通常 [raku-sika-ops](https://github.com/deervery/raku-sika-ops) の deploy フローから行う。
バイナリを手動取得する場合は以下。

```bash
curl -sL $(curl -s https://api.github.com/repos/deervery/raku-sika-hub/releases/latest \
  | grep browser_download_url | cut -d '"' -f 4) -o raku-sika-hub
chmod +x raku-sika-hub
```

### ソースからビルド

```bash
git clone https://github.com/deervery/raku-sika-hub.git
cd raku-sika-hub
go build -o raku-sika-hub .
```

クロスコンパイル（開発 PC → Pi）:

```bash
GOOS=linux GOARCH=arm64 go build -o raku-sika-hub .
scp raku-sika-hub rakusika@raku-sika-hub.local:~/raku-sika-hub/
```

### Pi への配置メモ

Pi 全体の初期化は [raku-sika-ops](https://github.com/deervery/raku-sika-ops) 側で行う。
このリポジトリだけでは、OS 書き込み、キオスク設定、Tailscale、systemd 周りは完結しない。

最低限必要な作業:

```bash
# Pi 上
git clone https://github.com/deervery/raku-sika-ops.git

# Hub バイナリの反映は raku-sika-ops 側の deploy フローを使う
sudo raku-sika-deploy-hub
```

反映後の確認:

```bash
sudo systemctl start raku-sika-hub
curl http://localhost:19800/health
```

### Brother プリンタの CUPS 登録

Hub は Brother プリンタを USB 直叩きせず、CUPS の登録済みプリンタに対して `lp` コマンドで印刷する。
そのため、初回セットアップ時に CUPS へのプリンタ登録が必要。

Pi セットアップ全体の流れは [raku-sika-ops](https://github.com/deervery/raku-sika-ops) を参照。

これは毎回必要ではなく、基本的には初回セットアップ時に 1 回だけ行えばよい。
再度必要になるのは、OS 再構築、CUPS 設定消失、プリンタ名変更、接続方式変更のときだけ。

状態確認:

```bash
lpstat -p -d
```

未登録時の例:

```bash
lpstat: No destinations added.
no system default destination
```

driverless IPP で見えている場合の登録例:

```bash
sudo lpadmin -p Brother_QL_820NWB_USB -E \
  -v "ipp://Brother%20QL-820NWB%20(USB)._ipp._tcp.local/" \
  -m everywhere
sudo lpoptions -d Brother_QL_820NWB_USB
```

登録確認:

```bash
lpstat -p -d
curl http://localhost:19800/health
```

`PRINTER_NAME` は CUPS に登録した名前と一致させること。
例:

```bash
PRINTER_NAME=Brother_QL_820NWB_USB
```

## 設定

設定ファイル `config.json`（オプション）。未指定フィールドはデフォルト値を使用。環境変数で上書き可能。

```json
{
  "vid": "0403",
  "pid": "6015",
  "port": "",
  "baudRate": 2400,
  "dataBits": 7,
  "parity": "even",
  "stopBits": 1,
  "printerName": "",
  "fontPath": "",
  "scannerVid": "",
  "scannerPid": "",
  "scannerDeviceName": "",
  "listenAddr": "0.0.0.0:19800",
  "logLevel": "INFO",
  "enableWebSocket": false
}
```

| フィールド | デフォルト | 環境変数 | 説明 |
|-----------|-----------|---------|------|
| `vid` | `"0403"` | `VID` | FTDI ベンダー ID |
| `pid` | `"6015"` | `PID` | FTDI プロダクト ID |
| `port` | `""` (自動検出) | `PORT` | シリアルポートパス |
| `baudRate` | `2400` | `BAUD_RATE` | ボーレート |
| `dataBits` | `7` | `DATA_BITS` | データビット |
| `parity` | `"even"` | `PARITY` | パリティ |
| `stopBits` | `1` | `STOP_BITS` | ストップビット |
| `printerName` | `""` | `PRINTER_NAME` | CUPS プリンタ名（空 = 自動選択） |
| `fontPath` | `""` (自動検出) | `FONT_PATH` | 日本語フォントパス |
| `scannerDeviceName` | `""` | `SCANNER_DEVICE_NAME` | バーコードリーダーのデバイス名パターン |
| `scannerVid` | `""` | `SCANNER_VID` | バーコードリーダーの USB VID |
| `scannerPid` | `""` | `SCANNER_PID` | バーコードリーダーの USB PID |
| `listenAddr` | `"0.0.0.0:19800"` | `LISTEN_ADDR` | HTTPサーバーのリッスンアドレス |
| `logLevel` | `"INFO"` | `LOG_LEVEL` | ログレベル（`ERROR` / `WARN` / `INFO`） |
| `enableWebSocket` | `false` | `ENABLE_WEBSOCKET` | `/ws/health` WebSocket を有効化 |

印刷データは API/WS 経由で完全に渡す前提。Hub は `companyBlock` / `facilityBlock` / `captureLocation` を環境変数で補完しない。

### WebSocket

WebSocket を使う場合は `ENABLE_WEBSOCKET=1` を設定する。
HTTP API はそのまま利用でき、追加で以下の WebSocket エンドポイントが有効になる。

| パス | 用途 |
|------|------|
| `/ws/health` | 接続状態・プリンタ状態・定期 health 配信 |
| `/ws` | 旧クライアント互換 |
| `/ws/status` | WebSocket サーバー側 health JSON |

主な受信イベント:

- `connection_status`
- `printer_status`
- `health`
- `print_progress`

### バーコードリーダーの設定

USB バーコードリーダーは HID キーボードデバイスとして認識される。接続後にデバイス名を確認:

```bash
cat /sys/class/input/event*/device/name
```

表示された名前を `scannerDeviceName` に設定する。空の場合はスキャナー無効。

## ディレクトリ構成

```
.
├── main.go                         # エントリポイント（バージョン変数、シグナルハンドリング）
├── go.mod / go.sum
├── config.json.example             # 設定ファイルサンプル
├── logs/                           # 月次ログファイル（自動生成、1年で自動削除）
├── docs/hub-v0.2/
│   ├── plan.md                     # v0.2 修正指示書
│   └── api.md                      # HTTP REST API リファレンス
├── .github/workflows/
│   └── release.yml                 # GitHub Releases ワークフロー（ARM64 ビルド）
└── internal/
    ├── app/app.go                  # App コンテナ（HTTP/WS/スキャナーの統合）
    ├── config/config.go            # 設定読み込み（JSON + 環境変数）
    ├── logging/logger.go           # 月次ローテーションロガー
    ├── httpapi/                    # HTTP REST API サーバー
    │   ├── server.go               # ルーティング
    │   ├── handler.go              # エンドポイントハンドラ
    │   ├── middleware.go           # LAN 制限 + CORS ミドルウェア
    │   └── response.go            # JSON レスポンスヘルパー
    ├── scale/                      # A&D はかり制御
    │   ├── client.go               # 再接続ループ、weigh/tare/zero、500ms キャッシュ
    │   ├── protocol.go             # A&D コマンド・レスポンスパーサ
    │   ├── detector.go             # Linux FTDI デバイス自動検出
    │   ├── port.go                 # Port インターフェース
    │   └── serial.go               # go.bug.st/serial ラッパー
    ├── printer/                    # Brother QL ラベルプリンタ
    │   ├── brother.go              # CUPS lp コマンドドライバ
    │   ├── label.go                # ラベル画像レンダラ（62mm, 300DPI, QR コード）
    │   └── types.go                # LabelData 型・テンプレート定義
    ├── scanner/                    # USB バーコードリーダー
    │   ├── client.go               # evdev 読み取り・排他アクセス・自動再接続
    │   ├── detector.go             # USB HID デバイス自動検出
    │   └── keymap.go               # HID キーコード → 文字変換
    └── ws/                         # WebSocket サーバー（/ws/health, /ws）
        ├── server.go
        ├── handler.go
        └── message.go
```

## テスト

```bash
go test ./... -v
```

## リリース

Git タグを push すると GitHub Actions が ARM64 バイナリをビルドし Release を作成する:

```bash
git tag v0.3.0
git push origin v0.3.0
```

Pi 側での更新は [raku-sika-ops](https://github.com/deervery/raku-sika-ops) を使用:

```bash
sudo raku-sika-deploy-hub
```

> **Note**: 端末初期設定、systemd ユニット、キオスク設定、Tailscale 導入補助等は
> [raku-sika-ops](https://github.com/deervery/raku-sika-ops) に移管済み。
> このリポジトリはアプリケーションコードとリリースのみを管理する。

## メモリ・パフォーマンス

2GB Raspberry Pi で安定稼働するよう設計。systemd サービスに `GOMEMLIMIT=256MiB` を設定済み。

| 操作 | レスポンス時間 | メモリ使用量 |
|------|--------------|-------------|
| ヘルスチェック | < 10ms | 無視可能 |
| 計量（安定時） | < 500ms | 無視可能 |
| スキャナー読み取り | < 5ms | 無視可能 |
| ラベル印刷 | < 3秒 | 一時的に ~20MB |
| アイドル時 | - | ~15-20MB |

## 技術スタック

- **Go 1.24** — 標準ライブラリ `net/http` でルーティング
- **go.bug.st/serial** — シリアル通信
- **golang.org/x/image** — ラベル画像レンダリング
- **skip2/go-qrcode** — QR コード生成
- **CUPS `lp`** — Brother QL プリンタ制御
- **Linux evdev** — USB バーコードリーダー（syscall 直接利用、外部ライブラリなし）
