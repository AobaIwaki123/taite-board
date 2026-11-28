# TODO-06: チケット料金 (`ticket.prices`) の抽出・登録・家計簿連携の改善

- **ステータス**: 完了（抽出・保存・管理画面編集・家計簿連携を統合済み）
- **目的**: X 告知・AI パース・TimeTree 同期・管理画面の全体を通じてチケット料金（券種別価格）を欠落なく取得・保存し、公演詳細および家計簿クイック選択に反映する。

## 現状と課題

1. **パイプライン側の抽出・受け渡し経路が未実装**:
   - フロントエンド（`apps/web/src/components/CockpitView.tsx`・`apps/web/src/components/ExpenseMemoCard.tsx`）はすでに `session.ticket.prices`（`tier`, `price`, `amount`）の表示および家計簿チケット代ボタン生成に対応している。
   - 一方で、`internal/schedule/add.go` の `AddSessionOptions` には `TicketPlatform` / `TicketURL` しか定義されておらず、新規作成時に `"prices": []interface{}{}` が空のまま初期化される。
   - `internal/resolve/resolver.go`（正規表現抽出）、`internal/parser/gemini.go`（AI スキーマ）、`internal/timetreesync/sync.go`（TimeTree 本文抽出）、`internal/monitor/server.go`（`/api/intake/promote`・`/api/intake/patch`）、`apps/web/src/components/AdminView.tsx` のいずれにも `ticket.prices` を構造化して受け渡す経路が不足している。
2. **価格表記の揺れ**:
   - 実際の告知では「前方 ¥3,000 / 一般 ¥1,500（各+1D）」「Sチケット 5,000円」「無料（別途1D代）」「当日+¥500」「整理番号別」など書式が多様であり、`parseTicketFee` / `isIncrementalFee` だけでは単一文字列以外の複数券種を取り込めない。

## 検討・実装項目

1. **抽出ロジックとデータモデルの統一**:
   - `internal/candidate` および `internal/parser/gemini.go` に `ticket_prices`（`tier`, `price`, `amount`）を追加する。
   - `internal/resolve/resolver.go` で代表的な料金表記（「前売」「前方」「S」「VIP」「一般」「A」「通常」「当日」「無料」等と金額）の決定論的抽出を追加し、曖昧または複数券種で判別が難しい場合のみ AI フォールバック結果を補完に使う。
   - `internal/timetreesync/sync.go` でも TimeTree の description 欄から同様の抽出ロジックを呼び出す。
2. **保存・マージ経路の拡張**:
   - `internal/schedule/add.go` の `AddSessionOptions` に `TicketPrices` を追加し、既存セッションへのマージ時は「既に手動登録済みの `prices` がある場合は上書きせず、空の場合のみ補完する」方針を徹底する。
   - `internal/monitor/server.go`（`/api/intake/promote`・`/api/intake/patch`）と `apps/web/src/components/AdminView.tsx` に `ticket.prices` の編集・送信欄を追加する。
3. **フロントエンド家計簿連携の確認**:
   - `ExpenseMemoCard.tsx` の `parseTicketFee` / `isIncrementalFee` が抽出された `prices`（例: 「無料」「各+¥500」「¥3,500 (+1D)」）を正しく選択候補として扱えるか検証する。

## 検証条件

- [x] 複数券種（前方 / 一般 / 無料 / 当日増分）を含む X 投稿および TimeTree 予定から `ticket.prices` が構造化抽出され、`schedule.schema.json` バリデーションを通過する。
- [x] `AddSession` / `timetreesync` の再実行で既に入力済みの `ticket.prices` が消えたり重複したりしない。
- [x] `CockpitView` のチケット料金表示と `ExpenseMemoCard` のクイック選択ボタンに金額が反映される。

## 実装内容サマリ

- `internal/candidate/candidate.go`, `internal/resolve/ticket.go`, `internal/resolve/resolver.go`: `TicketPrice` (`tier`, `price`, `amount`) 型と `ExtractTicketPrices`（前方/一般/前売/当日/無料・1D注記・特典相当額除外・日付時刻非誤爆対応）を追加し、`Resolver.Resolve` から抽出。
- `internal/parser/gemini.go`: Gemini 抽出 JSON テンプレートと `EventCandidate` 変換へ `ticket_prices` を追加。
- `internal/schedule/add.go`, `internal/schedule/update.go`: `AddSessionOptions`・`UpdateSessionOptions` へ `TicketPrices` を追加し、新規作成時の保存および既存 `ticket.prices` 非空時の保護付き補完を実装。
- `internal/timetreesync/extract.go`, `internal/scrape/runner.go`, `internal/xsync/xsync.go`, `internal/monitor/server.go`: TimeTree 説明文、X スクレイプ/補完、AI パースマージ、`/api/intake/promote`・`/api/intake/patch` 全経路で `TicketPrices` を伝播。
- `apps/web/src/utils/ticket.ts`, `apps/web/src/components/AdminView.tsx`: 候補値・AI パース結果からの自動プリフィルと編集入力欄（`前方 ¥4,000 / 一般 ¥2,500 (+1D)` 形式）および `/intake/promote` 送信を追加。
