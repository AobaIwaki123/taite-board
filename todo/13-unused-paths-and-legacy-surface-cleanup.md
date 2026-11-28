# TODO-13: 可視化構成に基づく無駄なパス・ポート・レガシー資産の整理

- **ステータス**: 未着手（セルフレビュー起票）
- **優先度**: 中（保守性・構成簡素化・ビルド/運用コスト削減）
- **目的**: `docs/system-graph.yaml` で可視化した現行経路を基準に、重複した HTML・HTTP ポート・リバースプロキシ定義・休止中マニフェスト・CI イメージビルドを整理し、保守対象を最小化する。

## 監査で確認された現状と課題

1. **レガシー HTML 管理画面と二重 HTTP サーバの残存**:
   - TODO-04 で管理画面を `apps/web/` の `AdminView.tsx`（`/admin`）へ統合したにもかかわらず、`internal/monitor/admin.html`（1,679 行）と `internal/monitor/dashboard.html`、`internal/monitor/server.go` の `/`, `/register`, `/admin` ルートが残存している。
   - TODO-06〜10 の仕様変更は Preact 版 `AdminView.tsx` のみに適用されており、旧 `admin.html` は仕様ドリフトの温床となっている。
   - `cmd/monitor/main.go` が `:8082`（旧構成・Tailscale 直結）と `:8083`（`syncapi`・Nginx 経由）の 2 つの HTTP サーバを起動し、`syncapi/handler.go` が `/api/admin/*` → `/api/*` へパス書き換えして `monitor.Handler` へ転送する迂回構造になっている。
   - `AdminView.tsx` ヘッダーの「監視」リンクが旧 Go テンプレート画面 `/admin/dashboard`（`DashboardHTML`）へ向いており、UI/UX と設計システムが分断されている。
2. **Nginx の冗長プロキシ定義とリライト漏れ (`deploy/nginx.conf`)**:
   - GCS バケットへの透過プロキシブロック（`proxy_pass https://storage.googleapis.com/taite-board-data/...` + `@local_fallback`）が 5 箇所（`/schedule.json`, `/mock_schedule.json`, `/data/groups.json`, `/data/groups/`, `/data/events/`）に重複記述されている。
   - `/admin`, `/graph`, `/arch` の SPA ルーティングと末尾スラッシュリダイレクトも 3 セット重複している。
   - TODO-12 でレガシーミラーを廃止すれば `/schedule.json`・`/mock_schedule.json` を別 GCS オブジェクトに向ける必要がなくなり、すべて `data/groups/ribonclock/schedule.json` への内部リライトまたは `/data/` 単一プロキシにまとめられる。
3. **休止中 (`suspend: true`) のマニフェストと常時ビルドされ続ける不要イメージ**:
   - `k8s/manifests/crawler-cronjob.yaml`（`suspend: true`）と `k8s/manifests/access-report-cronjob.yaml`（`suspend: true`）は現行の `log_tailer` + Prometheus + `timetree-sync` + `scrape` 構成で代替されており、運用では使われていない。
   - それにもかかわらず `k8s/manifests/kustomization.yaml` に登録されたままで、さらに CI（`Deploy ribonclock`）が毎回 `deploy/access-report.Dockerfile`（EOL の `python:3.9-alpine` ベース）と crawler イメージをビルド・プッシュしている。
4. **ローカル/CLI 同期ターゲットの整理 (`internal/schedule/sync.go`, `scripts/sync-schedule.sh`, `cmd/taite/gcs.go`)**:
   - `ribonclock` 1 グループのために 4 箇所へコピーする運用を見直し、不要なルートミラーコピーを削減できる。

## 検討・実装項目

1. **HTTP サーバ・管理ルートの一本化**:
   - `:8082` リスナーおよび `internal/monitor/admin.html`（`/register` 含む）を廃止し、認証・RBAC を統合した単一のアプリケーション HTTP ポート（`:8083`）+ Metrics ポート（`:9090`）へ整理する。
   - `dashboard.html`（監視ビュー）も `/api/admin/stats` を使う軽量 JSON ビューとして Preact SPA（`/?view=stats` 等）へ統合するか、最小維持にするかを決定し、使われないレガシーエンドポイント（例: 重複 HTML 配信や未利用ルート）を削除する。
2. **Nginx ルーティングと GCS 公開パスの整理 (`deploy/nginx.conf`)**:
   - `/schedule.json` / `/mock_schedule.json` をレガシー互換リライトに統一し、重複した GCS プロキシ設定を統合・簡素化する。
   - `/admin`, `/graph`, `/arch` などの SPA エントリとセキュリティヘッダ（`X-Forwarded-Proto` 引き回し、`Cache-Control` 等）をクリーンに整理する。
3. **休止中ワークロード・CI ビルドの見直し**:
   - `crawler-cronjob.yaml` と `access-report-cronjob.yaml`（および `access-report` / `crawler` の CI 自動ビルドと Dockerfile）を実際に再利用する予定があるか整理し、不要であれば `kustomization.yaml` と CI ビルド対象から外す（必要な場合でも `python:3.9-alpine` の更新や明示的分離を行う）。
4. **`docs/system-graph.yaml` と設計図の同期更新**:
   - 削除・統合したノード（`legacy_mirror`, `suspended` 資産, `:8082`）を `docs/system-graph.yaml` に反映し、`gen-system-graph.mjs` で図と検証テストを最新化する。

## 検証条件

- [ ] 管理エンドポイントと SPA ルートの二重定義が解消され、`/admin`・`/graph`・`/arch`・`/api/admin/*`・`/api/sync/*` が単一統合経路で動作すること。
- [ ] 不要なコンテナビルドおよび休止リソースが整理され、CI デプロイ時間が短縮または依存が削減されていること。
- [ ] `node scripts/gen-system-graph.mjs --check`（`npm test`）および `./scripts/verify.sh`、`go test -count=1 ./...` がすべて通過すること。
