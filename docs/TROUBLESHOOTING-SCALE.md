# A&D 計量器 トラブルシューティング (Pi)

## 対象機器
- A&D HV-C シリーズ (HV-60KCWP-K)
- USB接続 (FTDI VID:0403, PID:6015)

## 確認手順（上から順に）

### 1. USB デバイス認識
```bash
# カーネルログでUSB接続を確認
dmesg | tail -30
# → "FTDI USB Serial Device converter now attached to ttyUSB0" が出ればOK

# USB デバイス一覧
lsusb
# → "Future Technology Devices International" が見えればOK

# シリアルデバイス確認
ls -la /dev/ttyUSB*
# → /dev/ttyUSB0 等が存在すればOK
```

### 2. アクセス権限
```bash
# 現在のユーザーがdialoutグループに入っているか
groups
# → "dialout" が含まれていればOK

# なければ追加（要再ログイン）
sudo usermod -aG dialout $USER
```

### 3. FTDI 自動検出パス
hub は3つのパスを順に試行する:

```bash
# パス1: /sys/bus/usb-serial/devices
ls -la /sys/bus/usb-serial/devices/
# → ttyUSB0 等のシンボリックリンクがあればOK

# パス2: /dev/serial/by-id (udev)
ls -la /dev/serial/by-id/
# → FTDI を含むリンクがあればOK

# パス3: /sys/class/tty/ttyUSB*
ls -d /sys/class/tty/ttyUSB* 2>/dev/null
# → 存在すればOK
```

### 4. VID/PID 手動確認
```bash
# ttyUSB0 の VID/PID を確認
cat /sys/class/tty/ttyUSB0/device/../../../idVendor
cat /sys/class/tty/ttyUSB0/device/../../../idProduct
# → 0403 / 6015 であればOK
```

### 5. hub ログ確認
```bash
# systemd ログ
journalctl -u raku-sika-hub -n 100 --no-pager

# ファイルログ
tail -n 100 ~/raku-sika-hub/logs/service-*.log
```

ログに出る主なメッセージ:
- `FTDI検出: ... で /dev/ttyUSB0 を発見` → 自動検出成功
- `FTDI検出: ... 失敗` → そのパスでは見つからない（次を試行）
- `ポートオープン失敗` → デバイスは見つかったが開けない（権限/使用中）
- `スケール応答なし` → ポートは開いたが計量器が応答しない
- `スケール接続完了` → 正常接続

### 6. WebSocket テスト
```bash
wscat -c ws://127.0.0.1:19800

# 接続直後に connection_status が返る
# → {"type":"connection_status","connected":true,"port":"/dev/ttyUSB0"}

# 計量テスト
{"type":"weigh","requestId":"w1"}
# → {"type":"weight","requestId":"w1","value":0.0,"unit":"kg","stable":true}

# 風袋引き
{"type":"tare","requestId":"t1"}
# → {"type":"tare_ok","requestId":"t1"}

# ゼロリセット
{"type":"zero","requestId":"z1"}
# → {"type":"zero_ok","requestId":"z1"}

# ヘルスチェック
{"type":"health","requestId":"h1"}
# → {"type":"health_ok","requestId":"h1","connected":true,"port":"/dev/ttyUSB0"}
```

## よくある問題と対処

### /dev/ttyUSB* が存在しない
- USB ケーブルを抜き差し
- `dmesg | tail -30` でカーネルエラーを確認
- 別の USB ポートを試す
- `lsusb` で FTDI デバイスが見えるか確認

### Permission denied
```bash
sudo usermod -aG dialout rakusika
# → 再起動 or 再ログイン必須
sudo systemctl restart raku-sika-hub
```

### 自動検出が不安定
systemd override でポートを明示指定:
```bash
sudo systemctl edit raku-sika-hub
```
以下を記載:
```ini
[Service]
Environment=PORT=/dev/ttyUSB0
```
```bash
sudo systemctl restart raku-sika-hub
```

### スケール応答なし (ポートは開ける)
- スケールの電源が入っているか確認
- 通信設定: 2400bps, 7bit, Even parity, 1 stop bit
- スケール側の通信モード設定を確認 (dip switch / メニュー)
- 別のプログラムがポートを使用していないか: `fuser /dev/ttyUSB0`

### 実機を抜いた時
- hub は 3秒間隔で再接続を試行する
- 差し直し後、自動的に再接続される
- 再接続しない場合: `sudo systemctl restart raku-sika-hub`

## SCALE_DRIVER 切り替え

### 実機モード (デフォルト)
```bash
# SCALE_DRIVER を設定しない、または:
sudo systemctl edit raku-sika-hub
```
```ini
[Service]
Environment=SCALE_DRIVER=real
```

### モックモード (テスト用)
```bash
sudo systemctl edit raku-sika-hub
```
```ini
[Service]
Environment=SCALE_DRIVER=mock
```
```bash
sudo systemctl restart raku-sika-hub
```

モックモードでは weigh は常に 0.00 kg を返す。
