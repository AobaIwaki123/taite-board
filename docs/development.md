# taite-board 開発者ガイド (development.md)

本ドキュメントは、`taite-board` の開発環境セットアップ、ローカル実行、テスト、検証、データ更新の手順をまとめた開発者向けガイドです。

---

## 1. 開発環境の要件

- **macOS / Linux** (zsh または bash)
- **Go**: `1.27` 以上
- **Node.js**: `18.x` 以上 (npm または npx)
- **Docker**: 最新版 (コンテナビルド検証用)
- **Python 3**: ローカルHTTPプレビュー用 (`python3 -m http.server`)

---

## 2. ディレクトリの役割と編集規約

| ディレクトリ | 役割 | 編集方針 |
|---|---|---|
| `apps/web/` | Webフロントエンド正本ソース | **HTML、JS、CSSの改修は必ずここを編集する**。`docs/` や `web/` を直接編集しない。 |
| `cmd/taite/` | 統合Go CLIのエントリポイント | サブコマンドの受付と引数パースのみを記述。 |
| `internal/` | コアビジネスロジック・アダプター | 外部I/O、生取得、名寄せ、バリデータをパッケージごとに分離。 |
| `data/curated/` | 人間確定の名寄せ辞書・上書き設定 | 新規会場や特殊タイテのオーバーライド時に更新。 |
| `data/generated/` | クライアント公開用の確定JSON | 手動編集せず、生成・ビルドパイプラインを通じて更新。 |
| `schema/` | JSON Schema 定義 | スキーマ拡張時のみ更新（破壊的変更はメジャーバージョン更新）。 |
| `adr/` | アーキテクチャ意思決定記録 | 新たな技術選定や方針変更時に `000X-xxx.md` を追加。 |

---

## 3. 日常の開発サイクル

### ステップ1: フロントエンド（Web UI）の改修
フロントエンドは保守性と視読性を高めるため、以下の3ファイルに分離されています。

- `apps/web/index.html`: 静的マークアップ（Tailwind CSSクラス適用）
- `apps/web/app.js`: アプリケーションロジック（データ読み込み、カルーセル同期、Lightbox、DOM注入）
- `apps/web/styles.css`: アニメーション定義、視認性フォント調整

編集後は、同期スクリプトを実行して配布ディレクトリへ反映します。
```bash
./scripts/build.sh
```

### ステップ2: ローカルプレビュー
ローカルHTTPサーバーを起動して動作を確認します。
```bash
python3 -m http.server 8089 --directory docs/
```
ブラウザで以下のURLを開きます。
- PC / Mac: `http://localhost:8089/index.html`
- スマホ実機（同一LAN内）: `http://<ローカルIP>:8089/index.html`

### ステップ3: データの更新と自己検証
新規公演の追加やスケジュール更新を行った際は、必ず2重の検証を実行します。

```bash
# 1. Goネイティブスキーマバリデータ
./bin/taite validate data/generated/schedule.json

# 2. 画像URLのHTTP 200疎通およびツイートID照合
node scripts/validate_schedule.js
```

### ステップ4: Dockerコンテナのローカルビルド検証
デプロイ前に、Nginxコンテナが正常にビルドでき、ヘルスチェックおよび静的配信が動作するかを検証します。

```bash
# ビルド
docker build -t taite-board-local .

# テスト起動
docker run --rm -d -p 8081:8080 --name taite-test taite-board-local

# 疎通確認
curl -I http://localhost:8081/healthz     # HTTP 200 "ok"
curl -I http://localhost:8081/schedule.json # HTTP 200 (application/json)

# コンテナ停止
docker stop taite-test
```

---

## 4. GitコミットとPR運用ルール

本リポジトリは、保護された `main` ブランチへの直接Pushを禁止し、**PR-Only Workflow** を適用しています。

1. **ブランチ作成**: `feat/<feature-name>` または `fix/<bug-name>`
2. **変更・ビルド**: `apps/web/` を編集し `./scripts/build.sh` で同期。
3. **コミット**: Conventional Commits（`feat:`, `fix:`, `docs:`, `refactor:`）に準拠。
4. **PR起票とマージ**:
   ```bash
   git push -u origin <branch-name>
   gh pr create --title "..." --body "..."
   gh pr merge <pr-number> --squash --delete-branch
   ```

---

## 5. ガバナンス・セキュリティ留意事項

- **Zero Telemetry**: Cookie、UUID、Google Analytics等の無断トラッキングコードを混入させないこと。
- **No Hallucination**: ツイート本文や告知画像から読み取れない出演時間は、推測で埋めず `--:--`（告知待ち）として処理すること。
- **秘密情報の隔離**: APIキーや認証トークンをクライアント側のコードやJSONに含めないこと。
