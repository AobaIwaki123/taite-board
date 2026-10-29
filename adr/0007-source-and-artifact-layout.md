# ADR-0007: ソースコード・ビルド生成物のディレクトリ配置再編

## ステータス
Accepted

## 決定日
2026-10-30

## 対象コンポーネント
- リポジトリトップレベル構成
- `apps/web/`
- `data/`
- `cmd/taite/`
- `internal/`
- `docs/`

## 背景と課題
MVP段階では迅速なプロトタイピングとGitHub Pagesでの即時プレビューを最優先したため、以下の技術的負債・重複が存在していた。
1. `docs/index.html` と `web/index.html` に同一のHTMLファイルが二重管理されており、UI改修のたびに手動コピーが必要であった。
2. スケジュールデータが `docs/mock_schedule.json`、`data/mock_schedule.json`、`web/mock_schedule.json` の3箇所に散らばっていた。
3. Go CLI（`cmd/taite-fetch`）のコードと、Node.js製の生成・検証スクリプト（`scripts/`）が未整理のまま混在していた。
Kubernetesデプロイ環境が整備された現在、ソースコードと成果物（Artifact）の責務を綺麗に分離する必要がある。

## 決定事項
1. **クライアントソースの集約（`apps/web/`）**:
   - Webクライアントの正本ソースを `apps/web/` に一本化する。
   - `web/index.html` と `docs/index.html` の重複を廃止する。
2. **静的生成物とデータパイプラインの分離（`data/`）**:
   - データのライフサイクルに応じてディレクトリを明確に分ける。
   ```text
   data/
   ├── input/      # クローラーへの入力URLリスト (urls.txt)
   ├── raw/        # 外部取得の不変生スナップショット (runs/<run-id>/)
   ├── candidate/  # 自動抽出された候補データ
   ├── curated/    # 人間が確定した辞書・上書き設定 (entities.json, overrides.json)
   └── generated/  # クライアントへ配信する確定公開JSON (schedule.json)
   ```
3. **Go CLIの集約（`cmd/taite/`）**:
   - `cmd/taite-fetch` を廃止し、サブコマンド形式の統合CLI `cmd/taite/main.go` を正とする。
4. **内部ロジックの隠蔽（`internal/`）**:
   - `internal/source/` (外部通信)
   - `internal/fetch/` (生取得実行)
   - `internal/extract/` (本文・画像候補抽出)
   - `internal/resolve/` (名寄せ辞書照合)
   - `internal/publish/` (マージと公開JSON生成)
   - `internal/validate/` (スキーマ・画像・整合性検証)
5. **配布・ホスティング先の統合**:
   - Kubernetes（Nginxコンテナ）およびGitHub Pagesへの配布物は、ビルド時に `apps/web/` のソースと `data/generated/schedule.json` をコンテナイメージまたは配信ディレクトリへ同期・コピーするビルドステップ（またはMakefile / scripts）で生成する。

## 採用理由と設計根拠
- **二重管理によるヒューマンエラーの根絶**: 過去に発生していた「`docs/` だけ更新して `web/` の同期を忘れる」問題が原理的に発生しなくなる。
- **マルチクライアント拡張の容易性**: `apps/web/` の隣に将来 `apps/ios/` や `apps/android/` を自然に配置できる。
- **Go言語の標準パッケージ構成への適合**: `cmd/` と `internal/` を分けるGoの標準的レイアウトを採用し、外部からの不正インポートを防止する。

## 検討した代替案と却下理由
- **現在の `docs/` をそのままソースディレクトリとし続ける**:
  - 却下理由: `docs/` にソースコード・テストデータ・スクリプト・公開生成物が混在し、将来のビルドステップやネイティブアプリ追加時に破綻する。

## 影響範囲とリスク
- **既存パスの変更**: GitHub ActionsのデプロイワークフローやDockerビルド時のコピー元パスを `apps/web/` へ移行する必要がある。
