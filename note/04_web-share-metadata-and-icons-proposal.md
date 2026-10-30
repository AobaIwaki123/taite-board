# 04: URL共有時の OGP・Web App アイコン完成度改善案

## 1. 背景と現状

現在の `apps/web/index.html` は、文字コード・viewport・title・アプリの mount 要素を持つ最小構成であり、URLを共有したときのカード表示や、ブラウザ・iOS ホーム画面でのアイコン指定はまだない。

また、現在のフロントエンド移行では `apps/web/` が正本で、Vite の `public/` 配下がビルド時に `dist/` へコピーされ、Docker の Nginx から配信される構成になっている。したがって、共有用メタデータとアイコンの正本も `apps/web/` に置くのが自然である。`docs/` は現在削除・生成方針が移行中のため、同じ HTML や画像を手動で二重管理しない。

## 2. 目的

- X、Discord、LINE、Slack などでサイト URL を共有した際に、サイト名・説明・ブランド画像が一貫して表示されるようにする。
- ブラウザタブ、ブックマーク、iOS の「ホーム画面に追加」で、リボンクロックを識別できるアイコンを表示する。
- Web App Manifest とテーマカラーを用意し、将来の PWA 化や iOS ネイティブアプリとのブランド整合に備える。
- OGP 用画像にイベント固有のタイテ画像を使わず、常にサービスの用途が分かる静的なブランド画像を使う。

## 3. 実装候補

### 3.1 `apps/web/index.html` の head

サイトの公開 URL は `https://ribonclock.iorin.net/` に固定し、クローラーが相対パスやローカル開発 URLを解釈しないよう、canonical と OGP 画像は絶対 URL で記述する。

追加候補は次のとおり。

```html
<meta name="description" content="リボンクロックのライブ出演時間、特典会、会場、チケット、タイムテーブルをすぐに確認できるボード。">
<link rel="canonical" href="https://ribonclock.iorin.net/">

<meta property="og:type" content="website">
<meta property="og:site_name" content="リボンクロック">
<meta property="og:title" content="リボンクロック｜ライブ・タイテ情報">
<meta property="og:description" content="ライブの出番、特典会、会場、チケット、タイムテーブルをすぐに確認できます。">
<meta property="og:url" content="https://ribonclock.iorin.net/">
<meta property="og:image" content="https://ribonclock.iorin.net/og-image.png">
<meta property="og:image:width" content="1200">
<meta property="og:image:height" content="630">
<meta property="og:image:alt" content="リボンクロック ライブ・タイテ情報">
<meta property="og:locale" content="ja_JP">

<meta name="twitter:card" content="summary_large_image">
<meta name="twitter:title" content="リボンクロック｜ライブ・タイテ情報">
<meta name="twitter:description" content="ライブの出番、特典会、会場、チケット、タイムテーブルをすぐに確認できます。">
<meta name="twitter:image" content="https://ribonclock.iorin.net/og-image.png">

<meta name="theme-color" content="#0c0a09">
<meta name="color-scheme" content="dark">
<link rel="manifest" href="/manifest.webmanifest">
<link rel="icon" href="/icons/favicon.svg" type="image/svg+xml">
<link rel="alternate icon" href="/favicon.png" type="image/png" sizes="48x48">
<link rel="apple-touch-icon" sizes="180x180" href="/icons/apple-touch-icon.png">
<meta name="apple-mobile-web-app-title" content="リボンクロック">
```

`twitter:site` は公式 X アカウントの運用方針が決まるまで追加しない。アカウントを所有していることを示せない状態で、推測したハンドルを埋め込まない。

### 3.2 OGP 用画像

`apps/web/public/og-image.png` をサービス専用の静的画像として追加する。

- 推奨サイズ: 1200 x 630
- 画像内の主題: `リボンクロック`、`ライブ・タイテ情報`、必要なら短い補足文
- 背景: 現行 UI の `#0c0a09` 系を基本に、ローズ系アクセントを使う
- 可読性: 端末上の小さなカードでもロゴと用途が判読できること
- 内容: 特定の日付、特定の会場、チケット価格を含めない
- 更新方針: ブランドや用途を変更したときだけ更新し、イベント更新のたびには変更しない

現行の `20261107_marquee_mini.png` はイベント固有画像でサイズも大きいため、サイト URL の共有カードには使わない。OGP 画像をイベント情報と分離することで、古いイベント画像が長期間共有カードに残る問題を避ける。

### 3.3 favicon と iOS ホーム画面アイコン

`apps/web/public/` に次の静的アセットを置く。

| ファイル | 用途 | 方針 |
| --- | --- | --- |
| `icons/favicon.svg` | 対応ブラウザのタブ・ブックマーク | ベクター。小サイズで判別できる単純なリボン記号または頭文字 |
| `favicon.png` | SVG 非対応環境や自動探索のフォールバック | 48 x 48px の PNG |
| `icons/apple-touch-icon.png` | iOS ホーム画面 | 180 x 180px、背景込みの正方形 |
| `icons/icon-192.png` | Manifest の標準アイコン | 192 x 192px |
| `icons/icon-512.png` | Manifest の高解像度アイコン | 512 x 512px |

