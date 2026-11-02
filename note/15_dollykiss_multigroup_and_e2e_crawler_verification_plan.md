# Dolly Kiss マルチグループ拡張とクローラー・AIパース E2E 検証計画書

## 1. 目的と背景

本計画書は、Ribbon Clock（タイテ掲示板）において長らく単一グループ（リボンクロック）特化となっていたデータパイプラインおよび Web アプリケーションを、マルチグループ対応アーキテクチャへと拡張し、新規検証対象グループである「**Dolly Kiss（ドリキス）**」を用いてクローラーおよびキーレス AI パーサーの完全な End-to-End（E2E）実証を行うための設計・作業手順を定めたものです。

本検証の最大の特長は、Dolly Kiss が公式 TimeTree カレンダーを公開している点にあります。クローラーが X から取得した画像・本文から AI が抽出した出番・特典会時刻と、TimeTree の確定スケジュールデータを機械的に突き合わせることで、「**正解データ（Ground Truth）との客観的・自動的な妥当性検証**」が成立します。

また、本プロジェクトではトップページ（ポータル一覧）を作成せず、各グループの独立したサブドメイン（`*.iorin.net`）によるダイレクトアクセス運用を基本方針とします。

---

## 2. 対象グループ仕様とインフラ設計

### 2.1. グループ基本情報

| 項目 | Ribbon Clock（既存・デフォルト） | Dolly Kiss（新規検証対象） |
|---|---|---|
| **グループ識別子 (id)** | `rincent` | `dollykiss` |
| **正式名称** | リボンクロック | Dolly Kiss |
| **略称 (short_name)** | RiNCENT# | ドリキス |
| **X アカウント** | `@Ribbon_Clock` | `@dollykiss_staff` |
| **TimeTree カレンダー** | なし（独自運用） | `lui_17707`（2026年11月: 28件登録済み） |
| **サブドメイン** | `ribonclock.iorin.net` / `rincent.iorin.net` | `dollykiss.iorin.net` |
| **テーマカラー** | Rose / Sky | Pink / Purple |
| **データ配置パス** | `data/curated/schedule.json` | `data/groups/dollykiss/schedule.json` |
| **Since ID チェックポイント** | `/var/log/ribonclock/checkpoint.json` | `/var/log/ribonclock/groups/dollykiss/checkpoint.json` |

### 2.2. サブドメイン方式（`*.iorin.net`）の優位性

1. **Origin（オリジン）レベルでの完全隔離**:
   - ブラウザの LocalStorage、IndexedDB、参戦チェック、タイテ遅延オフセット、経費メモがドメイン単位で完全に独立し、グループ間で混ざりません。
2. **独立 PWA アイコンの実現**:
   - スマホの「ホーム画面に追加」を実行した際、それぞれのグループが個別のネイティブ風 PWA アプリとしてインストールされます。
3. **トップページ不要の直感性**:
   - 各グループのファンは自分の推しグループのサブドメインへ直接アクセスし、ポータル画面を経由せずに最短で直近ライブを確認できます。

---

## 3. 管理画面（モニタリングダッシュボード）への影響と洗練設計

別グループのデータを同時に扱うにあたり、管理画面（`internal/monitor/dashboard.html` / `cmd/monitor`）において以下の課題を解決し、洗練されたマルチグループ管理を実現します。

### 3.1. 課題と解決策

1. **マルチテナントデータの混濁防止**:
   - `Collector` がスキャンする際、`runs` や `intakes` をグループごとにタグ付けし、別々のファネル集計（通過件数・足切り件数）を保持する。
2. **グループ切替ピル（Group Switcher）の新設**:
   - ヘッダー直下に `[すべてのグループ]` / `[リボンクロック]` / `[Dolly Kiss]` の切替ボタンを配置。
   - 選択されたグループに応じて、ファネル統計、コンベア表示、インテーク履歴、手動判定ルールを完全連動させる。
3. **手動インテーク判定のグループ連動**:
   - 画面上から X の URL を貼り付けてテストする際、選択中のグループのプロファイル（キーワード・略称）を用いて正しく判定する。

---

## 4. 実行手順（5つのフェーズ）

### Phase 1: マルチグループ基盤と管理画面の整備
- **タスク**:
  1. グループ設定ファイル `data/groups.json` の作成。
  2. `apps/web/src/config/group.ts` に `dollykiss` 定義を追加（`dollykiss.iorin.net` 対応、カラーテーマ、専用データパス）。
  3. `internal/filter/filter.go` のターゲットキーワードを動的注入可能に改修。
  4. `crawler/index.mjs` に `GROUP=dollykiss` 対応を追加し、保存先とチェックポイントを分離。
  5. `internal/monitor/dashboard.html` および `server.go` にグループ切替セレクターとグループ別集計 API（`GET /api/stats?group=xxx`）を実装。
- **Done Criteria**: `./scripts/verify.sh`（単体テスト・型チェック・ビルド）がオールグリーンであること。

### Phase 2: `@dollykiss_staff` の実クロール実行
- **タスク**:
  1. クローラーを実行し、`@dollykiss_staff` の最新ポスト群を取得。
  2. `data/groups/dollykiss/runs/<run_id>/` 配下に生 JSON とフライヤー画像が保存されることを確認。
  3. `data/groups/dollykiss/checkpoint.json` に最新 `latest_since_id` が初期記録されることを確認。
- **Done Criteria**: ドリキスの告知ポスト群（11月3日の秋葉原サーキットや虎ノ門マリアージュ等）が手元に正常取得されること。

