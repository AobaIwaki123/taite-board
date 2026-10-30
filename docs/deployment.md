# taite-board デプロイ・インフラ設計ガイド (deployment.md)

本ドキュメントは、`taite-board` の本番配信基盤（Kubernetes、ArgoCD、Cloudflare Tunnel、GitHub Actions）のアーキテクチャおよび、コンテナ設定（`deploy/nginx.conf`）の設計意図を解説するドキュメントです。

---

## 1. インフラ全体図

```text
GitHub (main push)
        ↓
GitHub Actions (.github/workflows/deploy.yml)
        ↓ (Docker build & push)
ghcr.io/aobaiwaki123/taite-board:sha-<commit_sha>
        ↓ (GitOps: k8s/manifests/deployment.yaml のイメージタグ自動更新)
ArgoCD (k8s/argocd/application.yaml)
        ↓ (Auto-Sync & Rolling Update)
Kubernetes Cluster (Namespace: ribonclock)
        │
        ├── Pod (Nginx Container: port 8080, /healthz)
        ├── Service (ClusterIP: port 80 -> 8080)
        └── Ingress (ingressClassName: cloudflare-tunnel)
                    ↓
Cloudflare Tunnel
        ↓
公開ドメイン: https://ribonclock.iorin.net/
```

---

## 2. `deploy/nginx.conf` が必要な理由と設定内容

本リポジトリには `deploy/nginx.conf` が配置されています。公式の `nginx:alpine` デフォルト設定をそのまま使用せず、独自の設定ファイルをコンテナへ配置している理由は以下の3点です。

```nginx
map $request_uri $privacy_route {
    default other;
    ~^/(\?.*)?$ document;
    ~^/index\.html(\?.*)?$ document;
    ~^/schedule\.json(\?.*)?$ schedule;
    ~^/mock_schedule\.json(\?.*)?$ schedule;
    ~^/(assets|icons|images)(/|\?|$) asset;
}

log_format privacy_json escape=json
    '{"ts":"$time_iso8601","route":"$privacy_route","status":$status}';

server {
    listen 8080;
    server_name _;

    access_log /dev/stdout privacy_json;

    root /usr/share/nginx/html;
    index index.html;

    location = /healthz {
        access_log off;
        default_type text/plain;
        return 200 "ok\n";
    }

    location = /manifest.webmanifest {
        default_type application/manifest+json;
        try_files $uri =404;
    }

    location / {
        try_files $uri $uri/ =404;
    }
}
```

### 理由1: Kubernetesの非特権ポート要件（Port 8080）
- デフォルトのNginxは特権ポートである80番ポートでリッスンします。
- Kubernetes環境のベストプラクティスおよびコンテナセキュリティ方針（Non-root実行・ポート競合回避）に従い、一般ユーザー権限でもリッスン可能な `8080` 番ポートを明示的に指定しています。
- `k8s/manifests/deployment.yaml` の `containerPort: 8080` と完全同期しています。

### 理由2: Kubernetes ヘルスチェック（Probe）用の `/healthz` エンドポイント
- `deployment.yaml` には以下のProbeが設定されています。
  - **`readinessProbe`** (準備完了監視): Podがトラフィックを受け入れ可能か判定。
  - **`livenessProbe`** (生存監視): コンテナが正常に動作しているか判定。
- デフォルトのNginxでは `/healthz` というファイルは存在しないため、HTTP 404エラーとなります。もしデフォルト設定のまま起動すると、Kubernetesが「Podが死んでいる」と誤認し、**数分おきにコンテナが強制再起動される CrashLoopBackOff に陥ります**。
- `location = /healthz` を設け、`return 200 "ok\n";`（アクセスログ無効化）を返すことで、監視トラフィックでログを埋め尽くすことなく安全なヘルスチェックを実現しています。

### 理由3: 静的ページの存在確認と404
- Webアプリはクライアントサイドルーティングを使用していないため、未知のパスを `index.html` にフォールバックさせません。
- `try_files $uri $uri/ =404;` により、実在する静的ファイルだけを配信し、`privacy.html` のような未定義のパスは404を返します。
- App Store Connectで使用するプライバシーポリシーとサポートページは、次の実在する静的ページです。
  - `https://ribonclock.iorin.net/privacy-policy.html`
  - `https://ribonclock.iorin.net/support.html`
- 将来クライアントサイドルーティングを導入する場合は、ルート定義と404の扱いを追加検討してから設定を変更します。

---

## 3. Dockerfile の設計

