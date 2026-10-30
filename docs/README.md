# ドキュメント

このディレクトリには、Ribbon Clock の開発・アーキテクチャ資料を置いています。

## ブランドアセット比較

URL共有、ブラウザ、PWA、iOS Web Clip で使用する画像を、用途別に比較できるようにまとめています。現在の完成版アセットの正本は `apps/web/public/` 配下です。

<table>
  <tr>
    <td align="center" width="48%"><strong>OGP画像</strong><br><img src="../apps/web/public/og-image.png" alt="OGP image" width="600"></td>
    <td align="center" width="13%"><strong>favicon</strong><br><img src="../apps/web/public/icons/favicon.svg" alt="Favicon" width="96"></td>
    <td align="center" width="13%"><strong>Web App</strong><br><img src="../apps/web/public/icons/icon-512.png" alt="Web App icon" width="96"></td>
    <td align="center" width="13%"><strong>Maskable</strong><br><img src="../apps/web/public/icons/icon-512-maskable.png" alt="Maskable Web App icon" width="96"></td>
    <td align="center" width="13%"><strong>Apple</strong><br><img src="../apps/web/public/icons/apple-touch-icon.png" alt="Apple touch icon" width="96"></td>
  </tr>
</table>

| アセット | 用途 | サイズ |
| --- | --- | ---: |
| [`og-image.png`](../apps/web/public/og-image.png) | URL共有時のOGPカード | 1200 x 630 |
| [`icons/favicon.svg`](../apps/web/public/icons/favicon.svg) | ブラウザタブ・ブックマーク | SVG |
| [`icons/apple-touch-icon.png`](../apps/web/public/icons/apple-touch-icon.png) | iOS Web Clip | 180 x 180 |
| [`icons/icon-192.png`](../apps/web/public/icons/icon-192.png) | Web App標準アイコン | 192 x 192 |
| [`icons/icon-512.png`](../apps/web/public/icons/icon-512.png) | Web App標準アイコン | 512 x 512 |
| [`icons/icon-192-maskable.png`](../apps/web/public/icons/icon-192-maskable.png) | Web App maskableアイコン | 192 x 192 |
| [`icons/icon-512-maskable.png`](../apps/web/public/icons/icon-512-maskable.png) | Web App maskableアイコン | 512 x 512 |

### 現行デザインの判断

- カレンダーを主役にし、上部に蝶々型のリボンを配置する。
- アイコン内に「出演時間」「特典会」などの説明文は入れない。
- アイコン背景は黒ではなく、濃いローズ `#3a1320` を使う。
- 通常アイコンと maskable アイコンを分け、maskable 版は端末のマスク処理に備えて安全領域を確保する。
- OGP画像はサービス全体を表す静的画像とし、イベント固有の画像や日付を含めない。

画像の検討経緯と未確認事項は [`note/06_web-share-asset-decision-log.md`](../note/06_web-share-asset-decision-log.md) を参照してください。

## データ intake とリリース

端末から URL・画像を候補 JSON に変換する skill と、名寄せ・検証・PR・GitOps デプロイまでの初期手順は [`manual-ingest-release.md`](manual-ingest-release.md) にまとめています。候補は公開用 JSON へ自動昇格せず、人間レビューを通してから反映します。

## プライバシー配慮型アクセス集計

クライアント計測を追加せず、Nginx の匿名ログを必要なときだけ集計して Discord へ通知する案は [`privacy-preserving-access-counting.md`](privacy-preserving-access-counting.md) にまとめています。初期案では origin request count のみを扱い、個人単位の追跡や raw log の Discord 送信は行いません。

Pod交換後も時間帯別集計を保持する次期案として、Web Podの内部metrics公開、専用Metrics storeのPVC保存、通知CronJobのquery利用を [`access-metrics-prometheus-like-design.md`](access-metrics-prometheus-like-design.md) に整理しています。現時点では設計案のみで、実装は停止中です。
