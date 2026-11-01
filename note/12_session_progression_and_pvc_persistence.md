# セッション進行状況判定の厳密化と Ceph RBD PVC によるテレメトリ永続化

本ドキュメントは、taite-board における「直近ライブの自動送り（Auto-Advance）および分単位カウントダウン表示」の厳密な仕様定義と、Kubernetes デプロイ（Pod ローアウト）に伴いダッシュボードの集計値が消失していた原因の究明および Rook-Ceph RBD による永続化アーキテクチャを記録します。

---

## 1. 直近ライブ判定と進行状況表示の仕様定義

### 1.1 課題と不整合の背景
これまでの実装では、セッション（出番および特典会）がすべて終了した際に `getSessionProgress()` が「本日終了」を返却していた一方で、直近ライブを特定する `isSessionPast()` の終了猶予時間（`graceHours`）がデフォルトで `2時間` に設定されていました。

この結果、当日の出番・特典会が終了しているにもかかわらず、カード上には「本日終了」というラベルと「★ 直近」というステータスバッジが同時に表示され、同日の別枠公演や翌日以降の予定へ自動的に切り替わらないという認知的不整合が生じていました。

### 1.2 状態遷移と分単位カウントダウンの仕様
ユーザーが会場移動や待機時に「あと何分で始まるか」「あと何分で終わるか」を即座に判断できるよう、進行状況判定ロジック（[apps/web/src/utils/date.ts](file:///Users/aobaiwaki/ai-workspace/space/taite-board/apps/web/src/utils/date.ts)）を刷新し、以下の状態遷移とメッセージ表示を厳密に定義しました。

| フェーズ | 判定条件 | 進行状況ラベル (`message`) | 分単位カウントダウン (`countdownMessage`) |
| :--- | :--- | :--- | :--- |
| **ライブ前** | 現在時刻 < 出番開始時刻 | `開始前` | `出番開始まで あとX分` |
| **ライブ中** | 出番開始時刻 <= 現在時刻 < 出番終了時刻 | `ライブ中` | `出番終了まで あとX分` |
| **待機インターバル** | 出番終了時刻 <= 現在時刻 < 特典会開始時刻 | `特典会待機` | `特典会開始まで あとX分` |
| **特典会中** | 特典会開始時刻 <= 現在時刻 < 特典会終了時刻 | `特典会中` | `特典会終了まで あとX分` |
| **全日程終了** | 現在時刻 >= セッション終了時刻 | `本日終了` | `本日終了` |

※ カウントダウン表示は残り 120 分（2時間）以内で「あとX分」を表示し、2時間以上前の場合は「X時間後」と直感的に整理されます。

### 1.3 直近セッション解決規則（Auto-Advance）
- `graceHours = 0`: セッションの最終予定（特典会終了、または出番終了）の時刻を迎えた瞬間、`isSessionPast(session)` は即座に `true` を返却します。
- `resolveNextSession(sessions)` は、`isSessionPast` が `false` であるセッションのうち最も早い未来のセッションを直近として選択します。
- これにより、1部（昼公演）が終了した瞬間に2部（夜公演）へ自動的にフォーカスが切り替わり、夜公演終了後は翌日以降の直近ライブへスムーズにスライドします。
- 本挙動は [apps/web/src/utils/date.test.ts](file:///Users/aobaiwaki/ai-workspace/space/taite-board/apps/web/src/utils/date.test.ts) における 18 件の単体テストにより検証・保証されています。

---

## 2. ダッシュボード集計値消失の原因と Ceph RBD 永続化

### 2.1 原因究明（Root Cause Analysis）
Kubernetes クラスタ上のモニター Pod（`ribonclock`）において、PR マージや CI/CD デプロイに伴ってダッシュボードのアクセス数（PV）や登録ポスト履歴が消失していた原因は、マニフェスト定義（[k8s/manifests/deployment.yaml](file:///Users/aobaiwaki/ai-workspace/space/taite-board/k8s/manifests/deployment.yaml)）におけるログボリュームの設計不備にありました。

- **従来の構成**: `/var/log/ribonclock` に Kubernetes の `emptyDir: {}` を使用。
- **発生メカニズム**:
  1. `git push` または PR マージにより GitHub Actions（`deploy.yml`）が起動。
  2. 新しいコンテナイメージタグが `deployment.yaml` にコミットされ、ArgoCD がローリングアップデートを実行。
  3. 新規 Pod が起動する際、`emptyDir` は完全に新しい空ディレクトリとして初期化され、旧 Pod の `access.log` およびメモリ内状態が破棄。
  4. モニタープロセスの起動時ログリプレイ（`StartTailer`）が空ファイルを読み取るため、累計アクセス数および直近リクエストが 0 件にリセットされていた。

### 2.2 永続化アーキテクチャの刷新
Rook-Ceph による分散ブロックストレージ（Ceph RBD）を活用し、Pod の再起動や再デプロイを跨いでログとインテーク履歴を完全永続化する構成へ移行しました。

1. **PersistentVolumeClaim の新設 ([k8s/manifests/pvc.yaml](file:///Users/aobaiwaki/ai-workspace/space/taite-board/k8s/manifests/pvc.yaml))**:
   - `storageClassName: ceph-rbd`
   - 容量: `1Gi`
   - アクセスモード: `ReadWriteOnce` (RWO)
2. **デプロイ戦略の Recreate 指定**:
   - Ceph RBD はブロックデバイスであるため、複数ノードからの同時アタッチを防止する必要があります。
   - `spec.strategy.type: Recreate` を設定し、旧 Pod のボリュームアンマウントが完了した後に新 Pod を起動することで安全なマウントを保証。
3. **デュアル永続化（`access.log` + `intakes.json`）**:
   - Nginx アクセスログ: `/var/log/ribonclock/access.log`
   - Tailscale インテーク登録データ: `/var/log/ribonclock/intakes.json`
   - モニターコレクター（[internal/monitor/collector.go](file:///Users/aobaiwaki/ai-workspace/space/taite-board/internal/monitor/collector.go)）は起動時に `access.log` をオフセット 0 から走査して PV・ステータス分布・時間別推移を完全復元し、`intakes.json` から過去に登録された X ポスト一覧をリロードします。

---

## 3. 実機クラスタでの永続性検証

Kubernetes クラスタ（Worker ノード: `k8s-worker-2`）における動作検証を実施しました。

1. **初期リクエストの生成**:
   - Web コンテナ（ポート 8080）へリクエストを送信し、`/var/log/ribonclock/access.log` にログが追記されることを確認。
   - ダッシュボード API（`/api/stats`）にて `total_requests: 1`, `total_page_views: 1` を記録。
2. **強制 Pod 削除とフェイルオーバー**:
   - `kubectl delete pod -n ribonclock <pod-name>` を実行し、Pod を強制再作成。
3. **データ復元の確認**:
   - 新規 Pod 起動直後（`uptime_seconds: 5`）に `/api/stats` を取得した結果、`total_requests: 1`, `total_page_views: 1`, および直近リクエスト履歴（`22:17:01, route: document, status: 200`）が消失せずに維持されていることを実証。

---

## 4. 参考文献・公的技術仕様

- [Kubernetes Persistent Volumes - Official Documentation](https://kubernetes.io/docs/concepts/storage/persistent-volumes/)
- [Rook Ceph Block Storage (RBD) Configuration](https://rook.io/docs/rook/latest/Storage-Configuration/Block-Storage-RBD/block-storage/)
- [W3C Page Visibility Level 2 Recommendation](https://www.w3.org/TR/page-visibility-2/)
- [RFC 8259: The JavaScript Object Notation (JSON) Data Interchange Format](https://datatracker.ietf.org/doc/html/rfc8259)
