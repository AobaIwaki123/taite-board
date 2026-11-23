# ADR-0022: Google Cloud Storage (GCS) によるデータ正本移行とリアルタイム配信同期

## ステータス
Accepted (Supersedes ADR-0021)

## 決定日
2026-11-23

## 対象コンポーネント
- `internal/gcs/`
- `internal/schedule/`
- `internal/validate/`
- `internal/monitor/`
- `cmd/monitor/`
- `cmd/taite/`
- `deploy/nginx.conf`
- `k8s/manifests/base/rollout.yaml`

## コンテキストと課題 (Context & Problem Statement)
従来の `taite-board` は、確定スケジュールデータ（`schedule.json`, `data/groups/*/schedule.json`, `data/events/*/*.json`）を Git リポジトリ内に保持し、コンテナビルド時（`Dockerfile`）に Nginx イメージの `/usr/share/nginx/html/` へ静的コピーする構成をとっていた。この構成では以下の運用上の課題が生じていた。

1. **モバイル入力と本番配信の断絶**:
   - Tailscale 管理ダッシュボード（`ribonclock-dashboard`）に X 投稿の取得・AI 解析・登録 UI（`/api/intake`, `/api/intake/promote`, `/api/intake/patch`）が存在していたが、書き込み先がローカル開発前提の相対パス（`data/generated/schedule.json`, `apps/web/public/`）のままであり、本番 Kubernetes Pod 内の一時ストレージにしか書かれず、`web` コンテナの Nginx 配信データに反映されなかった。
2. **デプロイサイクルの重さ**:
   - スケジュール 1 件の追加・時刻確定・特典更新を行うたびに、ローカル PC での編集・Git Push・GitHub Actions でのイメージビルド・Argo Rollouts デプロイという一連の工程が必要となっていた。
3. **Blue/Green デプロイとローカル状態永続化の競合**:
   - `Rollout` が `blueGreen` 戦略を採用しているため、ReadWriteOnce (RWO) のブロックストレージ PVC を新旧 Pod で同時マウントできず、`emptyDir` 運用では Pod 入れ替え時に `intakes.json` や `pipeline_state.json` が揮発していた。

## 決定要因・評価軸 (Decision Drivers)
- **即時反映**: スマートフォンから X の投稿リンクを送信し確定した直後に、コンテナ再ビルドなしで公開サイトへ反映されること。
- **Zero Telemetry 原則の維持**: 訪問者のブラウザが外部クラウドエンドポイントへ直接アクセスして IP 等のメタデータが露出するのを防ぎ、自ドメインからの同一オリジン配信を維持すること。
- **最小依存・低コスト**: 既存の GCP プロジェクト（`taite-board`, `asia-northeast1`）を活用し、Go バイナリおよびコンテナサイズの肥大化を抑えること。
- **Git バックアップ互換性**: クラウドをデータの正本としつつ、必要に応じてローカル `data/` と双方向同期できる CLI を維持すること。

## 検討した選択肢 (Considered Options)
### 選択肢 1: GCS バケット正本 ＋ Nginx リバースプロキシ配信 ＋ Go 標準ライブラリクライアント
- **概要**:
  - 公開スケジュール用バケット（`taite-board-data`）と非公開状態用バケット（`taite-board-state`）を GCP `asia-northeast1` に作成する。
  - `monitor` コンテナが最小権限のサービスアカウント（`taite-storage`）経由で GCS JSON API を用いてスケジュール JSON と Intake 状態を直接読み書きする。
  - `web` コンテナの Nginx が `/schedule.json`, `/data/groups/`, `/data/events/` 等を `taite-board-data` へ `proxy_pass` し、`Cache-Control: no-cache` で配信する。
- **メリット (Pros)**:
  - フロントエンド（`apps/web`）および Service Worker のフェッチ構造を変更せず同一オリジン配信と Zero Telemetry を維持できる。
  - 標準ライブラリのみで GCS JSON API と OAuth2 JWT 認証を実装でき、`go.mod` への大型 SDK 依存追加が不要。
  - `intakes.json` / `pipeline_state.json` を非公開バケットへ逃がすことで Blue/Green Rollout 中も状態が揮発しない。
- **デメリット (Cons)**:
  - 自宅 k8s クラスタから GCS へ書き込むためのサービスアカウントキーを Kubernetes Secret（`ribonclock-gcs`）として管理する必要がある。
  - Git 上の `data/` とクラウド上の最新データに差分が生じるため、バックアップ用の同期コマンドが必要となる。

