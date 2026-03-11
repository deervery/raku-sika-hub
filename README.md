# raku-sika-hub

Raspberry Pi 常駐の機器制御サービス。
`raku-sika-wsa` 互換の WebSocket API を提供し、計量器とラベルプリンタを制御する。

## クイックスタート

```bash
npm install
npm run build
npm start
```

## 開発

```bash
# Mock scale で起動
npm run dev

# Real scale で起動（A&D 実機接続時）
SCALE_DRIVER=real SERIAL_PORT=/dev/ttyUSB0 npm run dev

# シリアルポート一覧
npm run ports
```

## 環境変数

| 変数 | デフォルト | 説明 |
|------|-----------|------|
| `HUB_PORT` | `19800` | WebSocket サーバーポート |
| `SCALE_DRIVER` | `mock` | `mock` or `real` |
| `SERIAL_PORT` | `` | シリアルポート (例: `/dev/ttyUSB0`) |
| `BAUD_RATE` | `2400` | ボーレート |
| `DATA_BITS` | `7` | データビット |
| `PARITY` | `even` | パリティ |
| `STOP_BITS` | `1` | ストップビット |
| `PRINTER_NAME` | `Brother QL-800` | プリンタ名 |
| `LOG_LEVEL` | `INFO` | ログレベル |

## WebSocket API (WSA 互換)

### 接続
```
ws://<hub-ip>:19800
```

### Client → Server

| type | fields | 説明 |
|------|--------|------|
| `weigh` | `requestId` | 安定重量を取得 |
| `tare` | `requestId` | 風袋引き |
| `zero` | `requestId` | ゼロリセット |
| `health` | `requestId` | ヘルスチェック |
| `status` | - | 接続状態を取得 |

### Server → Client

| type | fields | 説明 |
|------|--------|------|
| `weight` | `requestId`, `value`, `unit`, `stable` | 安定重量 |
| `weighing` | `requestId`, `retry`, `maxRetry` | 計量中の進捗 |
| `tare_ok` | `requestId` | 風袋引き成功 |
| `zero_ok` | `requestId` | ゼロリセット成功 |
| `health_ok` | `requestId`, `connected`, `port` | ヘルスチェック応答 |
| `error` | `requestId`, `code`, `message` | エラー |
| `connection_status` | `connected`, `port` | 接続状態 (接続時自動送信) |

### エラーコード
`UNSTABLE`, `TIMEOUT`, `OVERLOAD`, `PORT_ERROR`, `INVALID_REQUEST`, `UNKNOWN_TYPE`

## Raspberry Pi デプロイ

```bash
# ビルド
npm run build

# systemd に登録
sudo cp raku-sika-hub.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable raku-sika-hub
sudo systemctl start raku-sika-hub

# ログ確認
sudo journalctl -u raku-sika-hub -f
```

## lite 側の接続設定

`raku-sika-lite` の `lib/hooks/useScale.tsx` で接続先を変更:
```typescript
const SCALE_WS_URL = 'ws://<hub-ip>:19800';
```

## A&D 実機接続手順

1. A&D デバイスを USB 接続
2. `npm run ports` でポート確認
3. `SCALE_DRIVER=real SERIAL_PORT=/dev/ttyUSB0 npm run dev` で起動
4. `npm install serialport` が必要
