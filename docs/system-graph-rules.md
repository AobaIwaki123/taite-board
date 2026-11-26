# システム構成グラフ記述規約 (`docs/system-graph.yaml`)

本規約は、`taite-board` の現行アーキテクチャ、データ読み書き経路、および排他制御の実態を単一の YAML（[docs/system-graph.yaml](system-graph.yaml)）として構造化するための定義である。

---

## 1. 基本方針

1. **手動の正確な記述を正本とする（自動同期は行わない）**
   - コードからの AST 解析や自動同期機構は導入せず、コード・マニフェストの実態を読み解いた構造化データとして保守する。
2. **表現と描画の分離**
   - YAML は「何がどこにどう繋がっているか」を表すデータ層に徹し、レイアウトや色分けなどの描画責任を持たせない。
   - 後続のトランスパイラ（D2 / SVG ビュー生成、Markdown 逆引き表生成、JSON 出力など）がこの YAML を参照して用途別の出口を生成する。
3. **既存マニフェストとの棲み分け**
   - [architecture.manifest.json](architecture.manifest.json)（既存の全体概要・D2・3D ビュー入力）は本規約の確定段階までそのまま維持し、まずは本規約に基づく詳細グラフで実態把握を完了させる。

---

## 2. レイヤー定義とノード粒度

図や検索時に抽象度が混在して絡まるのを防ぐため、すべてのノードを **L1〜L5 の 5 レイヤー** のいずれかに分類し、レイヤーごとの粒度を固定する。

| レイヤー | 名前 | 粒度の定義（含める単位 / 含めない単位） | 代表例 |
|---|---|---|---|
| **L1** | `edge` (Edge / Client) | 利用者側ブラウザアプリおよびネットワーク入口。UI コンポーネントや個別 API パス単位には分割しない。 | `web_spa`, `cloudflare_ingress`, `nginx_proxy` |
| **L2** | `service` (Runtime Service) | 常駐プロセスの待受機能・常時稼働バックグラウンドループ単位。Go 関数単位には分割しない。 | `monitor_api (:8082)`, `sync_api (:8083)`, `metrics_endpoint (:9090)`, `log_tailer` |
| **L3** | `worker` (Async / Batch / CLI) | 起動契機ごとの実行単位（CronJob、Cloud Run Service/Job、一時 Job、手動 CLI）。 | `timetree_sync`, `taite_reconcile`, `taite_parser`, `taite_resync_x`, `taite_gcs_cli`, `crawler_cron`, `access_report_cron` |
| **L4** | `store` (Data & State) | バケット / PVC / クライアント領域における論理データ契約単位。個別の日付ファイル等には分割しない。 | `gcs_data_groups`, `gcs_data_mirrors`, `gcs_state_monitor`, `gcs_state_auth`, `local_pvc`, `browser_local_storage` |
| **L5** | `external` (External Systems) | リポジトリ外で稼働する外部 API・SaaS・通知先。 | `x_apis`, `lumitree_api`, `vertex_ai`, `identity_platform`, `discord_webhook` |

### ノードの必須・任意フィールド

```yaml
nodes:
  - id: monitor_api              # 一意な snake_case 識別子
    layer: service               # edge | service | worker | store | external
    kind: http_server            # spa | ingress | proxy | http_server | loop | cronjob | cloud_run | job | cli | gcs | pvc | image_fs | browser_storage | external_api
    label: "Monitor / Admin API (:8082)"
    status: active               # active | suspended | legacy_mirror | manual
    host_group: monitor_process  # 任意: 同居先（Pod / Process）ID。containment 枠で表現し矢印にはしない
    entry: "internal/monitor/server.go"
    notes: "補足（必要な場合のみ）"
```

### 同居境界（`host_groups`）の扱い

- 「`nginx` と `monitor` が同一 Pod に同居する」「`monitor_api (:8082)` と `sync_api (:8083)` が単一 Go バイナリ・単一コンテナメモリ上限（例: `64Mi`）を共有している」といった物理的・プロセス的同居関係は、**ノード間を結ぶ矢印にはせず `host_groups`（包含枠）として独立定義する**。

---

## 3. レイヤー間の接続（エッジ）表現ルール

すべての矢印が同じ記号で描かれることによる可読性低下と双方向矢印の乱立を防ぐため、エッジは以下の規約で記述する。

### 3.1 エッジ種別（`kind`）の 3 分類

