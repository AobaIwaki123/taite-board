# TODO-15: 特記事項 (`special_notes`) の実データ検査とパーサー修正

- **ステータス**: 未着手（セルフレビュー起票）
- **優先度**: 中（情報欠落・誤分類の防止）
- **目的**: 公開中および今後取り込む公演データの `special_notes`（特記事項）に欠落・ノイズ混入・誤抽出がないかを実データベースで検査し、問題が見つかった場合は決定論的リゾルバ・AI パーサー・TimeTree 同期パーサーを修正する。

## 検討・作業項目

1. **実データおよび抽出経路の検査**:
   - 本番 GCS (`data/groups/*/schedule.json`)・X 告知・TimeTree 説明文を対象に、特記事項に載るべき情報（メンバー欠席・出演形態、早割キャッシュバック、撮影可否以外の諸注意、動員重要、衣装指定など）が正しく `special_notes` に反映されているかを確認する。
   - 反対に、他フィールド（`admission_bonus`・`ticket.prices`・`benefit.booth`・`title`）と重複した文言や不要な装飾・テンプレート定型文が `special_notes` に紛れ込んでいないかを検査する。
2. **パーサー・プロンプト・マージ処理の修正（課題検出時）**:
   - `internal/resolve/`（決定論的抽出）・`internal/parser/gemini.go`（Gemini JSON スキーマ/プロンプト）・`internal/timetreesync/extract.go`（TimeTree 抽出）のうち、欠落や誤抽出を起こしている経路を修正する。
   - `internal/schedule/add.go`・`update.go`・`reconcile.go` における `special_notes` の区切り文字（` / ` 連結）・重複排除・既存手入力保護が `apps/web/src/utils/notes.ts`（`parseSpecialNotes`）の表示仕様と噛み合うよう調整する。
3. **既存データの再検証・補完**:
   - パーサー修正後、該当セッションの再取り込みまたは正規化検証を行い、表示崩れや重複表示がないことを確認する。

## 検証条件

- [ ] 実投稿および TimeTree 実予定を用いたテストケースが `internal/resolve` / `internal/timetreesync` / `internal/parser` に追加され、すべて通過すること。
- [ ] `go test -count=1 ./...` および `./scripts/verify.sh` が通過すること。
- [ ] `CockpitView`・`ListView` 上で特記事項が欠落・過剰重複なく表示されること。
