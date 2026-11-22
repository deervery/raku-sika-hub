# Androidに同梱する顔UI

既存 raku-sika-lite の VoiceStation と依存部品を切り出したソース。通常のNextサーバーは不要。
元: /home/sikax/ws/rs/raku-sika-voice-station-20261104 (2026-11-22時点)。コピー後のAndroid固有差分はこのディレクトリで管理。

`npm ci && npm test && npm run build` で `../client/src/main/assets/face` を更新してからGradleでAPKを作る。
React/ReactDOMのライセンスは配布assetsのTHIRD_PARTY_LICENSES.txtに同梱。
APIキーや本番マスターは同梱しない。ブラウザーだけで開くと端末機能は動作しない。
機能・未検証事項は `docs/station/ANDROID.ja.md` を参照。
