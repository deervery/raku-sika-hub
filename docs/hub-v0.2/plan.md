# raku-sika-hub v0.2.0 修正指示書

## 概要

raku-sika-hub v0.2.0 は、raku-sika-lite v0.6.0 と連携するためのメジャーアップデートである。
WebSocket中心のアーキテクチャからHTTP REST APIへの移行、LAN内限定アクセス、GitHub Releasesによるリリース管理、およびメモリ効率の改善を行う。

**後方互換性は不要。** 既存のWebSocket APIとの互換性は維持しない。

---

## 1. アーキテクチャ変更

### 1-1. WebSocket → HTTP REST API への移行

**方針**: WebSocketコードは削除せずソース上に残すが、v0.2.0ではHTTPのみを有効化する。

- `internal/ws/` パッケージはソースコード上に残す（将来の再有効化に備える）
- ただしデフォルトでは無効。起動時にWebSocketサーバーは立ち上げない
- 設定ファイルで `"enableWebSocket": true` とした場合のみWSサーバーを起動する（デフォルト: `false`）
- raku-sika-lite v0.6.0 はHTTP APIのみを使用する

### 1-2. HTTP REST APIサーバーの実装

現在の `/health` エンドポイントを拡張し、以下のREST APIを新設する。
既存のWebSocketハンドラのロジック（`internal/ws/handler.go`）を再利用してHTTPハンドラを実装する。

**ルーター**: Go標準ライブラリ `net/http` の `http.ServeMux` を使用。外部ルーターライブラリは不要。

### 1-3. LAN内アクセス制限

**Hub はLAN内のサービスからのみAPIリクエストを受け付ける。**

- リッスンアドレスは `0.0.0.0:19800` のまま（LAN内の他端末からアクセス可能にするため）
- ミドルウェアでリクエスト元IPをチェックし、プライベートIPアドレスのみ許可する:
  - `127.0.0.0/8` (localhost)
  - `10.0.0.0/8`
  - `172.16.0.0/12`
  - `192.168.0.0/16`
  - `::1` (IPv6 localhost)
  - `fe80::/10` (IPv6 link-local)
- 許可外のIPからのリクエストには `403 Forbidden` を返す
- CORSは不要（同一LAN内のブラウザからのfetchリクエストのため、ブラウザからはlocalhostまたはIPアドレスでアクセス）

**注意**: CORSヘッダーの設定は必要。ブラウザからのクロスオリジンリクエスト（例: `https://raku-sika-lite.web.app` → `http://192.168.50.40:19800`）のため、以下のヘッダーを返す:
- `Access-Control-Allow-Origin: *`（LAN内限定なのでワイルドカードで十分）
- `Access-Control-Allow-Methods: GET, POST, OPTIONS`
- `Access-Control-Allow-Headers: Content-Type`

---

## 2. HTTP REST API エンドポイント仕様

### 共通仕様

- Content-Type: `application/json`（レスポンス。プレビュー画像のみ `image/png`）
- エラーレスポンス形式:
  ```json
  {
    "status": "error",
    "code": "ERROR_CODE",
    "message": "日本語エラーメッセージ"
  }
  ```
- エラーコードは既存の `internal/ws/message.go` で定義済みのものを流用

### 2-1. ヘルスチェック

```
GET /health
```

**レスポンス**:
```json
{
  "status": "ok",
  "scale": {
    "connected": true,
    "port": "/dev/ttyUSB0"
  },
  "printer": {
    "connected": true,
    "name": "Brother_QL-820NWB"
  },
  "scanner": {
    "connected": true,
    "device": "/dev/input/event3"
  }
}
```

**変更点**: 現在の `/health` レスポンス形式を上記に統一。`connected`, `port`, `printerConnected` 等のフラットな構造から、`scale` / `printer` / `scanner` のネスト構造に変更。

### 2-2. はかり操作

#### 計量

```
POST /scale/weigh
```

**レスポンス（成功・安定）**:
```json
{
  "status": "ok",
  "value": 12.34,
  "unit": "kg",
  "stable": true
}
```

**レスポンス（計測中・不安定）**:
```json
{
  "status": "weighing",
  "retry": 3,
  "maxRetry": 10
}
```

**実装**: 既存の `handleWeigh()` ロジックをそのまま使用。WebSocketのリクエスト/レスポンスをHTTP request/responseに変換するだけ。

**Hub側キャッシュ**: シリアルポートの読み取り結果を500msキャッシュし、複数リクエストに同じ値を返す。

#### 風袋引き

```
POST /scale/tare
```

