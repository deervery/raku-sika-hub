# raku-sika-hub

Raspberry Pi 上で常駐する機器制御ゲートウェイ（Go 実装）。
A&D 防塵・防水デジタル台はかり HV-C シリーズ、Brother ラベルプリンタ（QL-800/QL-820）、USB バーコードリーダーを制御し、フロントエンド（[raku-sika-lite](https://github.com/deervery/raku-sika-lite)）に対して HTTP REST API を提供する。

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

### GitHub Releases からインストール（推奨）

```bash
bash deploy/update.sh
```

または手動:

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

### Pi 一括セットアップ

```bash
sudo bash deploy/setup.sh
```

これにより hostname (`raku-sika-hub`)、mDNS (`raku-sika-hub.local`)、dialout 権限、日本語フォント、CUPS、systemd サービスが一括設定される。

```bash
sudo systemctl start raku-sika-hub
curl http://raku-sika-hub.local:19800/health
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
| `enableWebSocket` | `false` | `ENABLE_WEBSOCKET` | WebSocket サーバーを有効化（レガシー互換用） |

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
├── deploy/
│   ├── setup.sh                    # Pi 一括セットアップ
│   ├── setup-printer.sh            # Brother QL プリンタ自動検出・CUPS 登録
│   ├── update.sh                   # リリースバイナリのダウンロード・更新
│   ├── raku-sika-hub.service       # systemd ユニットファイル
│   └── avahi/
│       └── raku-sika-hub.service   # mDNS サービス定義
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
    └── ws/                         # WebSocket サーバー（レガシー、enableWebSocket: true 時のみ）
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
git tag v0.2.0
git push origin v0.2.0
```

Pi 側での更新:

```bash
bash deploy/update.sh
```

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
