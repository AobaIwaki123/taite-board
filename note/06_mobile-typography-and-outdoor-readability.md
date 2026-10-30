# 06: モバイル屋外視認性と文字サイズ階層基準

## 1. 背景と課題

Ribbon Clock（taite-board）は、ライブアイドルファンの日常的なタイテ確認だけでなく、日中の野外音楽フェスティバルや屋外会場、ライブハウス前など、強烈な直射日光や移動中の揺れが発生する環境下での利用を強く想定している。

これまでのモバイルUIにおいて、情報密度を優先した結果、一部のラベルや補足情報に `text-[10px]` や `text-[11px]` といった微小文字（Micro Fonts）が混入していた。これにより、日差しの強い屋外では画面輝度を最大にしても文字が潰れて判読できず、出演時間や特典会場所を見落とす危険性が指摘された。

## 2. 屋外タイポグラフィの設計基準

屋外環境における視認性と瞬読性を担保するため、以下の文字サイズ基準を策定・適用した。

### 2.1 ルート文字サイズの拡大
- ルートの HTML 要素に対し `font-size: 17.5px` を指定（従来のブラウザデフォルト 16px から約9.4%拡大）。
- これにより、`rem` 基準のすべての文字サイズおよびレイアウト余白が比例して拡大され、屋外での可読性が向上する。

### 2.2 最小文字サイズの下限値（Floor Standard）
- アプリケーション全体で **実測 12px 未満のフォント使用を完全禁止** とする。
- Tailwind CSS の最小クラスとして `text-xs`（0.75rem = 実測 13.1px）を下限とし、チケット料金、入場特典、ステータスバッジなどの補足情報もすべて 13px 以上を維持する。

### 2.3 視認性ヒエラルキー（情報優先度と文字サイズ）

| 情報カテゴリ | 該当要素 | 採用クラス | 実効サイズ (px) | フォントウェイト |
|---|---|---|---:|---|
| 最重要時刻 | 出番時刻・特典会時刻 | `text-3xl` | 32.8 | ExtraBold (800) |
| 公演タイトル | イベント名 | `text-xl` | 21.9 | Bold (700) |
| 日程・会場 | 日付、会場名、ブース名 | `text-base` | 17.5 | SemiBold (600) |
| アクションボタン | チケット購入、フライヤー切替 | `text-base` | 17.5 | Bold (700) |
| 属性・補足 | チケット価格、入場特典、カウントダウン | `text-sm` / `text-xs` | 15.3 / 13.1 | Medium (500) |

## 3. 実装と適用箇所

`apps/web/src/styles/main.css` および各コンポーネントにおいて、以下の改修を実施した。

1. **ベースCSS ([apps/web/src/styles/main.css](../apps/web/src/styles/main.css))**:
   ```css
   html {
     font-size: 17.5px;
   }
   ```
2. **コンポーネント ([apps/web/src/components/CockpitView.tsx](../apps/web/src/components/CockpitView.tsx))**:
   - チケット料金エリアのテキストを `text-[11px]` から `text-xs font-medium` へ引き上げ。
   - 特典会ブース表記を `text-xs`（実測 13.1px）へ引き上げ。

## 4. 検証結果

Google Chrome Headless を用いたモバイル実画面キャプチャ（幅 414px）において、文字の重なりやレイアウト破綻が生じないことを確認した。
1行あたりの文字数（Line Length）が適切に維持され、直射日光を模した高コントラスト表示下でも主要な時刻・会場名が一瞥で認識可能となった。

## 5. 参考文献

- [W3C: Web Content Accessibility Guidelines (WCAG) 2.2 - Visual Presentation (1.4.8)](https://www.w3.org/TR/WCAG22/#visual-presentation)
- [Apple Developer: Human Interface Guidelines - Typography](https://developer.apple.com/design/human-interface-guidelines/typography)
- [Google Material Design 3: Typography System](https://m3.material.io/styles/typography/overview)
