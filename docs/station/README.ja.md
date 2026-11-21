# ラクシカステーション：Go状態管理・音声接続の検証基盤

2026-11-22のユーザー設計方針を基準にした、**実機操作を行わない検証実装**。
既存5施設の設定・サービス・業務データは変更しない。
既存のOpenAI Realtime / Web内状態管理PoCは比較用として保持し、この構成への移行完了とは扱わない。

## 現在の設計基準

- 対象業務は計量→在庫→ラベル印刷→販売記録。Pi 5とAndroid、日本語・騒音・訂正・応答速度を評価する。
- Goハブが業務状態・入力検証・確定・履歴の正本。音声、Web、API、MCPは同じ処理を使う。
- Pipecatは別プロセス。ASR/TTS接続、発話終了、割り込み、再生を担う。Rustへの全面移行はしない。
- 明確な定型入力はLLMを経由しない。参照は必要条件が揃えば実行し、印刷・登録は確認後に実行する。
- 雑談・曖昧な解釈・複雑な分析にはGeminiを使う方針。ただし本実装では未接続。
- 分析は期間・施設・指標・比較条件を明示し、数値は集計処理から得る。LLMの補完値を実行・報告の正本にしない。
- 発話ごとに経路を再判定する。雑談中も作業を保持し、毎回の業務復帰案内は不要。
- Androidはまず施設内Pi/ハブへ接続するクライアントとして検証する。
- 定義済みUI部品を状態に応じて構成する。操作中の位置・フォーカスを維持し、分析通知で作業を置き換えない。

## この変更で動く範囲

| 部分 | 実装・検証状態 |
|---|---|
| Go作業状態 | セッション・作業・操作ID、版、メディア世代、入力・不足・変更項目、状態、履歴 |
| 定型入力 | 商品・部位・ロット・重量・単位・枚数・円額、保守的な日本語文字列抽出 |
| 確認・訂正・取消 | 最新確認IDのみ受理。訂正で旧確認を無効化。雑談中の「はい」は実行しない |
| 永続化 | JSONチェックポイント、ファイル同期・rename・ディレクトリ同期、Linux排他ロック |
| 操作 | 印刷・在庫登録・販売記録はモック。実プリンタ/施設DBのアダプターなし |
| 結果未確認 | タイムアウト/実行中の再起動でunknown。再送せずLookup。完了証跡がなければunknown維持 |
| Web | 顔・応答バブル・状態カード。1項目は大きく、2項目は縦2段。入力済み/未入力/全項目切替、ボタン編集 |
| Pipecat | 別プロセスのJSONL→TextFrame→Go→応答封筒。マイク/スピーカー接続ではない |
| MCP | stdioの初期化・ツール一覧・参照・提案・模擬分析。RESTと同じGo検証処理 |
| バックグラウンド分析 | 検証用固定データを整数集計、結果を永続化。作業版・フォーム・確認を変更しない |
| 評価器 | 重要項目完全一致、計測値の中央値/p95。未計測値はnullを維持 |

音声モデル、Gemini、実際の録音・再生はまだ接続していない。
Goの状態遷移・接続契約を先に通すための実装であり、以前の実機印刷成功を本実装の実機検証に流用しない。

## 起動とテスト

Linux/WSLで実行する。永続化の排他・同期処理もLinux向け。Pi上での実行確認は別工程。

```sh
go run ./cmd/station-lab --port 3110 --data /tmp/rakusikastation-lab
# http://localhost:3110/ （/experimental/station も同じ検証画面）

go test -race ./...
go vet ./...
python3 -m unittest discover -s experimental/station -v
node --check internal/station/web/app.js

go build -o /tmp/station-lab ./cmd/station-lab
python3 experimental/station/smoke.py --binary /tmp/station-lab \
  --voice-python /home/sikax/.cache/rakusika-voice-bench/venv/bin/python
```

Pipecatは添付検証環境を再利用した。`sources.lock.json`指定コミットは
`77bb17660805a7fb1db5f02307044255d49340a6`。
再利用先はソースアーカイブで`.git`を持たないため、今回リモートから再インストールして一致を監査したものではない。
将来の再構築では添付のbootstrap/lockと依存条件を使い、最新版へ無条件に更新しない。

