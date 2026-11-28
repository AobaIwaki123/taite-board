# taite-board 改善タスク一覧 (todo/)

本ディレクトリは、別セッションや並列 Worktree から過去の会話コンテキストなしで独立して調査・実装できるよう、現状の課題・再現箇所・設計方針・検証条件を自己完結した形でまとめたタスク仕様書を管理します。

## タスク一覧と並列着手の可否

| ID | ドキュメント | 概要 | 他セッションでの並列着手 | 主な変更対象 |
| --- | --- | --- | --- | --- |
| **01** | [`01-calendar-view-selection-and-today.md`](01-calendar-view-selection-and-today.md) | カレンダー選択時の白縁取り・拡大廃止と「今日」の日付表示追加 | **可能（完全独立）** | `apps/web/src/components/CalendarView.tsx` |
| **02** | [`02-deterministic-upsert-and-media-link-priority.md`](02-deterministic-upsert-and-media-link-priority.md) | 新規/既存更新トグル廃止に向けた決定論的 Upsert 統合、画像 ID 重複排除、代表リンク優先順位付け | **可能（Go バックエンド中心で独立可）** | `internal/schedule/`, `internal/resolve/`, `internal/monitor/server.go`, `internal/reconcile/` |
| **03** | [`03-arch-schema-and-tenant-verification.md`](03-arch-schema-and-tenant-verification.md) | システム構成・テナント分離レベルの JSON スキーマ化、設定ドリフト検証スクリプト整備、既知ズレ修正 | **可能（完全独立）** | `schema/`, `scripts/`, `docs/architecture.md`, `apps/web/src/config/group.ts`, `k8s/manifests/` |
| **04** | [`04-mobile-admin-ui-redesign.md`](04-mobile-admin-ui-redesign.md) | 管理画面（`/admin`）の `apps/web/` デザインシステム統合、情報削減、モバイル縦積み最適化 | **完了（UI/UX刷新・`/admin`統合済み）** | `apps/web/`, `deploy/nginx.conf` |
| **05** | [`05-timetree-automatic-intake.md`](05-timetree-automatic-intake.md) | lumitree による公開カレンダーの定期取得と、確定済み情報を保護した自動同期 | **実装済み（本番稼働中）** | `internal/source/timetree/`, `internal/timetreesync/`, 定期実行基盤 |
| **06** | [`06-ticket-price-extraction.md`](06-ticket-price-extraction.md) | チケット料金（`ticket.prices`）の抽出・保持パイプライン整備と家計簿連携改善 | **可能（バックエンド中心・07と連携）** | `internal/resolve/`, `internal/parser/`, `internal/schedule/`, `internal/monitor/`, `AdminView.tsx` |
| **07** | [`07-expense-memo-drink-input-and-privacy-notice.md`](07-expense-memo-drink-input-and-privacy-notice.md) | 家計簿ドリンク代の任意金額入力対応とプライバシー非共有の注意書き明記 | **完了（任意入力＋注意書き反映済み）** | `apps/web/src/utils/expense.ts`, `ExpenseMemoCard.tsx`, `SyncAccountModal.tsx` |
| **08** | [`08-google-calendar-export.md`](08-google-calendar-export.md) | 公演詳細からの Google カレンダー登録導線（時間・会場・URL 出力）追加 | **完了（CockpitView 導線追加済み）** | `apps/web/src/utils/calendar.ts`, `CockpitView.tsx` |
| **09** | [`09-special-notes-visibility.md`](09-special-notes-visibility.md) | 特記事項（`special_notes`）の表示欠落解消と視認性強化 | **完了（Cockpit/List 表示強化済み）** | `apps/web/src/components/CockpitView.tsx`, `ListView.tsx` |
| **10** | [`10-timetree-x-image-dedupe.md`](10-timetree-x-image-dedupe.md) | TimeTree 既存予定への X 追加時における画像重複とソース優先順位の解消 | **完了（単一セッション走査化・`/patch` 適用・X 優先保持）** | `internal/schedule/visual_dedupe.go`, `internal/monitor/server.go`, `internal/reconcile/` |

## 共通の開発ルール・前提

各タスクに着手する前に、必ずリポジトリ直下の [`AGENTS.md`](../AGENTS.md) を確認してください。要点は以下のとおりです。

1. **PR-Only Workflow と機械化出荷**:
   - `main` ブランチへの直接プッシュは行わず、ブランチを切って [`./scripts/ship.sh`](../scripts/ship.sh) で検証・PR作成・Squash マージ・`main` 同期を行います。
   - 事前検証は `./scripts/verify.sh`（または `make verify`）と関連 Go テスト（`go test ./internal/...`）を実行します。
2. **表記・ドキュメント規約**:
   - コミットメッセージ、PR 本文、ドキュメント、コードコメントに絵文字を使用しないでください。
   - Markdown の太字（`**`）が全角鉤括弧と密着してレンダリング崩れを起こさないよう、鉤括弧の内側を囲む（例: `「**重要**」`）か半角スペースを空けてください。
3. **タスク完了時の後処理**:
   - 該当タスクを完了・マージした際は、同じ PR または後続コミットで該当 `todo/*.md` のステータス更新（または削除）と必要に応じた [`discussion.md`](../discussion.md) 更新を行ってください。
