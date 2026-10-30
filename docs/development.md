# taite-board 開発者ガイド (development.md)

本ドキュメントは、`taite-board` の開発環境セットアップ、ローカル実行、テスト、検証、データ更新の手順をまとめた開発者向けガイドです。

---

## 1. 開発環境の要件

- **macOS / Linux** (zsh または bash)
- **Go**: `1.27` 以上
- **Node.js**: `26.x` (LTS)
- **Docker**: 最新版 (マルチステージコンテナビルド検証用)

---

## 2. ディレクトリの役割と編集規約

| ディレクトリ | 役割 | 編集方針 |
|---|---|---|
| `apps/web/` | Webフロントエンド正本 (Preact + TypeScript) | **UI・ロジックの改修は必ずここを編集する**。`docs/` にビルド成果物をコミットしない。 |
| `cmd/taite/` | 統合Go CLIのエントリポイント | サブコマンドの受付と引数パースのみを記述。 |
| `internal/` | コアビジネスロジック・アダプター | 外部I/O、生取得、名寄せ、バリデータをパッケージごとに分離。 |
| `data/` | 5層データパイプライン | 詳細は `data/README.md` を参照。 |
| `data/curated/` | 人間確定の名寄せ辞書・上書き設定 | 新規会場や特殊タイテのオーバーライド時に更新。 |
| `data/generated/` | クライアント公開用の確定JSON | 手動編集せず、生成・ビルドパイプラインを通じて更新。 |
| `schema/` | JSON Schema 定義 | スキーマ拡張時のみ更新（破壊的変更はメジャーバージョン更新）。 |
| `adr/` | アーキテクチャ意思決定記録 | 新たな技術選定や方針変更時に `000X-xxx.md` を追加。 |
| `docs/` | 開発者・アーキテクチャドキュメント | ドキュメント Markdown のみを配置。 |

---

## 3. 日常の開発サイクル

### ステップ1: CLI のインストールと検証
```bash
# taite バイナリを $GOBIN にインストール
make install

# スケジュールデータの検証
make validate
```

### ステップ2: フロントエンド（Preact + TSX）の開発
フロントエンド設計規約の詳細は [`docs/frontend-guidelines.md`](frontend-guidelines.md) を参照してください。

```bash
# 開発サーバー起動 (HMR)
make web-dev

# TypeScript 型検査
npm --prefix apps/web run typecheck

# 最適化バンドルビルド
make web-build
```

### ステップ3: Dockerマルチステージビルドの検証
デプロイ前に、コンテナビルド（Node.js での TypeScript ビルドから Nginx Alpine への配置）が正常に完結するかを検証します。

```bash
# ビルド
docker build -t taite-board-local .

# テスト起動
docker run --rm -d -p 8081:8080 --name taite-test taite-board-local

# 疎通確認
curl -I http://localhost:8081/healthz     # HTTP 200 "ok"
curl -I http://localhost:8081/index.html   # HTTP 200 (HTML)
curl -I http://localhost:8081/schedule.json # HTTP 200 (application/json)

# コンテナ停止
docker stop taite-test
```

---

## 4. GitコミットとPR運用ルール

本リポジトリは、保護された `main` ブランチへの直接Pushを禁止し、**PR-Only Workflow** を適用しています。

1. **ブランチ作成**: `feat/<topic>`, `fix/<topic>`, `refactor/<topic>`, `docs/<topic>`
2. **事前検証**: `make test`, `make validate`, `make web-build` がパスすることを確認。
3. **コミット**: Conventional Commits に準拠。絵文字は使用しない。
4. **PR起票とマージ**:
   ```bash
   git push -u origin <branch-name>
   gh pr create --title "..." --body "..."
   gh pr merge <pr-number> --squash --delete-branch
   ```

---

## 5. 本番環境リンク

- **本番環境 (Kubernetes GitOps)**: `https://ribonclock.iorin.net/`
