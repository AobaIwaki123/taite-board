# TaiteBoard iOS app

`TaiteBoardCore` を利用するSwiftUIアプリです。Web版の主要導線に合わせ、直近公演、公演一覧、カレンダーの3画面を横スワイプと下部ナビゲーションで切り替えます。

## ローカルビルド

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

ビルド時に `data/generated/schedule.json` をアプリバンドルへ同期するため、起動直後からseedを表示できます。リモートfeed URLは `Info.plist` の `ScheduleFeedURL` に注入するまで未設定です。

## UI レビュー

- `TaiteBoard.xcodeproj` を Xcode で開き、`TaiteBoard` scheme を選択する
- iPhone Simulatorで、直近ライブ、公演一覧、カレンダーの各タブを確認する
- `PreviewData.swift` の `#Preview` で固定データの画面を確認する
- Simulator のダークモード、Dynamic Type、VoiceOver、長いタイトルを確認する
- `Assets.xcassets` は `apps/web/public/icons/` のアイコン素材を利用している
- `review-assets/ios-brand-assets-simulator.png` にブランドアセット反映後のSimulator画面を保存している
