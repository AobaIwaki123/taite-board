# ADR-0018: 自宅k8sヘッドレス巡回と多段前処理による告知自動取得パイプライン

## ステータス

Proposed

## 決定日

2026-11-02

## 対象コンポーネント

- `cmd/taite/`
- `internal/crawler/`
- `internal/filter/`
- `internal/source/x/`
- `internal/promote/`
- `k8s/manifests/crawler-cronjob.yaml`
- `data/candidate/`
- `data/curated/`

---

## コンテキストと課題 (Context & Problem Statement)

Ribbon Clock（taite-board）は、ライブ出演・特典会・レギュレーション情報を集約して高速配信するシステムである。
現在、外部ポストからの情報インテークは、Tailscale ダッシュボード（`ribonclock-monitor`）経由で手動投入された URL を Cloud Run 上の AI パーサー（Gemini 2.5 Flash Vision）へ渡す構成（Step 6, note/11）によって実用化されている。

しかし、運用の完全自動化および複数グループ展開（Step 7, Step 10）に向けて、以下の課題が存在する。

1. **タイムライン監視の属人性と検知遅延**:
   - 公式Xアカウントがライブ・タイテ情報を告知した際、人間がスマートフォン等でURLをコピーして Intake フォームへ貼り付けない限り、システムへ情報が取り込まれない。
2. **無差別クローリングに伴うコスト高騰とノイズ混入リスク**:
   - アイドル公式アカウントは、ライブ告知以外にもメンバーの日常リポスト、通販・生誕祭グッズ告知、出演御礼、おやすみツイートなどを大量に投稿する。
   - すべてのポストを無条件に Cloud Run / LLM に流すと、不要な API 費用が発生し、さらに `data/candidate/` に無関係なゴミ候補データが蓄積する。
3. **X (旧Twitter) のクライアントサイドレンダリング (SPA) の壁**:
   - X のタイムラインは静的 HTML を返さず、クライアント側 JavaScript で非同期描画されるため、単純な HTTP GET では投稿一覧を収集できない。
4. **外部スクレイピング SaaS 依存のセキュリティ・運用リスク**:
   - サードパーティの有料スクレイピングサービスや非公式プロキシへの依存は、API遮断、月額費用発生、データ改ざんリスクを伴う。

---

## 決定要因・評価軸 (Decision Drivers)

1. **完全自前・ゼロコスト運用 (Self-Hosted & Zero Marginal Cost)**:
   - 外部有料サービスを利用せず、自宅 Kubernetes クラスタのリソース（遊休 CPU / メモリ）内で完結させること。
2. **コスト・ノイズ完全防御 (Strict Pre-filtering Guardrails)**:
   - Cloud Run / LLM の呼び出し前に、自宅クラスタ内のローカル処理（コスト0円）で90%以上の無関係ポストを確実に足切りすること。
3. **プロトコル堅牢性 (Protocol Stability over DOM Fragility)**:
   - X 側のフロントエンド UI 改修によって頻繁に壊れる DOM セレクタ解析ではなく、ブラウザ内部の通信（GraphQL JSON）をインターセプトして構造化データを安定抽出すること。
4. **人間承認境界の維持 (Human-in-the-Loop Governance)**:
   - クローラーが自動抽出したデータは直接本番 `schedule.json` に上書きせず、`data/candidate/` に隔離保存し、機械検証とワンライナーの昇格コマンド（`taite promote`）で人間が最終確認できる安全性を担保すること。

---

## 検討した選択肢 (Considered Options)

### 選択肢 1: 外部スクレイピング SaaS / 検索 API 連携
- **概要**: Apify, Twitter API v2 (有料), RapidAPI などの外部サービスを利用して定期的にアカウントの新着ポストを取得する。
- **メリット**: 自宅クラスタでのブラウザ管理やプロキシ対策が不要。
- **デメリット**: 月額費用（数千〜数万円）が発生する。外部サービスの仕様変更や停止に依存する。

### 選択肢 2: 自宅 RSSHub / Nitter インスタンスによる RSS 監視
- **概要**: 自宅 k8s に RSSHub 等をセルフホストし、X アカウントの RSS フィードを定期取得する。
- **メリット**: 実装が HTTP ポーリングのみで軽量。
- **デメリット**: X 側のゲストトークン規制やスクレイピング対策により、RSSHub の X ルートは頻繁に壊れるため、継続的なメンテナンス負荷が過大となる。

