# ADR-0021: 管理画面インテークにおける GitHub API 書き戻しパイプラインと GitOps 整合性の担保

## ステータス
Accepted

## 決定日
2026-11-05

## 対象コンポーネント
- `cmd/monitor`
- `internal/monitor`
- `k8s/manifests/base/rollout.yaml`
- `data/groups/`
- `data/events/`

---

## コンテキストと課題 (Context & Problem Statement)

現在、`taite-board` は公式 X 等の告知ポストやタイムテーブル画像から情報を取得し、Cloud Run（Vertex AI Gemini 2.5 Flash）で出番・特典会時刻を抽出するインテーク基盤を備えています。
しかし、管理ダッシュボード（`internal/monitor`）からパース結果を確定し、スケジュールデータへ反映しようとした際に、以下の3重の構造的課題により書き込み拒否（`permission denied`）およびアーキテクチャ上の矛盾が発生していました：

1. **コンテナ権限とファイル所有者の不一致（OSレベルの拒否）**:
   - `deploy/monitor.Dockerfile` では、コンテナセキュリティ標準に基づきプロセスを `USER 65532:65532`（非root）で実行しています。
   - 一方で、コンテナイメージビルド時に配置された `/app/data/` ディレクトリは `root:root` 所有（権限: `drwxr-xr-x`）であり、非rootユーザーからのファイル作成・更新が Linux カーネルによって拒否（`permission denied`）されます。
2. **Pod 内コンテナ間のストレージ分離**:
   - `ribonclock` Pod は、Web（Nginx: `/usr/share/nginx/html/data`）と Monitor（Go: `/app/data`）の2コンテナ構成です。
   - 仮に Monitor 側でローカルファイルに書き込めたとしても、Nginx 側と共有ボリュームになっていないため、Web 画面側には反映されません。
3. **GitOps（ArgoCD）とローカル書き込みの構造的矛盾**:
   - 本システムは Git リポジトリ（`space/taite-board`）を単一の情報源（Single Source of Truth: SSOT）とし、静的 JSON をコンテナにパッケージングして ArgoCD が配備する GitOps 原則を採用しています。
   - コンテナローカルのディスクにデータを書き込むと、Pod のローリングアップデート、再起動、または次回の CI/CD リリース時にデータがすべて破棄（ロールバック）され、Git リポジトリとの乖離が発生します。

---

## 決定要因・評価軸 (Decision Drivers)

1. **GitOps 原則（Single Source of Truth）の維持**:
   - すべてのスケジュール更新およびイベント追加は Git リポジトリに履歴として記録され、ロールバックや監査が容易であること。
2. **Pod 再起動・ローリングアップデートに対する耐障害性**:
   - クラスタのノード切り替えや CI/CD デプロイに伴って登録データが消失しないこと。
3. **過大実装の排除（Architectural Simplicity）**:
   - 独自の RDBMS（PostgreSQL）や分散ストレージの新規立ち上げを行わず、既存の静的 JSON 配信基盤を活かすこと。
4. **Human-in-the-Loop（人間の確証）と即時性の両立**:
   - AI による画像抽出結果を管理画面で人間がプレビュー・微調整し、ワンクリックで本番データへ反映できること。

---

## 検討した選択肢 (Considered Options)

### 選択肢 1: コンテナ内ローカル書き込み + Rook-Ceph PVC 共有ボリューム
- **概要**:
  - `data/` 配下を Rook-Ceph RBD PVC として切り出し、`web` と `monitor` の両コンテナにマウント。`fsGroup: 65532` を指定して権限問題を解決する。
- **メリット (Pros)**:
  - Git コミットや CI ビルドを経由せず、数秒で即時に Nginx に反映される。
- **デメリット (Cons)**:
  - Git リポジトリと本番データの差分が乖離する（GitOps の破綻）。
  - Pod の外で定期的に Git へ同期・エクスポートするバッチが必要となり、コンフリクト解決や二重管理の保守負債が生じる。

### 選択肢 2: 独立した RDBMS / NoSQL データベースの導入
- **概要**:
  - PostgreSQL や Cloud Firestore を導入し、静的 JSON ファイル運用から動的 API サーバーへ全面移行する。
