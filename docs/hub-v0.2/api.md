# raku-sika-hub v0.2 HTTP REST API リファレンス

Base URL: `http://<host>:19800`

## 共通仕様

- レスポンスは全て `Content-Type: application/json; charset=utf-8`（プレビュー画像のみ `image/png`）
- LAN内（プライベートIP）からのみアクセス可能。外部IPは `403 Forbidden`
- CORS: `Access-Control-Allow-Origin: *`
- `OPTIONS` リクエストには `204 No Content` を返す（プリフライト対応）

### エラーレスポンス共通形式

```json
{
  "status": "error",
  "code": "ERROR_CODE",
  "message": "日本語エラーメッセージ"
}
```

---

## GET /health

全デバイスの接続状態を返す。

### レスポンス `200 OK`

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

| フィールド | 型 | 説明 |
|---|---|---|
| `scale.connected` | bool | はかりが接続中か |
| `scale.port` | string | シリアルポートパス（未接続時は空） |
| `printer.connected` | bool | CUPSでプリンタが使用可能か |
| `printer.name` | string | 選択されたプリンタ名 |
| `scanner.connected` | bool | バーコードリーダーが接続中か |
| `scanner.device` | string | evdevデバイスパス（未接続時は空） |

---

## GET /version

ビルドバージョン情報を返す。

### レスポンス `200 OK`

```json
{
  "version": "0.2.0",
  "commit": "abc1234",
  "buildDate": "2026-03-17T00:00:00Z"
}
```

---

## POST /scale/weigh

計量を実行する。安定値が得られるまで内部で最大10回リトライ（各500ms）。
結果は500msキャッシュされ、連続リクエストには同じ値を返す。

### レスポンス `200 OK`（安定）

```json
{
  "status": "ok",
  "value": 12.34,
  "unit": "kg",
  "stable": true
}
```

### レスポンス `200 OK`（不安定 — 10回リトライ超過）

```json
{
  "status": "weighing",
  "retry": 10,
  "maxRetry": 10
}
```

### エラー `503 Service Unavailable`

```json
{
  "status": "error",
  "code": "SCALE_NOT_CONNECTED",
  "message": "スケールが接続されていません。USBケーブルを確認してください。"
}
```

### エラー `500 Internal Server Error`

| code | 状況 |
|---|---|
| `OVERLOAD` | 最大計量（60kg）超過 |
| `PORT_ERROR` | シリアルポート通信エラー |
| `PERMISSION_DENIED` | ポートのアクセス権限なし |
| `UNKNOWN_ERROR` | その他 |

---

## POST /scale/tare

風袋引き（ゼロ点をリセット）を実行する。

### レスポンス `200 OK`

```json
{
  "status": "ok"
}
```

### エラー

`/scale/weigh` と同一のエラーコード体系。

---

## POST /scale/zero

ゼロリセットを実行する。

### レスポンス `200 OK`

```json
{
  "status": "ok"
}
```

### エラー

`/scale/weigh` と同一のエラーコード体系。

---

## POST /printer/print

ラベルを印刷する。同期レスポンス（タイムアウト: 60秒）。

### リクエスト `Content-Type: application/json`

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

### テンプレート一覧

| template | 用途 |
|---|---|
| `traceable` | トレサ製品（汎用） |
| `traceable_deer` | トレサ製品（シカ） |
| `traceable_bear` | トレサ製品（クマ） |
| `non_traceable` | 非トレサ製品（汎用） |
| `non_traceable_deer` | 非トレサ製品（シカ） |
| `processed` | 加工食品 |
| `pet` | ペットフード |
| `individual_qr` | 個体QRラベル |

### data フィールド

