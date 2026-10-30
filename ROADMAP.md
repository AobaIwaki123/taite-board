# taite-board リファクタリングロードマップ

本ドキュメントは、[docs/architecture.md](docs/architecture.md) および各 ADR（[adr/](adr/)）で合意されたアーキテクチャ方針に基づき、各段階の実装進捗を追跡・記録するための管理シートです。

---

## ADR実装ステータス一覧

| ADR 番号 | タイトル | 主な実装スコープ | ステータス | 完了内容 / 実装場所 |
|---|---|---|---|---|
| **ADR-0001** | **Web・ネイティブ共通のschedule.jsonデータ契約とスキーマ管理** | `schema/schedule.schema.json` 策定、バリデータ照合 | 完了 | `schema/schedule.schema.json`, `internal/validate` |
| **ADR-0002** | **生データ（Raw Snapshot）の保存構造と不変性の保証** | `data/raw/runs/<run-id>/` 保存、マニフェスト生成 | 完了 | `internal/raw`, `internal/fetch`, `cmd/taite` |
| **ADR-0003** | **canonical_id とエンティティ名寄せ辞書の設計** | `data/curated/entities.json` 辞書 | 完了 | `data/curated/entities.json` |
| **ADR-0004** | **自動抽出・名寄せと手動キュレーション（Overrides）の境界** | `data/curated/overrides.json` による手動補正 | 完了 | `data/curated/overrides.json` |
| **ADR-0005** | **静的JSON配信の継続方針とAPIサーバー化の条件** | `data/generated/schedule.json` 配信、Docker/Nginx同期 | 完了 | `data/generated/schedule.json`, `Dockerfile` |
| **ADR-0006** | **外部取得アダプター（Source Adapter）の責務分離** | `internal/source/x` 隔離、oEmbed/Syndication統合 | 完了 | `internal/source/x/x.go` |
| **ADR-0007** | **ソースコード・ビルド生成物のディレクトリ配置再編** | `apps/web/` 集約、`cmd/taite` 統合、二重管理廃止 | 完了 | `apps/web/`, `cmd/taite/`, `scripts/build.sh` |
| **ADR-0008** | **クライアント側のキャッシュとデータ鮮度（Freshness）の表示** | `apps/web/` での `generated_at` 検証・オフライン表示 | 完了 | `apps/web/index.html` (オフラインバッジ＆localStorage) |

---

## 実装ステップと依存関係

### Step 1: データ契約とスキーマの固定 (ADR-0001)
- [x] `schema/schedule.schema.json`（JSON Schema Draft-07）を新規作成。
- [x] 現在の `data/generated/schedule.json` を検証し、スキーマ適合を確認。
- [x] スキーマ検証用テストおよびCLIコマンド（`taite validate`）を実装。

### Step 2: ソースコード・データ配置の再編 (ADR-0007, ADR-0005)
- [x] `apps/web/` ディレクトリを作成し、Webクライアントソースを集約。
- [x] `docs/` と `web/` の重複を解消するビルドスクリプト（`scripts/build.sh`）を整備。
- [x] `data/` 配下を `input/`, `raw/`, `candidate/`, `curated/`, `generated/` へ再編。
- [x] `data/generated/schedule.json` を生成・配置し、Web UIがこれを参照するように更新。
- [x] `Dockerfile` を新ディレクトリ構造（`apps/web/`, `data/generated/schedule.json`）に対応。

### Step 3: キュレーション層と名寄せ辞書の導入 (ADR-0003, ADR-0004)
- [x] `data/curated/entities.json`（グループ・メンバー・会場・チケット別名辞書）を定義。
- [x] `data/curated/overrides.json`（画像内タイテや手動確定情報の上書き用）を定義。
- [x] 手動補正と名寄せを反映した確定データ構造を固定。

### Step 4: クライアント側の鮮度検証とオフラインキャッシュ (ADR-0008)
- [x] `apps/web/index.html` にて `generated_at` のパースと24時間鮮度判定を実装。
- [x] オフライン時／通信失敗時の `localStorage` キャッシュフォールバックと控えめなインジケーター（`#freshness-indicator`）を実装。

### Step 5: Go統合CLI (`cmd/taite`) とアダプター分離 (ADR-0006, ADR-0002)
- [x] `cmd/taite` にサブコマンド（`fetch`, `validate`, `build`）を統合。
- [x] `internal/source/x` を作成し、外部通信処理（oEmbed/Syndication）を隔離。
- [x] `taite fetch` コマンドで `data/raw/runs/<run-id>/` に不変スナップショットを保存するパイプラインを実装。
- [x] `taite validate` コマンドで `schema/schedule.schema.json` によるスキーマ検証をネイティブ実行。
- [x] `taite build` コマンドでアセットと確定JSONの同期を実行可能に。

---

## 完了の判定基準 (Definition of Done)
1. すべてのADR（0001〜0008）のステータスが「完了」となっていること。
2. `web/` と `docs/` のHTML二重管理が解消され、`apps/web/` が正本ソースになっていること。
3. `cmd/taite` によるデータパイプライン（fetch -> publish -> validate）がローカルで動作すること。
4. DockerコンテナビルドおよびKubernetes環境での配信（`ribonclock.iorin.net`）が正常に動作すること。
5. Webクライアントがオフライン時にも安全にキャッシュを表示でき、空白画面や崩れが発生しないこと。
