# ADR-0015: 軽量セマンティックバージョニングとリリースタグ運用

## ステータス

Accepted

## 決定日

2026-11-01

## 対象コンポーネント

- `VERSION`
- `apps/web/package.json`
- `apps/web/vite.config.ts`
- `apps/web/src/` (`App.tsx`, `Navigation.tsx`, `vite-env.d.ts`)
- `cmd/taite/` (`main.go`, `internal/version/`)
- `scripts/release.sh`
- `.github/workflows/deploy.yml`
- `Dockerfile`

---

## コンテキストと課題 (Context & Problem Statement)

Ribbon Clock (taite-board) は、コミット SHA（`sha-<hash>`）を元にした継続的デプロイ（GitHub Actions + ArgoCD GitOps）を採用しているが、これまで Git タグ（`vX.Y.Z`）の発行規約および正式なバージョン管理が存在しなかった。
そのため、以下の運用課題が生じていた。

1. **バージョン認識の欠如**:
   - Web アプリを利用するユーザーや開発者が、現在動作しているアプリケーションがどのリリースバージョンであるかを画面上から把握できない。
2. **リリース追跡の困難さ**:
   - Git リポジトリ内に release tag が存在しないため、特定の機能や修正がどの段階でリリースされたかのトレーサビリティがコミット履歴の探索に依存している。
3. **運用負荷のトレードオフ**:
   - 重量級のリリースマネージャー（`semantic-release` や `release-please` 等）は、GitHub Actions の追加権限設定、ボットによる自動 PR の管理、PR-Only ワークフローとの競合など、個人〜小規模開発におけるメンテナンス摩擦が大きい。
   - 一方で完全手動での `git tag` 発行は、`package.json` とのバージョン乖離やタグ名の打ち間違い、検証漏れのリスクがある。

「運用が軽いこと（摩擦ゼロ・ワンコマンド）」を最上位の制約条件とし、機械的かつ安全にセマンティックバージョニング（SemVer）を運用する仕組みが必要である。

---

## 決定要因・評価軸 (Decision Drivers)

1. **運用の軽さ（Minimal Operational Overhead）**:
   - 複雑な設定ファイルや外部 SaaS に依存せず、標準的な CLI / スクリプト（`./scripts/release.sh [patch|minor|major]`）1発で検証・採番・同期・タグ発行・GitHub Release が完結すること。
2. **単一の真実（Single Source of Truth）**:
   - バージョン番号の二重管理を防止し、リポジトリ直下の `VERSION` および `apps/web/package.json` を常に完全同期させること。
3. **UI非侵食性と透過性（Zero UI Regression & Transparency）**:
   - アプリの最下部に控えめな書式（`v1.0.0`）でバージョンを常時表示し、画面のレイアウト、スクロール動作、カルーセル遷移に一切悪影響を与えないこと。
4. **GitOps・コンテナ基盤との親和性**:
   - Git タグの発行が GitHub Actions（`deploy.yml`）に連動し、GHCR のコンテナイメージにも同一バージョンタグ（`vX.Y.Z`）を自動付与できること。

---

## 検討した選択肢 (Considered Options)

### 選択肢 1: 完全自動ボット方式 (semantic-release / release-please)
- **概要**: GitHub Actions 上でコミットメッセージを解析し、ボットが Release PR を起票・自動タグ付けを行う。
- **メリット**:
  - コミット規則が守られていれば人手による発行コマンドすら不要。
- **デメリット**:
  - GITHUB_TOKEN の書き込み権限拡張、Branch Protection ルールとの衝突回避、マージ待ち PR の滞留など保守コストが高い。
  - Worktree による高速なローカル開発・手動検証サイクルと相性が悪い。

### 選択肢 2: 完全手動 git tag 運用
- **概要**: リリースしたいタイミングで開発者が `git tag v1.0.0` を直接実行する。
- **メリット**:
  - 追加ツールが不要。
- **デメリット**:
  - `package.json` のバージョン更新忘れ、タイポ、SemVer 規約違反が発生しやすい。
  - Web UI に表示されるバージョンと Git タグが不整合を起こす。

### 選択肢 3: 単一真実ファイル + 軽量リリーススクリプト + UIビルド時インジェクション（採用案）
- **概要**:
  - ルートの `VERSION` および `apps/web/package.json` をバージョンの正本とし、`./scripts/release.sh [patch|minor|major]` で機械的に更新・検証・タグ打ち・GitHub Release 発行を実行。
  - Vite の `define` を通じて `__APP_VERSION__` をフロントエンドへ埋め込み、アプリ最下部に `vX.Y.Z` を表示。
  - Go CLI に `taite version` サブコマンドを追加。