**レスポンス**: `{ "status": "ok" }`

#### ゼロリセット

```
POST /scale/zero
```

**レスポンス**: `{ "status": "ok" }`

### 2-3. ラベル印刷

#### 印刷

```
POST /printer/print
Content-Type: application/json
```

**リクエスト**:
```json
{
  "template": "traceable_deer",
  "copies": 2,
  "data": {
    "productName": "シカ ロース",
    "productQuantity": "1.5 kg",
    "individualNumber": "20240315-01",
    "deadlineDate": "2024-06-15",
    "storageTemperature": "-18℃以下",
    "qrCode": "https://example.com/t/abc123/def456",
    "facilityName": "○○ジビエ加工施設",
    "ingredient": "シカ肉（北海道産）"
  }
}
```

**テンプレート一覧**:
| テンプレートID | 用途 | 獣種/カテゴリ |
|---------------|------|-------------|
| `traceable_deer` | トレサ製品（シカ） | シカ |
| `traceable_bear` | トレサ製品（クマ） | クマ |
| `non_traceable_deer` | 非トレサ製品 | - |
| `processed` | 加工食品 | - |
| `pet` | ペットフード | - |
| `individual_qr` | 個体QRラベル（解体完了時自動印刷） | - |

**レスポンス（成功）**: `{ "status": "ok", "copies": 2, "message": "印刷完了" }`

**タイムアウト**: 60秒（同期レスポンス）

#### プレビュー

```
POST /printer/preview
Content-Type: application/json
```

リクエストは `/printer/print` と同構造（`copies` は無視）。
レスポンス: `Content-Type: image/png` のPNG画像バイナリ。

#### テスト印刷

```
POST /printer/test
```

ボディ不要。レスポンス: `{ "status": "ok", "message": "テスト印刷完了" }`

### 2-4. バーコードリーダー（QRスキャナー）

USB接続のバーコードリーダーをラズパイに接続し、QRコードの読み取り結果をHTTP APIで提供する。

#### 対象デバイス

**Symcode 定置式 2D バーコードスキャナー（ハンズフリー）**

| 項目 | 仕様 |
|------|------|
| タイプ | 2D CMOS 無線バーコードスキャナー |
| インタフェース | USB（HIDキーボードデバイスとして動作） |
| プロセッサ | ARM 32-bit |
| イメージセンサ | 640x480 CMOS |
| 走査スピード | 200回/秒 |
| 電源 | DC 5V / 400mA±5% |
| ドライバ | 不要（Linux HID互換） |

**動作原理**: USBキーボードデバイスとしてOSに認識され、QRコード内容をキーストロークとして入力する（末尾にEnter）。Linuxでは `/dev/input/eventN` として認識される。

#### Hub側の実装方針

- `/dev/input/eventN` からHIDイベント（`input_event` 構造体）を直接読み取る
- **排他アクセス（`EVIOCGRAB` ioctl）** でデバイスを占有し、OS側へのキーストローク漏れを防止する
- キーイベント（`EV_KEY`）を文字に変換し、Enter（`KEY_ENTER`）で区切って1スキャン分の文字列を組み立てる
- スキャンされた値をメモリ上にバッファリング（最新1件のみ保持）
- 外部ライブラリ: なし（Linux evdev は syscall で直接読める。Goの `syscall.IoctlSetInt` + `os.File.Read` で十分）
- デバイス切断時は自動再接続ループ（3秒間隔。はかりと同じパターン）

#### デバイス自動検出

USB HIDバーコードリーダーの検出方法:

1. `/sys/class/input/event*/device/` 配下の `name` を読み取り
2. 既知のバーコードリーダー名パターン（設定ファイルで指定可能）にマッチするデバイスを選択
3. マッチしなければ、VID/PID（設定ファイル `scannerVid` / `scannerPid`）でフィルタ
4. 該当する `/dev/input/eventN` を開く

**設定ファイル例**:
```json
{
  "scannerVid": "",
  "scannerPid": "",
  "scannerDeviceName": ""
}
```

空の場合は自動検出をスキップ（バーコードリーダーなし）。
Symcodeスキャナーの場合、接続後に `cat /sys/class/input/event*/device/name` でデバイス名を確認し、設定に記入する。

#### 4-1. 最新スキャン値の取得

```
GET /scanner/scan
```

**目的**: バーコードリーダーが最後にスキャンしたQRコードの内容を取得する。

**レスポンス（スキャン値あり）**:
```json
{
  "status": "ok",
  "value": "https://example.com/t/abc123xyz/product456",
  "scannedAt": "2026-03-17T05:30:00.123Z"
}
```

