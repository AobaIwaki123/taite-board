# 05: Web・iOSブランドアセットの生成と同期運用

## 1. 背景と現状

Web版のブランドアイコンをiOSネイティブアプリでも利用するため、`origin/main` のアセット構成と既存の自動化を確認した。

現時点で、画像の生成・リサイズ・iOS Asset Catalogへの同期を一括で行う専用スクリプトやnpm scriptは存在しない。`scripts/build.sh` は `apps/web` の依存関係を `npm ci` で用意し、`npm run build` を実行するだけであり、アイコン変換は担当しない。`scripts/`、`docs/`、`AGENTS.md` を検索しても、`sips`、Asset Catalog、AppIconを扱う既存自動化は見つからなかった。

Web側のアセット仕様は、既存の [note/04](./04_web-share-metadata-and-icons-proposal.md) に記録されている。Webの正本は `apps/web/public/` であり、現在は次のアセットが存在する。

| アセット | サイズ | 用途 |
|---|---:|---|
| `favicon.png` | 48 x 48 | PNG形式のfaviconフォールバック |
| `icons/favicon.svg` | ベクター | ブラウザのfavicon |
| `icons/apple-touch-icon.png` | 180 x 180 | iOS Web Clip |
| `icons/icon-192.png` | 192 x 192 | Web App Manifestの標準アイコン |
| `icons/icon-512.png` | 512 x 512 | Web App Manifestの高解像度アイコン |
| `icons/icon-192-maskable.png` | 192 x 192 | Manifestのmaskableアイコン |
| `icons/icon-512-maskable.png` | 512 x 512 | Manifestの高解像度maskableアイコン |
| `og-image.png` | 1200 x 630 | URL共有時のOGP画像 |

## 2. 今回のiOS実装で行ったこと

PR #56の初期実装では、Web側の `icon-512-maskable.png` を元にiOSの `AppIcon.appiconset` 用PNGを各サイズへ手動展開し、`apple-touch-icon.png` をアプリ内ヘッダーのブランドマークとして `RibbonClockIcon.imageset` に登録した。

この展開には既存の規約や自動生成スクリプトはなく、作業時に手動で行ったものだった。その後、Appleの現行ドキュメントに合わせてPR #56を整理し、AppIconは `AppIcon-1024.png` 1枚を正本として、XcodeのAsset Catalogに各サイズの生成を任せる構成へ変更した。ヘッダーの `RibbonClockIcon.imageset` にある48、96、144pxの3画像は、SwiftUIの1x、2x、3x表示密度に対応するためのUI画像であり、AppIconの派生サイズとは役割が異なる。

## 3. 運用ルール

### 3.1 正本の置き場所

- Webブラウザ、PWA、Web Clipで使うブランドアセットの正本は `apps/web/public/` に置く。
- iOS固有のAppIconとアプリ内画像は `apps/ios/TaiteBoard/Assets.xcassets/` に置く。
- `docs/` には画像やビルド生成物を複製しない。
- OGP画像はサービス全体を表す静的画像とし、特定イベントのフライヤーを共有カードの正本にしない。

### 3.2 アセットの使い分け

- AppIconには、ホーム画面のマスクで重要部分が切れない正方形のブランド画像を使う。
- Web Manifestの `maskable` 用画像は、W3Cが定める安全領域を満たすように重要な図形を中央へ配置する。
- アプリ内ヘッダーのマークはAppIconと分離したImage Setとして扱い、UI上のサイズに適した余白と角丸を持つ画像を使う。
- イベント固有の画像はスケジュールデータの `images` またはWebの公開画像として管理し、AppIconやブランドマークへ混ぜない。

### 3.3 変更時の手順

1. Web側の正本アセットを追加・更新する。
2. `npm --prefix apps/web run typecheck` と `npm --prefix apps/web run build` を実行する。
3. `apps/web/dist/` にfavicon、Manifest、OGP画像がコピーされていることを確認する。
4. iOS側で再利用する場合は、1024 x 1024のAppIcon正本と、必要なアプリ内Image SetだけをAsset Catalogへ追加する。
5. `xcodebuild` でAsset Catalogを含むiOSビルドを実行する。
6. Simulatorと実機で、ホーム画面のアイコン、ヘッダー、safe area、ダークモードでの表示を確認する。
7. レビュー用スクリーンショットをPRブランチの `review-assets/` に保存する。

## 4. 自動化の未整備と今後の候補

現時点では、次の処理を自動化するスクリプトは存在しない。

- Web正本からiOS Image Setへ画像をコピーする処理
- Web側の正本から必要なWeb派生画像を作る処理
- PNGの寸法、色空間、透過、maskable安全領域を検査する処理
- Web側とiOS側でブランドアセットのハッシュ差分を検出する処理

同じ作業を繰り返す場合は、`scripts/sync-brand-assets.sh` などを追加し、入力元、出力先、生成サイズ、検証結果を固定する。ただし、iOS AppIconについてはXcodeが1024px正本から派生サイズを生成するため、現段階ではiOS用の画像生成スクリプトを追加しない。Web側の派生画像やImage Setの同期が増えた段階で、スクリプト追加による保守対象の増加と釣り合うかを確認する。

## 5. 検証コマンド

```bash
npm --prefix apps/web run typecheck
npm --prefix apps/web run build
xcodebuild \
  -project apps/ios/TaiteBoard/TaiteBoard.xcodeproj \
  -scheme TaiteBoard \
  -configuration Debug \
  -sdk iphonesimulator \
  -destination 'generic/platform=iOS Simulator' \
  -derivedDataPath /private/tmp/taite-board-ios-derived \
  CODE_SIGNING_ALLOWED=NO \
  build
```

Asset Catalogがビルドへ取り込まれたことは、生成された `TaiteBoard.app/Assets.car` と `Info.plist` の `CFBundleIconName` が `AppIcon` になっていることでも確認できる。

## 6. 参考文献

- [Apple Developer: Configuring your app icon using an asset catalog](https://developer.apple.com/documentation/xcode/configuring-your-app-icon)
- [Apple Developer: Asset management](https://developer.apple.com/documentation/Xcode/asset-management)
- [Apple Developer: Build settings reference](https://developer.apple.com/documentation/xcode/build-settings-reference)
- [W3C: Web Application Manifest](https://www.w3.org/TR/appmanifest/)
- [MDN: `icons` member of the Web App Manifest](https://developer.mozilla.org/en-US/docs/Web/Progressive_web_apps/Manifest/Reference/icons)
