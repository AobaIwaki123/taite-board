# 管理画面画像アップロードと GitHub API 書き戻しによる GitOps 自動インテークパイプライン設計書

## 1. 概要と背景課題

### 1.1 課題認識
アイドル現場におけるタイムテーブル情報は、主催者公式 X（旧 Twitter）から「画像チラシ」として告知されることがほぼ 100% であり、テキスト化された標準 API は存在しません。
手作業によるデータ入力は、20〜30組のタイムテーブル1本につき 20〜30 分の時間を要し、月間（15〜25公演）で 7〜10 時間ものメンテナンスコストが発生します。

これを解決するため、Cloud Run 上の Vertex AI（Gemini 2.5 Flash）を用いた画像認識抽出と、Tailscale 経由の管理ダッシュボード（`internal/monitor`）を開発しました。
しかし、管理画面から抽出結果を保存（Promote）しようとした際、**OS レベルの書き込み拒否（`permission denied`）** が発生し、本番データへの書き戻しが遮断されていました。

### 1.2 書き込み拒否の技術的要因（Root Cause）
1. **プロセス実行権限とファイル所有権の不一致**:
   - セキュリティ標準として、Monitor コンテナは `USER 65532:65532`（非root）で実行されています。
   - 一方で、コンテナイメージ生成時に配置された `/app/data/` ディレクトリは `root:root`（`drwxr-xr-x`）所有であり、非rootプロセスからの新規ファイル作成・既存ファイル変更が OS カーネルによって遮断されました。
2. **Pod 内コンテナ間のストレージ分離**:
   - `web` コンテナ（Nginx: 静的配信）と `monitor` コンテナ（Go: 管理・監視）のディスクは分離されており、仮に `monitor` 側でファイルを変更できても `web` 側には届きません。
3. **GitOps（ArgoCD）とローカルディスク書き込みの構造的矛盾**:
   - コンテナローカルのディスクに書き込んだデータは、Pod の再起動や新バージョンのデプロイ（イメージ入れ替え）によって即座にロールバック・破棄されます。

本設計は、これらの課題を根本から解消するため、「**管理画面で確定したデータを、バックエンドが GitHub REST API を通じて Git リポジトリへ直接コミットする（GitOps 書き戻し）**」アーキテクチャを定義します。

---

## 2. システムアーキテクチャ

### 2.1 エンドツーエンド処理フロー

```text
[管理者ブラウザ (iPhone / PC)] (http://ribonclock-dashboard:8082/)
       │
       ├─ Step 1: タイムテーブル画像（チラシ）をドラッグ＆ドロップ（または X URL 入力）
       │
       ▼
[k8s: ribonclock-dashboard (cmd/monitor)]
       │
       ├─ Step 2: Cloud Run AI Parser (taite-parser) を実行
       │          - Vertex AI Gemini 2.5 Flash による画像解析
       │          - 各組の出番時刻、特典会時刻、ステージ、ブース、エビデンスを抽出
       │
       ▼
[管理画面プレビュー UI (dashboard.html)]
       │
       ├─ Step 3: Human-in-the-Loop 検証
       │          - 抽出されたタイテのプレビュー表示
       │          - 誤字や時間のズレがあればフォーム上で 30秒〜1分 で修正
       │
       ├─ Step 4: 「本番反映（Git Commit）」ボタンをクリック
       │
       ▼
[Monitor Backend API (/api/intake/commit)]
       │
       ├─ Step 5: GitHub Contents API 呼び出し
       │          - k8s Secret (GITHUB_TOKEN) を使用
       │          - 対象ファイルの最新 SHA を取得（楽観的ロック）
       │          - Base64 エンコードした JSON を PUT
       │          - Commit: "feat(data): add session {date} via admin intake"
       │
       ▼
[GitHub Repository (AobaIwaki123/taite-board)]
       │
       ├─ Step 6: main ブランチへ安全にコミット確定
       │          - Git ログに完全な変更履歴が残る
       │
       ▼
[ArgoCD / Production Rollout]
       │
       └─ Step 7: 最新データが本番環境へ安全に反映
```

---

## 3. GitHub API クライアント詳細仕様

