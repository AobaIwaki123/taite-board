# ADR-0014: 画像オフラインキャッシュとGo CLIデータガバナンス

## ステータス

Proposed

## 決定日

2026-11-01

## 対象コンポーネント

- `apps/web/public/sw.js`
- `apps/web/src/main.tsx`
- `cmd/taite/`
- `internal/schedule/`
- `internal/validate/`
- `adr/0001-schedule-json-data-contract.md`
- `adr/0008-client-offline-cache-and-freshness.md`

---

## コンテキストと課題 (Context & Problem Statement)

Ribbon Clock (taite-board) は、地下ライブハウスや電波の不安定なライブ会場周辺において、次の出番時刻・特典会・会場・レギュレーションを迅速に確認できることを主目的としている。
現行システムは `localStorage` による `schedule.json` のオフライン復元（ADR-0008）を実装しているが、以下の2つの機能的・運用的な課題が存在する。

1. **地下ライブハウスにおける画像不達リスク (Media Offline Failure)**:
   - 新宿Zirco Tokyo、Spotify O-nest、渋谷RING などの地下会場や混雑環境では、携帯電話回線が完全に圏外となるケースが頻発する。
   - `schedule.json` のテキスト情報は `localStorage` から復元できるものの、タイムテーブルやフライヤーの画像実体（Twitter / X のメディア `pbs.twimg.com/media/...`）はブラウザの標準HTTPキャッシュに依存しており、通信遮断時には画像の読み込みに失敗し、タイテ画像の確認が不能となる。
   - UIの表示崩れやジェスチャー競合（UIデグレ）を一切招くことなく、ネットワーク層で画像を自動永続化する仕組みが必要である。

2. **手作業によるスケジュール編集とデータ不整合リスク (Data Governance Gaps)**:
   - スケジュールの追加やメンテナンスにおいて、手動での JSON 編集や ID 採番（`YYYYMMDD-1` 等）を行うと、日付順序の狂い、ID重複、必須フィールド欠落の人為的ミスが発生しやすい。
   - また、同日に2回公演（昼夜・回し）がある場合、1部の特典会終了時刻と2部の出番時刻が物理的に重複していたり、移動インターバルが不足している矛盾データが混入しても、現在の `taite validate` ではこれを機械的に警告できない。

---

## 決定要因・評価軸 (Decision Drivers)

1. **UI非侵食性 (Zero UI Regressions)**:
   - 既存の TSX コンポーネント（CockpitView, ListView, CalendarView）のレイアウト、スタイル、DOM 構造を1行も改変せず、ネットワーク層（ServiceWorker）および CLI ツール層のみで課題を解決すること。
2. **KISS / 依存フットプリント最小化 (Keep It Simple, Stupid)**:
   - 巨大な Workbox などの重量級 PWA ライブラリを導入せず、標準 Web API（ServiceWorker + Cache Storage）の最小限の実装（50行程度）で完結させること。
3. **機械的整合性防御 (Automated Guardrails)**:
   - ソート順序、ID自動採番、複数公演の時間重複チェックを Go CLI（`taite`）に統合し、手作業ゼロで完全なスキーマ整合性と物理的時間整合性を担保すること。

---

## 検討した選択肢 (Considered Options)

### 選択肢 1: 重量級 PWA プラグイン (vite-plugin-pwa + Workbox)
- **概要**: Vite の PWA プラグインを導入し、アセット一式を自動キャッシュするマニフェストと ServiceWorker を生成する。
- **メリット**:
  - 設定ファイルによる宣言的キャッシュルールの定義が可能。
- **デメリット**:
  - `workbox-window` などの外部依存が数百KB追加され、ビルドパイプラインが複雑化する。
  - バージョン更新時のキャッシュ破棄制御がブラックボックス化し、オフライン時の stale 制御でトラブルが起きやすい。

### 選択肢 2: 単機能 Vanilla Service Worker による画像 Cache-First + Go CLI データガバナンス（採用案）
- **概要**:
  - Web 側: `public/sw.js`（プレーン JS）を配置し、`main.tsx` の末尾で登録。Twitter 画像ドメイン（`pbs.twimg.com`）へのリクエストのみを Cache Storage（`ribonclock-images-v1`）へ Cache-First で保存・返却する。
  - CLI 側: `taite sort`, `taite session add`, `taite validate`（インターバル・順序検査）を Go 標準ライブラリで実装。