**レスポンス（スキャン値なし = 前回取得以降に新しいスキャンがない）**:
```json
{
  "status": "ok",
  "value": null
}
```

**レスポンス（バーコードリーダー未接続）**:
```json
{
  "status": "error",
  "code": "SCANNER_NOT_CONNECTED",
  "message": "バーコードリーダーが接続されていません"
}
```

**動作仕様**:
- スキャン値は **一度取得したら消費される**（次の `GET /scanner/scan` では `null` が返る）
- これにより、同じQRコードを意図せず二重処理することを防ぐ
- Hub内部では最新1件のスキャン値のみ保持（メモリ効率）

**ポーリングパターン（フロントエンド側）**:
- スキャン待機中の画面で **300ms間隔** でポーリング
- `value` が非null の場合、フロントの `parseQRCode()` でパースして処理
- 画面遷移・フォーカス喪失時はポーリング停止

#### 4-2. スキャナー状態確認

`GET /health` のレスポンスに `scanner` フィールドを追加:

```json
{
  "status": "ok",
  "scale": { "connected": true, "port": "/dev/ttyUSB0" },
  "printer": { "connected": true, "name": "Brother_QL-820NWB" },
  "scanner": { "connected": true, "device": "/dev/input/event3" }
}
```

#### フロントエンド側のQRコード形式

Hub APIは **QRコードの生テキストをそのまま返す**。パースはフロントエンド側で行う。
フロントの `parseQRCode()` 関数が対応するフォーマット:

| フォーマット | 例 | 抽出結果 |
|-------------|-----|---------|
| `/t/{individualId}/{productId}` | `https://example.com/t/abc123/product456` | `{ individualId: "abc123", productId: "product456" }` |
| `/trace/{individualId}/{productKey}` | `https://example.com/trace/abc123/product456` | 同上 |
| `/t/{individualId}` | `https://example.com/t/abc123` | `{ individualId: "abc123" }` |
| 直接ID（15-30文字の英数字） | `abc123xyz_def456` | `{ individualId: "abc123xyz_def456" }` |

#### 利用画面とスキャン対象

| 画面 | ルート | スキャン対象 | 用途 |
|------|--------|------------|------|
| 解体（枝肉入庫） | `/input/dismantling` | 個体QR | 個体選択 |
| 枝肉出庫 | `/input/carcass-dispatch` | 個体QR | 個体選択 |
| 精肉入庫 | `/input/meat-inbound` | 個体QR | 個体選択 |
| 精肉出庫 | `/input/meat-outbound` | 製品QR | 製品選択 |
| 販売記録 | `/input/sales/new` | 製品QR | 明細追加 |
| 個体詳細 | `/view/individuals/[id]` | 個体QR | 詳細表示遷移 |
| 製品詳細 | `/view/meat-products/[id]` | 製品QR | 詳細表示遷移 |

すべての画面で、バーコードリーダー未接続時は **リスト選択にフォールバック** する（任意デバイス扱い）。

---

### 2-5. バージョン情報（新規）

```
GET /version
```

**レスポンス**:
```json
{
  "version": "0.2.0",
  "commit": "abc1234",
  "buildDate": "2026-03-17T00:00:00Z"
}
```

ビルド時に `-ldflags` でバージョン情報を埋め込む。

---

## 3. プリンタ対応: QLシリーズのみ

### 変更内容

- **Brother TDシリーズへの対応は不要。** QLシリーズ（QL-800 / QL-820NWB）のみに対応する
- 現在の `internal/printer/` の実装はQL前提なので、大きな変更は不要
- `deploy/setup-printer.sh` のTD関連処理があれば削除
- QL-800/820のラベルサイズ: 62mm幅（現在の実装通り）

---

## 4. メモリ効率・2GBラズパイ対応

### 設計方針

2GBメモリのRaspberry Pi（Raspberry Pi 4 Model B 2GB / Raspberry Pi 5 2GB相当）で安定稼働することを条件とする。

### 具体的な制約・実装方針

1. **Goランタイム**: `GOMEMLIMIT` 環境変数で上限を設定（推奨: `256MiB`）
2. **goroutine数の制限**: HTTPリクエストハンドラは `net/http` のデフォルトで十分（同時接続はLAN内数台が上限）
3. **ラベル画像生成**: 1リクエストごとに画像を生成→印刷→メモリ解放。画像のキャッシュは行わない
4. **ログファイル**: 現行の月次ローテーション + 1年自動削除を維持
5. **外部プロセス呼び出し（CUPS `lp`コマンド）**: 現行通り。メモリ効率的に問題なし
6. **依存ライブラリの最小化**: 新たな重量級ライブラリの追加は避ける。HTTPルーターは標準ライブラリ `net/http` を使用
7. **バイナリサイズ**: Go標準ライブラリのみ追加であれば増加は最小限。目標: 15MB以下