```dockerfile
# Stage 1: Build Modern TypeScript Frontend
FROM node:26-alpine AS builder

WORKDIR /app/apps/web
COPY apps/web/package*.json ./
RUN npm ci

COPY apps/web/ ./
RUN npm run build

# Stage 2: Production Nginx Server
FROM nginx:alpine

LABEL org.opencontainers.image.source="https://github.com/AobaIwaki123/taite-board"

COPY deploy/nginx.conf /etc/nginx/conf.d/default.conf
COPY --from=builder /app/apps/web/dist/ /usr/share/nginx/html/
COPY data/generated/schedule.json /usr/share/nginx/html/schedule.json
COPY data/generated/schedule.json /usr/share/nginx/html/mock_schedule.json

EXPOSE 8080
```

- **マルチステージビルド**: Node 26 Alpine で Vite による TypeScript 最適化バンドル（`apps/web/dist/`）を生成し、最小構成の Nginx Alpine コンテナへ成果物のみを配置しています。
- **再現性と独立性**: ローカルの `dist/` や `node_modules` に依存せず、コンテナビルド単独で決定論的な最新静的成果物が生成されます。
- **後方互換性**: 既存クライアントや古いキャッシュへの配慮として、`mock_schedule.json` も同等に配置しています。
- **.dockerignore**: `data/raw`, `data/candidate`, `data/input` などの不要な大容量生データやGitログを除外し、ビルドコンテキストを軽量に維持しています。

---

## 4. Kubernetes マニフェスト構成 (`k8s/`)

- **`k8s/manifests/namespace.yaml`**: `ribonclock` ネームスペースの作成。
- **`k8s/manifests/deployment.yaml`**:
  - レプリカ数: 1
  - リソース制限（Requests / Limits）の明記。
  - Git short SHAによる不変イメージ（`ghcr.io/aobaiwaki123/taite-board:sha-<sha>`）の指定。
  - `/healthz` Probeによる常時監視。
- **`k8s/manifests/service.yaml`**: ClusterIP サービス（Port 80 -> TargetPort 8080）。
- **`k8s/manifests/ingress.yaml`**:
  - `ingressClassName: cloudflare-tunnel`
  - ホスト名: `ribonclock.iorin.net`
- **`k8s/argocd/application.yaml`**:
  - ArgoCD Application 定義。
  - `automated: { prune: true, selfHeal: true }` により、Gitへのコミット検知から数分以内に自動ローリングアップデートを実施。

---

## 5. CI/CD パイプライン (`.github/workflows/deploy.yml`)

1. **トリガー**: `main` ブランチへのプッシュ（ただし `k8s/manifests/**` の更新時は再帰トリガーをスキップ）。
2. **イメージビルド**: コミットハッシュ（`sha-${GITHUB_SHA:0:7}`）でタグ付け。
3. **レジストリプッシュ**: GitHub Container Registry（GHCR）へプッシュ。
4. **GitOps自動コミット**: `k8s/manifests/deployment.yaml` 内のイメージタグを更新し、`[skip ci]` を付けて `main` へコミット・プッシュ。
5. **ArgoCD同期**: クラスタ内のArgoCDが変更を検知し、ゼロダウンタイムでPodを入れ替えます。

---

## 6. プライバシー配慮型アクセス集計

Nginx は `/healthz` を除くリクエストについて、IPアドレス、User-Agent、Referer、Cookie、クエリ文字列を記録せず、時刻・固定ルート分類・HTTPステータスだけを標準出力へ出します。既定の combined access log が併記されないよう、匿名 `access_log` は server コンテキストで設定します。Kubernetes のログは短期間保持し、必要なときだけ次のスクリプトで指定期間を再集計します。

```bash
scripts/report-access-counts.sh \
  --from 2026-10-30T00:00:00+09:00 \
  --to 2026-10-31T00:00:00+09:00 \
  --dry-run
```

`--dry-run` で集計結果を確認した後、`DISCORD_WEBHOOK_URL` を環境変数から注入して Discord へ送信します。通知は時間帯別・分類別・ステータス別の件数を ASCII の `#` 棒グラフで表示し、Webhook へ raw log は送信しません。詳細な保持期間、二重計上防止、将来の管理ダッシュボード拡張は [`privacy-preserving-access-counting.md`](privacy-preserving-access-counting.md) を参照してください。

## 7. トラブルシューティング

### 画面で「オフライン表示中」が表示される場合
- クライアント側で `schedule.json` の取得に失敗し、前回ロードした `localStorage` キャッシュで描画されています。
- ブラウザの開発者ツール（Networkタブ）を開き、`schedule.json` に対するリクエストが 200 を返しているか確認してください。

### コンテナが起動しない場合
- `kubectl describe pod -n ribonclock` を確認し、ImagePullBackOff や Probe失敗がないか確認してください。
- ローカルで `docker run -p 8081:8080 <image>` を実行し、`curl -I http://localhost:8081/healthz` が 200 OK を返すかテストしてください。