1. **`request`**: L1〜L2 間のオンライン同期 HTTP リクエスト・プロキシ経路
2. **`data`**: アクター（L1〜L3）からデータストア（L4）への読み書き経路
3. **`invoke`**: ジョブ起動、Cloud Run 呼び出し、外部 API 参照、Webhook 通知、バッチによる別レイヤー駆動

### 3.2 矢印の向き（有向一方向の徹底）

- **`request` / `invoke`**: 必ず「呼び出し元 (`from`) -> 呼び出し先 (`to`)」とする。
- **`data`**: 読み取り（`read`）であっても書き込み（`write`）であっても、**必ず「アクター (`from`: L1〜L3) -> ストア (`to`: L4)」の向きに統一する**。読み書きの区別は矢印の向きではなく `op` 属性で表現する。
  - これにより、グラフ全体が上流（Edge / Service / Worker）から下流（Store / External）へ流れる有向構造となり、描画時の矢印交差を最小化できる。

### 3.3 エッジのスキーマ

```yaml
edges:
  - id: monitor_write_schedule
    from: monitor_api
    to: gcs_data_groups
    kind: data                   # request | data | invoke
    op: write                    # data の場合必須: read | write | rw / request・invoke の場合は省略可
    transport: gcs_json_api      # https | http_proxy | gcs_json_api | fs | memory | k8s_run | external_https
    concurrency: none_put        # data(write/rw) の場合必須: none_put | generation_lock | shared_pvc_single | client_local | read_only
    multi_target: true           # 任意: 1回の操作で複数ファイル/オブジェクトへ順次書き込む場合 true
    status: active               # active | suspended | legacy_mirror | manual
    targets:
      - "data/groups/<group>/schedule.json"
      - "schedule.json (ribonclock legacy mirror)"
      - "mock_schedule.json (ribonclock legacy mirror)"
    entry: "internal/monitor/server.go#writeGroupSchedule"
    notes: "Promote / Patch / CreateSession から呼ばれ、通常 PutObject で順次上書きする"
```

### 3.4 `concurrency` 値の意味

| 値 | 意味 |
|---|---|
| `none_put` | 世代番号やロックを持たない上書き書き込み（並行実行時に後勝ちで欠損しうる） |
| `generation_lock` | GCS の世代番号（`GetObjectVersion` + `PutObjectIfGeneration`）による楽観ロック＋リトライ付き書き込み |
| `shared_pvc_single` | 共有 PVC へのローカルファイル追記・上書き（NFS/RWO 等ストレージ制約に依存） |
| `client_local` | 利用者ブラウザ単位の `localStorage` |
| `read_only` | 読み取り専用 |

---

## 4. 描画・射影時のフィルタリング原則（絡まり防止）

1 枚の図へ全レイヤー・全 `kind` を同時に描くと線が交差するため、可視化・レビュー時は必ず目的に応じて射影（フィルタ）して出力する。

- **ビュー 1: データ競合・整合性ビュー**
  - 抽出対象: `kind == "data"` のエッジと、それに接続する L2/L3/L4 ノードのみ。
  - 強調軸: `concurrency`（例: `none_put` vs `generation_lock`）および `multi_target: true`。
- **ビュー 2: オンライン配信・API 導線ビュー**
  - 抽出対象: `kind == "request"` および `L1/L2 -> L4` の `read` / `write` エッジのみ。
- **ビュー 3: インジェスト・バッチ処理フロービュー**
  - 抽出対象: `L3` ノードと `kind in ["invoke", "data"]`（`L5` 含む）。`status: suspended` は破線表示または除外する。

---

## 5. トランスパイル方針（YAML からの出口）

[docs/system-graph.yaml](system-graph.yaml) を唯一の手動更新対象とし、[schema/system-graph.schema.json](../schema/system-graph.schema.json) による検証のうえ、`node scripts/gen-system-graph.mjs --render` により以下の出口を生成する（初期段階では手動スクリプト実行とし、常駐同期は行わない）。

1. **D2 / SVG 生成**: 全体構成図（[graph-overview.d2](graph-overview.d2) / [graph-overview.svg](graph-overview.svg)）およびデータ競合ビュー（[graph-dataflow.d2](graph-dataflow.d2) / [graph-dataflow.svg](graph-dataflow.svg)）を生成する。
2. **Markdown サマリー生成**: `to: <store_id>` かつ `op in [write, rw]` のエッジを逆引き集計し、[docs/system-graph.md](system-graph.md) へ図解リンク・Store 逆引きテーブル・`host_groups` 一覧を出力する。
3. **JSON エクスポート**: `node scripts/gen-system-graph.mjs --json` により、検証済みグラフを JSON へ変換して標準出力へ供給する。
