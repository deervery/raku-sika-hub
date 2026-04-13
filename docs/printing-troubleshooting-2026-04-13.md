# Printing Troubleshooting (2026-04-13)

## 1. 62mm 幅なのに内容が 29mm 相当に縮む / 100mm 固定で出る

原因:

- CUPS へ渡す media/PageSize の値形式が非標準（`custom_62x43mm_62x43mm`）だと、Brother driverless(IPP) 側で既定サイズへフォールバックする
- その結果、`PageSize=62x100mm` や `29mm` 系の既定値が優先され、見た目が縮小する

対応:

- Hub からの指定を `Custom.WxHmm` 形式に統一（例: `Custom.62x43mm`）
- `media` と `PageSize` の両方を同じ値で渡す
- `fit-to-page` を併用して、生成画像を指定ページ内に正規化する

確認ポイント:

- `/var/spool/cups/c000xx` を `strings` で確認し、`media` と `PageSize` が `Custom.62x...mm` になっていること
- `/var/spool/cups/d000xx-001` の実画像が `732px` 幅（62mm@300dpi相当）であること

## 2. sleep 時にキューが pending になる

観測:

- プリンタ sleep 中にジョブ投入すると pending になりやすい
- プリンタ画面が明るく復帰してから印刷すると成功率が高い

運用:

- 現場では「プリンタ画面が明るいことを確認してから印刷」を暫定運用にする
- 恒久対応は UI 側（raku-sika-lite）で、印刷前に復帰確認を促す導線を追加する
