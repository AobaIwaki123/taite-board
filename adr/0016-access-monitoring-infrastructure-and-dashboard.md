# ADR-0016: プライバシー配慮型アクセス監視基盤とリアルタイムダッシュボード

## ステータス

Accepted

## 決定日

2026-11-01

## 対象コンポーネント

- `cmd/monitor/`
- `internal/monitor/`
- `deploy/nginx.conf`
- `deploy/monitor.Dockerfile`
- `k8s/manifests/deployment.yaml`
- `k8s/manifests/service.yaml`
- `k8s/manifests/servicemonitor.yaml`
- `k8s/manifests/dashboard-ingress.yaml`
- `.github/workflows/deploy.yml`

---

## コンテキストと課題 (Context & Problem Statement)

Ribbon Clock (taite-board) は、Zero Telemetry（クライアント側での個人追跡・Cookie・解析SDKの完全排除）を基本原則としている。
運用者は「どの時間帯に、配信サーバーへ何件のリクエストが到達したか」「ルート別のアクセス比率やエラー発生状況」を把握する必要があるが、これまでの構成には以下の課題が存在した。

1. **Pod再起動に伴うログ消失**:
   - Nginx は標準出力に匿名JSONログを出力していたが、Pod の交換（ローリングアップデートやノード再配置）が行われると過去ログが消失し、累積アクセス推移を追跡できない。
2. **Prometheus / クラスタ可観測性との未接続**:
   - クラスタ内（`observability` 名前空間）に kube-prometheus-stack が稼働しているにもかかわらず、Ribbon Clock のメトリクス収集（`/metrics`）および ServiceMonitor が未整備であった。
3. **直感的な可視化ダッシュボードの欠如**:
   - 運用の健全性やアクセス状況を即座に確認できる Web UI が存在せず、手動での `report_access_counts.py` スクリプト実行に依存していた。

---

## 決定要因・評価軸 (Decision Drivers)

1. **Zero Telemetry / プライバシー保護の厳格な維持**:
   - IPアドレス、User-Agent、Cookie、クエリパラメータを一切保存せず、Nginx の固定分類ルート（`document`, `schedule`, `asset`, `other`）と HTTP ステータスコードのみを集計対象とすること。
2. **軽量・高信頼なアーキテクチャ**:
   - 重量級のログ基盤（Elasticsearch、Loki、Fluentd等）を導入せず、同一 Pod 内の `emptyDir` 共有ログと単一の軽量 Go バイナリ（`cmd/monitor`）によるサイドカー構成で完結させること。
3. **二系統の可観測性提供（Prometheus + 独立ダッシュボード）**:
   - ポート 9090 で Prometheus 標準フォーマット（`ribonclock_http_requests_total`）を内部公開し、クラスタの Prometheus が 15 秒間隔で scrape。
   - ポート 8082 で自己完結型のダークテーマ Web ダッシュボードを提供し、Cloudflare Tunnel 経由（`ribonclock-dashboard.aooba.net`）で即座にアクセス状況を閲覧可能にすること。

---

## 検討した選択肢 (Considered Options)

### 選択肢 1: Nginx の標準出力を CronJob / Fluentbit で外部収集
- **概要**: クラスタ全体で Fluentbit や Promtail を動かし、Loki などのログ集計ストレージへ転送する。
- **デメリット**: 個人開発・小規模サービスに対してストレージ消費や運用負荷が過剰であり、オーバーエンジニアリングとなる。

### 選択肢 2: Go サイドカーによるインメモリ集計 + Prometheus / Web ダッシュボード同時提供（採用案）
- **概要**:
  - Nginx は `emptyDir` 内の共有アクセスログ（`/var/log/ribonclock/access.log`）へ出力。
  - 同一 Pod のサイドカー `monitor`（Go製、メモリ使用量約10MB）がログをリアルタイムに tail して集計。
  - `/metrics`（Prometheus形式）と `/`（自己完結型リアルタイムWebダッシュボード）を提供。
- **メリット**:
  - 外部依存ゼロ、追加DB不要。
  - リクエスト処理と集計のオーバーヘッドが極小（数マイクロ秒）。
  - Prometheus と Grafana への長期保存にも、ブラウザからのリアルタイム確認にも両対応。

---

## 決定事項 (Decision Outcome)

**選択肢 2 を採用する。**

### 1. メトリクス契約
- **`ribonclock_http_requests_total{route="...", status="..."}`**:
  - `route`: `document`, `schedule`, `asset`, `other`
  - `status`: `200`, `304`, `404`, `500` 等
- **`ribonclock_monitor_uptime_seconds`**: モニター稼働秒数

### 2. ダッシュボード機能仕様 (`ribonclock-dashboard.aooba.net`)
- **Key Metrics**: 累計リクエスト数、本日アクセス数（JST）、直近1時間アクセス数、エラー率（%）。
- **Route Breakdown**: Document、Schedule API、Assets、Other の件数および割合プログレスバー。
- **HTTP Status Breakdown**: 2xx（正常）、3xx（キャッシュ）、4xx（クライアントエラー）、5xx（サーバーエラー）。
- **24-Hour Trend Chart**: 過去24時間の時間帯別リクエスト数の棒グラフ。
- **Live Stream**: 最新50件のアクセス履歴（時刻、ルート、ステータスバッジ）。
- **Auto-Refresh**: 5秒ごとの自動更新。

---

## 影響とトレードオフ (Consequences)

### 正の影響 (Positive)
- 運用者がいつでもブラウザから `https://ribonclock-dashboard.aooba.net/` にアクセスし、直近のアクセス状況とエラー発生をリアルタイムに監視できる。
- `observability` の Prometheus / Grafana ともシームレスに統合され、長期トレンド分析が可能になる。
- プライバシー保護要件（Zero Telemetry）を完全に遵守。

### 負の影響とリスク (Negative & Mitigations)
- Pod 再作成時にサイドカーのインメモリ直近バッファ（24時間データ）がリセットされる。
  - **対策**: 長期データは `ServiceMonitor` を通じて Prometheus が永続保持するため、長期的な分析は Prometheus 側で担保される。