### 3.1 使用エンドポイント
GitHub REST API v3 の [Create or update file contents](https://docs.github.com/en/rest/repos/contents#create-or-update-file-contents) を使用します。

```http
PUT /repos/{owner}/{repo}/contents/{path}
Authorization: Bearer {GITHUB_TOKEN}
Accept: application/vnd.github+json
X-GitHub-Api-Version: 2022-11-28

{
  "message": "feat(data): add session 2026-11-05 via admin intake",
  "content": "{Base64-encoded-JSON-content}",
  "sha": "{latest-file-blob-sha}",
  "branch": "main"
}
```

### 3.2 楽観的ロック（Optimistic Concurrency Control）
1. 書き込み前に `GET /repos/{owner}/{repo}/contents/{path}` を実行し、既存ファイルの `sha` を取得。
2. 取得した `sha` を PUT リクエストに含めて送信。
3. 他のコミットと競合して `409 Conflict` が返却された場合は、最新 `sha` を再取得して自動リトライ（最大2回）。
4. 新規ファイル（合同フェスタイテ `data/events/{date}/{slug}.json` 等）作成時は `sha` を空で送信。

### 3.3 Go バックエンド実装インターフェース (`internal/github`)

```go
package github

import (
	"context"
	"encoding/base64"
	"fmt"
	"net/http"
)

type Client struct {
	token      string
	owner      string
	repo       string
	httpClient *http.Client
}

func NewClient(token, owner, repo string) *Client {
	return &Client{
		token:      token,
		owner:      owner,
		repo:       repo,
		httpClient: &http.Client{Timeout: 15 * time.Second},
	}
}

// CommitFile commits a file directly to the specified branch in GitHub.
func (c *Client) CommitFile(ctx context.Context, branch, filePath, commitMessage string, fileContent []byte) error {
	// 1. 最新のファイルSHAを取得（存在する場合）
	sha, err := c.getFileSHA(ctx, branch, filePath)
	if err != nil && !isNotFound(err) {
		return fmt.Errorf("failed to get existing file sha: %w", err)
	}

	// 2. Base64 エンコード
	encoded := base64.StdEncoding.EncodeToString(fileContent)

	// 3. PUT リクエスト作成 & 実行
	return c.putContent(ctx, branch, filePath, commitMessage, encoded, sha)
}
```

---

## 4. セキュリティと認証ガバナンス

1. **最小権限の Fine-grained Personal Access Token**:
   - アカウント全体の広範な権限（AdminやUser情報）は一切付与せず、対象リポジトリ `AobaIwaki123/taite-board` の `Contents: Read and write` のみに限定した Fine-grained PAT を発行します。
2. **Kubernetes Secret 注入**:
   - トークンはコードやマニフェストに直接記述せず、Kubernetes Secret（`taite-board-github-token`）としてクラスタ内に登録します。
   - `rollout.yaml` の `monitor` コンテナ環境変数 `GITHUB_TOKEN` として安全にバインドします。

```yaml
env:
  - name: GITHUB_TOKEN
    valueFrom:
      secretKeyRef:
        name: taite-board-github-token
        key: token
```

---

## 5. 期待される効果と運用フロー

1. **データ化作業工数の劇的削減**:
   - 従来: 20〜30分 / 本（手動タイピング・確認）
   - 本設計導入後: **2〜3分 / 本**（画像アップロード → AI 自動パース → 差分確認・コミットボタン押下）
2. **GitOps 原則の完全維持**:
   - コンテナローカルの権限問題に悩まされることなく、Git が唯一の正本として機能し続けるため、Pod の再起動やクラスタ移行があってもデータが消失しません。
3. **人間による確証（Human-in-the-Loop）**:
   - AI による誤認識やタイテの直前変更があっても、コミット前に管理画面上で目視修正できるため、本番サイトに誤った情報が流出するのを確実に防ぎます。

---

## 6. 参考文献・公的技術仕様

- [GitHub REST API: Repositories Contents Documentation](https://docs.github.com/en/rest/repos/contents)
- [OpenGitOps: GitOps Principles v1.0.0](https://opengitops.dev/)
- [Google Cloud: Vertex AI Multimodal Prompting Guide](https://cloud.google.com/vertex-ai/generative-ai/docs/multimodal/overview)
- [RFC 8259: The JavaScript Object Notation (JSON) Data Interchange Format](https://datatracker.ietf.org/doc/html/rfc8259)