サーバーは127.0.0.1のみ。Origin/Host検査とJSON限定POSTを使う。
施設LAN公開用の認証・施設スコープ管理は未実装なので、0.0.0.0公開に変更しない。
AndroidはADB接続後に検証ポートのreverseとブラウザーで試す。現時点でADB端末一覧は空であり、実機表示は未確認。

## 接続契約 v0.1

### 取得と入力

- `POST /api/sessions`：モックセッション作成。
- `GET /api/sessions/{id}`：`state`、任意の`operation`、`jobs`、`mock:true`。
- `GET /api/patterns`：適用業務、必要項目、画面部品、質問・確認・訂正・取消・聞き直し・結果用パターン。
- `POST /api/commands`：UI/音声の状態変更。許可された固定actionのみ。
- `POST /api/analysis`：検証データ集計ジョブを開始。完了はセッションのjobsで照会する。

コマンド例（IDは説明用。実際はGET結果を使う）：

```json
{
  "session_id": "...",
  "revision": 3,
  "media_epoch": 4,
  "action": "patch",
  "fields": {"copies": "2"}
}
```

`action`は`begin`、`patch`、`utterance`、`confirm`、`cancel`、`repeat`、`interrupt`、`reconcile`、`chat`、`analysis`。
`kind`は`print`、`inventory`、`sale`、`stock`。整数・小数・ロットは文字列で保持し、ロット先頭ゼロを失わない。
このPoCの枚数上限は30。重量の単位を推定せず、kg/gを明示させる。商品辞書・数量上限は本番仕様として未確定。

revision/epoch不一致は409。**クライアントは古いASRや承認を、新しい版へ置き換えて自動再送してはいけない。**
Webは最新状態を再取得し、利用者が再操作する。音声側は発話開始時の版・世代を保持する。

### 確認と実行

確認はGoが返す`confirmation.id/revision/text`で構成する。画面も同じstate.fieldsを表示する。
確定には同じ版・世代と`confirmation_id`が必要。訂正でrevision/epoch/確認IDを更新する。
雑談で業務の版は変えないがメディア世代と会話経路を更新し、雑談中の「はい」「いいね」を承認にしない。
「もういい」は取り消し/会話終了の確認質問にする。

操作を**永続化してから**Executeする。同じ確認IDの再送は保存済み操作を返す。
外部呼出しのエラーを「未実行」と断定しない。`unknown`中は訂正・新規操作・取消を止め、読取り専用Lookupで確認する。
印刷機自身の実行証跡が得られない場合、ソフトウェアだけでexactly-onceの実印刷を保証できない。オペレーターによる現物確認の契約は今後必要。

状態は`drafting / awaiting_confirmation / executing / done / failed / unknown / cancelled`。
ストア破損・書込み失敗はフェイルクローズし、台帳を空に戻して続行しない。
保存方式はPoC用の全体チェックポイントで、履歴/セッションの無期限増加・高負荷・ディスク故障を解決済みとはしない。

### MCPとモデル境界

```sh
STATION_HUB=http://127.0.0.1:3110 python3 experimental/station/mcp_bridge.py
```

