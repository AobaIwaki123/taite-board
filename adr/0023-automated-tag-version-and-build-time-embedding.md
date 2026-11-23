# ADR-0023: Git タグ起点の自動バージョニングとコンテナビルド時バージョン焼き込み

## ステータス
Accepted

## 決定日
2026-11-23

## 対象コンポーネント
- `scripts/next-version.sh`
- `internal/version/`
- `apps/web/vite.config.ts`
- `Dockerfile`
- `deploy/*.Dockerfile`
- `crawler/Dockerfile`
- `.github/workflows/deploy.yml`
- `k8s/argocd/application.yaml`

## コンテキストと課題 (Context & Problem Statement)
これまで `taite-board` では ADR-0015 に基づき、手動スクリプト（`./scripts/release.sh`）で `VERSION` ファイルと `apps/web/package.json` を更新し、Git タグを付与していた。この方式には以下の課題があった。

1. **本番リリースごとのバージョン手動更新漏れ**:
   - PR を `main` へマージするたびに CI（`deploy.yml`）は `sha-<7桁>` タグのコンテナイメージを自動ビルド・デプロイしていたが、`VERSION` や Git タグは手動実行しない限り更新されなかった。
2. **Go コンテナにおけるバージョン実行時解決の失敗**:
   - `internal/version/version.go` が実行時のカレントディレクトリから上位ディレクトリに向かって `VERSION` ファイルを探索する設計だったため、`VERSION` ファイルを含まない本番実行ステージ（`deploy/monitor.Dockerfile`, `deploy/parser.Dockerfile`）では常にフォールバック値 `1.0.0` が返却されていた。
3. **CI からの自動コミットによる Git 履歴汚染の回避要件**:
   - `AGENTS.md` の Anti-Git Churn 原則により、CI パイプラインが `main` ブランチへバージョン更新用の自動コミットを書き戻す運用は避ける必要がある。

## 決定要因・評価軸 (Decision Drivers)
- **完全自動化**: `main` への PR マージのみで次バージョンの決定・イメージ焼き込み・Git タグ付与・GitHub Release 発行・クラスタ反映まで完結すること。
- **Anti-Git Churn 原則の遵守**: バージョン更新のために `main` ブランチへ自動コミットを追加しないこと。
- **全成果物のバージョン統一**: Web UI フッター、`monitor` / `parser` バイナリ、OCI イメージラベル、GHCR イメージタグ、Git タグ、GitHub Release がすべて同一の SemVer を指すこと。

## 検討した選択肢 (Considered Options)
### 選択肢 1: Git タグを正本とし、CI で SemVer 自動算出＋ビルド引数 (`--build-arg` / `-ldflags`) 焼き込み
- **概要**: `main` Push 時に直近の `v*` タグからのコミット規約（Conventional Commits: `feat!:`/`BREAKING CHANGE` -> major、`feat:` -> minor、その他 -> patch）を解析して次バージョンを算出し、全 Dockerfile へ `--build-arg APP_VERSION=...` として渡してバイナリ・JS バンドル・OCI ラベルへ焼き込み、イメージ Push 成功後に CI が Git タグと GitHub Release を作成する。
- **メリット (Pros)**:
  - `main` に自動バンプコミットが一切増えない。
  - Go バイナリは `-ldflags` で直接文字列を保持するため、実行時に `VERSION` ファイルを探索する必要がない。
  - Argo CD Image Updater の `semver` 戦略と組み合わせることで、クラスタ上の Pod イメージタグも `vX.Y.Z` で統一できる。
- **デメリット (Cons)**:
  - リポジトリ内の静的 `VERSION` ファイルは CI で自動更新されないため、ローカル開発時のフォールバック値としての位置づけになる。

### 選択肢 2: Release Please による Release PR 自動生成
- **概要**: `main` への変更蓄積に応じて CI が Release PR を作成し、人間がマージしたタイミングでリリース・タグ付けを行う。
- **メリット (Pros)**: `VERSION` ファイルと Git タグを一致させられる。
- **デメリット (Cons)**: PR マージ後に追加で Release PR をマージする手動操作が必要になり、完全ゼロタッチにならない。

## 決定事項 (Decision Outcome)
**選択肢 1（Git タグ起点の自動算出 ＋ ビルド時焼き込み）** を採用する。

1. **次バージョン算出スクリプト (`scripts/next-version.sh`)**:
   - `HEAD` に既に `vX.Y.Z` タグが付いている場合はそのバージョンを再利用する。
   - 未タグの場合は最新 `v*` タグからのコミットメッセージを検査し、Conventional Commits 規約に従って次の SemVer を算出する。
2. **ビルド時焼き込み (`internal/version`, `vite.config.ts`, 全 `Dockerfile`)**:
   - `internal/version/version.go` に `BuildVersion` 変数を設け、Go ビルド時に `-ldflags "-X github.com/AobaIwaki123/taite-board/internal/version.BuildVersion=..."` で注入する（未注入時のみ `VERSION` ファイルへフォールバック）。
   - `apps/web/vite.config.ts` は `process.env.APP_VERSION` を最優先で `__APP_VERSION__` に焼き込む。
   - 全 `Dockerfile` に `ARG APP_VERSION` / `ARG GIT_SHA` および OCI `org.opencontainers.image.version` / `revision` ラベルを追加する。
3. **CI / Argo CD 連携 (`.github/workflows/deploy.yml`, `k8s/argocd/application.yaml`)**:
   - `deploy.yml` が `fetch-depth: 0` でタグ履歴を取得し、算出した `APP_VERSION` で全イメージを `:sha-<7桁>` と `:vX.Y.Z` の両タグで GHCR へ Push した後、`GITHUB_TOKEN` により Git タグと GitHub Release を発行する（`GITHUB_TOKEN` によるタグ Push は二重ワークフローを起動しない）。
   - Argo CD Image Updater は `semver` 戦略（`regexp:^v[0-9]+\.[0-9]+\.[0-9]+$`）で最新リリースタグへ自動追従する。

## 影響とトレードオフ (Consequences)
### 正の影響 (Positive)
- PR マージ以外の操作なしで、Web 表示・Go サービス・コンテナタグ・GitHub Release がすべて同一の `vX.Y.Z` に自動更新される。
- `monitor` コンテナで `1.0.0` にフォールバックしていた不整合が解消される。

### 負の影響とリスク (Negative & Mitigations)
- ローカルの `VERSION` ファイルは CI により書き換わらないため、ローカル作業時に最新リリース番号を確認したい場合は `git fetch --tags && git describe --tags --abbrev=0` を参照する（手動固定リリースを行いたい場合は従来通り `scripts/release.sh` または `workflow_dispatch` の `version` 指定を利用可能）。
