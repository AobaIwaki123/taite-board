# Ribbon Clock (taite-board) エージェント規約 (AGENTS.md)

本ドキュメントは、ライブ情報・タイムテーブル即時把握ボード「Ribbon Clock（taite-board）」の開発・運用において、AIエージェントおよび開発者が遵守すべきアーキテクチャ方針、コーディング規約、データ管理プロトコル、およびPR運用ルールを定めます。

---

## 1. プロジェクト概要と基本理念

### 1.1 目的
Ribbon Clock は、複数の公式アカウント、告知画像、チケットプラットフォームに分散するアイドル・アーティストの出演スケジュールやタイムテーブル（タイテ）を、ファンが現場・移動中に一瞬で確認・把握できるように最適化されたモバイルファーストのWebダッシュボードです。

### 1.2 コアバリュー
1. **即時把握性（Glanceability）**: アプリを開いた瞬間に「今どこで誰が何をしているか」「次のライブは何時か」が判読できること。
2. **完全なプライバシーとゼロテレメトリ**: ユーザーの行動追跡、Cookie、広告トラッカー、UUID発行を一切行わず、完全な静的キャッシュとローカル状態のみで動作すること。
3. **オフライン耐性**: 会場の地下や通信制限下でも、直前に取得したスケジュールデータが `localStorage` から即座に展開されること。
4. **データ整合性の保証**: 終了時刻が開演時刻より前になるなどの矛盾データを排除し、スキーマバリデーションを通過した確定JSONのみを配信すること。

---

## 2. ディレクトリ構成と責務分離

ADR-0007 に基づき、ソースコード、データパイプライン、成果物の責務は以下のように分離されています。

```text
space/taite-board/
├── apps/
│   └── web/              # フロントエンド正本 (TypeScript + Vite + Tailwind CSS)
│       ├── src/          # TypeScript ソースコード
│       ├── public/       # 静的アセット (アイコン、画像等)
│       ├── index.html    # SPA エントリーポイント
│       ├── package.json  # パッケージ定義とスクリプト
│       ├── tsconfig.json # TypeScript 設定 (Strict Mode)
│       └── vite.config.ts# Vite バンドラ設定
├── cmd/
│   └── taite/            # データパイプライン統合 Go CLI
│       └── main.go       # fetch / extract / validate / build サブコマンド
├── data/                 # 5層データパイプライン
│   ├── input/            # クローラー巡回対象 (sources.json, input_urls.txt)
│   ├── raw/              # 不変生スナップショット (runs/<run-id>/)
│   ├── candidate/        # 自動抽出・OCR候補中間データ
│   ├── curated/          # 人間管理マスター (entities.json, overrides.json)
│   └── generated/        # 公開配信確定成果物 (schedule.json)
├── deploy/
│   └── nginx.conf        # Kubernetes Nginx コンテナ用設定 (port 8080, /healthz)
├── docs/                 # GitHub Pages 公開用成果物同期ディレクトリ
├── internal/             # Go CLI 内部実装パッケージ
│   ├── fetch/            # 一次データ取得
│   ├── source/           # 外部ソースアダプター
│   └── validate/         # スキーマ・整合性検証
├── k8s/                  # Kubernetes / GitOps (ArgoCD) マニフェスト
├── schema/
│   └── schedule.schema.json # スケジュールデータ標準スキーマ (Draft-07)
├── scripts/              # 開発・ビルド補助スクリプト
│   └── build.sh          # フロントエンドビルドと成果物同期スクリプト
├── adr/                  # アーキテクチャ決定記録 (0001-...形式)
└── note/                 # 技術調査・仕様メモ
```

> **注意**: `web/` ディレクトリは初期MVPの遺産であり廃止されました。クライアント側の改修は必ず `apps/web/` に対して行います。

---

## 3. Git & 変更管理プロトコル (PR-Only Workflow)

