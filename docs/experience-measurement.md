# 印刷段階の診断時間

Lite/Opsの `feat/experience-baseline-20261030` と協調する計測。機器待機時間・印刷方式・再試行方針は変更しない。
POST /printer/print の省略可能なexperienceTraceIdを使用。UUID以外はログに残さずHubで生成する。
同一リクエストのrequest、lock-acquired、status-ready、image-ready、spool-submitted、queue-verified、printer-confirmed、returnをINFOのJSONとして記録する。
durationMsはrequest開始からの累積、stageMsは直前段階からの時間。Goの単調時計を使用。
画像/状態確認のreturnedは成功とは限らず、最終return outcomeでfailed/pending等を判断する。
printer-confirmedは既存rakuql完了報告を読めた場合だけ。物理開始・排出・紙面確認は観察が必要。
return=backend-returnedを物理完了と読み替えない。時刻が似ている別ジョブとは結合しない。
Ops workflow-observation.py は許可されたenum・UUID・数値だけを端末から収集する。
本番未配備。既存のタグによるrelease手順で配布する。Go化自体は既に済んでおり、言語変更を結論にしない。