### パフォーマンス目安

| 操作 | 想定レスポンス時間 | メモリ使用量 |
|------|-------------------|-------------|
| ヘルスチェック | < 10ms | 無視可能 |
| 計量（安定時） | < 500ms | 無視可能 |
| スキャナー読み取り | < 5ms（バッファ読み出しのみ） | 無視可能 |
| ラベル印刷（画像生成 + CUPS） | < 3秒 | 一時的に ~20MB（画像バッファ） |
| アイドル時 | - | ~15-20MB（Go runtime + デバイス接続維持） |

---

## 5. リリース管理: GitHub Releases

### 方針

GitHub Releasesでバイナリをリリースし、ラズパイへのデプロイを簡素化する。

### バージョニング

- セマンティックバージョニング: `v0.2.0`, `v0.2.1`, ...
- Gitタグ: `v0.2.0`

### GitHub Actions ワークフロー

`.github/workflows/release.yml` を新設:

```yaml
name: Release

on:
  push:
    tags:
      - 'v*'

jobs:
  release:
    runs-on: ubuntu-latest
    permissions:
      contents: write
    steps:
      - uses: actions/checkout@v4

      - uses: actions/setup-go@v5
        with:
          go-version: '1.24'

      - name: Build for Raspberry Pi (ARM64)
        env:
          GOOS: linux
          GOARCH: arm64
        run: |
          VERSION=${GITHUB_REF_NAME}
          COMMIT=$(git rev-parse --short HEAD)
          BUILD_DATE=$(date -u +%Y-%m-%dT%H:%M:%SZ)
          go build -ldflags "-s -w -X main.version=${VERSION} -X main.commit=${COMMIT} -X main.buildDate=${BUILD_DATE}" -o raku-sika-hub-linux-arm64

      - name: Create Release
        uses: softprops/action-gh-release@v2
        with:
          files: raku-sika-hub-linux-arm64
          generate_release_notes: true
```

### デプロイ手順（ラズパイ側）

```bash
# 最新リリースをダウンロード
curl -sL $(curl -s https://api.github.com/repos/deervery/raku-sika-hub/releases/latest \
  | grep browser_download_url | cut -d '"' -f 4) -o raku-sika-hub

chmod +x raku-sika-hub
sudo systemctl stop raku-sika-hub
mv raku-sika-hub ~/raku-sika-hub/raku-sika-hub
sudo systemctl start raku-sika-hub
```

または、`deploy/update.sh` スクリプトを新設して上記を自動化する。

### ビルド時のバージョン埋め込み

`main.go` にバージョン変数を追加:

```go
var (
    version   = "dev"
    commit    = "unknown"
    buildDate = "unknown"
)
```

`/version` エンドポイントでこれを返す。

---

## 6. 設定ファイルの変更

### config.json の変更

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

**変更点**:
- `maxClients` を削除（HTTPはステートレスなので不要）
- `enableWebSocket` を追加（デフォルト: `false`）
- `scannerVid` / `scannerPid` / `scannerDeviceName` を追加（バーコードリーダー検出用。空の場合はスキャナー無効）

---

## 7. プロジェクト構造の変更

### 新規追加

```
internal/
  http/
    server.go       # HTTPサーバー（ルーティング、ミドルウェア）
    handler.go      # HTTPリクエストハンドラ
    middleware.go    # LAN制限ミドルウェア、CORSミドルウェア
    response.go     # JSON/エラーレスポンスヘルパー

  scanner/
    client.go       # バーコードリーダークライアント（evdev読み取り、排他アクセス、バッファリング）
    detector.go     # USB HIDデバイス自動検出（/sys/class/input/ スキャン）
    keymap.go       # HIDキーコード → 文字変換テーブル

.github/
  workflows/
    release.yml     # GitHub Releases ワークフロー

deploy/
  update.sh         # リリースバイナリのダウンロード・更新スクリプト
```

### 変更

```
main.go             # バージョン変数追加、HTTP/WSの起動分岐
internal/
  app/app.go        # HTTPサーバーの統合、WSの条件分岐、スキャナー統合
  config/config.go  # enableWebSocket, maxClients削除, scanner設定追加
```

