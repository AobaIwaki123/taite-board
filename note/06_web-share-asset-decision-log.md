# 06: Web共有メタデータとブランドアセットの判断記録

## 1. 背景と対象

このノートは、URL共有時の OGP、favicon、Web App Manifest、PWA用アイコンについて、検討から公開反映までに行った判断を時系列で記録する。大規模な Web リファクタリングと iOS ネイティブアプリの設計は別セッションで進めているため、このノートの対象は既存 Web 配信とそのブランドアセットに限定する。

作業は現行実装との競合を避けるため独立した worktree と PR に分けた。候補画像はレビュー用ディレクトリに集約して比較し、採用後にレビュー用の不要な画像を削除した。最終的な配信正本は `apps/web/` に置き、root README は完成版の紹介、`docs/README.md` は用途別アセットの比較に分ける方針とした。

## 2. 初期要求と判断軸

当初の要求は、サイト URL を共有したときのカード表示を整え、ブラウザ・iOS ホーム画面・PWA 起動時に同じサービスとして認識できるよう、細かなメタデータとアイコンの完成度を上げることだった。

判断軸は次のとおり。

- OGP、Manifest、favicon、Apple touch icon の URL と MIME type が実際の配信環境で成立すること。
- 将来の iOS ネイティブアプリとブランドを合わせつつ、現在の大規模リファクタリングと変更範囲を分離すること。
- 小さなアイコンでも用途が伝わり、説明文やイベント固有情報に依存しないこと。
- 端末側のアイコンマスクで図形が切れないこと。
- 画像生成・変換後の解像度と色味を、PR と配信 URL の両方で確認できること。

## 3. アイコン案の変遷

| 段階 | 観察 | 判断 |
| --- | --- | --- |
| 初期候補 | 最初のアイコンは丸みと親しみやすさが良かったが、何を表すアイコンかが弱かった。カレンダー単体は整っている一方、独自性が弱かった。 | カレンダーを主役にし、リボンを加える方向へ絞った。 |
| リボン案 | 蝶々型のリボンと、リボンの端を強調する案を比較した。 | カレンダー上部に蝶々型のリボンを置く構成を採用した。 |
| 文字情報 | 「出演時間」「特典会」などをアイコンへ入れる案は、縮小時に情報量が増え、見た目も説明的になった。 | アイコンから説明文を外し、記号としてのカレンダーとリボンだけを残した。 |
| 色と質感 | 黒背景、低い解像感、彩度の低さがチープに見える懸念が出た。 | 黒背景を避け、濃いローズを背景にした muted rose 系の配色へ変更した。 |
| 余白 | 実機検証前のセルフレビューでは、カレンダーとリボンの周囲が広く見える可能性が残った。 | maskable 版の安全領域は維持し、通常アイコンだけを必要に応じて拡大する方針とした。最終判断は実機確認後とする。 |

## 4. 実装と問題診断の変遷

### 4.1 URL共有メタデータの追加

初期実装では、`apps/web/index.html` に title、description、canonical、Open Graph、Twitter Card、theme-color、favicon、Apple touch icon、Manifest の参照を追加した。OGP はイベント画像ではなく、サービス全体を表す静的な `1200 x 630` 画像とした。

### 4.2 PWA 配信の修正

PWA の検証では、Manifest の MIME type とアイコンの `purpose` が重要であることが分かった。PR #50 では、Nginx で `/manifest.webmanifest` を `application/manifest+json` として配信し、通常アイコンと `maskable` アイコンを分けた。

### 4.3 画像表示不良の切り分け

実環境で画像が表示されない事象があったが、PWA のアイコン仕様だけを原因とみなさず、公開 schedule の外部画像 URL を個別に確認した。その結果、`pbs.twimg.com` の一部 URL が `name=orig` のままだと HTTP 351 を返し、同じ画像の `name=large` は取得できることを確認した。

このため、PWA の仕組みそのものと、外部画像 URL の互換性問題を分離して扱うことにした。PR #51 では `pbs.twimg.com` の URL を `name=large` へ正規化し、このノート作成後にマージコミット `1750398` として本番へ反映した。

### 4.4 色と画像品質の調整

PR #53 では、favicon、通常の Web App アイコン、`maskable` アイコン、Apple touch icon、OGP 内の埋め込みアイコンを再生成した。アイコン背景を黒から `#3a1320` へ変更し、主色 `#d4546b`、補助色 `#ee9aaa`、深いアクセント `#a93d56`、明るい面 `#fffaf8` を使う配色に統一した。

OGP 全体の背景はカード上での視認性を保つため暗色グラデーションを残し、埋め込みアイコンの背景だけを新しい濃いローズへ変更した。これは「PWA アイコンの黒背景を避ける」判断と「OGP カード全体を暗色ブランド画像にする」判断を分けたものである。

