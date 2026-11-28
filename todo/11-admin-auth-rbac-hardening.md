# TODO-11: 管理者権限（RBAC）・認証境界・入力防御のハードニング

- **ステータス**: 未着手（セルフレビュー起票）
- **優先度**: 高（セキュリティ境界）
- **目的**: 管理 API・認証 Cookie・SPA 管理画面およびバックエンド入力経路に存在する認可バイパス経路と防御の抜けを解消し、本番構成での安全性を担保する。

## 監査で確認された現状と課題

1. **`:8082` 無認証ポートの露出による RBAC バイパス (`cmd/monitor/main.go`, `k8s/manifests/`)**:
   - `cmd/monitor/main.go` は認証なしハンドラ（`monitor.NewHandlerWithConfig`）を `:8082` で待ち受け、RBAC 付きハンドラ（`syncapi.NewHandler`）を `:8083` で待ち受ける二段構成になっている。
   - `k8s/manifests/base/service.yaml` が `port: 8082` (`dashboard`) を公開しており、`k8s/manifests/tailscale-service.yaml` (`ribonclock-dashboard`) も **`targetPort: 8082` を直接公開** している。このため `:8082` 宛ての経路では `requireAdmin` を通過せず `POST /api/intake/promote`、`POST /api/reconcile`、`POST /api/scrape/run` などの変更系 API へ到達可能。
2. **本番 HTTPS での `Secure` Cookie フラグ欠落リスク (`deploy/nginx.conf`, `internal/syncapi/handler.go`)**:
   - `internal/syncapi/handler.go` の `isSecureRequest` は `r.TLS != nil` または `X-Forwarded-Proto == "https"` を参照して `taite_session` Cookie の `Secure` 属性を決定する。
   - しかし `deploy/nginx.conf` の `/api/sync/` および `/api/admin/` プロキシ設定は `proxy_set_header Host $host;` のみで `X-Forwarded-Proto` を転送しておらず、Pod 内 Nginx → `:8083` 間は平文 HTTP のため `isSecureRequest` が偽と判定されやすい。また、任意クライアントによる `X-Forwarded-Proto` ヘッダの偽装防止も不十分。
3. **Cookie 認証の CSRF 耐性・失効追従・署名鍵生成 (`internal/syncapi/handler.go`)**:
   - `authenticateRequest` は `taite_session` Cookie が有効なら無条件で認証成功とし、`POST /api/admin/*`、`PUT /api/sync/data`、`POST /api/sync/logout` に対して `Origin` / `Referer` / カスタムヘッダ検証などの CSRF 対策を行っていない。
   - `verifySessionCookie` は HMAC 署名と有効期限（30 日）のみを検証するため、Identity Platform 側でアカウントが無効化（`USER_DISABLED`）または削除・パスワード変更された場合でも、Cookie 有効期限内は `ResolveRole` が通る限りセッションが存続する。
   - `getOrInitCookieSecret` は環境変数未設定時に `auth/cookie_secret.json` をロックなしの通常 `PutObject` で生成するため、ローリング更新や Blue/Green 同時起動時に鍵が分岐するおそれがある。
   - クライアント側（`apps/web/src/utils/sync.ts`）では HttpOnly Cookie とは別に `localStorage` (`taite_sync_session`) へ `idToken` と長期 `refreshToken` を平文保持している。
4. **管理 UI ガードの不統一と API 入力検証不足 (`apps/web/src/App.tsx`, `internal/monitor/server.go`)**:
   - `apps/web/src/App.tsx` で `/graph` と `/arch` は `<AdminAuthGate>` 配下にあるが、`/admin` (`<AdminView />`) だけは直接描画されており、未ログイン・非管理者にも画面レイアウトが開いた状態で API 401/403 エラーを出す実装になっている。
   - `internal/monitor/server.go` の `POST /api/scrape/run` はリクエスト JSON の `fixture_path` をそのまま `&scrape.FixtureSource{Path: ...}` に渡してファイル読み込みを行う。また複数の JSON デコード箇所で `io.LimitReader` によるボディサイズ上限が設定されておらず、64Mi メモリ制限環境での過大ペイロード耐性が弱い。

## 検討・実装項目

1. **無認証ポート `:8082` の廃止と全管理経路の RBAC 統一**:
   - `k8s/manifests/tailscale-service.yaml` の `targetPort` を `:8083` へ変更する（または `:8082` リスナー自体を廃止し、`:8083` 経由の統一 RBAC 構成へ統合する。TODO-13 と連携）。
   - Prometheus 用の `:9090` (`/metrics`, `/healthz`) 以外の HTTP 待ち受けに無認証で状態変更 API を露出させない。
2. **`taite_session` Cookie の `Secure` 強制と署名鍵管理の改善**:
   - `deploy/nginx.conf` で `$http_x_forwarded_proto`（または HTTPS 本番前提フラグ env `REQUIRE_SECURE_COOKIE`）を確実に引き回し、ローカル HTTP 開発時を除き `Secure: true` を強制する。
   - `getOrInitCookieSecret` は Secret 環境変数（`SESSION_COOKIE_SECRET`）を必須または推奨とし、GCS 自動生成時のみ `PutObjectIfGeneration(..., "0")` による排他的初期化へ変更する。
   - 重要な認証・管理操作（`POST /api/admin/*`, `PUT /api/sync/data`, `POST /api/sync/*`）に `Origin` / `Referer` 同一オリジン検証ミドルウェアを導入する。
   - セッション TTL・失効方針を見直し、admin 権限操作時のトークン再検証間隔またはセッション短縮（例: admin 権限確認を伴う定期リフレッシュ）を設計する。
3. **入力検証と管理 UI 認証境界の統一**:
   - `internal/monitor/server.go` の全 JSON エンドポイントに `io.LimitReader`（例: 1MiB 上限）を適用し、`fixture_path` は許可ディレクトリ限定または HTTP API から削除（CLI 専用化）する。
   - `apps/web/src/App.tsx` で `/admin` も `<AdminAuthGate>` でラップし、未認証・一般ユーザーに対して管理画面本体や不要な `/api/admin/*` 事前リクエストを実行しない構成に統一する。
   - 認証方式（Cookie 主体運用へ寄せるか `refreshToken` の `localStorage` 保持を限定するか）の整理も併せて検討する。

## 検証条件

- [ ] `:8082` 相当の無認証管理 API 経路が本番マニフェストおよびプロセス待ち受けから排除されていること。
- [ ] `X-Forwarded-Proto: https` 経路および本番設定で `taite_session` Cookie に `HttpOnly; Secure; SameSite=Lax` が付与され、他オリジンからの状態変更 POST/PUT が 403 で拒否されること。
- [ ] `/api/admin/scrape/run` から任意ローカルファイルパス指定ができず、過大 JSON ボディが `413` / `400` で遮断されること。
- [ ] `/admin`, `/graph`, `/arch` のすべてで `<AdminAuthGate>` による共通 RBAC 制御が機能し、単体テスト（`internal/syncapi`, `internal/monitor`）が通過すること。
