# TaiteBoard iOS app

`TaiteBoardCore` を利用するSwiftUIアプリです。Web版の主要導線に合わせ、直近公演、公演一覧、カレンダーの3画面を横スワイプと下部ナビゲーションで切り替えます。

## ビルドとテスト

### Swift Coreのテスト

```bash
cd apps/ios/TaiteBoardCore
swift test
cd ../../..
```

### iOS Simulator向けビルド

リポジトリルートから次のコマンドで iOS Simulator 向けにビルドします。

```bash
xcodebuild \
  -project apps/ios/TaiteBoard/TaiteBoard.xcodeproj \
  -scheme TaiteBoard \
  -sdk iphonesimulator \
  -destination 'generic/platform=iOS Simulator' \
  -derivedDataPath /private/tmp/taite-board-ios-derived \
  -clonedSourcePackagesDirPath /private/tmp/taite-board-ios-source-packages \
  CODE_SIGNING_ALLOWED=NO \
  build
```

生成されたアプリは `/private/tmp/taite-board-ios-derived/Build/Products/Debug-iphonesimulator/TaiteBoard.app` に出力されます。

ビルド時に `data/generated/schedule.json` をアプリバンドルへ同期するため、起動直後からseedを表示できます。リモートfeed URLは `Info.plist` の `ScheduleFeedURL` に注入するまで未設定です。

### Simulatorで起動

```bash
xcrun simctl list devices available
xcrun simctl install booted /private/tmp/taite-board-ios-derived/Build/Products/Debug-iphonesimulator/TaiteBoard.app
xcrun simctl launch booted net.aooba.taiteboard
```

`booted` は起動済みのSimulatorを指します。未起動の場合は、Xcodeの実行先メニューからSimulatorを選んで起動してから実行します。

### 実機向けビルド

実機は署名が必要なため、初回はXcodeから次の手順で実行します。

1. `apps/ios/TaiteBoard/TaiteBoard.xcodeproj` をXcodeで開く
2. `TaiteBoard` targetの `Signing & Capabilities` で `Automatically manage signing` を有効にし、Apple Developer ProgramのTeamを選ぶ
3. Xcode上部の実行先で接続したiPhoneを選ぶ
4. `Product > Run`（`⌘R`）を実行する

署名設定が済んでいる場合は、UDIDを指定してCLIからもビルドできます。

```bash
xcodebuild \
  -project apps/ios/TaiteBoard/TaiteBoard.xcodeproj \
  -scheme TaiteBoard \
  -configuration Debug \
  -destination 'id=<接続したiPhoneのUDID>' \
  build
```

### リポジトリ全体の検証

```bash
make test
make validate
npm --prefix apps/web run typecheck
npm --prefix apps/web run build
```

## UI レビュー

- `TaiteBoard.xcodeproj` を Xcode で開き、`TaiteBoard` scheme を選択する
- iPhone Simulatorで、直近ライブ、公演一覧、カレンダーの各タブを確認する
- `PreviewData.swift` の `#Preview` で固定データの画面を確認する
- Simulator のダークモード、Dynamic Type、VoiceOver、長いタイトルを確認する
- `Assets.xcassets` は `apps/web/public/icons/` のアイコン素材を利用している
- AppIconは `AppIcon-1024.png` を正本とし、XcodeのAsset Catalogが各サイズを生成する
- `review-assets/ios-brand-assets-simulator.png` にブランドアセット反映後のSimulator画面を保存している
