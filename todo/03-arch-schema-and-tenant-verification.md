# TODO-03: システム構成・マルチテナント分離スキーマ定義、整合検証スクリプト整備、既知ドリフト修正

- **ステータス**: 未着手
- **優先度**: 中（構造可視化と事故防止・既知不整合の解消）
- **並列着手**: 可能（完全独立）

---

## 1. 背景とスコープの境界

### 解決したい課題
- `ribonclock` に加え `dollykiss` 等の複数グループ展開を進める中で、「現在どのレイヤーまで共有されていて、どこからがテナントごとに分離されているのか（K8s Namespace / Ingress / Service / Pod / Nginx / GCS / 認証 RBAC / Monitor 状態ファイル / クライアントストレージ）」がコードやマニフェストを横断して追わないと把握できなくなっています。
- AI 中心で機能追加を積み上げた結果、人間の頭の中の設計イメージと実コードの乖離に気づきにくく、実際に設定の食い違い（後述のドリフト）も発生しています。

### 今回は「作らない」もの（スコープ制限）
- JSON 定義から Go/TS/K8s コードを自動生成（コンパイル）するメタプログラミングエンジンは作りません。
- JSON をインタラクティブに描画する独自 Web アプリケーションの一からの構築も行いません（別プロダクト級の工数になるため）。
- **今回の落とし所**: 「軽量な構造宣言 JSON + JSON Schema」「実コード/マニフェストとの食い違いを機械検知する検証スクリプト」「GitHub 上で視認可能なアーキテクチャ・テナント分離ドキュメント」「テナント追加スキル更新」の 4 点に絞り、最速で可視化と事故防止を実現します。

---

## 2. 現時点で判明しているテナント分離の実態と設定ドリフト

コード・マニフェスト調査で確認済みの事実は以下のとおりです。実装時には最新状態を再確認したうえで反映してください。

### 現行の分離境界（実態）
- **共有レイヤー**:
  - Kubernetes Namespace / Rollout / Service / Pod: 単一 Namespace `ribonclock`、単一 Rollout（`web` と `monitor` が同居）。
  - [`k8s/manifests/ingress.yaml`](../k8s/manifests/ingress.yaml): 単一 Ingress `ribonclock` で `taite.iorin.net`, `ribonclock.iorin.net`, `rincent.iorin.net`, `dollykiss.iorin.net` を同一 Service `ribonclock:http` へルーティング。
  - [`deploy/nginx.conf`](../deploy/nginx.conf): 単一 `server_name _` で全ホストを共通処理（`/data/groups/` 配下は GCS `taite-board-data/data/groups/` へのプロキシだが `Host` ヘッダに基づくアクセス制限は未設定）。
  - 認証・同期バックエンド（[`internal/syncapi/handler.go`](../internal/syncapi/handler.go)）: `auth/roles.json`（グローバル RBAC）、`users/<sha256(uid)>/sync.json`（ユーザー別、テナント別バケット分割なし）。
  - Monitor 状態ファイル（[`internal/monitor/collector.go`](../internal/monitor/collector.go)）: `intakes.json`, `activities.json`, `pipeline_state.json` は単一バケット直下で全テナントのログ・キューを共有。
- **論理分離レイヤー**:
  - スケジュール JSON: GCS `gs://taite-board-data/data/groups/<slug>/schedule.json` およびリポジトリ内 `data/groups/<slug>/schedule.json`。
  - Web クライアント設定（[`apps/web/src/config/group.ts`](../apps/web/src/config/group.ts)）: ドメイン/`?group=` により `storagePrefix`（`localStorage` 分離）とデータ取得先 URL を切り替え。

### 修正すべき既知のドリフト（不整合）
1. **`dollykiss.fallbackDataUrl` の誤設定**
   - [`apps/web/src/config/group.ts`](../apps/web/src/config/group.ts) 53行目で、`dollykiss` の `fallbackDataUrl` が `'schedule.json'`（`ribonclock` のデータ）になっており、エラー時フォールバックで別グループのデータが表示されうる状態です。グループ固有の安全なフォールバック先へ修正する必要があります。
