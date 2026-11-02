# キーレス Vertex AI パースとチェックポイント駆動型ストリームパイプライン

## 1. 概要とアーキテクチャ

本ドキュメントでは、Ribbon Clock（タイテ掲示板）におけるデータ収集・選別・AIパース・永続化の完全自動化ストリームパイプラインと、Google Cloud Run / Vertex AI による「**キーレス（Keyless）認証アーキテクチャ**」について記録します。

秘密情報（Gemini API キー）をクライアント環境や Kubernetes クラスタ内に一切保持せず、GCP の IAM サービスアカウントとメタデータサーバー（Application Default Credentials: ADC）を活用して、安全かつ低レイテンシに出番・特典会時刻を抽出します。

また、X の Snowflake ID 特性を利用した「**チェックポイント（since_id）制御**」により、重複取得や無駄なスクロールを完全に排除し、クラスタ内 PVC（`/var/log/ribonclock/`）と Tailscale 経由のダッシュボードへリアルタイムに状態循環を実現しています。

---

## 2. データパイプライン・ストリーム可視化

以下は、X の生ポストが検出されてから、ブラウザクローラー、静的フィルタ、キーレス AI パーサー、そしてダッシュボードへとデータが流れる実態を擬似ストリームとして可視化したものです。

```text
[X / Twitter Service]
   │
   │  Snowflake ID: 2119330614138921102 (2026-11-04 新宿Motion 告知)
   │  Snowflake ID: 2118991204893712011 (一般雑談・画像なし)
   │  Snowflake ID: 2118801948210948194 (チェキ券・物販告知 / タイテ無)
   │  Snowflake ID: 2116002151944401001 (← [CHECKPOINT: latest_since_id])
   ▼
[Step 1: Headless Playwright Crawler (BigInt Since-ID Gating)]
   │
   ├── 2119330614138921102: ID > since_id -> CAPTURED (画像 2枚 保持)
   ├── 2118991204893712011: ID > since_id -> CAPTURED (画像 0枚)
   ├── 2118801948210948194: ID > since_id -> CAPTURED (画像 1枚)
   └── 2116002151944401001: ID <= since_id -> [HALT & SHORT-CIRCUIT]
   │    └─ 過去ポストの重複取得を完全防止（余計なAPI呼出・スクロールを0回で終了）
   ▼
[Step 2: Static Deduplication & Heuristic Gatekeeper (taite filter)]
   │
   ├── [POST: 2118991204893712011]
   │     └─ 判定: SKIP (Rule: media_count == 0) -> [COST DEFENSE: 破棄]
   ├── [POST: 2118801948210948194]
   │     └─ 判定: SKIP (Rule: target_keywords missing) -> [COST DEFENSE: 破棄]
   └── [POST: 2119330614138921102]
         └─ 判定: PASS (出番・特典会・画像2枚検出 / 重複なし) -> Candidate 生成
   ▼
[Step 3: Keyless Cloud Run AI Parser (Vertex AI gemini-2.5-flash-image)]
   │
   │  GCP Compute Metadata Server (http://metadata.google.internal)
   │  └─ Request: /instance/service-accounts/default/token
   │  └─ Response: Bearer ya29.c.b0... (GCP IAM 内部発行 OAuth Token)
   │
   │  POST https://us-central1-aiplatform.googleapis.com/.../gemini-2.5-flash-image:generateContent
   │  Headers: [Authorization: Bearer ya29..., X-Goog-User-Project: taite-board]
   │  Payload: { SystemInstruction: "アイドルタイテ抽出...", Images: [base64, base64] }
   │
   ▼
[AI Vision Inference Output]
   │
   ├── Date: 2026-11-04
   ├── Venue: 新宿Motion
   ├── Live: 19:30 - 19:50 (Confidence: 1.0)
   ├── Benefit: 20:00 - 21:00 (Confidence: 1.0)
   └── Evidence: "新宿Motion 出番19:30〜 / 特典会20:00〜"
   ▼
[Step 4: End-to-End Closed Loop Persistence & Telemetry]
   │
   ├── PVC Sync: /var/log/ribonclock/runs/20261102T.../manifest.json (21件記録)
   ├── State Writeback: /var/log/ribonclock/checkpoint.json
   │     └─ latest_since_id: "2119330614138921102"
   ├── Pipeline State: intakes.json / pipeline_state.json (緑バッジ・即時同期)
   └── Web Dashboard: Next Session "11/04 新宿Motion (あと2日)" 自動繰り上がり完了
```

