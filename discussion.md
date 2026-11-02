# タイテ・ライブ情報即時把握ボード (taite-board) 進捗管理 (discussion.md)

本ドキュメントは、公式告知や画像から抽出したスケジュール情報を集約・表示する Web アプリケーション（taite-board）の開発目的、ロードマップ、進捗を記録・管理します。

---

## 目的・要件

1. **ライブ・タイテ情報の集約**:
   - 告知画像やテキストに分散する出番時刻、特典会時刻、会場、撮影可否ルールを一元化。
2. **高速表示と地下会場オフライン耐性**:
   - 静的配信JSONと `localStorage` キャッシュに加え、Service Worker によるフライヤー・タイテ画像の Cache-First 保持により、電波の届かない地下会場でも直前のデータを表示。
3. **データ整合性の担保とデータガバナンス**:
   - JSON Schema（Draft-07）による厳格なデータバリデーション、Go CLI による時系列自動整列および同日複数公演のインターバル重複検証。

---

## 開発ロードマップと進捗状況

| Step | 項目 | 内容 | 状況 |
| :--- | :--- | :--- | :--- |
| **Step 1** | **要件定義 & データスキーマ策定** | 必須項目（出番、特典会、会場、チケット、撮影可否）の定義と JSON Schema 化 (ADR-0001) | 完了 |
| **Step 2** | **データパイプライン構築** | 5層構造（input, raw, candidate, curated, generated）の分離と Go CLI（taite）実装 (ADR-0002〜0007) | 完了 |
| **Step 3** | **フロントエンド刷新 (Preact + TSX)** | 宣言的コンポーネント化、Vite + Tailwind CSS、ツリーシェイキング最適化 (ADR-0010, ADR-0013) | 完了 |
| **Step 4** | **Kubernetes / GitOps デプロイ** | マルチステージ Dockerfile、Nginx 設定、ArgoCD 自動同期、Cloudflare Tunnel (ADR-0009) | 完了 |
| **Step 5** | **地下会場オフラインキャッシュ & CLIデータガバナンス** | Service Worker による画像 Cache-First、Go CLI による自動整列・安全なセッション追加・重複検査 (ADR-0014) | 完了 |
| **Step 6** | **クローラー・OCR自動抽出強化** | Tailscale モバイル Intake、k8s ゼロコスト取得、決定論的名寄せ（internal/resolve）、Cloud Run AI パーサー（Gemini 2.5 Flash Vision）の構築 (note/11) | 完了 |
| **Step 7** | **候補確定パイプラインと定期クローラー** | 多段ローカル前処理フィルター (`internal/filter`)、昇格CLI (`taite promote`)、Playwright ヘッドレス巡回 (`crawler/`)、および k8s CronJob 実装 (ADR-0018) | 完了 |
| **Step 8** | **進行状況厳密化 & テレメトリ永続化** | 分単位カウントダウン表示、終了セッションの即時自動送り（graceHours=0）、Ceph RBD PVC によるログ・インテーク完全永続化 (note/12) | 完了 |
| **Step 9** | **GCP コストガバナンス & クリーンアップ** | Artifact Registry 自動クリーンアップポリシー（最新2件保持）、Cloud Run スケール・トゥ・ゼロ、コスト実測スクリプト整備 (note/13) | 完了 |
| **Step 10** | **家計簿・月間収支集計 & 複数グループ展開構想** | チェキ・チケット代の現場簡易家計簿、カレンダー月間・週別収支集計、マルチグループハブおよび推し活コンパニオンアプリ設計策定 (docs/multi-group-hub...) | 完了 |

> **備考**:
> - ADR-0011 に基づく iOS ネイティブ版（SwiftUI）は、Web との二重保守オーバーヘッド防止のため 2026-11-01 に正式凍結し、コード資産（`apps/ios/`）および作業 Worktree を完全撤去しました。クライアント開発は Preact Web PWA へ一本化されています。
> - 参戦チェック（localStorage 永続化）およびスケジュール押し吸収（+5〜+20分オフセット）を実装済みです。
> - 外出先スマホからの X ポスト登録は Tailscale ダッシュボード（`ribonclock-dashboard`）上の Intake フォームからワンタップで実行可能です。
> - セッション進行状況（出番開始/終了、特典会開始/終了）の分単位カウントダウン表示および、終了時の即時自動送り（Auto-Advance）を実装し 18 件の単体テストで保証しています。
> - Kubernetes 上のモニター（`ribonclock-monitor`）は Ceph RBD PVC（1Gi）をマウントし、Pod 再起動後もアクセスログとインテーク履歴が完全復元されます。
> - Artifact Registry（`asia-northeast1`）は最新 2 バージョン（約 14.4MB）のみを自動保持し、Google Cloud 月間 500MB 無料枠（消費率 2.8%）内に完全収容しています。