- **メリット**:
  - 外部依存が完全にゼロ（追加ライブラリ 0 件）。
  - UI コードや HTML 構造への影響が皆無で、UI デグレのリスクが完全にゼロ。
  - キャッシュ対象が「一度閲覧した告知・タイテ画像」に限定されるため、ストレージを浪費しない。
  - CLI による機械的ソートと重複検査により、手動作業のミスが根絶される。
- **デメリット**:
  - ServiceWorker のライフサイクル管理（スコープ、更新制御）を自前で把握する必要がある。

---

## 決定事項 (Decision Outcome)

**選択肢 2 を採用する。**

### 1. 画像オフラインキャッシュの仕様 (`apps/web/public/sw.js`)
- **キャッシュスコープ**:
  - URL が `https://pbs.twimg.com/media/` で始まる画像リクエストを捕捉。
- **キャッシュ戦略 (Cache-First with Network Fallback)**:
  1. `caches.match(request)` でローカルキャッシュを確認。存在すれば即座に返却（0ms）。
  2. キャッシュにない場合、ネットワークへフェッチし、ステータス 200 であれば `caches.open('ribonclock-images-v1')` にクローンを保存した上でクライアントに返却。
  3. ネットワーク不達かつキャッシュにもない場合は標準のエラーを返却。
- **登録処理**:
  - `apps/web/src/main.tsx` の最下部にて、`'serviceWorker' in navigator` の存在確認の上、非同期で登録（UI初期化を阻害しない）。

### 2. Go CLI データガバナンス機能の仕様 (`cmd/taite`, `internal/`)

1. **`taite sort` サブコマンド**:
   - `sessions[]` を `date`（昇順） → `live.start_time`（昇順） → `id`（昇順）で機械的にソート。
   - `data/generated/schedule.json` を更新し、自動で Web 公開ディレクトリへ同期。
2. **`taite session add` サブコマンド**:
   - 引数: `-date YYYY-MM-DD`, `-title <title>`, `-venue <name>`, `-live-start <HH:MM>`, `-live-end <HH:MM>`, `-benefit-type <type>`, `-benefit-start <HH:MM>`, `-benefit-end <HH:MM>`, `-booth <booth>`, `-ticket-platform <platform>`, `-ticket-url <url>`
   - 自動ID採番: 同一日の既存セッション数から `YYYYMMDD-1`, `YYYYMMDD-2` を自動導出。
   - 自動ソート & 同期: 追加後、自動で日付順に整列して保存・同期。
3. **`taite validate` の時間整合性・順序防御拡張**:
   - **順序検査**: `sessions[]` が日付・出番時刻順に整列されているかを検証（狂いがあれば警告またはエラー）。
   - **回し・重複インターバル検査**: 同一日に2公演以上ある場合、前のセッションの `benefit.end_time` と次のセッションの `live.start_time` を比較。
     - 時刻が逆転（重複）している場合は `✕ Error: session %s benefit end overlaps with session %s live start` を出力。
     - インターバルが 30 分未満の場合は `⚠ Warning: tight interval (%d min) between session %s and %s` を出力。

---

## セルフレビュー結果 (Self-Review against Design Rules)

1. **オーバーエンジニアリングの排除 (YAGNI / KISS)**:
   - npm 依存（PWA ライブラリ等）の追加は 0 件。ServiceWorker はプレーンな単一ファイルで完結。
   - CLI の追加機能も Go 標準ライブラリ（`sort`, `time`, `flag`, `encoding/json`）のみで実装し、新規 Go モジュールの追加は行わない。
2. **UI非侵食性の担保**:
   - UI コンポーネント（TSX / CSS）の修正はゼロ。既存の画面レイアウト・操作性に1ピクセルのデグレも発生させない。
3. **可観測性と検証可能性**:
   - `make verify` パイプライン内で `taite validate` による順序・インターバル検証が自動実行されるため、CI 上でデータ破損が未然に防止される。
