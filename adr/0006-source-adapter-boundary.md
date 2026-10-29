# ADR-0006: 外部取得アダプター（Source Adapter）の責務分離

## ステータス
Accepted

## 決定日
2026-10-30

## 対象コンポーネント
- `internal/source/` (`internal/source/x/`, `internal/source/ticket/`)
- `internal/fetch/`

## 背景と課題
外部情報源（特にX / Twitter）は、APIの有料化・仕様変更、DOM構造の変更、CDNの画像URL仕様変更、レート制限（429 Too Many Requests）の強化など、外部要因による破壊的変更が頻繁に発生する。
もし外部サービスとの通信処理やスクレイピング処理が、イベント抽出処理（日時のパース、会場名の正規化など）と密結合していると、Xの仕様変更が起きるたびにプロジェクト全体のコードを広範に修正しなければならなくなる。

## 決定事項
1. **Source Adapter パターンの適用**:
   - 外部サービスとのI/O処理はすべて `internal/source/<provider>/` パッケージ内に完全に隔離する。
   - 例: `internal/source/x/` はXからの投稿取得、リトライ、タイムアウト、レート制限ハンドリングのみを担当し、イベント日時の解釈や名寄せは一切行わない。
2. **抽象インターフェースの定義**:
   ```go
   type PostFetcher interface {
       FetchPost(ctx context.Context, sourceURL string) (*raw.Post, error)
       FetchBatch(ctx context.Context, urls []string) ([]*raw.Post, *raw.RunReport, error)
   }
   ```
3. **FxTwitter / Syndication / GraphQL 等のフォールバック内部化**:
   - 取得手段（公式API、FxTwitter、Syndication API等）の切り替えやフォールバック処理は `internal/source/x` の内部実装とし、呼び出し側（`internal/fetch`）からは単一の `raw.Post` 構造体としてのみ扱えるようにする。
4. **外部通信を排除したテスト（Fixture Testing）の強制**:
   - `extract` や `resolve` などの後続コンポーネントのテストでは、外部ネットワークアクセスを一切行わず、`testdata/fixtures/` に保存された生JSON（Raw Snapshot）のみを入力として利用する。

## 採用理由と設計根拠
- **変更の局所化**: Xの仕様が変わった場合、修正・検証が必要なコードは `internal/source/x/` のみとなり、名寄せ辞書やWeb UI、データスキーマは1行も触る必要がない。
- **CIの高速化と安定性**: 外部通信に依存しない決定論的なユニットテストが実行できるため、GitHub Actions等のCIが外部APIのフレイキー（Flaky）な障害で落ちることがない。

## 検討した代替案と却下理由
- **CLIのメイン処理（`main.go`）に直接HTTPリクエストを記述する**:
  - 却下理由: MVP初期は手軽だが、URL単体取得とファイル一括取得、標準入力対応などの要件が増えた段階で重複コードとエラーハンドリングの漏れが発生する。

## 影響範囲とリスク
- **アダプター追加のオーバーヘッド**: 新しい外部情報源（例: チケット販売サイトの専用クローラー）を追加する際、専用のアダプター構造体を実装する初期コストが発生する。