### Phase 3: フィルタ選別 & Cloud Run AI パース
- **タスク**:
  1. `taite filter` を実行し、雑談ポストの足切りとタイテ告知ポスト（画像付き）の Candidate 昇格を確認。
  2. 通過した告知ポストを本番 Cloud Run（`taite-parser`）へ送信。
  3. Vertex AI（`gemini-2.5-flash-image`）がキーレスで画像を認識し、出番・特典会時刻、会場名を抽出して Candidate JSON を生成。
- **Done Criteria**: Candidate JSON 内に出番時刻（例: `14:55`、`18:40`）とエビデンスが正常に出力されること。

### Phase 4: TimeTree 正解データとの手元 AI 自動照合
- **タスク**:
  1. `lumitree` スキルを実行し、TimeTree カレンダー `lui_17707` から確定データを取得。
  2. AI 抽出結果と TimeTree データを機械的に突き合わせる照合スクリプト（`scripts/verify-dollykiss.py`）を実行。
  3. 日付、会場、出番時刻、特典会時刻の一致率（Accuracy）を算出し、レポート出力。
- **Done Criteria**: 機械的な照合レポートが出力され、AI パースの妥当性と精度が客観的スコアとして証明されること。

### Phase 5: `dollykiss.iorin.net` への反映と実画面検証
- **タスク**:
  1. Candidate を `data/groups/dollykiss/schedule.json` へ昇格（`taite promote`）。
  2. `apps/web/public/data/groups/dollykiss/schedule.json` へ同期。
  3. ブラウザで `dollykiss.iorin.net`（ローカルでは `?group=dollykiss`）を表示し、画面確認。
  4. ドリキス専用テーマで次回ライブ（出番・特典会時刻、カウントダウン）が正しく描画されることを確認。
  5. PR を作成してマージ、バージョン管理。
- **Done Criteria**: リボンクロックの画面を一切壊さず、ドリキス専用のタイテ画面が完全に独立して閲覧可能であること。

---

## 5. 検証結果と実証記録（E2E Verification Report）

2026年11月2日、Dolly Kiss を検証対象としたマルチグループ拡張および E2E パイプラインの完全実証が完了しました。

### 5.1. 実績サマリー

| フェーズ | 検証項目 | 結果 | 判定 |
|---|---|---|---|
| **Phase 1** | マルチグループ基盤（`groups.json`、設定、管理画面セレクター） | 実装完了・全単体テスト通過 | PASS |
| **Phase 2** | 完全キーレス・ゼロコストでの公式告知ポスト取得（`taite fetch`） | `auth_token` 不使用・画像付き公式告知2件取得 | PASS |
| **Phase 3** | 静的フィルタ足切り ＆ Cloud Run Vertex AI 推論 | 画像なし引用ポスト除外・出番特典会自動抽出 | PASS |
| **Phase 4** | TimeTree 公式データ（`lui_17707`）との機械的自動照合 | 10/10 フィールド完全一致（一致率 100.0%） | PASS |
| **Phase 5** | `dollykiss.iorin.net` 専用データ配置と PWA レンダリング | ピンク/パープルテーマ・出番カウントダウン描画確認 | PASS |

### 5.2. TimeTree Ground Truth 照合スコア（`verify-dollykiss.py` 実測）

```text
============================================================
Dolly Kiss AI Parse vs TimeTree Ground Truth Verification Report
============================================================
Timestamp: 2026-11-02T20:57:48.473750
Target Group: Dolly Kiss (lui_17707 / @DollyKiss_staff)

Event: ＠JAM×アイドル横丁presents 秋葉原アイドルサーキットvol.6 (Source: 2111019330574127141)
  Status: completed (Model: gemini-2.5-flash-image)
  - Date          : Expected='2026-11-03' | Actual='2026-11-03' -> [MATCH]
  - Venue         : Expected='ANTHEM AKIBA' | Actual='ANTHEM AKIBA' -> [MATCH]
  - Live Start    : Expected='18:40' | Actual='18:40' -> [MATCH]
  - Live End      : Expected='19:00' | Actual='19:00' -> [MATCH]

Event: アイドルマリアージュVol.79 (Source: 2115639770944586018)
  Status: completed (Model: gemini-2.5-flash-image)
  - Date          : Expected='2026-11-03' | Actual='2026-11-03' -> [MATCH]
  - Venue         : Expected='ベルサール虎ノ門' | Actual='ベルサール虎ノ門' -> [MATCH]
  - Live Start    : Expected='14:55' | Actual='14:55' -> [MATCH]
  - Live End      : Expected='15:15' | Actual='15:15' -> [MATCH]
  - Benefit Start : Expected='15:20' | Actual='15:20' -> [MATCH]
  - Benefit End   : Expected='16:20' | Actual='16:20' -> [MATCH]

============================================================
Summary: 10/10 fields matched exactly (100.0% accuracy)
============================================================
```

---

## 6. 参考文献・一次情報源

- [Dolly Kiss 公式 X アカウント (@dollykiss_staff)](https://x.com/dollykiss_staff)
- [Dolly Kiss 公式 TimeTree 公開カレンダー (lui_17707)](https://timetreeapp.com/public_calendars/lui_17707)
- [Google Cloud: Vertex AI Model Garden REST API](https://cloud.google.com/vertex-ai/docs/reference/rest)
- [Twitter / X Snowflake Specification](https://github.com/twitter-archive/snowflake/tree/snowflake-2010)
