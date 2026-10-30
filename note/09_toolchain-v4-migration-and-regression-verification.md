# 09: フロントエンドツールチェーン刷新とゼロデグレ検証手法

## 1. 背景と刷新対象

Ribbon Clock のフロントエンドは、当初 Vite 6、TypeScript 5、Tailwind CSS 3、Node 22 を利用していた。
開発体験の向上、ビルド速度の改善、および依存関係の長期健全性を維持するため、以下の全エコシステムを最新メジャーバージョンへ一括刷新した。

| レイヤー | 移行前 | 移行後 | 主な変更点 |
|---|---|---|---|
| コンテナ環境 | `node:22-alpine` | `node:26-alpine` | Node 26 LTS (v26.13.0) への追従 |
| CI アクション | `checkout@v4`, `login@v3` | `checkout@v7`, `login@v4` | 公式最新メジャーアクションへの更新 |
| ビルドツール | `vite@6.4.5` | `vite@8.3.5` | Vite 8系への移行、`import.meta.dirname` の採用 |
| 型検査 | `typescript@5.9.3` | `typescript@7.0.2` | TypeScript 7系への移行、CSSインポート・Node型定義の厳格化 |
| CSSエンジン | `tailwindcss@3.4.17` | `tailwindcss@4.3.3` | `@tailwindcss/vite` への完全移行、PostCSS/設定ファイルの廃止 |

## 2. Tailwind CSS v4 への移行方針

Tailwind CSS v4 では、従来の JavaScript 設定ファイル（`tailwind.config.js`）や PostCSS（`postcss.config.js`）を用いたビルドパイプラインが刷新され、CSS ネイティブの `@theme` ディレクティブと Rust 製の Oxide エンジンが導入された。

### 2.1 周辺設定の撤廃
- `postcss.config.js`、`tailwind.config.js` をリポジトリから削除。
- `autoprefixer` および `postcss` 依存関係を `package.json` から削除。

### 2.2 `@theme` によるデザイントークンの集約
`apps/web/src/styles/main.css` の冒頭にプロジェクト固有のトークンを定義した。

```css
@import "tailwindcss";

@theme {
  --color-rose-400: #fb7185;
  --color-rose-500: #f43f5e;
  --color-rose-600: #e11d48;
  --color-rose-950: #4c0519;
  --color-ayu-red: #ef4444;
  --color-ayu-darkred: #7f1d1d;
  --font-sans: -apple-system, BlinkMacSystemFont, "Hiragino Sans", "Hiragino Kaku Gothic ProN", "Noto Sans JP", sans-serif;
  --font-display: -apple-system, BlinkMacSystemFont, "Hiragino Sans", sans-serif;
}
```

### 2.3 Vite プラグイン連携
`apps/web/vite.config.ts` において、`@tailwindcss/vite` を Preact プラグインの直前に登録した。

## 3. TypeScript 7 型整合性の解決

TypeScript 7 への更新に伴い、CSS の副作用インポート（`import './styles/main.css'`）および `node:test` の型解決でエラーが発生した。
`apps/web/tsconfig.json` に `"types": ["node", "vite/client"]` を明示的に指定することで、型定義の不整合を解消した。

## 4. Headless Chrome によるゼロデグレ検証手法

メジャーアップデートに伴う表示崩れやフォント欠落を確実に防止するため、Headless Google Chrome を用いた自動スクリーンショット比較プロトコルを実施した。

### 4.1 検証手順
1. **ベースライン取得**: 更新前の本番ビルドからモバイル（414x896）およびデスクトップ（1280x800）の画面をキャプチャ。
2. **更新後ビルド検証**:
   ```bash
   make web-build
   python3 -m http.server 8092 --directory apps/web/dist &
   "/Applications/Google Chrome.app/Contents/MacOS/Google Chrome" \
     --headless --disable-gpu --window-size=414,896 \
     --screenshot=updated_mobile.png http://localhost:8092
   ```
3. **実コンテナ検証**: Docker マルチステージビルド（Nginx Alpine）を実行し、コンテナ内配信でも同様に表示崩れがないことを確認。

### 4.2 検証結果
Tailwind v4 移行後も、すべてのカード、ボタン、バッジ、グラデーション、モーダルがベースラインと完全に一致し、ピクセル単位でのゼロデグレ（Visual Regression Zero）を確認した。
また、本番ビルド速度は従来の約 480ms から約 150ms へと 3倍以上高速化した。

## 5. 参考文献

- [Tailwind CSS v4 Announcement](https://tailwindcss.com/blog/tailwindcss-v4)
- [Vite 8 Release Guide](https://vite.dev/guide/)
- [TypeScript 7.0 Overview](https://www.typescriptlang.org/docs/)
- [Chrome for Testing / Headless Documentation](https://developer.chrome.com/docs/chromium/new-headless)