### 選択肢 2: フロントエンドから GCS 公開 URL を直接 Fetch する
- **概要**: `apps/web` の `fetch` 先を `https://storage.googleapis.com/<bucket>/...` へ切り替える。
- **メリット (Pros)**: Nginx 側のプロキシ設定が不要。
- **デメリット (Cons)**: CORS 設定が必要になり、かつ訪問者ブラウザから直接 Google サーバーへの通信が発生するため Zero Telemetry 方針に反する。

### 選択肢 3: Cloud Firestore / Cloud SQL 等のデータベースへ全面移行
- **概要**: スケジュール JSON を RDB/NoSQL テーブルへ分解し API サーバーから配信する。
- **メリット (Pros)**: レコード単位のトランザクションや検索に優れる。
- **デメリット (Cons)**: 既存の `schema/schedule.schema.json`、Go CLI（`validate`, `sort`, `add-session`, `update-session`）、およびオフラインキャッシュ構造の全面書き換えが必要となり過大実装になる。

## 決定事項 (Decision Outcome)
**選択肢 1（GCS バケット正本 ＋ Nginx リバースプロキシ配信 ＋ Go 標準ライブラリクライアント）** を採用する。

1. **GCS バケット構成 (`asia-northeast1`)**:
   - `gs://taite-board-data`: 公開スケジュールデータ専用（Uniform bucket-level access、`allUsers` に `roles/storage.objectViewer`）。
   - `gs://taite-board-state`: 非公開の運用状態（`intakes.json`, `pipeline_state.json`）専用（Public Access Prevention 有効）。
   - サービスアカウント `taite-storage@taite-board.iam.gserviceaccount.com` に両バケット限定の `roles/storage.objectAdmin` を付与し、k8s Secret `ribonclock-gcs` としてマウントする。
2. **軽量 GCS クライアント (`internal/gcs`)**:
   - 外部 SDK を追加せず、Go 標準ライブラリ（`crypto/rsa`, `net/http`）で RS256 JWT OAuth2 トークン取得および GCS JSON API（`GetObject`, `PutObject` with `multipart/related`, `ListObjects`）を実装する。
3. **変換ロジックの純粋関数化とスキーマ検証統合**:
   - `internal/schedule` の追加・更新・ソート処理をバイト列ベースの純粋関数（`ApplyAddSession`, `ApplyUpdateSession`, `ApplySortSchedule`）へ分離し、ローカル FS と GCS の双方で同一ロジックと `internal/validate`（`ValidateBytes`）を適用する。
4. **モバイル Intake UI とタイムアウト改善**:
   - `cmd/monitor` の `WriteTimeout` を 60 秒へ延長し、Cloud Run AI パーサー待機時の切断を防止する。
   - `dashboard.html` の抽出プレビューを編集可能フォーム化し、対象セッション選択（`GET /api/sessions`）と合わせてスマートフォン単体で補正・確定できるようにする。
5. **CLI 同期 (`taite gcs push / pull`)**:
   - 必要時にローカル `data/` と GCS バケットを同期し、Git バックアップを行えるコマンドを `cmd/taite` に追加する。

## 影響とトレードオフ (Consequences)
### 正の影響 (Positive)
- スマートフォン上の `ribonclock-dashboard` から X 投稿リンクを入力・確認・確定するだけで、コンテナ再ビルドなしに数秒で本番サイトへ反映される。
- `web` コンテナの同一オリジン配信・Service Worker キャッシュ・Zero Telemetry 要件がすべて維持される。
- ローカル開発時は `GCS_DATA_BUCKET` 未設定のまま従来のローカルファイル読み書きで動作するため、開発体験を損なわない。

### 負の影響とリスク (Negative & Mitigations)
- **Git とクラウドの乖離**: 本番データは GCS が正本となるため、Git リポジトリ上の `data/` は自動更新されない。定期的に `taite gcs pull` を実行して Git へスナップショットを反映する運用で補う。
- **サービスアカウントキー管理**: 自宅 k8s クラスタ側に SA キー Secret を配置するため、権限は対象 2 バケットのオブジェクト操作のみに限定し、定期的なキーローテーション対象とする。
- **外部疎通依存**: Nginx が GCS へプロキシするため、自宅クラスタから `storage.googleapis.com` へのアウトバウンド通信が遮断された場合はローカル静的ファイルフォールバック（`error_page 404` / 502 等）またはクライアント側 `localStorage` キャッシュで継続表示する。