本リポジトリ（外部公開プロダクト `taite-board`）では、保護された `main` ブランチへの直接Pushは厳格に禁止されており、以下のPR運用を徹底します。

1. **Feature ブランチの作成**:
   - `feat/<topic>`, `fix/<topic>`, `refactor/<topic>` の形式でブランチを分岐します。
2. **完全なローカル検証**:
   - コミット前に必ず TypeScript の型検査（`npm run typecheck` または `npm run build`）、Go の検証（`go test ./...`）、およびスキーマ検証（`./bin/taite validate`）をパスさせます。
3. **PR起票とCI確認**:
   - GitHub CLI (`gh pr create`) を用いてPRを作成します。
   - PR本文には「背景・目的」「変更内容」「検証手順（コマンドと結果）」を簡潔・論理的に記載します。
4. **マージ**:
   - CI（GitHub Actions）の通過を確認後、`gh pr merge --squash --delete-branch` でマージします。

---

## 4. ドキュメント・コミュニケーション規約

1. **絵文字の完全不使用**:
   - コミットメッセージ、PR本文、コード内コメント、ドキュメント（README、ADR、note等）において、原則として絵文字を使用しません。
2. **チャット本文への生 Mermaid 出力の禁止**:
   - ターミナルでの可読性を損なうため、チャット返答本文に生の Mermaid コードブロックを出力せず、テキストや表で要点を説明します。
   - 構造図や状態遷移図が必要な場合は `docs/*.md` や `note/*.md` 内に記述し、ファイルリンクを提示します。
3. **Mermaid 構文エラーの防止**:
   - ドキュメント内で Mermaid を記述する際は、ノード名やエッジ文字列を必ずダブルクォート（`"..."`）で囲みます。
4. **Markdown 強調表示の構文崩れ防止**:
   - 日本語全角文字や鉤括弧（「」）の直前・直後に太字タグ `**` を密着させず、鉤括弧の内側を囲む（`「**重要**」`）か、半角スペースを空けます。

---

## 5. フロントエンド設計・コーディング規約 (`apps/web`)

1. **TypeScript ファースト**:
   - すべてのUIロジックは TypeScript で記述し、`any` 型の使用を禁止します。
   - スケジュールデータの型は `schema/schedule.schema.json` から正確に導出された `src/types/schedule.ts` を使用します。
2. **App-Shell とスクロール分離**:
   - モバイルビューでのスワイプ切り替えや縦スクロールの干渉を防ぐため、親コンテナは `h-[100dvh] overflow-hidden` を維持し、各タブスライド（直近コックピット、全日程リスト、月間カレンダー）が個別に `overflow-y-auto` で独立スクロールする設計を厳守します。
3. **DOM操作とコンポーネント分割**:
   - 単一の巨大なスクリプトファイルにすべてを詰め込まず、関心事ごとにモジュールを分離します（`state.ts`, `cockpit.ts`, `scheduleList.ts`, `calendar.ts`, `carousel.ts`, `lightbox.ts`）。
4. **外部CDN依存の最小化**:
   - オフライン時やCDN障害時にも動作を担保するため、主要なロジックやバンドルはローカル成果物に包含させます。

---

## 6. データパイプライン規約 (`data/` & `cmd/taite`)

1. **生データ不変の原則（ADR-0002）**:
   - `data/raw/` 配下のスナップショットは追記のみとし、過去データを上書き・改ざんしません。
2. **名寄せ・補正の分離（ADR-0003, ADR-0004）**:
   - 表記揺れ（例: 「神楽あゆ」「あゆちゃん」）の解決は `data/curated/entities.json` に委ね、プログラムコード内に個別文字列のハードコードを行いません。
   - 自動抽出の誤り修正は `data/curated/overrides.json` を通じて宣言的に適用します。
3. **スキーマバリデーション必須（ADR-0001）**:
   - `data/generated/schedule.json` を更新する際は、必ず `taite validate` または JSON Schema バリデータを通し、エラーがゼロであることを確認します。