---

## 3. 実証データ・メトリクス一覧

本パイプラインの手動クローラー実行（`urls.txt` 21件）における実測結果は以下のとおりです。

| パイプライン層 | 入力件数 | 通過件数 | 足切り・抑止 | 処理内容・効果 |
|---|---|---|---|---|
| **Raw Crawl Intake** | 21件 | 21件 | 0件 | X GraphQL レスポンスから生テキスト・画像URL・Snowflake IDを完全収集 |
| **Since ID Gating** | 21件 | 14件 | 7件 | `BigInt(id) <= BigInt(since_id)` により、調査済み過去ポストを早期脱出・遮断 |
| **Filter Pre-Screening** | 14件 | 14件 | 0件 | 本文キーワード判定とメディア（画像）有無判定による多重検査 |
| **Cloud Run AI Parser** | 14件 | 14件 | 0件 | Vertex AI キーレス認証による画像認識・出番特典会時刻の自動抽出 |
| **Dashboard Telemetry** | 14件 | 14件 | 0件 | PVC `/var/log/ribonclock/runs` と自動同期し、リアルタイムファネル画面に即時反映 |

---

## 4. キーレス運用の技術的優位性と設計判断

### 4.1. 秘密情報ゼロの認証（Keyless Architecture）
- **従来の課題**: Google AI Studio 等の API キーを環境変数（`GEMINI_API_KEY`）としてクラスタ内 Secret やコンテナ環境に埋め込む必要があり、トークン漏洩やローテーションの手間が発生していた。
- **解決策**: Cloud Run にバインドした IAM サービスアカウント（`254829169830-compute@developer.gserviceaccount.com`）に `roles/aiplatform.user` を付与。Go サーバーが起動時および実行時に GCP メタデータサーバー（`http://metadata.google.internal/computeMetadata/v1/instance/service-accounts/default/token`）から有効期限付き OAuth トークンを自動取得し、Vertex AI の REST エンドポイントへアクセスする。これにより、ディスクやマニフェスト上に一切の静的秘密鍵が存在しない「**完全キーレス運用**」を実現した。

### 4.2. Snowflake ID の単調増加性によるゼロ重複クローリング
- X のステータス ID は Twitter Snowflake 仕様に準拠しており、上位 41 ビットがミリ秒単位のタイムスタンプ、続くビット群がマシンIDおよびシーケンス番号で構成される単調増加の 64 ビット整数です。
- クローラー側で最新取得ポストの ID を `checkpoint.json` に記録し、次回クロール時に `BigInt(rest_id) <= BigInt(latest_since_id)` を満たした時点でページスクロールを即時打ち切ることで、API の余計な消費・取りこぼし・多重処理を数学的にゼロにします。

---

## 5. 参考文献・一次情報源

- [Google Cloud: Set up Application Default Credentials (ADC)](https://cloud.google.com/docs/authentication/application-default-credentials)
- [Google Cloud: Vertex AI Model Garden REST API](https://cloud.google.com/vertex-ai/docs/reference/rest)
- [Twitter / X Snowflake Specification](https://github.com/twitter-archive/snowflake/tree/snowflake-2010)
- [Kubernetes: Persistent Volumes & Claims Concepts](https://kubernetes.io/docs/concepts/storage/persistent-volumes/)