ツールは`station_create / station_get / station_propose / station_analyze`。
モデルの提案もGoの同じ検証を受ける。モデル用ツールから`confirm`や偽の`utterance`を呼べない。
利用者の明示確認はUIまたは決定的な音声経路から渡す。MCPを本番へつなぐ際もこの境界を維持する。
MCPは[公式tools仕様](https://modelcontextprotocol.io/specification/2025-03-26/server/tools)と
[stdio仕様](https://modelcontextprotocol.io/specification/2025-03-26/basic/transports)に基づく小さな検証用サブセットで、認証付き本番MCPサーバーではない。

### 音声・割り込み

`pipecat_bridge.py`は入力済みテキストに対してGoを呼び、応答とsession/task/revision/epochを返す。
`speech.audio_generated:false`を明示し、音声生成済みを装わない。
`PlaybackGate`はローカル割り込み時に停止コールバックと待機音声の破棄を行い、古い世代の出力を拒否する。
実スピーカーへの接続・停止時刻計測は未実装。推論結果を棄却することと、推論ワーカーを停止することは別である。
固定音声のキャッシュ生成・モデル/声/言語/本文を含むキャッシュキー設計は次のTTS工程で行う。

### 分析・UI

分析APIはsession_id、facility、start/end、compare_start/compare_end、metricを要求する。
今回はfacility=`fixture`、2026年10〜11月、metric=`sales_yen`または`weight_g`のみ。
固定4レコードをGoの整数で集計し、対象・比較・差・単位・データ出典を返す。Geminiの説明は未接続。
ジョブ完了はtaskのfields/view/revision/確認文を変更しない。UIは別の通知ボタンを表示し、利用者が開いた時だけ結果を表示する。
分析後の変更依頼は新しい業務入力・確認へ戻す。分析結果から操作は実行しない。

Webは入力済み・未入力・全項目モードを持ち、編集ダイアログを開いている間は画面再構成を保留する。
同時編集で古くなった入力は409で拒否する。任意の新規商品名、英字ロット入力、実ラベル画像プレビュー、読み上げ、AIによる表示モード選択は未実装。
現在の印刷カードは項目確認であり、プリンタの描画結果プレビューではない。

## 添付の再確認と実測

元の`README.ja.md`、`REPORT.ja.md`、`rakusika-voice-validation.zip`を確認した。
ZIP SHA-256：`6958174c37295a4ad5ffd73536870b13b40ee0213419fa43ebf615947f9441bd`（同梱値と一致）。
分離展開先：`/home/sikax/ws/rs/station-validation-20261122/validation`。
`implementation.lock.json`の43ファイルを読取り専用で照合し、全件一致。元の証跡・lock・モデルは変更していない。

| 今回の再実行 | 結果 |
|---|---|
| 添付test_safety.py | 20件成功 |
| Pipecat固定制御・idle | n=30、中央値1.566ms、p95 1.758ms、失敗0 |
| Pipecat固定制御・mock_hub負荷 | n=30、中央値1.546ms、p95 1.783ms、失敗0 |
| 新Go状態管理テスト | 15件成功、race検査を含む |
| 既存Goパッケージ回帰 | `go test -race ./...`と`go vet ./...`成功 |
| Python契約・評価器 | 5件成功 |
| Go/Pipecat/MCP別プロセス通し検証 | 11項目成功（[記録](evidence/integration.json)） |
| Linux ARM64 | ビルド成功。Pi実行確認ではない |
| Web | JS構文・HTTP配信成功。タブレットでの表示・タッチ・音声は未確認 |

p95はnearest-rank。生データと要約は[evidence](evidence/reproduction.json)に保存。
これはモデル推論・人の発話終了・実音再生を含まない固定処理。Rust候補およびASR/TTS全組合せは今回再実行していない。
旧報告のPi tiny 1.67秒、nano初PCM約250ms、RSS約1.3GiB等は添付の測定であり、新構成の実測として転載しない。

## 次の検証と未達条件

1. 添付全候補の再現：モデル本体/vendorはZIP外。model lock照合と推論再測定を追加する。
2. 人の日本語・現場騒音：複数話者、マイク距離、ロット先頭ゼロ、枚数、kg/g、訂正文、否定、曖昧な了承を含む同一コーパスを使う。
3. Whisper tiny/base/smallを基準に、Haas ONNX int8/int4を同条件比較。モデルID・量子化元・ストリーミング条件を確定してから測る。
4. nano-edge-ttsとtiny-voca-ja-tsukuyomi-chanを同じ確認文で比較。コードと声モデルのライセンス/再配布条件を分けて確認する。
5. Silero VAD単独とSmart Turn v3.2併用を比較。初期試作は明示的な会話開始を必須とする。
6. 端末マイク/スピーカーを接続し、発話終了→**実音**開始、割り込み→実音停止を同じ音声時計で測る。first PCMを代用しない。
7. Geminiの雑談・曖昧解釈・根拠付き分析を接続する。モデル出力は提案のみで、Goが検証・確定する。
8. Pi上でUI+Go+音声を同時稼働。Androidクライアント、切断/復帰/ハブ再起動、長時間、CPU/RAM/温度/ドロップアウトを測る。

`experimental/station/evaluate.py`はモデル・端末・騒音別の項目完全一致率と計測値の中央値/p95を出す。
欠測はnull、ロット`00123`と`123`は不一致。実音遅延を初PCMから補完しない。
合格閾値・測定時間・重要項目の許容誤り率は現場条件とともに決める。
本変更だけで「完全に安定」「Piで全体1秒」「Android実機検証済み」とは判断しない。
