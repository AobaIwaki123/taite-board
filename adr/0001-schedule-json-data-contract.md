# ADR-0001: Web・ネイティブ共通のschedule.jsonデータ契約とスキーマ管理

## ステータス
Accepted

## 決定日
2026-10-30

## 対象コンポーネント
- `schema/schedule.schema.json`
- `data/generated/schedule.json`
- `apps/web/`
- 将来のネイティブアプリ (`apps/ios/`, `apps/android/`)

## 背景と課題
現在、MVPのWebクライアント（`docs/index.html` および `web/index.html`）は `mock_schedule.json` を直接読み込んで動作している。
しかし、今後Go CLIによる自動生成パイプラインの構築や、ネイティブアプリ（iOS / Android）への展開を進めるにあたり、以下の課題がある。
1. クライアントが解釈するデータ構造の仕様が暗黙的であり、スクリプトの改修やクローラーの更新によってUIが破壊されるリスクがある。
2. 外部サービス（Xやチケット販売サイト）の生データ変更が直接クライアントに波及してしまう。
3. 複数プラットフォームで表示揺れや時間パースの不整合が発生する。

## 決定事項
1. **公開データ契約の単一化**:
   - Webおよびネイティブアプリが参照する公開データファイルを `schedule.json` に統一する。
   - すべてのクライアントは生データ（Raw Snapshot）を直接解釈せず、この確定された公開データのみを読む。
2. **スキーマバージョン管理の導入**:
   - ルートオブジェクトに `schema_version`（整数値、初期値: `1`）を必須とし、破壊的変更時はメジャーバージョンをインクリメントする。
3. **JSON Schemaの定義**:
   - `schema/schedule.schema.json`（Draft-07準拠）を策定し、CIおよびビルドパイプライン（`taite validate`）で厳格に検証する。
4. **契約モデルの必須属性**:
   - `generated_at`: 生成時刻（ISO 8601、タイムゾーン付き）。
   - `timezone`: 対象タイムゾーン（原則 `Asia/Tokyo`）。
   - `target_group`: 対象グループの `canonical_id` および表示名。
   - `sessions`: 公演リスト。各セッションは一意な `id`（例: `20261031-1`）、`date`、`title`、`live`、`benefit`、`venue`、`images`、`ticket`、`source_refs`（出典リンク配列）を保持する。

## 採用理由と設計根拠
- **疎結合性の確保**: 外部サービス仕様（XのDOMやAPIレスポンス形式）の変化をデータ抽出層に閉じ込め、クライアントへの影響をゼロにする。
- **型安全性と回帰テストの容易性**: JSON Schemaが存在することで、Goの構造体（`internal/schema`）やTypeScript / Swiftのモデルコードを安全に整合させることができる。
- **実証済みUIとの完全互換**: 現在のWeb UIで検証済みの複数画像プレビュー（`images: [{ type, label, url }]`）やチケットステータス（`ticket: { platform, url }`）を包含する。

## 検討した代替案と却下理由
- **GraphQL / 動的REST APIの採用**:
  - 却下理由: 現状のユーザー規模および更新頻度（1日数十回未満）に対して過剰なインフラ運用コスト（APIサーバー稼働、DBコネクション管理、障害点増加）を招く。静的ファイル配信で十分な応答性と可用性が得られる。
- **クライアント側での動的Xスクレイピング**:
  - 却下理由: CORS制約、APIトークン露出、外部仕様変更への追従不可、クライアント側通信量の増大により採用不可。

## 影響範囲とリスク
- **スキーマ変更の制約**: `schema_version: 1` をリリースした後は、後方互換性のないフィールド削除や型変更に慎重な移行計画が必要となる。
- **移行手順**: 既存の `mock_schedule.json` を `schema/schedule.schema.json` に照合してバリデーションを通し、`data/generated/schedule.json` へ昇格させる。
