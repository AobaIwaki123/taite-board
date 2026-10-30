# App Storeリリース準備

この文書は、TaiteBoard iOSアプリをApp Storeへ提出するためのリポジトリ側の確認事項と、App Store Connect上で必要な作業を整理したものです。App Store Connectの画面やAppleの要件は更新されるため、提出時には公式ドキュメントも確認します。

## 現在のアプリ設定

| 項目 | 値 |
|---|---|
| 表示名 | リボンクロック |
| Bundle ID | `net.aooba.taiteboard` |
| Version | `1.0` |
| Build | `1` |
| Deployment Target | iOS 17.0 |
| 対応デバイス | iPhone / iPad |
| データ配信 | `https://ribonclock.iorin.net/schedule.json` |
| App Store価格 | 無料候補。App Store Connectで決定 |
| アカウント・課金 | なし |

Bundle IDはApp Store ConnectのアプリレコードとXcodeの値を一致させる必要があります。App Store Connectでは、最初のビルドをアップロードする前にアプリレコードを作成します。

## App Store Connectの入力候補

最終決定が必要な項目は候補として記録しています。

| 項目 | 候補 / 状態 |
|---|---|
| App Name | リボンクロック |
| Primary Language | 日本語 |
| SKU | `taite-board-ios` |
| Primary Category | エンターテインメント候補。App Store Connectで決定 |
| Privacy Policy URL | `https://ribonclock.iorin.net/privacy-policy.html` |
| Support URL | `https://ribonclock.iorin.net/support.html`。実際の連絡先として十分か確認 |
| Marketing URL | `https://ribonclock.iorin.net/` |
| Copyright | 権利者名と取得年を確定して入力 |
| Age Rating | 実際の表示内容に合わせて回答 |

### 説明文案

ライブの出番、特典会、会場、チケット、タイムテーブルを確認できるスケジュールアプリです。直近の公演をすぐに確認できるほか、全日程一覧とカレンダーから予定を探せます。取得済みの予定は端末に保存され、通信できない場合も最後に検証済みのデータを表示します。

### キーワード案

`ライブ,イベント,予定,アイドル,タイムテーブル`

100バイト以内であること、アプリ名や会社名を重複させないことをApp Store Connect上で確認します。

### App Review notes案

```text
リボンクロックは、公開されているライブ出演情報を表示するアプリです。
アカウント登録やログインは必要ありません。
審査時は起動後に直近ライブ、全日程、カレンダーを確認できます。
スケジュール取得に失敗した場合は、同梱または端末に保存された最後の正常データを表示します。
```

## リポジトリに含めたリリース対応

- `MARKETING_VERSION` を導入し、Info.plistのバージョン番号をbuild settingから展開
- `ITSAppUsesNonExemptEncryption = NO` を追加。独自暗号化は実装せず、HTTPSはAppleのOS機能を使用
- `PrivacyInfo.xcprivacy` をアプリバンドルへ追加
- 公開URLとして `privacy-policy.html` と `support.html` を追加
- App Store用のアイコンは `Assets.xcassets/AppIcon.appiconset/AppIcon-1024.png` を使用

Privacy Manifestは、現在のコードと依存関係において、ユーザーデータ収集・追跡・AppleのRequired Reason API利用を宣言していません。依存SDKや端末APIを追加する場合は、Privacy ManifestとApp Store Connectのプライバシー回答を再確認します。

## アーカイブとTestFlight

実機署名をXcodeで設定した後、最初はXcode OrganizerからArchiveとUploadを実行する手順を推奨します。

```bash
ARCHIVE_DIR=/private/tmp/taiteboard-archives
mkdir -p "$ARCHIVE_DIR"

xcodebuild \
  -project apps/ios/TaiteBoard/TaiteBoard.xcodeproj \
  -scheme TaiteBoard \
  -configuration Release \
  -destination 'generic/platform=iOS' \
  -archivePath "$ARCHIVE_DIR/TaiteBoard.xcarchive" \
  archive
```

1. Xcodeで `TaiteBoard.xcodeproj` を開く
2. `TaiteBoard` targetのTeamとAutomatically manage signingを確認する
3. `Product > Archive` を実行する
4. OrganizerでArchiveを選び、`Distribute App > App Store Connect > Upload` を実行する
5. App Store Connectでビルド処理が完了するまで待つ
6. TestFlightのInternal Testingへ追加し、実機で確認する

CLIでexportする場合は、署名チームとプロビジョニング方式を含むExportOptions.plistをローカルで用意します。証明書、秘密鍵、Team IDをリポジトリへコミットしません。

## 提出前チェック

### 実機

- [ ] 新規インストールで起動できる
- [ ] 初回起動でseedが表示され、リモート更新後も予定が正しい
- [ ] Wi-Fi / モバイル通信で本番JSONを取得できる
- [ ] 機内モード相当でseedまたはキャッシュが表示される
- [ ] 直近ライブ、全日程、カレンダーの全導線を確認する
- [ ] 告知画像、外部URL、長い日本語タイトルを確認する
- [ ] iPhoneとiPadのレイアウトを確認する。現在のtargetは両方を対象にしている
- [ ] 起動直後・バックグラウンド復帰・通信失敗でクラッシュしない

### App Store Connect

- [ ] Account Holderが最新の契約に同意している
- [ ] App recordを作成し、Bundle IDを `net.aooba.taiteboard` にする
- [ ] 名前、カテゴリ、年齢レーティング、権利者、価格、公開地域を確定する
- [ ] プライバシーポリシーURLとSupport URLが実際にアクセスできる
- [ ] App Privacyでデータ収集の有無を、配信基盤のログ運用を含めて回答する
- [ ] Export Complianceの回答を行う。HTTPSのみで免除扱いにする場合も、App Store Connectの質問に沿って確認する
- [ ] iPhoneのスクリーンショットを最終UIから撮影する
- [ ] iPad対応を維持する場合は、iPad 13-inchのスクリーンショットを用意する
- [ ] TestFlight内部テストで実機検証を完了する
- [ ] BuildをVersion 1.0へ紐付ける
- [ ] What’s New、Description、Keywords、Review notesを確定する
- [ ] `Submit for Review`前に本番のschedule.jsonが稼働している

App Store Connectのスクリーンショットは、現在の仕様ではiPhoneのDynamic Island対応サイズが必須です。現行のtargetはiPadも含むため、iPad 13-inchのスクリーンショットも必要です。実際の必要サイズは提出時のAppleの仕様を優先します。

## 公式ドキュメント

- [App Store Connect workflow](https://developer.apple.com/help/app-store-connect/get-started/app-store-connect-workflow)
- [Add a new app](https://developer.apple.com/help/app-store-connect/create-an-app-record/add-a-new-app)
- [Required, localizable, and editable properties](https://developer.apple.com/help/app-store-connect/reference/app-information/required-localizable-and-editable-properties)
- [Manage app privacy](https://developer.apple.com/help/app-store-connect/manage-app-information/manage-app-privacy)
- [Upload app previews and screenshots](https://developer.apple.com/help/app-store-connect/manage-app-information/upload-app-previews-and-screenshots/)
- [Screenshot specifications](https://developer.apple.com/help/app-store-connect/reference/app-information/screenshot-specifications)
- [Upload builds](https://developer.apple.com/help/app-store-connect/manage-builds/upload-builds)
- [Overview of export compliance](https://developer.apple.com/help/app-store-connect/manage-app-information/overview-of-export-compliance)
- [Privacy manifest files](https://developer.apple.com/documentation/bundleresources/privacy-manifest-files)
- [App Review Guidelines](https://developer.apple.com/app-store/review/guidelines/)
