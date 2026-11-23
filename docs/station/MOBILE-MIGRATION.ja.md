# Liteへの製品コード移管

2026-11-24。モバイル製品コードの正本を raku-sika-lite に統一する。

| 旧Hubパス | Liteの移管先 |
| --- | --- |
| experimental/android-client（face-ui以外） | mobile/android |
| experimental/android-client/face-ui | packages/assistant-ui |
| face-ui内のWebと共通の11定義 | packages/domain |
| experimental/station/assistant_service.py とそのテスト | services/assistant |
| docs/station/ANDROID* と screenshots | docs/mobile/history |

[Liteの開発手順](https://github.com/deervery/raku-sika-lite/blob/dev/mobile/README.ja.md)
と[責務・接続契約](https://github.com/deervery/raku-sika-lite/blob/dev/docs/mobile/ARCHITECTURE.ja.md)を参照。

HubのGo機器処理・APIとPi向けリリースは変更しない。Go/Pipecat/API/MCPの研究コードはここに残す。
`experimental/station/secret_card.py` は既存のキー入力カードを再利用するため残す。
これは開発用の秘密入力ツールであり、Androidのビルド依存ではない。
稼働済みのプロセスやAPKはソース削除だけで停止・更新されない。施設への配布は別管理。