- **メリット**:
  - 外部依存ゼロ、追加ライブラリゼロ。
  - 開発者が `./scripts/release.sh patch` または `minor` を叩くだけで、SemVer 計算、全テスト検証、コミット、タグ打ち、GitHub Release まで自動完結。
  - `--dry-run` オプションにより、本番コミット前に変更予定を安全に事前確認可能。
  - UI 側のバージョン表示がビルド成果物と 100% 同期。
- **デメリット**:
  - リリース実行のトリガーを人（または開発エージェント）が明示的に実行する必要がある（ただし「意図しない勝手なリリース」を防ぐ意味では利点となる）。

---

## 決定事項 (Decision Outcome)

**選択肢 3 を採用する。**

### 1. セマンティックバージョニング方針
- 形式: `MAJOR.MINOR.PATCH`（Git タグは `vMAJOR.MINOR.PATCH`）
  - **`patch`**: バグ修正、文言調整、ドキュメント更新、内部リファクタリング、軽微なパフォーマンス改善（例: `1.0.0` -> `1.0.1`）。
  - **`minor`**: 新機能の追加、新しい画面やタブの追加、CLI サブコマンドの追加、スキーマの下位互換な拡張（例: `1.0.0` -> `1.1.0`）。
  - **`major`**: 既存 API・データ契約の破壊的変更、システム全体の抜本的再設計（例: `1.0.0` -> `2.0.0`）。

### 2. 単一真実ファイルと同期
- `VERSION`: リポジトリ直下のプレーンテキストファイル（改行付きのバージョン文字列 `1.0.0`）。
- `apps/web/package.json`: `"version": "1.0.0"`。
- `scripts/release.sh` が SemVer 計算時に両者を一括更新。
- `Dockerfile` に `COPY VERSION /app/VERSION` を追加し、Docker ビルド環境でも整合性を保証。

### 3. リリーススクリプト仕様 (`scripts/release.sh`)
- コマンド書式:
  ```bash
  ./scripts/release.sh [patch|minor|major|<specific-version>] [--dry-run]
  ```
- 処理フロー:
  1. `VERSION` から現在バージョンを読み取り。
  2. 指定された増分（`patch` / `minor` / `major`）に基づき新バージョンを計算。
  3. Git ワークツリーが clean かつ `main` ブランチであることを確認。
  4. `--dry-run` の場合は更新予定を表示して終了。
  5. `VERSION` と `apps/web/package.json` を更新。
  6. `./scripts/verify.sh` を実行して回帰テスト・ビルドが通ることを検証。
  7. バージョン更新コミットを作成（`chore(release): bump version to vX.Y.Z`）。
  8. 注釈付き Git タグ（`git tag -a vX.Y.Z -m "Release vX.Y.Z"`）を作成。
  9. `origin main` およびタグを Push。
  10. `gh` コマンドが利用可能な場合、`gh release create vX.Y.Z --generate-notes` で GitHub Release を起票。

### 4. アプリ最下部のバージョン表示仕様
- `apps/web/vite.config.ts`:
  - `define: { __APP_VERSION__: JSON.stringify('v' + version) }`
- `apps/web/src/vite-env.d.ts`:
  - `declare const __APP_VERSION__: string;`
- `apps/web/src/components/Navigation.tsx` / `App.tsx`:
  - アプリ最下部のナビゲーションバー直下に、目立たないモノスペースフォント（`text-[10px] text-stone-600 font-mono tracking-wider text-center select-none`）で `vX.Y.Z` を表示。
  - 横スクロールやボタン押下に干渉しない静的要素として配置。

### 5. Go CLI のバージョン表示 (`taite version`)
- `taite version` または `taite -v` を実行した際、リポジトリの `VERSION` を読み取って `taite vX.Y.Z` を標準出力に出力。

### 6. GitHub Actions 連動 (`.github/workflows/deploy.yml`)
- タグプッシュ時（`tags: ['v*']`）もビルド・デプロイパイプラインを発火させ、GHCR イメージにコミット SHA だけでなく `vX.Y.Z` のイメージタグを付与。

---

## 影響とトレードオフ (Consequences)

### 正の影響 (Positive)
- 運用者は `./scripts/release.sh patch` または `minor` を叩くだけで、SemVer の採番から検証、タグ打ち、GitHub Release 作成まで完全自動化される。
- Web 画面上から現在稼働中の正確なバージョンが即座に確認でき、キャッシュ残りやデプロイ反映のトラブルシューティングが容易になる。
- Git タグ、GitHub Releases、GHCR コンテナイメージタグが SemVer で一元化され、トレーサビリティが大幅に向上する。

### 負の影響とリスク (Negative & Mitigations)
- アプリ最下部にバージョン表示行が追加されるため、縦方向の領域（約16px）を消費する。
  - **対策**: `text-[10px]` の極小フォントとタイトなパディングを採用し、iPhone SE 等の小画面デバイスでもカルーセルの高さを圧迫しないよう設計。
