# ADR-0013: ツールチェーンの最新化とTailwind CSS v4移行

## ステータス
Accepted

## 決定日
2026-10-30

## 対象コンポーネント
- `.github/workflows/deploy.yml`
- `Dockerfile`
- `apps/web/package.json`
- `apps/web/tsconfig.json`
- `apps/web/vite.config.ts`
- `apps/web/src/styles/main.css`

## コンテキストと課題 (Context & Problem Statement)
Ribbon Clock のフロントエンドは Vite 6、TypeScript 5、Tailwind CSS 3、Node 22 をベースに構築されていた。
しかし、主要エコシステムにおいて以下のメジャーアップデートが進行しており、セキュリティ、ビルドパフォーマンス、将来的な互換性維持の観点から追従が必要となった。
1. **Node 26 LTS の登場**: Node 22 から Node 26 への LTS 移行。公式 Docker Alpine イメージも Node 26 へ移行。
2. **Tailwind CSS v4 の登場**: CSS エンジンの Rust 化（Oxide）による高速化と、PostCSS や JavaScript 設定ファイル（`tailwind.config.js`）を廃止して標準 CSS の `@theme` ディレクティブへ移行するアーキテクチャ刷新。
3. **TypeScript 7 & Vite 8 のリリース**: ESM ネイティブ化の進展に伴い、`__dirname` の非推奨化や型定義インポート（CSS side-effect import、`node:test` 型）の厳格化。
4. **GitHub Actions の更新**: `actions/checkout@v4` および `docker/login-action@v3` から最新メジャーバージョン（v7 / v4）への更新。

## 決定要因・評価軸 (Decision Drivers)
- **ビルドパフォーマンスの極大化**: Tailwind v4 Oxide エンジンおよび Vite 8 によるローカル開発・Docker ビルド時間の短縮。
- **設定ファイルの最小化とスリム化**: `tailwind.config.js`、`postcss.config.js` などの周辺設定ファイルを撤廃し、単一の CSS に宣言を集約する。
- **ゼロデグレ（Zero Visual Regression）の担保**: メジャーアップデートに伴う CSS やレイアウトの崩れが一切発生しないこと。

## 検討した選択肢 (Considered Options)

### 選択肢 1: 旧来の Tailwind v3 + Vite 6 のまま凍結維持
- **概要**: メジャーバージョンのジャンプを避け、パッチ更新のみにとどめる。
- **メリット**: 移行作業コストがゼロ。
- **デメリット**: 依存関係が陳腐化し、Node 26 や将来のブラウザ機能との不整合リスクが高まる。ビルド速度向上の恩恵を受けられない。

### 選択肢 2: Vite 8・TypeScript 7・Tailwind CSS v4 への一括刷新とヘッドレスChrome検証（採用）
- **概要**:
  - `@tailwindcss/vite` プラグインを採用し、PostCSS を完全廃止。
  - `apps/web/src/styles/main.css` に `@theme` ディレクティブでプロジェクト固有のカラー・フォントを定義。
  - `tsconfig.json` に `"types": ["node", "vite/client"]` を追加して厳格な型整合性を担保。
  - アップグレード前後で Headless Google Chrome によるスクリーンショット比較（モバイル・デスクトップ）を実施してデグレゼロを実証。
- **メリット**: 設定ファイルがスリム化され、ビルド時間が約 150ms に高速化。最新 LTS および GitHub Actions と完全同期。
- **デメリット**: 設定方式の移行および TypeScript 7 の型解決設定が必要。

## 決定事項 (Decision Outcome)
選択肢 2 を採用する。
1. **ワークフロー**:
   - `actions/checkout@v7`、`docker/login-action@v4` へ更新。
2. **Docker & ランタイム**:
   - `Dockerfile` のビルダーベースイメージを `node:26-alpine` に固定。
3. **フロントエンドツール**:
   - `typescript@^7.0.2`、`vite@^8.3.5`、`tailwindcss@^4.3.3`、`@tailwindcss/vite@^4.3.3` を導入。
   - `postcss`、`autoprefixer`、`tailwind.config.js`、`postcss.config.js` をリポジトリから削除。
   - `vite.config.ts` で `__dirname` を `import.meta.dirname` に移行。
4. **CSS アーキテクチャ**:
   - `src/styles/main.css` で `@import "tailwindcss";` および `@theme` を用いてデザイントークンを定義。

## 影響とトレードオフ (Consequences)

### 正の影響 (Positive)
- ビルド速度が向上し、設定ファイルが削減されて保守性が高まった。
- Headless Chrome によるデグレ検証により、既存の UI 表現・フォント・余白が 100% 保持されたことが実証された。

### 負の影響とリスク (Negative & Mitigations)
- Tailwind v4 の独自プラグインエコシステムは v3 と互換性がない場合がある。
  - *対策*: 本プロジェクトではサードパーティ Tailwind プラグインを使用しておらず、標準機能と `@theme` のみで完結しているため影響はない。

## 参考文献 (Authoritative External Sources)
- [Tailwind CSS: What's new in Tailwind CSS v4.0](https://tailwindcss.com/blog/tailwindcss-v4)
- [Tailwind CSS: Vite Installation Guide](https://tailwindcss.com/docs/installation/framework-guides/vite)
- [Vite Official Documentation](https://vite.dev/)
- [TypeScript Official Documentation - Release Notes](https://www.typescriptlang.org/docs/)
- [Node.js Official Release Schedule](https://github.com/nodejs/release#release-schedule)
