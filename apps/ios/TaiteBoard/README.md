# TaiteBoard iOS app shell

このディレクトリは、`TaiteBoardCore` を利用する SwiftUI アプリの最小実装です。

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

ビルド時に `data/generated/schedule.json` をアプリバンドルへ同期するため、起動直後から seed を表示できます。リモート feed URL は `Info.plist` の `ScheduleFeedURL` に注入するまで未設定です。

## UI レビュー

- `TaiteBoard.xcodeproj` を Xcode で開き、`TaiteBoard` scheme を選択する
- iPhone Simulator で次回、公演一覧、カレンダー、データ状態の各タブを確認する
- `PreviewData.swift` の `#Preview` で固定データの画面を確認する
- Simulator のダークモード、Dynamic Type、VoiceOver、長いタイトルを確認する
