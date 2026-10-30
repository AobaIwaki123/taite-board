# Ribbon Clock taite-board

アイドルグループ「リボンクロック」および神楽あゆ（赤色担当）のライブ告知・タイムテーブル情報を即時把握するための超低摩擦計器盤Webアプリケーションです。

混雑したライブハウスや移動中などの過酷な環境下でも、スマートフォンを開いた瞬間に「出番・特典会・会場・チケット・フライヤー・タイテ表」が0.5秒で把握できる体験を提供します。

- **本番URL**: [https://ribonclock.iorin.net/](https://ribonclock.iorin.net/)
- **ローカル開発URL**: `http://localhost:8089/index.html`

---

## 主な特徴

1. **App-Shell 計器盤 UI**:
   - 画面上部に重要情報（出番・特典会・会場・チケット）を集約したコックピットデザイン。
   - スムーズな横スワイプカルーセルと、スワイプ移動量に1:1で完全追従するリアルタイム・スライディングタブインジケーター。
   - 各タブ（直近ライブ・全日程・カレンダー）が完全に独立した縦スクロールコンテナを持ち、長い一覧をスクロールした後でもカレンダーが空白にならず常に最上部から表示。
2. **マルチ画像プレビュー & 高視認性 Lightbox**:
   - 告知ポストの複数画像（フライヤー画像とタイムテーブル画像）をアプリ内でワンタップ切り替え。
   - ピンチズーム対応、背景タップ即座終了のフローティングLightbox。
3. **オフライン・ファースト & 鮮度インジケーター**:
   - 電波が届かない地下ライブハウスでも、ローカルキャッシュから即座に前回のスケジュールを表示。
   - `generated_at` に基づき、データが24時間以上経過している場合やオフライン動作時に控えめなインジケーター（`#freshness-indicator`）でデータ鮮度を明示。
4. **プライバシー & ガバナンス（Zero Telemetry）**:
   - クライアント側での非合意な訪問者追跡、UUID生成、外部テレメトリ収集を完全排除。

---

## アーキテクチャの概要

本リポジトリは、外部サービス（Xやチケットサイト）からの生データ取得、決定論的名寄せ・キュレーション、スキーマ検証済みJSONの生成、そして静的クライアント配信を完全に分離した単一方向パイプラインで構成されています。

```text
URL単体 / URLリスト / 標準入力
        ↓
Go CLI: taite fetch
        ↓
data/raw/runs/<run-id>/ (取得した不変の事実)
        ↓
taite extract (本文抽出 + 画像解析)
        ↓
data/candidate/ (機械抽出候補)
        ↓
taite publish ← data/curated/ (entities.json, overrides.json)
        ↓
data/generated/schedule.json (schema/schedule.schema.json 準拠)
        ↓
apps/web/ (Nginxコンテナ / Cloudflare Tunnel / k8s)
```

詳細な設計思想と意思決定は [docs/architecture.md](docs/architecture.md) および [adr/](adr/) を参照してください。

---

## ディレクトリ構成

```text
taite-board/
├── apps/
│   └── web/                   # Webクライアント正本ソース (HTML, JS, CSS)
│       ├── index.html         # 静的HTMLマークアップ
│       ├── app.js             # アプリケーションロジック・レンダリング
│       ├── styles.css         # アニメーション・スタイル定義
│       └── images/            # 静的アセット
├── cmd/
│   └── taite/                 # 統合Go CLI (fetch, validate, build)
│       ├── main.go
│       └── README.md
├── internal/                  # パイプライン内部パッケージ (非公開)
│   ├── source/x/              # X (Twitter) 外部通信アダプター
│   ├── raw/                   # 生スナップショット・マニフェスト定義
│   ├── fetch/                 # 並列取得・不変保存オーケストレータ
│   └── validate/              # JSON Schema ドメイン整合性バリデータ
├── data/
│   ├── input/                 # クローラー入力 (urls.txt)
│   ├── raw/                   # 実行別・投稿別の不変生JSONスナップショット
│   ├── candidate/             # 自動抽出候補データ
│   ├── curated/               # 人間確認済みの名寄せ辞書と手動上書き
│   │   ├── entities.json      # グループ・メンバー・会場・チケット別名辞書
│   │   └── overrides.json     # 画像内タイテ・確定値の上書き設定
│   └── generated/
│       └── schedule.json      # クライアント共通の確定公開データ
├── schema/
│   └── schedule.schema.json   # 公開データ契約のJSON Schema (Draft-07)
├── k8s/                       # Kubernetesマニフェスト & ArgoCD設定
│   ├── manifests/             # deployment, service, ingress, namespace
│   └── argocd/                # application.yaml (GitOps自動同期)
├── deploy/
│   └── nginx.conf             # 本番コンテナ用Nginx設定 (ポート8080, /healthz)
├── docs/                      # ドキュメント & GitHub Pages公開成果物
│   ├── architecture.md        # 全体アーキテクチャ設計書
│   ├── development.md         # 開発者向けセットアップ・コマンドガイド
│   └── deployment.md          # デプロイ・Kubernetesインフラ解説
├── adr/                       # アーキテクチャ意思決定記録 (0001〜0009)
├── scripts/
│   ├── build.sh               # アセットおよび確定JSONの配布同期スクリプト
│   └── validate_schedule.js   # 画像HTTP 200疎通および整合性検証スクリプト
└── Dockerfile                 # 本番ホスティング用Alpine Nginxイメージ
```

---

## 開発クイックスタート

### 必要なツール
- Go 1.27以上
- Node.js 18以上
- Docker（コンテナビルド・検証時）

### 1. CLIのビルド
```bash
go build -o bin/taite ./cmd/taite
```

### 2. スケジュールデータの検証
```bash
./bin/taite validate data/generated/schedule.json
node scripts/validate_schedule.js
```

### 3. アセットの同期ビルド
`apps/web/` の変更や `data/generated/schedule.json` の変更を配信ディレクトリへ反映します。
```bash
./bin/taite build
# または
./scripts/build.sh
```

### 4. ローカル確認用Webサーバーの起動
```bash
python3 -m http.server 8089 --directory docs/
# ブラウザで http://localhost:8089/index.html を開く
```

---

## デプロイ & GitOps

本プロジェクトは、保護された `main` ブランチへのプルリクエストマージをトリガーにして、自動的にコンテナイメージビルドとKubernetesへの反映が行われます。

1. **GitHub Actions (`.github/workflows/deploy.yml`)**:
   - `main` ブランチへのプッシュを検知し、Dockerイメージをビルド。
   - `ghcr.io/aobaiwaki123/taite-board:sha-<commit_sha>` としてGHCRへプッシュ。
   - `k8s/manifests/deployment.yaml` のイメージタグを自動更新してコミット。
2. **ArgoCD GitOps**:
   - リポジトリの最新マニフェストを検知し、自宅Kubernetesクラスタへ自動同期（Auto-Sync / Self-Heal）。
3. **Cloudflare Tunnel**:
   - `ribonclock.iorin.net` 宛のトラフィックを安全にKubernetes Ingressへ転送。

詳細は [docs/deployment.md](docs/deployment.md) を参照してください。

---

## 規約とポリシー

- **PR-Only Workflow**: `main` への直接Pushは行わず、必ずフィーチャーブランチからPRを作成して検証後にマージします。
- **Zero Telemetry**: 個人を特定するCookieやトラッキングは使用しません。
- **No Hallucination**: ツイート本文や画像で確認できない時刻は推測で埋めず、告知待ち（`--:--`）として扱います。
- **絵文字不使用**: ドキュメントおよびコミットメッセージには原則として絵文字を使用しません。