| フィールド | 必須(traceable) | 必須(その他) | 説明 |
|---|---|---|---|
| `productName` | o | o | 品名 |
| `productQuantity` | o | o | 内容量 |
| `deadlineDate` | o | o | 消費期限 |
| `storageTemperature` | o | o | 保存温度 |
| `individualNumber` | o | - | 個体識別番号 |
| `captureLocation` | - | - | 捕獲場所 |
| `qrCode` | - | - | QRコードURL |
| `facilityName` | - | - | 加工施設名 |
| `ingredient` | - | - | 原材料 |
| `productIngredient` | - | - | 原材料名（加工/ペット用） |
| `nutritionUnit` | - | - | 栄養成分表示単位 |
| `caloriesQuantity` | - | - | エネルギー |
| `proteinQuantity` | - | - | たんぱく質 |
| `fatQuantity` | - | - | 脂質 |
| `carbohydratesQuantity` | - | - | 炭水化物 |
| `saltEquivalentQuantity` | - | - | 食塩相当量 |
| `attentionText` | - | - | 注意書き |

### レスポンス `200 OK`

```json
{
  "status": "ok",
  "copies": 2,
  "message": "印刷完了"
}
```

### エラー `400 Bad Request`

| code | 状況 |
|---|---|
| `INVALID_REQUEST` | JSONパースエラー / 不明なテンプレート / 部数超過(1〜30) / 必須フィールド不足 |

### エラー `500 Internal Server Error`

| code | 状況 |
|---|---|
| `PRINTER_NOT_CONFIGURED` | CUPSプリンタが見つからない |
| `PRINTER_PERMISSION_DENIED` | プリンタのアクセス権限なし |
| `PRINTER_DISABLED` | プリンタが無効化されている |
| `PRINTER_PAPER_ERROR` | 用紙切れ / ジャム |
| `PRINTER_ERROR` | その他の印刷エラー / レンダラ未初期化 |

---

## POST /printer/preview

ラベルのプレビュー画像を生成する。リクエストは `/printer/print` と同構造（`copies` は無視）。

### レスポンス `200 OK`

```
Content-Type: image/png

(PNG画像バイナリ)
```

### エラー

`/printer/print` と同一のエラーコード体系（JSONで返る）。

---

## POST /printer/test

テスト印刷を実行する。リクエストボディ不要。

### レスポンス `200 OK`

```json
{
  "status": "ok",
  "message": "テスト印刷完了"
}
```

### エラー

`/printer/print` と同一のプリンタエラーコード体系。

---

## GET /scanner/scan

バーコードリーダーの最新スキャン値を取得する。
値は**一度取得したら消費される**（次回リクエストでは `null`）。

### レスポンス `200 OK`（スキャン値あり）

```json
{
  "status": "ok",
  "value": "https://example.com/t/abc123xyz/product456",
  "scannedAt": "2026-03-17T05:30:00.123Z"
}
```

### レスポンス `200 OK`（スキャン値なし）

```json
{
  "status": "ok",
  "value": null
}
```

### エラー `503 Service Unavailable`

```json
{
  "status": "error",
  "code": "SCANNER_NOT_CONNECTED",
  "message": "バーコードリーダーが接続されていません"
}
```

---

## エラーコード一覧

### スケール

| code | 意味 |
|---|---|
| `SCALE_NOT_CONNECTED` | スケール未接続 |
| `UNSTABLE` | 計量値が安定しない |
| `OVERLOAD` | 過負荷（60kg超） |
| `PORT_ERROR` | シリアルポート通信エラー |
| `PERMISSION_DENIED` | ポートアクセス権限なし |
| `UNKNOWN_ERROR` | 予期しないエラー |

### プリンタ

| code | 意味 |
|---|---|
| `PRINTER_NOT_CONFIGURED` | CUPSプリンタ未設定/未検出 |
| `PRINTER_PERMISSION_DENIED` | プリンタアクセス権限なし |
| `PRINTER_DISABLED` | プリンタ無効 |
| `PRINTER_PAPER_ERROR` | 用紙切れ/ジャム |
| `PRINTER_ERROR` | その他の印刷エラー |

### スキャナー

| code | 意味 |
|---|---|
| `SCANNER_NOT_CONNECTED` | バーコードリーダー未接続 |

### 共通

| code | 意味 |
|---|---|
| `INVALID_REQUEST` | リクエスト不正 |
| `FORBIDDEN` | LAN外からのアクセス |