PR #53 はマージコミット `7969ca0` として取り込まれ、後続のデプロイ更新コミット `23a6801` まで本番へ反映された。公開確認では、Manifest のテーマ色、favicon、キャッシュバスター付き OGP PNG が新しい内容になっていることを確認した。その後、PR #51のデプロイ更新コミット `fac1828` により、外部X画像URLの正規化も本番へ反映された。

## 5. 現在の確定事項

| 項目 | 現在の方針 |
| --- | --- |
| 正本 | Web のブランドアセットと共有メタデータは `apps/web/public/` と `apps/web/index.html` に置く。 |
| favicon | `icons/favicon.svg` を主に使い、`favicon.png` をフォールバックにする。 |
| Web App アイコン | `any` 用と `maskable` 用を別 PNG として Manifest に登録する。 |
| デザイン | カレンダーを主役にし、上部に蝶々型のリボンを置く。アイコン内に用途説明文は入れない。 |
| 色 | アイコン背景は `#3a1320`。主色は `#d4546b`、補助色は `#ee9aaa`、深いアクセントは `#a93d56`。 |
| Manifest | `display: standalone` を採用し、`background_color` と `theme_color` は `#3a1320`。 |
| OGP | `og-image.png` は静的な `1200 x 630` ブランドカードとし、イベント固有情報は載せない。 |
| 余白の光学補正 | 通常版の内部図形を `1.05` 倍、maskable版を安全領域内で `0.84` 倍に調整し、Web・OGP・iOSの派生画像を同じ変更単位で再生成する。 |
| iOS固有設定 | `apple-mobile-web-app-capable` と status bar 専用設定は、safe area の実機検証後に判断する。 |
| キャッシュ | 通常 URL の CDN・共有サービス側キャッシュと、`?v=...` の確認用 URL は別に扱う。キャッシュバスターは旧キャッシュの削除ではない。 |

## 6. 検証結果と未確認事項

確認できた範囲は次のとおり。

- PR #53 の CI とデプロイが成功している。
- 公開トップページは HTTP 200 で、`theme-color` は `#3a1320` になっている。
- `/manifest.webmanifest` は HTTP 200、`application/manifest+json` で配信され、通常用と maskable 用のアイコンが登録されている。
- キャッシュバスター付き `/og-image.png?v=53` は HTTP 200、`1200 x 630` の PNG として新しい内容を返す。
- キャッシュバスターなしの OGP URL は Cloudflare の旧キャッシュが残る場合があり、SNS側のプレビューキャッシュも別途残る可能性がある。

未確認の範囲は次のとおり。

- iOS 実機で「ホーム画面に追加」したときの最終的なアイコンの見え方。
- PWA 起動時の safe area、status bar、端末ごとの maskable 切り抜き。
- 実機で通常アイコンの余白が過剰に見えるかどうか。
- PR #51 の外部 X 画像 URL 正規化が本番へ反映された後の全画像取得。

## 7. ドキュメント整合性の確認

- `note/04_web-share-metadata-and-icons-proposal.md` は、実装前の候補値と実装後の確定値を区別できるように更新した。黒背景、Manifest の採用、favicon の意匠を「未決定のまま」と読める記述は、現在の状態に合わせて整理した。
- root `README.md` は完成版の紹介に絞り、用途別比較と安全領域の説明を `docs/README.md` へ分離した。
- `note/03_schedule-metadata-expansion-proposal.md` は schedule のデータ契約に関する別テーマであり、Web共有アセットの決定と直接の矛盾はない。
- 並行セッションで作成された iOS ブランドアセットの記録は、Web正本、maskable の安全領域、イベント画像をブランドアセットへ混ぜない方針の点で本記録と整合する。iOS固有の派生画像や Xcode Asset Catalog の詳細は本ノートの対象外とする。
- `docs/deployment.md` の公開ホストと GitOps 配信方針は、今回の公開確認と整合する。

現時点で、追跡対象のドキュメント間に今回の方針と矛盾する記述は見つけていない。実機確認後に余白や safe area の判断が変わる場合は、このノートと `note/04` の「現在の状態と残課題」を同時に更新する。

## 参考文献

- [The Open Graph protocol](https://ogp.me/)
- [MDN: Web page metadata](https://developer.mozilla.org/en-US/docs/Learn_web_development/Core/Structuring_content/Webpage_metadata)
- [MDN: Web application manifest](https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Manifest)
- [MDN: `icons` member of the Web App Manifest](https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Manifest/Reference/icons)
- [W3C: Web Application Manifest](https://www.w3.org/TR/appmanifest/)
- [Apple Developer: Configuring Web Applications](https://developer.apple.com/library/archive/documentation/AppleApplications/Reference/SafariWebContent/ConfiguringWebApplications/)
- [PR #53: refine calendar ribbon icon palette](https://github.com/AobaIwaki123/taite-board/pull/53)