### 変更なし（そのまま利用）

```
internal/
  scale/            # 全ファイルそのまま
  printer/          # 全ファイルそのまま
  logging/          # そのまま
  ws/               # コード残置（enableWebSocket: true時のみ利用）
```

---

## 8. 実装タスク一覧

### Phase 1: HTTP REST API

| # | タスク | 対象ファイル |
|---|-------|------------|
| 1 | `internal/http/server.go` - HTTPサーバー・ルーティング | 新規 |
| 2 | `internal/http/middleware.go` - LAN制限 + CORS | 新規 |
| 3 | `internal/http/handler.go` - 全エンドポイントのハンドラ | 新規 |
| 4 | `internal/http/response.go` - レスポンスヘルパー | 新規 |
| 5 | `main.go` - バージョン変数、HTTP/WS起動分岐 | 変更 |
| 6 | `internal/app/app.go` - HTTPサーバー統合 | 変更 |
| 7 | `internal/config/config.go` - 新設定項目 | 変更 |
| 8 | はかり計量の500msキャッシュ実装 | `internal/scale/client.go` |

### Phase 2: バーコードリーダー

| # | タスク | 対象ファイル |
|---|-------|------------|
| 9 | `internal/scanner/detector.go` - USB HIDデバイス自動検出 | 新規 |
| 10 | `internal/scanner/keymap.go` - HIDキーコード→文字変換 | 新規 |
| 11 | `internal/scanner/client.go` - evdev読み取り・排他アクセス・バッファリング・自動再接続 | 新規 |
| 12 | `internal/http/handler.go` に `/scanner/scan` ハンドラ追加 | 変更 |
| 13 | `internal/app/app.go` にスキャナー統合 | 変更 |
| 14 | `GET /health` レスポンスに `scanner` フィールド追加 | 変更 |

### Phase 3: リリース管理

| # | タスク | 対象ファイル |
|---|-------|------------|
| 15 | GitHub Actions release.yml | 新規 |
| 16 | `deploy/update.sh` - 更新スクリプト | 新規 |
| 17 | `/version` エンドポイント | `internal/http/handler.go` |

### Phase 4: WebSocket無効化

| # | タスク | 対象ファイル |
|---|-------|------------|
| 18 | WSサーバー起動を条件分岐（`enableWebSocket`） | `internal/app/app.go` |
| 19 | デフォルトでWS無効の設定 | `config.json` |

### Phase 5: テスト・整備

| # | タスク | 対象ファイル |
|---|-------|------------|
| 20 | HTTPハンドラのユニットテスト | `internal/http/handler_test.go` |
| 21 | LAN制限ミドルウェアのテスト | `internal/http/middleware_test.go` |
| 22 | スキャナークライアントのユニットテスト | `internal/scanner/client_test.go` |
| 23 | README.md の更新 | `README.md` |
| 24 | systemdサービスファイルに `GOMEMLIMIT` 追加 | `deploy/raku-sika-hub.service` |
| 25 | systemdサービスファイルにスキャナーデバイスのアクセス権設定 | `deploy/setup.sh` |

---

## 9. 非対応・スコープ外

以下はv0.2.0のスコープ外とする:

- **Brother TDシリーズ対応**: 不要。QLシリーズのみ
- **WebSocketの積極的な利用**: コードは残すがデフォルト無効
- **認証・認可**: LAN内限定なので不要
- **HTTPS**: LAN内通信なので不要
- **Docker化**: ラズパイにsystemdで直接デプロイ
- **データベース**: Hubはステートレス。永続化は不要

---

## 10. raku-sika-lite v0.6.0 との連携確認事項

| 項目 | lite側 | hub側 |
|------|--------|-------|
| ヘルスチェック | `useHubHealth` フック（30秒ポーリング） | `GET /health` |
| 計量 | `useScale` フック（1秒ポーリング） | `POST /scale/weigh` |
| 風袋引き | ボタン押下時 | `POST /scale/tare` |
| ゼロリセット | ボタン押下時 | `POST /scale/zero` |
| QRスキャン | `useScanner` フック（300msポーリング） → `parseQRCode()` | `GET /scanner/scan` |
| ラベル印刷 | `HubLabelPrintButton` | `POST /printer/print` |
| プレビュー | プレビューダイアログ | `POST /printer/preview` |
| テスト印刷 | 施設設定画面 | `POST /printer/test` |
| Hub URL | 施設設定で変更可能（デフォルト: `http://192.168.50.40:19800`） | `listenAddr: 0.0.0.0:19800` |