iOS の Web Clip 用アイコンは通常の `rel="icon"` だけではなく `apple-touch-icon` を明示する。Apple 向けには透過を前提にせず、ホーム画面上で角丸処理されることを前提に安全領域を確保する。

### 3.4 Web App Manifest

`apps/web/public/manifest.webmanifest` の候補は次のとおり。

```json
{
  "name": "リボンクロック｜ライブ・タイテ情報",
  "short_name": "リボンクロック",
  "lang": "ja",
  "start_url": "/",
  "scope": "/",
  "display": "standalone",
  "background_color": "#0c0a09",
  "theme_color": "#0c0a09",
  "icons": [
    {
      "src": "/icons/icon-192.png",
      "sizes": "192x192",
      "type": "image/png"
    },
    {
      "src": "/icons/icon-512.png",
      "sizes": "512x512",
      "type": "image/png"
    }
  ]
}
```

`display: standalone` はホーム画面から起動したときの表示を変えるため、実機でヘッダー・safe area・戻る操作を確認してから確定する。Manifest を追加することと、完全オフライン PWA にすることは別であり、Service Worker の導入は今回のメタデータ修正に含めない。

今回の先行実装では、Manifest の `display: standalone` とブランドアイコンを追加した。ブランドアイコンはカレンダーを主役にし、上部に蝶々型のリボンを置く構成とした。主色にはデータ上の `#ef4444`、補助色には既存 UI の `#fb7185` を使用している。一方、`apple-mobile-web-app-capable` と status bar の専用設定は、iOS 実機で safe area を確認するまで追加していない。

## 4. 変更を分離する単位

競合を避けるため、実装時は次の小さな変更単位に分ける。

1. `apps/web/public/` に OGP 画像、favicon、iOS アイコン、Manifest を追加する。
2. `apps/web/index.html` の `head` に title、description、canonical、OGP、Twitter Card、icon、theme-color を追加する。
3. Vite build 後の `dist/` に全アセットが含まれることを確認する。
4. Docker/Nginx 配信時にアセットの MIME type と URL が正しいことを確認する。
5. 共有サービスと iOS 実機で表示を確認する。

`docs/` の生成物削除・再生成、Web コンポーネントの Preact 化、schedule JSON のフィールド追加とは別の変更として扱う。特に現在の `scripts/validate_schedule.js` と `scripts/generate_real_schedule.js` は `docs/` を参照しているため、`docs/` の整理をこの作業に混ぜない。

## 5. 検証チェックリスト

### 静的チェック

- `npm run typecheck`
- `npm run build`
- `dist/index.html` に必要な `<meta>` と `<link>` が残っている。
- `dist/manifest.webmanifest` が JSON として読める。
- `dist/og-image.png`、favicon、180/192/512px アイコンが存在する。
- SVG、PNG、JPEG、Web Manifest の Content-Type が配信環境で正しい。
- `og:url`、`og:image`、canonical に localhost や相対 URL が残っていない。

### 実機・共有チェック

- X、Discord、LINE、Slack の URL プレビューでタイトル・説明・画像が意図どおりになる。
- 画像が古い場合は、共有サービス側のキャッシュを考慮して画像ファイル名または URL のバージョンを更新する。
- Safari のタブとブックマークに favicon が表示される。
- iOS Safari の「ホーム画面に追加」で、サイトのスクリーンショットではなく指定アイコンが表示される。
- ホーム画面から起動した場合、`display: standalone` によって画面上部の safe area が隠れない。
- アイコンの文字や細線が 180px から縮小されても読める。

## 6. 未決定事項

- OGP 画像を PNG のまま配信するか JPEG に変換するか。一般共有向けの容量と文字の鮮明さで決める。
- favicon の意匠は、カレンダーを主役にして上部に蝶々型のリボンを置く構成で確定した。配色は `#ef4444` と `#fb7185` を基準とする。
- Manifest の `display: standalone` を採用するか、まず `browser` のままアイコンだけ提供するか。
- `apple-mobile-web-app-capable` を追加するか。追加する場合は standalone 起動時の safe area を実機確認する。
- OGP 画像を将来の iOS ネイティブアプリのストア用素材と共用するか。

## 7. 参考文献

- [The Open Graph protocol](https://ogp.me/)
- [MDN: What's in the head? Web page metadata](https://developer.mozilla.org/en-US/docs/Learn_web_development/Core/Structuring_content/Webpage_metadata)
- [MDN: `rel` HTML attribute](https://developer.mozilla.org/en-US/docs/Web/HTML/Reference/Attributes/rel)
- [MDN: Web application manifest](https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Manifest)
- [MDN: `icons` manifest member](https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Manifest/Reference/icons)
- [W3C: Web Application Manifest](https://www.w3.org/TR/appmanifest/)
- [Apple Developer: Configuring Web Applications](https://developer.apple.com/library/archive/documentation/AppleApplications/Reference/SafariWebContent/ConfiguringWebApplications/ConfiguringWebApplications.html)
- [Apple Developer: Supported Meta Tags](https://developer.apple.com/library/archive/documentation/AppleApplications/Reference/SafariHTMLRef/Articles/MetaTags.html)
