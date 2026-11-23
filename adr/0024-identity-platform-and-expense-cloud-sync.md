# ADR-0024: Google Cloud Identity Platform によるユーザーID/パスワード認証と家計簿クラウド同期

## ステータス
Accepted

## 決定日
2026-11-23

## 対象コンポーネント
- `internal/syncapi/`
- `cmd/monitor/main.go`
- `deploy/nginx.conf`
- `k8s/manifests/base/rollout.yaml`
- `apps/web/src/utils/sync.ts`
- `apps/web/src/utils/expense.ts`
- `apps/web/src/utils/attendance.ts`
- `apps/web/src/components/SyncAccountModal.tsx`
- `apps/web/src/components/Header.tsx`
- `apps/web/src/App.tsx`

## コンテキストと課題 (Context & Problem Statement)
`taite-board` の現場メモ・簡易家計簿（`ExpenseMemoCard`）および参戦チェックはこれまで端末の `localStorage` のみに保存されていたため、スマートフォンと PC など複数デバイス間で記録が共有されず、端末を跨ぐと集計が分断される課題があった。

一方で、本システムは Zero Telemetry（未ログインの通常訪問者への追跡・Cookie・UUID 発行を行わない）を基本制約としており、かつ OAuth サーバーの自前構築やコンソール中心の煩雑な OAuth 同意画面運用を避け、`gcloud` / CLI 主体で再現可能にセットアップできる認証方式が求められた。

## 決定要因・評価軸 (Decision Drivers)
- **CLI (`gcloud`) 完結性**: OAuth 同意画面や外部クライアント ID の手動発行を必要とせず、GCP CLI と REST API のみでプロビジョニングできること。
- **全 OS・全ブラウザ互換性**: Apple / Google 間のエコシステム差異に依存せず、どのスマートフォン・PC からでも同一のアカウントで即座に同期できること。
- **Zero Telemetry 原則との両立**: ログイン機能を使わない一般訪問者には一切の外部通信や識別子発行を行わないこと。
- **最小依存**: Go 側・Web 側ともに大型の Firebase/GCP SDK を追加せず軽量に実装すること。

## 検討した選択肢 (Considered Options)
### 選択肢 1: Google Cloud Identity Platform（メール/パスワード基盤＋ユーザーID仮想ドメイン変換）＋ 同一 Pod 分離ポート同期 API
- **概要**: GCP Identity Platform（`identitytoolkit.googleapis.com`）の Email/Password プロバイダを CLI で有効化し、Go バックエンドの公開同期ポート（`:8083`）経由で入力されたユーザーIDへ固定サフィックス（`@users.taite.local`）を付与して登録・認証し、発行された RS256 ID トークンを Google 公開 X.509 証明書でローカル検証したうえで `gs://taite-board-state/users/<sha256(sub)>/sync.json` へ保存する。
- **メリット (Pros)**:
  - `gcloud` と REST API のみで有効化から API キー発行まで完結する（50,000 MAU/月まで無料枠）。
  - Identity Platform の API キーを Kubernetes Secret に閉じ込め、クライアント JS に露出させない。
  - パスワード自動入力（`autoComplete="username"` / `"current-password"`）によりモバイルブラウザでも生体認証自動入力と相性が良く、全 OS/ブラウザで復旧可能。
  - 管理ダッシュボード（`:8082`, Tailscale 限定）とは別ポート（`:8083`）かつ Nginx `/api/sync/` 限定プロキシのため、管理 API が外部へ露出しない。
- **デメリット (Cons)**:
  - 仮想ドメイン補完の ID 形式のため、Identity Platform 標準のパスワードリセットメール送信は使えない（将来のメールアドレス任意紐づけ等で拡張可能）。

### 選択肢 2: WebAuthn パスキー強制
- **概要**: パスワードを廃止し、ブラウザの WebAuthn のみで認証する。
- **メリット (Pros)**: 生体認証 1 タップで入力が不要。
- **デメリット (Cons)**: 異なる OS・エコシステム間での同期や端末紛失時のアカウント復旧に別途リカバリーコード設計が必要となり、WebAuthn 検証ライブラリの依存が増える。

### 選択肢 3: Google OAuth (Sign In With Google)
- **概要**: Google OAuth クライアント ID を用いてワンタップログインを行う。
- **メリット (Pros)**: 既存 Google アカウントをそのまま使える。
- **デメリット (Cons)**: 外部 Web 用 OAuth クライアント ID の作成が `gcloud` 単独では完結せず、Google Cloud Console での手動設定が必要になる。

## 決定事項 (Decision Outcome)
**選択肢 1（Google Cloud Identity Platform ＋ ユーザーID形式 ＋ 同一 Pod 分離ポート `:8083` 同期 API）** を採用する。

1. **GCP / Kubernetes 構成**:
   - `identitytoolkit.googleapis.com` を有効化し、`signIn.email.enabled = true`・`passwordRequired = true` を設定。
   - `identitytoolkit.googleapis.com` に用途限定した API キーを k8s Secret `ribonclock-auth` に格納し、`monitor` コンテナの同期専用ポート（`8083`）へ渡す。
2. **Nginx プロキシとポート分離**:
   - `deploy/nginx.conf` では `/api/sync/` のみを `127.0.0.1:8083` へ中継し、管理用ポート `8082` は Tailscale 専用のまま分離する。
3. **競合解決（マージ）ルール**:
   - 家計簿メモ（`SessionExpense`）はセッション ID 単位で `updatedAt` タイムスタンプを比較し、より新しい記録を採用する（Last-Write-Wins）。
   - 参戦チェックはログイン時の初回マージでは和集合をとり、以降の操作では最新の状態リストをクラウドへ同期する。

## 影響とトレードオフ (Consequences)
### 正の影響 (Positive)
- 既存の `localStorage` データを失うことなく、ユーザー ID とパスワードを入力した別端末へ家計簿・参戦チェックを自動同期できる。
- 未ログインの訪問者には従来の静的配信と完全なゼロテレメトリが維持される。

### 負の影響とリスク (Negative & Mitigations)
- 複数端末から完全に同時刻に同一公演を編集した場合は後勝ちとなるが、個人用途のライブ別メモであるためセッション単位の `updatedAt` 比較で実運用上の衝突を最小化できる。
