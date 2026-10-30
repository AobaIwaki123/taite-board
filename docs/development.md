# taite-board 開発者ガイド (development.md)

本ドキュメントは、`taite-board` の開発環境セットアップ、ローカル実行、テスト、検証、データ更新の手順をまとめた開発者向けガイドです。

---

## 1. 開発環境の要件

- **macOS / Linux** (zsh または bash)
- **Go**: `1.27` 以上
- **Node.js**: `20.x` 以上 (npm または pnpm)
- **Docker**: 最新版 (マルチステージコンテナビルド検証用)
- **Python 3**: ローカルHTTPプレビュー用 (`python3 -m http.server`)

---

## 2. ディレクトリの役割と編集規約

| ディレクトリ | 役割 | 編集方針 |
|---|---|---|
| `apps/web/` | Webフロントエンド正本 (TypeScript + Vite) | **UI・ロジックの改修は必ずここを編集する**。`docs/` を直接編集しない。 |
| `cmd/taite/` | 統合Go CLIのエントリポイント | サブコマンドの受付と引数パースのみを記述。 |
| `internal/` | コアビジネスロジック・アダプター | 外部I/O、生取得、名寄せ、バリデータをパッケージごとに分離。 |
| `data/` | 5層データパイプライン | 詳細は `data/README.md` を参照。 |
| `data/curated/` | 人間確定の名寄せ辞書・上書き設定 | 新規会場や特殊タイテのオーバーライド時に更新。 |
| `data/generated/` | クライアント公開用の確定JSON | 手動編集せず、生成・ビルドパイプラインを通じて更新。 |
| `schema/` | JSON Schema 定義 | スキーマ拡張時のみ更新（破壊的変更はメジャーバージョン更新）。 |
| `adr/` | アーキテクチャ意思決定記録 | 新たな技術選定や方針変更時に `000X-xxx.md` を追加。 |

---

## 3. 日常の開発サイクル

### ステップ1: フロントエンド（TypeScript + Vite）の開発

フロントエンドは TypeScript Strict Mode とモジュール分割（ADR-0010）を採用しています。

- `apps/web/src/types/schedule.ts`: `schema/schedule.schema.json` 準拠の厳格な型定義
- `apps/web/src/state.ts`: 状態管理、オフラインキャッシュ、更新鮮度判定
- `apps/web/src/components/cockpit.ts`: 直近ライブコックピット表示、画像切替、チケットボタン
- `apps/web/src/components/scheduleList.ts`: 全日程リスト、アコーディオン
- `apps/web/src/components/calendar.ts`: 月間カレンダーグリッド、月送り、日付タップ詳細
- `apps/web/src/components/promotion.ts`: 生誕クラファン等の告知枠
- `apps/web/src/components/carousel.ts`: 3画面スワイプとナビゲーションバーの1:1連動
- `apps/web/src/components/lightbox.ts`: 画像拡大モーダル
- `apps/web/src/main.ts`: エントリーポイント、初期化

#### 開発コマンド (apps/web/)
```bash
cd apps/web

# 高速開発サーバー起動 (HMR)
npm run dev

# 型検査のみ実行
npm run typecheck

# TypeScript型チェック ＋ 最適化バンドルビルド
npm run build
```

#### 配布成果物の同期
```bash
# プロジェクトルートから実行: apps/web をビルドし docs/ (GitHub Pages用) へ同期
./scripts/build.sh
```

### ステップ2: データの更新と自己検証
新規公演の追加やスケジュール更新を行った際は、必ずスキーマ検証を実行します。

```bash
# Goネイティブスキーマバリデータ
./bin/taite validate --file data/generated/schedule.json
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

1. **ブランチ作成**: `feat/<feature-name>` または `fix/<bug-name>`
2. **変更・ビルド**: `apps/web/` を編集し `./scripts/build.sh` で同期。
3. **コミット**: Conventional Commits（`feat:`, `fix:`, `docs:`, `refactor:`）に準拠。絵文字は使用しない。
4. **PR起票とマージ**:
   ```bash
   git push -u origin <branch-name>
   gh pr create --title "..." --body "..."
   gh pr merge <pr-number> --squash --delete-branch
   ```

---

## 5. 本番環境リンク

- **本番環境 (Kubernetes GitOps)**: `https://ribonclock.iorin.net/`
- **GitHub Pages フォールバック**: `https://aobaiwaki123.github.io/taite-board/`
