# 10: iOS実機レビューで確認したWeb版との差分とApp Store検証エラー

## 1. 背景と現象の観察

App Store ConnectへのiOSビルドアップロード中に、次の検証エラーが発生した。

> Invalid bundle. No orientations were specified in the `net.iorin.ribonclock` bundle.

原因は、iPad対応アプリの `Info.plist` に `UISupportedInterfaceOrientations` が指定されていなかったことだった。画面回転設定を追加するPR #70はmerge済みだが、修正前に作成したArchiveは内容が更新されないため、修正後に新しいArchiveを作成する必要がある。

## 2. UIレビューで記録した未対応事項

今回の確認では、iOS版とWeb版のUI差分として以下を認識した。いずれもこの時点では実装せず、バックログとして扱う。

- フライヤー画像をタップしても、画像を拡大表示できない。
- フライヤーモーダルを閉じるとき、モーダル外側の背景をタップしても閉じられない。
- CAMPFIREの「支援する」リンクが、Web版と異なり赤色で表示されない。
- Web版とのUI差分を、リリース後の実機レビューで追加確認する。

## 3. 再アップロード手順

1. Xcodeのアップロードエラー画面を `Done` で閉じる。
2. PR #70のmerge後のmainを取得する。
3. `TaiteBoard.xcodeproj` を最新状態で開き直す。
4. `Any iOS Device (arm64)` を実行先にする。
5. `Product > Archive` で新しいArchiveを作成する。
6. Organizerから `Distribute App > App Store Connect > Upload` を実行する。

既存Archiveの再アップロードでは、修正後の `Info.plist` は反映されない。必要に応じてArchive内の `Info.plist` を確認し、`UISupportedInterfaceOrientations` と `UISupportedInterfaceOrientations~ipad` に4方向が含まれていることを確認する。

## 4. 今回の対応範囲

- 画面回転設定エラー: PR #70で対応済み。
- フライヤーモーダル、背景タップ、CAMPFIREリンク色: メモのみ。実装は未対応。
- App Store Connectの再アップロード: 新しいArchive作成後に再実行する。

## 5. 参考文献

- [Apple: UISupportedInterfaceOrientations](https://developer.apple.com/documentation/bundleresources/information-property-list/uisupportedinterfaceorientations)
- [Apple: UIRequiresFullScreen](https://developer.apple.com/documentation/bundleresources/information-property-list/uirequiresfullscreen)
- [Apple: Upload builds](https://developer.apple.com/help/app-store-connect/manage-builds/upload-builds)
