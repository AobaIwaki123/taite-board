# ADR-0010: フロントエンドのVite・モダンTypeScript刷新とマルチステージビルド化

## ステータス
Accepted

## 決定日
2026-10-30

## 対象コンポーネント
- `apps/web/`
- `Dockerfile`
- `scripts/build.sh`
- `web/` (削除・廃止)

## 背景と課題
Ribbon Clock の初期プロトタイプは迅速な立ち上げを重視し、vanilla JavaScript（単一 `app.js`）、Tailwind CDN、Lucide CDN を使用していた。
しかし、運用が本格化し Kubernetes への GitOps デプロイが稼働するにつれ、以下の課題が顕在化していた。
1. **型安全性の欠如**: `schema/schedule.schema.json`（Draft-07）で策定した統一スキーマの制約がフロントエンドのコード上で静的に検査されず、プロパティの誤参照や null 安全性の欠如によるランタイムエラーのリスクがあった。
2. **外部 CDN への依存とオフライン耐性の低下**: 会場の地下など通信状態の悪い環境（オフライン・低速回線）において、Tailwind や Lucide の外部 CDN スクリプトの取得に失敗すると UI やレイアウトが崩壊するリスクがあった（ADR-0008 のオフライン耐性原則に抵触）。
3. **不要なレガシーディレクトリ（`web/`）の残存**: `apps/web/` を正本として集約したにもかかわらず、過去の `web/` が残存し、ビルドスクリプトでも不要なコピーが発生していた。
4. **CI/Docker ビルドの分離**: コンテナビルド時に最新の JS/CSS ビルドが自動で完結せず、成果物の手動管理や二重同期が必要になっていた。

## 決定事項
1. **Vite + モダン TypeScript への完全移行**:
   - `apps/web/` に Vite および TypeScript（Strict Mode）を導入した。
   - `schema/schedule.schema.json` から厳密に導出した `src/types/schedule.ts` を定義し、全データアクセスを型安全化した。
   - 単一の `app.js`（680行）を関心事ごとにクリーンなモジュールに分割した（`state.ts`, `cockpit.ts`, `scheduleList.ts`, `calendar.ts`, `promotion.ts`, `carousel.ts`, `lightbox.ts`）。
2. **バンドル最適化とゼロ CDN 依存**:
   - Tailwind CSS を PostCSS 経由でローカルビルドし、外部 CDN スクリプトを完全撤廃した。
   - `lucide` は全件インポート（約800kB）を避け、実際に使用する16個のアイコンのみを個別インポートすることで、JS バンドルサイズをわずか 30kB（gzip 後 8.7kB）に圧縮した。
3. **Docker マルチステージビルドの採用**:
   - `Dockerfile` を 2 ステージ（Stage 1: `node:26-alpine` で TypeScript/Vite ビルド、Stage 2: `nginx:alpine` で静的配信）に改修した。
   - これにより、リポジトリに `dist/` をコミットすることなく、Docker ビルド（および GitHub Actions CI）単独で常に再現性のある最新バイナリが生成される。
4. **レガシー `web/` ディレクトリの完全削除**:
   - 重複していた旧 `web/` ディレクトリを削除し、`scripts/build.sh` の同期処理を `apps/web/dist` から `docs/`（GitHub Pages 用）への片方向同期に簡素化した。

## 採用理由と設計根拠
- **オフライン現場での圧倒的耐性と表示速度**: 外部 CDN へのリクエストが完全ゼロとなり、地下会場でも ServiceWorker / ローカルキャッシュと連携して即座に画面が描画される。
- **保守性と可読性の向上**: 巨大な vanilla JS が TypeScript の型安全な責務分離モジュールとなり、今後の機能追加やバグ修正時の影響範囲が自明になった。
- **軽量フットプリント**: 30kB の超軽量バンドルにより、モバイルのファーストビュー表示速度（LCP）が極小化された。

## 影響範囲とリスク
- **ビルド手順の変更**: ローカルでの開発には Node.js 環境および `npm run build` が必要となる。開発環境には既に Node.js 26.12.1 が導入されており問題ない。
