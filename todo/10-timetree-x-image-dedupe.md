# TODO-10: TimeTree 登録済み公演へ X を追加した際の画像重複とソース優先順位の解消

- **ステータス**: 完了（セッション単位視覚重複排除・`/patch` 適用・X 優先保持実装済み）
- **目的**: すでに TimeTree 由来の画像が入っている公演に対して X 投稿を追加・統合した際、同一内容のフライヤーやタイムテーブル画像が重複残存する問題を根絶する。

## 現状と課題

1. **パッチ・再整合・編集経路の一部で視覚重複排除 (`VisualDedupeScheduleBytes`) が呼ばれていない**:
   - `internal/monitor/server.go` の `/api/intake/promote`（431行目）および `timetreesync` / `xsync` では `VisualDedupeScheduleBytes` を呼び出しているが、以下の更新経路には組み込まれていない。
     - `/api/intake/patch`（既存セッションへの追記パッチ、`internal/monitor/server.go#L536`）
     - `/api/sessions`（手動セッション作成・更新）
     - `internal/reconcile/runner.go`（名寄せバッチ本体での実行確認・視覚重複マージ）
2. **TimeTree CDN と X CDN 間の変換差・ダウンロード失敗（fail-open）**:
   - `internal/schedule/visual_dedupe.go` は dHash ハミング距離 `<= 8`・16x16 輝度 MAE `<= 7.0`・縦横比差 `<= 0.12` で判定し、画像取得やデコードに失敗した場合は安全側（fail-open）として画像を削除しない。
   - TimeTree 添付画像（`attachments.timetreeapp.com`）と X 画像（`pbs.twimg.com`）の間で、リサイズ以上の縦横比トリミング・圧縮劣化が起きて閾値を超えているか、あるいは `server.go#L92` の 6 秒タイムアウト・ヘッダー制約等でダウンロード失敗して無言スキップされているケースがあるか、実データで切り分ける必要がある。
3. **残すべき画像の優先順位（TimeTree 先行時の「後から来た高画質 X」の扱い）**:
   - 現在の `VisualDedupeSession` は「先に出現した画像を残し、後続の重複画像を削除する」仕様になっている。
   - そのため TimeTree（再エンコード済み画像）が先に入っている公演へ X（元の `pbs.twimg.com/...?name=large`）を追加して重複判定が効いた場合でも、低画質な TimeTree 側が残り高画質な X 側が捨てられる挙動になる。

## 検討・実装項目

1. **実重複ケースの再現計測**:
   - 本番 `schedule.json` または FB 対象公演の TimeTree 画像 URL と X 画像 URL をペアで `ComputeImageSignature` / `IsVisuallyDuplicate` にかけ、ハミング距離・MAE・縦横比差・HTTP 取得成否を実測する。
   - 実測結果に基づき、誤削除（別タイムテーブル誤統合）を起こさない安全範囲で閾値調整またはログ可視化（fail-open 時の警告ログ・スキップ理由出力）を行う。
2. **すべての画像更新経路への `VisualDedupe` 適用**:
   - `/api/intake/patch`・`/api/sessions`・`reconcile.Run` 等でも `ImageFingerprinter` を通して視覚重複排除を実行する（あるいは共通のスケジュール保存前フックに統合する）。
   - ※ サービング Pod（`monitor` 64Mi メモリ制限）内で大量画像デコードすると OOM リスクがあるため、単一セッション単位の `VisualDedupeSession` のみに限定して全セッション走査を避ける等のメモリ制御も確認する。
3. **同一画像と判定された際の優先順位見直し**:
   - 重複ペアの片方が TimeTree (`attachments.timetreeapp.com`) で他方が X (`pbs.twimg.com`) の場合、より高解像・原本に近い X 側を優先して残し、関連 `image_url` も整合させるポリシーを検討・テストする。

## 検証条件

- [x] TimeTree 画像と X 画像の実重複ペアに対し、重複判定が機能し画像枚数が意図通り削減される単体テストを追加する。
- [x] `/api/intake/promote` だけでなく `/api/intake/patch` および `reconcile` 経由でも同様に重複排除が動作する。
- [x] TimeTree 先行時に X と視覚重複した場合の置換・保持ルールがテストで固定されている。
- [x] 異なる画像（フライヤー vs タイムテーブル等）が誤って削除されない回帰テストを通過する。
