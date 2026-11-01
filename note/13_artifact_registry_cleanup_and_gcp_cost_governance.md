# Artifact Registry クリーンアップポリシーと GCP コストガバナンス

本ドキュメントは、taite-board プロジェクトにおいて Google Cloud Artifact Registry のストレージ課金を恒久的に最小化（無料枠内に完全収容）するための自動クリーンアップポリシー設定と、Cloud Run を含めたリアルタイムコスト計測手法を記録します。

---

## 1. コスト構造と課題認識

### 1.1 Artifact Registry の課金仕様
Google Cloud Artifact Registry における課金要素は主に以下の通りです。

- **ストレージ容量**:
  - 東京リージョン（`asia-northeast1`）の単価: `$0.10 / GB・月`
  - **無料枠（Always Free）**: プロジェクト全体で毎月 **0.5 GB (500 MB)** まで無料。
- **ネットワーク転送（Egress）**:
  - 同一リージョンの Cloud Run（`asia-northeast1`）へのイメージ Pull は **無料（$0.00）**。
- **脆弱性スキャン（Container Analysis）**:
  - 自動スキャンが有効な場合、イメージ 1 件につき `$0.26` のスキャン課金が発生するリスクがある（本リポジトリでは API を無効化して完全抑止）。

### 1.2 課題（イメージ肥大化リスク）
CI/CD パイプライン（GitHub Actions）や手動ビルドでコミット毎にコンテナをプッシュし続けると、古いイメージ（中間レイヤーや未タグリビジョン）が累積し、無料枠（500 MB）を超過して毎月不要なストレージ課金が発生します。

---

## 2. 自動クリーンアップポリシー（最新 2 件のみ保持）の設計と適用

### 2.1 ポリシー仕様（[deploy/cleanup-policy.json](file:///Users/aobaiwaki/ai-workspace/space/taite-board/deploy/cleanup-policy.json)）
Artifact Registry のライフサイクル管理機能（Cleanup Policies）を利用し、リポジトリ単位で「最新 2 件を保護し、それ以前の古いバージョンを日次で自動削除」するルールを定義しました。

```json
[
  {
    "name": "keep-latest-2-versions",
    "action": {
      "type": "Keep"
    },
    "mostRecentVersions": {
      "keepCount": 2
    }
  },
  {
    "name": "delete-older-versions",
    "action": {
      "type": "Delete"
    },
    "condition": {
      "tagState": "any",
      "olderThan": "1d"
    }
  }
]
```

- **`keep-latest-2-versions` (KEEP)**:
  - 最も新しい 2 バージョン（`keepCount: 2`）を無条件で保護。
  - 現在本番稼働中のリビジョンおよび 1 つ前のロールバック用リビジョンを確保。
- **`delete-older-versions` (DELETE)**:
  - 作成から 1 日（`86400s`）以上経過したタグ付き・未タグ（`tagState: any`）の古いイメージを Google Cloud 側で自動削除。

### 2.2 本番リポジトリへの適用
以下のコマンドにより、東京リージョンのリポジトリ `taite` に適用済みです。

```bash
gcloud artifacts repositories set-cleanup-policies taite \
  --location=asia-northeast1 \
  --project=taite-board \
  --policy=deploy/cleanup-policy.json \
  --no-dry-run
```

---

## 3. コスト計測とリソース監視

### 3.1 実測値（現状のストレージ使用量）
- **コンテナイメージ 1 件のサイズ**: 約 `7.19 MB`（Alpine ベースの軽量 Go バイナリ）
- **リポジトリ総容量**: `7.189 MB`（1 バージョン保持時）
- **2 バージョン保持時の想定総容量**: 約 `14.4 MB`
- **無料枠（500 MB）に対する消費率**: **約 2.88%**（485 MB 以上の余剰枠あり）
- **月額推定コスト**: **$0.00 / 月**（完全無料枠内）

### 3.2 リアルタイム監視スクリプト（[scripts/check-gcp-cost.sh](file:///Users/aobaiwaki/ai-workspace/space/taite-board/scripts/check-gcp-cost.sh)）
いつでも手元から GCP の使用量とコストを 1 コマンドで計測・検証できるスクリプトを整備しました。

```bash
./scripts/check-gcp-cost.sh
```

出力例:
```
==========================================================
 Google Cloud Cost & Resource Status Check
 Project: taite-board | Region: asia-northeast1
==========================================================

--- [1] Artifact Registry: taite ---
Repository URI:       asia-northeast1-docker.pkg.dev/taite-board/taite
Current Stored Size:  7.189MB
Free Tier Allowance:  500.00 MB / month (0.5 GB across project)
Free Tier Usage:      1.44% of monthly allowance
Estimated Cost:       $0.00 / month (Fully covered by Free Tier)

Active Cleanup Policies:
      action: DELETE
        olderThan: 86400s
      id: delete-older-versions
      action: KEEP
      id: keep-latest-2-versions
        keepCount: 2

--- [2] Cloud Run: taite-parser ---
Endpoint:             https://taite-parser-3b7eomko6a-an.a.run.app
Min Instances:        0 (Scale-to-zero active: $0 when idle)
Max Instances:        3 (Budget protection upper bound)
Resources per Pod:    CPU: 1000m, Memory: 512Mi
Free Tier Allowance:  2,000,000 requests/month, 360,000 GB-seconds/month
Idle Cost:            $0.00
```

---

## 4. 参考文献・公的技術仕様

- [Google Cloud Artifact Registry Pricing](https://cloud.google.com/artifact-registry/pricing)
- [Google Cloud Artifact Registry: Configure cleanup policies](https://cloud.google.com/artifact-registry/docs/repositories/cleanup-policy)
- [Google Cloud Run Pricing](https://cloud.google.com/run/pricing)
- [Google Cloud Free Program: Always Free usage limits](https://cloud.google.com/free/docs/free-cloud-features)
