# Ribbon Clock (taite-board)

アイドルグループ「リボンクロック」のライブ出演情報およびタイムテーブルを集約・表示するWebアプリケーションです。
外部SNSや告知画像から抽出・名寄せされた確定スケジュールデータを静的JSONとして配信し、モバイルブラウザ向けに表示します。

- **本番URL**: [https://ribonclock.iorin.net/](https://ribonclock.iorin.net/)
- **ローカル開発URL**: `http://localhost:8089/index.html`

---

## 主な機能と技術的特徴

1. **モバイル最適化UI (Preact + TSX + Tailwind CSS)**:
   - 直近公演・全日程一覧・月間カレンダーの3画面を横スワイプカルーセルで切り替え。
   - 画面ごとに独立した縦スクロールコンテナを保持し、画面遷移時のスクロール位置の干渉を防止。
2. **フライヤー・タイムテーブル画像のプレビュー**:
   - 告知ポストに含まれる複数画像（フライヤー画像・タイテ表画像）の切り替え表示および拡大モーダル表示。
3. **オフラインキャッシュとデータ鮮度表示**:
   - 取得済みスケジュールデータを `localStorage` に保持し、オフライン環境でも直前のデータを表示。
   - スケジュール生成日時（`generated_at`）から24時間以上経過している場合、ヘッダーに更新通知バッジを表示。
4. **プライバシー保護 (Zero Telemetry)**:
   - 訪問者追跡、Cookie、外部トラッキングタグ、識別用UUIDの発行を行わない設計。

---

## アーキテクチャ概要

外部サービスからのデータ取得、名寄せ・キュレーション、スキーマ検証済みJSONの生成、Webクライアント配信を単一方向パイプラインとして分離しています。

```text
外部ソース (X, Web等)
        ↓
Go CLI: taite fetch
        ↓
data/raw/runs/<run-id>/ (一次データスナップショット)
        ↓
taite extract (本文抽出・画像解析)
        ↓
data/candidate/ (抽出候補データ)
        ↓
taite publish ← data/curated/ (entities.json, overrides.json による名寄せ・補正)
        ↓
data/generated/schedule.json (schema/schedule.schema.json 準拠)
        ↓
Web クライアント (apps/web: Docker マルチステージビルド / Nginx)
```

詳細な設計方針は [docs/architecture.md](docs/architecture.md) および [adr/](adr/) を参照してください。

---

## ディレクトリ構成

```text
taite-board/
├── apps/
│   └── web/                   # Webクライアント正本 (Preact + TypeScript + Tailwind CSS)
│       ├── src/               # コンポーネントおよびロジック
│       ├── public/            # 静的アセット (画像、ローカルモックJSON)
│       ├── index.html         # SPA エントリーポイント
│       ├── package.json       # 依存パッケージ定義
│       ├── tsconfig.json      # TypeScript 設定 (Strict Mode)
│       └── vite.config.ts     # Vite 設定
├── cmd/
│   └── taite/                 # 統合Go CLI (fetch, validate, build)
│       ├── main.go
│       └── README.md
├── internal/                  # Go CLI 内部パッケージ
│   ├── source/x/              # 外部通信アダプター
│   ├── raw/                   # 生スナップショット定義
│   ├── fetch/                 # 取得オーケストレーター
│   └── validate/              # JSON Schema・ドメイン整合性検証
├── data/                      # 5層データパイプライン (詳細は data/README.md)
│   ├── input/                 # クローラー巡回対象
│   ├── raw/                   # 実行別スナップショット
│   ├── candidate/             # 自動抽出候補
│   ├── curated/               # 名寄せ辞書・手動上書き設定
│   └── generated/             # クライアント配信確定データ (schedule.json)
├── schema/
│   └── schedule.schema.json   # データ契約 JSON Schema (Draft-07)
├── k8s/                       # Kubernetes マニフェスト & ArgoCD 設定
│   ├── manifests/             # deployment, service, ingress, namespace
│   └── argocd/                # application.yaml (GitOps自動同期)
├── deploy/
│   └── nginx.conf             # 本番コンテナ用 Nginx 設定 (ポート8080, /healthz)
├── docs/                      # 開発・アーキテクチャドキュメント
│   ├── architecture.md        # アーキテクチャ設計書
│   ├── development.md         # 開発者ガイド
│   ├── deployment.md          # デプロイ・Kubernetesガイド
│   └── frontend-guidelines.md # フロントエンド設計・TypeScript規約
├── adr/                       # アーキテクチャ決定記録 (0001〜0010)
│   └── README.md              # ADR 作成ガイドライン
├── scripts/
│   ├── install.sh             # taite CLI インストールスクリプト
│   ├── build.sh               # フロントエンドビルドスクリプト
│   └── validate_schedule.js   # スケジュール整合性検証スクリプト
├── Makefile                   # 開発タスク実行定義
└── Dockerfile                 # マルチステージコンテナビルド定義
```

---

## 開発クイックスタート

### 必要なツール
- Go 1.27以上
- Node.js 20以上
- Docker（コンテナビルド・検証時）

### 1. CLI のインストール
```bash
make install
# または
go install ./cmd/taite
```

### 2. スケジュールデータの検証
```bash
make validate
# または
taite validate --file data/generated/schedule.json
```

### 3. フロントエンドの開発とビルド
```bash
# 開発サーバー起動 (HMR)
make web-dev

# 本番用バンドルビルド
make web-build
```

---

## デプロイ & GitOps

本プロジェクトは、`main` ブランチへのプルリクエストマージをトリガーにして、自動的にコンテナイメージビルドと Kubernetes への反映が行われます。

1. **GitHub Actions (`.github/workflows/deploy.yml`)**:
   - `main` へのプッシュを検知し、マルチステージ Dockerfile からコンテナイメージをビルド。
   - `ghcr.io/aobaiwaki123/taite-board:sha-<commit_sha>` として GHCR へプッシュ。
   - `k8s/manifests/deployment.yaml` のイメージタグを自動更新してコミット。
2. **ArgoCD GitOps**:
   - 最新マニフェストを検知し、Kubernetes クラスタへ自動同期。
3. **Cloudflare Tunnel**:
   - `ribonclock.iorin.net` 宛のトラフィックを安全に Kubernetes Ingress へ転送。

詳細は [docs/deployment.md](docs/deployment.md) を参照してください。

---

## 開発・運用規約

- **PR-Only Workflow**: `main` への直接Pushは禁止し、必ず feature ブランチから PR を作成して CI 検証後にマージします。
- **ドキュメント・コミットにおける客観的事実の重視**: 余計な比喩、誇張表現、絵文字を排除し、技術的事実・仕様のみを記述します。
- **Zero Telemetry**: ユーザーの行動追跡や外部テレメトリ送信を行いません。
