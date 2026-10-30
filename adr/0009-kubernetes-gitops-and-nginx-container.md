# ADR-0009: Kubernetes GitOps デプロイと Nginx コンテナ基盤

## ステータス
Accepted

## 決定日
2026-10-30

## 対象コンポーネント
- `Dockerfile`
- `deploy/nginx.conf`
- `k8s/manifests/` (`deployment.yaml`, `service.yaml`, `ingress.yaml`)
- `k8s/argocd/application.yaml`
- `.github/workflows/deploy.yml`

## 背景と課題
1. **プライバシーと独自ドメイン**:
   - 初期プロトタイプは GitHub Pages（`*.github.io`）を利用していたが、個人のGitHubアカウント名が外部に露出し、プライバシー上の懸念があった。
   - 独自ドメイン（`ribonclock.iorin.net`）を用いて、インフラ基盤（自宅KubernetesクラスタおよびCloudflare Tunnel）経由で安全に配信する必要が生じた。
2. **Nginx設定（`deploy/nginx.conf`）の必要性**:
   - 公式の `nginx:alpine` デフォルト設定は80番ポートでリッスンするが、Kubernetesのコンテナセキュリティ方針として非特権ポート8080番でのリッスンが求められる。
   - Kubernetesの `livenessProbe`（生存監視）および `readinessProbe`（準備監視）は、アクセスログを汚さない軽量な `/healthz` エンドポイントを必要とする。デフォルトのNginxでは `/healthz` が存在せず404エラーとなり、Podが再起動ループ（CrashLoopBackOff）に陥る。
   - Webアプリはクライアントサイドルーティングを使用していないため、存在しない静的パスに対して `index.html` を返すフォールバックは、入力ミスやリンク切れを隠してしまう。実在する静的ファイル以外は404を返す必要がある。

## 決定事項
1. **Nginxコンテナによる静的ホスティング**:
   - `Dockerfile` を定義し、Alpine Linuxベースの最小Nginxコンテナに `apps/web/` のアセットと `data/generated/schedule.json` を格納する。
2. **専用Nginx設定（`deploy/nginx.conf`）の配置と必須化**:
   - 以下の責務を持つ `deploy/nginx.conf` をコンテナ内にコピーして使用する。
     - ポート `8080` でのリッスン。
     - `/healthz` に対する200 OK（ログ無効化）の即時応答。
     - `try_files $uri $uri/ =404` による静的リソース解決。
3. **GitOps自動デプロイパイプライン（ArgoCD + GitHub Actions）**:
   - GitHub Actions（`.github/workflows/deploy.yml`）にて、コミットごとに不変な Git short SHA（`sha-<sha>`）タグを付与したDockerイメージをビルドし、GHCR（GitHub Container Registry）へプッシュする。
   - ビルド完了後、CIが `k8s/manifests/deployment.yaml` のイメージタグを自動更新してコミット。
   - ArgoCD がリポジトリの変更を検知し、宣言的マニフェストを自動同期（Auto-Sync）してクラスタ内のPodをローリングアップデートする。
4. **Cloudflare Tunnel Ingress による外部公開**:
   - IngressClassName `cloudflare-tunnel` を用い、外部ポート開放なしで `ribonclock.iorin.net` へのHTTPSトラフィックを安全にルーティングする。

## 採用理由と設計根拠
- **高可用性と自動復旧**: ArgoCDによる宣言的管理とKubernetesのセルフヒーリングにより、人手によるSSHログインや手動デプロイ作業を一切排除できる。
- **ヘルスチェックの信頼性**: `deploy/nginx.conf` に `/healthz` を明示的に設けることで、KubernetesがPodの正常稼働を正確に判定できる。
- **公開URLの正確性**: 静的ページ以外を404にすることで、App Store Connectに登録するURLや公開リンクの存在確認をHTTPステータスで検証できる。
- **完全なプライバシー保護**: 個人アカウントURLから完全に切り離された独自ドメインでの運用が確立される。

## 検討した代替案と却下理由
- **デフォルトの Nginx 設定をそのまま使用する**:
  - 却下理由: `/healthz` が存在せずKubernetes Probeが失敗してPodが強制終了されるため運用不可。
- **GitHub Pages でのカスタムドメイン設定（CNAME）のみ**:
  - 却下理由: リポジトリがPublicである限り個人アカウントとの紐付けが完全に隠蔽できない。コンテナ化して独自クラスタ・Cloudflare経由で配信することで完全な分離を達成した。

## 影響範囲とリスク
- **CIビルド時間**: mainブランチへのプッシュごとにDockerイメージビルド（約25秒）とArgoCD同期（約1〜3分）が発生する。
