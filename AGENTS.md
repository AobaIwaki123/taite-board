# Ribbon Clock (taite-board) 開発規約 (AGENTS.md)

本ドキュメントは、Ribbon Clock（taite-board）の開発・運用において、AIエージェントおよび開発者が遵守すべきアーキテクチャ方針、コーディング規約、データ管理プロトコル、およびPR運用ルールを定めます。

---

## 1. システム概要と非機能要件

### 1.1 システム概要
Ribbon Clock は、アイドルグループ「リボンクロック」のライブ出演情報およびタイムテーブルを集約し、静的配信JSONとモバイルWebクライアントを通じて提供するシステムです。

### 1.2 非機能要件・技術的制約
1. **完全なプライバシー保護 (Zero Telemetry)**:
   - ユーザーのトラッキング、Cookie、アクセス解析用外部タグ、識別用UUIDの発行を行わない。
2. **オフライン耐性**:
   - スケジュールデータ取得後は `localStorage` にキャッシュし、次回以降およびオフライン時にも同一データを即座に復元・表示する。
3. **データ整合性の検証**:
   - `schema/schedule.schema.json`（Draft-07）に完全準拠し、スキーマ違反や時刻逆転などの矛盾データはビルド・配信パイプラインで自動検知して配信を遮断する。

---

## 2. ディレクトリ構成と責務分離

ADR-0007 および ADR-0010 に基づき、各ディレクトリの責務を厳格に分離しています。

```text
space/taite-board/
├── apps/
│   └── web/              # フロントエンド正本 (Preact + TypeScript + Tailwind CSS)
├── cmd/
│   └── taite/            # データパイプライン統合 Go CLI
├── data/                 # 5層データパイプライン (data/README.md 参照)
│   ├── input/            # クローラー巡回対象定義
│   ├── raw/              # 不変生スナップショット (runs/<run-id>/)
│   ├── candidate/        # 自動抽出中間データ
│   ├── curated/          # 人間管理マスター (entities.json, overrides.json)
│   └── generated/        # 公開配信確定成果物 (schedule.json)
├── deploy/
│   └── nginx.conf        # Kubernetes Nginx 設定 (port 8080, /healthz)
├── docs/                 # 開発・アーキテクチャドキュメント (ビルド成果物の混入禁止)
├── internal/             # Go CLI 内部実装パッケージ
├── k8s/                  # Kubernetes / GitOps (ArgoCD) マニフェスト
├── schema/
│   └── schedule.schema.json # スケジュールデータ標準スキーマ (Draft-07)
├── scripts/              # 開発・ビルドスクリプト
├── adr/                  # アーキテクチャ決定記録 (adr/README.md 参照)
├── Makefile              # 開発タスク定義
└── Dockerfile            # マルチステージコンテナビルド定義
```

> **禁止事項**: `docs/` に HTML、JS、CSS、画像などのビルド生成物をコミットすることは禁止します。生成物は `apps/web/dist/` にのみ出力され、Docker マルチステージビルドを通じて本番コンテナへ配置されます。

---

## 3. Git & PR 運用ルール (PR-Only Workflow)

本リポジトリは、保護された `main` ブランチへの直接Pushを禁止し、**PR-Only Workflow** を適用します。

1. **ブランチ命名規則**:
   - `feat/<topic>`, `fix/<topic>`, `refactor/<topic>`, `docs/<topic>`
2. **事前ローカル検証**:
   - PR起票前に必ず以下の検証コマンドを実行し、エラーがゼロであることを確認します。
   - `make test`: Go テスト
   - `make validate`: スケジュールJSONスキーマ検証
   - `npm --prefix apps/web run typecheck`: TypeScript 型検査
   - `npm --prefix apps/web run build`: フロントエンドバンドルビルド
3. **PR起票とマージ**:
   - `gh pr create` でPRを作成し、GitHub Actions の通過を確認後、`gh pr merge --squash --delete-branch` でマージします。

---

## 4. ドキュメント・文章規約

1. **客観的事実の記述**:
   - 個人の主観、情緒的な比喩、誇張表現、宣伝文句（「超低摩擦」「過酷な環境」「0.5秒で把握」「計器盤」「コックピット」など）を排除し、システムの仕様・技術的根拠・事実のみを論理的に記述します。
2. **絵文字の完全不使用**:
   - コミットメッセージ、PR本文、ドキュメント、コードコメントにおいて絵文字を使用しません。
3. **チャット本文への生 Mermaid 出力の禁止**:
   - チャット返答本文に生の Mermaid コードブロックを出力せず、テキストや表で要点を説明します。複雑な図は `docs/*.md` 内に記述し、ファイルリンクを提示します。
4. **Markdown 強調表示の構文崩れ防止**:
   - 日本語全角文字や鉤括弧（「」）の直前・直後に太字タグ `**` を密着させず、鉤括弧の内側を囲む（`「**重要**」`）か、前後に半角スペースを空けます。

---

## 5. ADR (Architecture Decision Record) 規約

新規 ADR 作成時は [`adr/README.md`](adr/README.md) の標準テンプレートに従い、以下の項目を網羅します。
1. **Title**: 簡潔な決定事項の要約
2. **Status**: `Proposed`, `Accepted`, `Rejected`, `Deprecated`, `Superseded by ADR-XXXX`
3. **Context & Problem Statement**: 発生していた技術的制約・課題（客観的事実ベース）
4. **Decision Drivers**: 判断基準・評価軸
5. **Considered Options**: 検討した各選択肢とそれぞれのトレードオフ（Pros & Cons）
6. **Decision Outcome**: 採択理由
7. **Consequences**: 決定に伴う正の影響（Positive）および負の影響・リスク（Negative & Mitigations）

---

## 6. フロントエンド設計規約 (`apps/web`)

詳細仕様は [`docs/frontend-guidelines.md`](docs/frontend-guidelines.md) を参照してください。

1. **宣言的 UI の徹底**:
   - `document.getElementById` や `innerHTML` による命令型 DOM 操作を禁止します。
   - コンポーネントは Preact による純粋な宣言的 TSX として実装し、状態変更に応じた仮想 DOM の自動差分更新を活用します。
2. **厳格な型安全性**:
   - `any` 型の使用を禁止します。
   - `schema/schedule.schema.json` から導出された `src/types/schedule.ts` を唯一の正本とします。
3. **グローバルスコープ汚染の禁止**:
   - `window` オブジェクトへの関数注入を禁止し、コンポーネントツリー内の props やイベントハンドラで完全に閉じます。
4. **アイコンのツリーシェイキング**:
   - Lucide アイコンの一括インポートを禁止し、使用するシンボルのみを個別インポートしてバンドルサイズを最小化します。

---

## 7. CLI コマンド標準 (`taite`)

- 実行バイナリの手動パス指定（`./bin/taite`）を廃止し、`make install` または `go install ./cmd/taite` によりインストールされた `taite` コマンド、あるいは `make validate` / `go run ./cmd/taite` を標準として使用します。
