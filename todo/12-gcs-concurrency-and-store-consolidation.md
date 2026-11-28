# TODO-12: バックエンド書込競合の根絶と GCS・State ストア統合整理

- **ステータス**: 未着手（セルフレビュー起票）
- **優先度**: 高（データ整合性・信頼性）
- **目的**: 複数ライターから GCS (`taite-board-data`, `taite-board-state`) への書き込みを世代ロック付き単一経路へ統一し、データ消失リスクと不要なミラー書き込み・重複ストアアクセスを解消する。

## 監査で確認された現状と課題

1. **GCS 書込における世代ロック (`IfGenerationMatch`) の不統一 (`docs/system-graph.yaml`)**:
   - `internal/timetreesync/sync.go` と `internal/xsync/xsync.go` は `GetObjectVersion` + `PutObjectIfGeneration` で競合上書きを防いでいる。
   - 一方で、`internal/monitor/server.go` (`writeGroupSchedule`)、`internal/reconcile/runner.go` (`Run`)、`internal/scrape/state.go` (`GCSStateStore.Save`)、`internal/syncapi/handler.go` (`handleSyncData`, `getOrInitCookieSecret`)、`internal/monitor/collector.go` (`persistIntakesLocked`, `persistPipelineStateLocked`, `persistActivitiesLocked`) はすべてロックなしの通常 `PutObject` となっている。
   - そのため、12:00 / 22:00 JST の `timetree-sync` CronJob・`reconcile` Job・管理画面操作が重なると、後勝ちの丸ごと上書きによるロストアップデートが発生しうる。
2. 「**まとめて反映**」の**並列実行によるセッション消失レース (`AdminView.tsx` x `monitor/server.go`)**:
   - `apps/web/src/components/AdminView.tsx` の「まとめて反映」ボタンは `readyEntries.forEach((item) => void commitEntry(item))` で複数件の `POST /api/admin/intake/promote` を同時並列発行している。
   - 受け手側の `monitor/server.go` にはグループ別排他ロックも `PutObjectIfGeneration` リトライもないため、同一グループに対して複数リクエストが同時に `readGroupSchedule` → `ApplyAddSession` → `writeGroupSchedule` を走り、片方のセッション追加がもう片方に上書き消去される。
3. **レガシーミラー 2 オブジェクト (`schedule.json`, `mock_schedule.json`) への順次・非原子的書込**:
   - `ribonclock` 更新のたびに `writeGroupSchedule`, `reconcile`, `timetreesync`, `xsync`, `cmd/taite/gcs.go` が `data/groups/ribonclock/schedule.json` に加えて `schedule.json` と `mock_schedule.json` の計 3 オブジェクト（ローカル同期は `internal/schedule/sync.go#DefaultSyncTargets` で 4 箇所）へ順次書き込みを行っている。
   - 途中の `PutObject` が失敗すると正本とミラーで内容が分岐し、`reconcile` / `timetreesync` にミラー整合チェックという余分な責務を背負わせている。
4. **State ストア設計・アクセス頻度の整理不足**:
   - `internal/syncapi/handler.go#ResolveRole` が保護 API（`/api/admin/*`, `/api/sync/me`, `/admin`）のリクエストごとに毎回 `auth/roles.json` を GCS から同期取得しており、短期 TTL キャッシュがない。
   - `internal/scrape/state.go` は全グループのチェックポイントと `pending_queue` を単一の `scrape/state.json` に保持しているため、別グループ処理や管理画面からの `DELETE /api/admin/scrape/pending` と競合しやすい。
   - `internal/monitor/collector.go` の `Snapshot()` は 10 秒ごとにローカル `data/` ディレクトリの `telemetry.CollectAndPersist` を走らせるが、K8s 本番 Pod では `data/` はコンテナイメージ内の静的コピーであり、GCS 実データやパイプライン状態との役割が混在している。
   - `internal/monitor/server.go` のインライン `reconcile.Run` や画像ダウンロードを伴う `VisualDedupeSessionByIDBytes` が 64Mi メモリ制限の `monitor` コンテナ内で同時実行制限なしに動きうる。

## 検討・実装項目

1. **GCS Read-Modify-Write 共通基盤の整備 (`internal/gcs` / `internal/schedule`)**:
   - `GetObjectVersion` → 更新コールバック → `PutObjectIfGeneration`（コンフリクト時リトライ）を共通ヘルパーとして実装する。
   - `internal/monitor/server.go`（`promote`, `patch`, `writeGroupSchedule`）、`internal/reconcile/runner.go`、`internal/scrape/state.go` の全 GCS 更新をこの共通ヘルパー経由に統一し、`monitor` プロセス内にもグループ単位の Mutex（およびインライン重量処理のセマフォ制限）を入れる。
2. **レガシーミラー廃止による「グループ単位単一正本」への統合**:
   - フロントエンド（`apps/web/src/config/group.ts`）と Nginx（`deploy/nginx.conf`）の読み込み先を `data/groups/<id>/schedule.json` に統一し（必要なら `/schedule.json` 等は Nginx 側で内部プロキシ先を `data/groups/ribonclock/schedule.json` へ向ける）、全 Go ライターの `mirrors` 書き込み処理を撤廃する。
3. **State バケットの軽量キャッシュと競合分離**:
   - `ResolveRole` に短いインメモリキャッシュ（例: 30 秒 TTL + エラー時フェイルクローズ）を導入して GCS 読み込み負荷と API レイテンシを削減する。
   - `scrape/state.json` への `GetObjectVersion` + `PutObjectIfGeneration` 適用（またはグループ別キー `scrape/<group>/state.json` への分割）を行う。
   - `collector.go` の `telemetry` スキャン（本番環境で静的ローカル `data/` を繰り返し読む無駄な処理）と GCS 状態保存の責務を整理し、テストも含めて整理する。

## 検証条件

- [ ] `monitor_api`, `reconcile`, `timetree-sync`, `xsync`, `scrape/state` の全 GCS ライターが世代ロック（+ プロセス内排他）で保護され、並列更新テストでロストアップデートが発生しないこと。
- [ ] `schedule.json` / `mock_schedule.json` の独立 GCS ミラー書き込みが廃止され、`data/groups/<group>/schedule.json` のみへの原子的更新へ一本化されていること。
- [ ] `go test -count=1 ./...` および `system-graph.yaml` 整合テスト（`npm test`）で更新後の構成が検証されていること。