### 選択肢 3: 自宅 k8s Playwright ヘッドレス巡回 + 3段ローカルフィルター + Cloud Run AI（採用案）
- **概要**:
  - 自宅 k8s 上に軽量 Chromium (Playwright) CronJob を配備し、1日2回（昼・夜）公式アカウントの「メディア」タブへアクセス。
  - ブラウザの通信をインターセプトして新着ポスト ID・画像を抽出。
  - 自宅 Pod 内で 3段フィルター（画像必須・キーワード判定・既存重複排除）を実行。
  - 合致したポストのみを既存の Cloud Run AI パーサーへ送信し、candidate を生成。
  - `taite promote` コマンドで安全に本番へ昇格。
- **メリット**:
  - 完全自前運用で外部依存がゼロ。
  - GraphQL 応答の傍受により DOM 変更に強い。
  - 多段フィルターにより Cloud Run 呼び出しを1日1〜2回（月額数円）に抑制。
- **デメリット**:
  - Chromium のコンテナイメージ（約 300〜400MB）および実行時メモリ（約 200MB）が必要。
  - X 側の IP ブロック対策として巡回頻度を控えめ（1日2〜3回程度）に制限する必要がある。

---

## 決定事項 (Decision Outcome)

選択肢 3 を採用する。

### アーキテクチャ構成

~~~text
[ 自宅 k8s クラスタ (CronJob: taite-crawler) ]
  │
  ├── 1. Playwright Headless Chromium 起動 (1日2回定期実行)
  │      └── 対象: https://x.com/<group_account>/media
  │      └── 通信横取り: GraphQL /UserMedia 応答の JSON を捕捉
  │
  ├── 2. 多段ローカル前処理フィルター (internal/filter, コスト0円)
  │      ├── Stage 1: メディア有無 (画像 1枚以上必須)
  │      ├── Stage 2: ライブ告知キーワード照合 (開場/開演/特典会/タイテ等)
  │      └── Stage 3: 重複排除 (既存 raw/runs, schedule.json との照合)
  │
  └── 3. インテーク送信 (合致した 1〜2件のみ)
         │
         ▼ (HTTP POST / Private Token)
[ Google Cloud Run (taite-parser) ]
  └── Gemini 2.5 Flash Vision による画像・テキスト構造化
  └── 成果物: data/candidate/<run_id>.json を自宅 PVC へ保存

[ 開発者 / 運用者 ]
  └── taite promote <run_id>
  └── スキーマ検証、インターバル重複検証、時系列自動整列、schedule.json への昇格
~~~

---

## 影響とトレードオフ (Consequences)

### 正の影響 (Positive)
1. **完全自律した検知と自動化**:
   - URLの手動コピペが不要となり、公式の最新告知が自動で `candidate` として蓄積される。
2. **コストの極小化**:
   - フィルターにより無関係なツイート（日常、グッズ、御礼等）をローカルで破棄するため、Cloud Run 呼び出しは月間 30〜60 回（月額 2〜5 円程度、無料枠内）に抑えられる。
3. **データ破壊の完全防止**:
   - クローラーは `candidate` までしか生成せず、本番 `schedule.json` への反映には `taite promote` による型チェックと重複検査を必須とするため、AI の誤読による本番事故を完全に防止できる。

### 負の影響と緩和策 (Negative & Mitigations)
1. **X からのアクセス遮断リスク**:
   - *リスク*: 同一IP（自宅グローバルIP）からの高頻度アクセスにより、一時的なレートリミット（HTTP 429）やCloudflare/Xのブロックを受ける可能性がある。
   - *緩和策*: 巡回間隔を1日2〜3回（例: 12:00, 22:00）と極小に保ち、ユーザーエージェントを正規ブラウザと同一に設定。メディアタブ（画像一覧）のみをピンポイントで参照してページ内遷移を最小化する。
2. **リソースフットプリント**:
   - *リスク*: 常時稼働 Pod として Chromium を置くとメモリを常時消費する。
   - *緩和策*: Deployment ではなく **CronJob** として配備し、巡回完了後（数十秒）に Pod が自動消滅するスケール・トゥ・ゼロ構成を採用する。