- **メリット (Pros)**:
  - リアルタイムな CRUD 操作とトランザクション管理が可能。
- **デメリット (Cons)**:
  - 「静的配信による極小リソース消費（CPU 10m / Mem 16Mi）と高可用性」という基本強みが喪失する。
  - DB 接続プーリング、マイグレーション管理、バックアップ運用など大幅な過大実装となる。

### 選択肢 3: GitHub REST API による Git リポジトリ書き戻し（採用案）
- **概要**:
  - 管理ダッシュボードで承認された JSON データを、Monitor コンテナが GitHub REST API（Contents API）を叩いて Git リポジトリの `main` ブランチへ直接コミット（または自動 PR 起票）する。
- **メリット (Pros)**:
  - Git が完全な正本（SSOT）として維持され、データの消失リスクがゼロ。
  - 静的 JSON 配信アーキテクチャをそのまま維持でき、新たな DB インフラを一切必要としない。
  - すべてのデータ変更が Git コミットログとして可視化され、変更理由や差分追跡が決定論的に行える。
- **デメリット (Cons)**:
  - GitHub Personal Access Token（PAT）のシークレット管理が必要。
  - 反映までに GitHub API 呼び出し + デプロイ同期のわずかな遅延（数秒〜数十秒）が生じる。

---

## 決定事項 (Decision Outcome)

**選択肢 3（GitHub REST API による Git リポジトリ書き戻し）を採用する。**

### アーキテクチャ仕様

```text
[管理者 iPhone / PC] (Tailscale http://ribonclock-dashboard:8082)
       │
       ├─ 1. タイテ画像アップロード または X 告知 URL 入力
       │
       ▼
[k8s: ribonclock-dashboard (cmd/monitor)]
       │
       ├─ 2. Vertex AI (Cloud Run: taite-parser) による画像/テキスト解析
       │     └─ 出番時刻・特典会時刻・ブース・エビデンスを抽出
       │
       ▼
[管理画面 UI (dashboard.html)]
       │
       ├─ 3. 人間によるプレビュー確認 & 必要に応じた微修正（30秒〜1分）
       ├─ 4. 「本番にコミット反映」ボタンをクリック
       │
       ▼
[Monitor Backend (/api/intake/commit)]
       │
       ├─ 5. k8s Secret (GITHUB_TOKEN) を用いて GitHub REST API を呼び出し
       │     PUT https://api.github.com/repos/AobaIwaki123/taite-board/contents/{path}
       │     - Target: data/groups/{group}/schedule.json
       │     - Target: data/events/{date}/{slug}.json (フェスタイテの場合)
       │     - Commit Message: "feat(data): add session {date} {venue} via admin intake"
       │
       ▼
[GitHub: AobaIwaki123/taite-board (main branch)]
       │
       ├─ 6. コミット確定 & GitOps 反映
       │     - CI ビルド または コンテナ内動的同期による本番即時反映
       │
       ▼
[ユーザー公開画面 (https://ribonclock.iorin.net/)]
```

---

## 影響とトレードオフ (Consequences)

### 正の影響 (Positive)
- **OS権限問題の完全回避**: コンテナ内のローカルファイルシステムへの書き込みを行わないため、UID 65532 による Permission Denied が原理的に発生しない。
- **GitOps の純粋性維持**: 全データが Git にバージョン管理され、誰が・いつ・どの告知から反映したかが Git ログに記録される。
- **データ消失リスクのゼロ化**: Pod の再起動やクラスタ障害が発生しても、Git に書き込まれているためデータは完全に永続化される。

### 負の影響とリスク対策 (Negative & Mitigations)
- **GitHub API レートリミットと認証**:
  - *対策*: アカウント全体で 5,000 req/hour の上限があり、管理操作（月数十回）では到底達しない。トークンは最小権限（`contents: write` のみ）の Fine-grained PAT を発行し、Kubernetes Secret として安全にマウントする。
- **同時書き込み時の SHA コンフリクト (409 Conflict)**:
  - *対策*: GitHub Contents API 呼び出し直前に最新ファイルの `sha` を取得し、Optimistic Locking でコミットを実行する。409 Conflict 発生時は 1 回リトライして最新コミットを取り込む。