2. **`group.ts` の `sample` エントリと [`data/groups.json`](../data/groups.json) / [`ingress.yaml`](../k8s/manifests/ingress.yaml) の不一致**
   - `group.ts` の `REGISTERED_GROUPS` には `sample` が入っていますが、`data/groups.json` や `ingress.yaml` には存在しません。ランタイム登録リストからテンプレート定数を分離する、あるいはテンプレートとしての扱いを明示して検証ルールと整合させる必要があります。
3. **残存ファイル [`k8s/manifests/tailscale-service.yaml`](../k8s/manifests/tailscale-service.yaml) の扱い確認**
   - `/admin` の RBAC 統合後も残っている `ribonclock-dashboard` 用マニフェストです。まだ運用上の要否確認が未確定のため、削除する場合は PR 前にユーザーへ要否を確認するか、アーキテクチャ定義上で現行位置付けを明記すること。

---

## 3. 実装要件

### 1. アーキテクチャ & テナント分離マニフェスト（JSON + JSON Schema）の策定
- `schema/` 配下にスキーマ（例: `schema/architecture.schema.json`）を定義し、実体マニフェスト JSON（例: `docs/architecture.manifest.json` または `data/architecture.json`）を置くこと。
- 少なくとも以下を表現できる構造にすること:
  - 主要コンポーネント一覧（`web`, `monitor`, `syncapi`, `crawler`, `taite-reconcile`, `taite-parser` 等）と接続・依存関係、データストア
  - レイヤー別のテナント分離レベル定義（`namespace`, `ingress`, `service_pod`, `nginx`, `schedule_gcs`, `auth_rbac`, `monitor_state`, `client_storage` 等それぞれに対し、`shared` / `host_routed` / `path_isolated` / `prefix_isolated` の種別とパス・注意点を記述）
  - 登録済みテナント定義と、各テナントに必須の不変条件（ホスト名、`dataUrl`、`fallbackDataUrl`、`storagePrefix` など）

### 2. ドリフト機械検証スクリプトの実装
- Go テストまたはシェルスクリプト/Node/Go コマンド（例: `scripts/verify-arch.sh` あるいは `go run ./cmd/taite verify-arch`）を作成し、[`./scripts/verify.sh`](../scripts/verify.sh) から自動実行されるようにすること。
- 検証すべき主な不変条件:
  1. `data/groups.json` の全グループと `apps/web/src/config/group.ts` の実稼働グループ一覧・`id`・`dataUrl`・`heroImage` 等が一致していること
  2. `fallbackDataUrl` が別グループの `schedule.json` を誤って指していないこと
  3. 稼働中の全テナントに対応するホスト（例: `<id>.iorin.net`）やエイリアスが [`k8s/manifests/ingress.yaml`](../k8s/manifests/ingress.yaml) に定義されていること
  4. Architecture JSON 自体が `schema/architecture.schema.json` に準拠し、実在するファイルパス・マニフェストと矛盾していないこと

### 3. 人間向けドキュメント & テナント追加スキルの更新
- [`docs/architecture.md`](../docs/architecture.md)（または関連セクション）に、上記 JSON の実態と完全一致する「テナント分離マトリクス表」および依存関係 Mermaid 図（GitHub で構文エラーにならないよう特殊文字は必ず `"` でクォートすること）を整備すること。
- リポジトリ上位ワークスペース側のテナント追加スキル（もしアクセス・変更対象に含まれる場合: `../../.agents/skills/taite-tenant-scaffold/SKILL.md`）にも、「アーキテクチャ定義・検証の更新」手順を含めるか確認・整理すること。

---

## 4. 検証条件

- [ ] `apps/web/src/config/group.ts` の `dollykiss.fallbackDataUrl` および `sample` 定義の不整合が解消されていること
- [ ] アーキテクチャ / テナント分離マニフェスト JSON とスキーマが追加され、整合性検証スクリプトが `verify.sh` に組み込まれていること
- [ ] 意図的に設定を壊したケース（別グループの `fallbackDataUrl` を設定する等）で検証スクリプトが失敗することをテストで確認すること
- [ ] `./scripts/verify.sh` および `go test ./internal/...` が全通過すること
