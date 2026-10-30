# ADR-0011: SwiftUI による iOS ネイティブクライアントの分離

## ステータス

Proposed

## 決定日

2026-10-30

## 対象コンポーネント

- `apps/ios/`
- `schema/schedule.schema.json`
- `data/generated/schedule.json`
- `apps/web/`
- `adr/0001-schedule-json-data-contract.md`
- `adr/0005-static-json-distribution.md`
- `adr/0008-client-offline-cache-and-freshness.md`

## コンテキストと課題

taite-board は、ライブ会場や移動中でも次の出演時刻、特典会、会場、チケットを短時間で確認できることを目的とする。現行 Web クライアントは Preact 化の途中であり、DOM と CSS の再利用を前提に iOS 版を作ると、Web のライフサイクルやレイアウト制約をネイティブアプリへ持ち込むことになる。

一方、データ取得と公開はすでに `schedule.json`、JSON Schema、静的配信、端末キャッシュ、鮮度表示という境界に分離されている。iOS 版ではこのデータ境界を維持しながら、SwiftUI の画面、アプリライフサイクル、ファイルキャッシュ、外部リンク操作に適合させる必要がある。

## 決定要因・評価軸

- 圏外・低速通信でも起動直後に予定を表示できること
- Web と iOS で同じ公開データを使い、取得ロジックを二重化しないこと
- UI が iOS のアクセシビリティとライフサイクルに適合すること
- 個人情報、広告 ID、テレメトリを追加しないこと
- 個人開発規模に対して、依存と運用を増やしすぎないこと
- 不正な配信データで最後の正常な表示を失わないこと

## 検討した選択肢

### 選択肢 1: SwiftUI の独立 iOS クライアント

- 概要: `apps/ios/` に SwiftUI アプリを作り、`ScheduleRepository` が seed、ディスクキャッシュ、静的 `schedule.json` を切り替える。
- メリット:
  - iOS の TabView、NavigationStack、Dynamic Type、VoiceOver、scenePhase を直接利用できる。
  - Web の DOM、CSS、JavaScript のライフサイクルから分離できる。
  - Foundation と SwiftUI の標準 API だけで v1 を構成できる。
  - 既存の静的配信・ゼロテレメトリ方針を維持できる。
- デメリット:
  - Web と iOS の UI 実装が二重になる。
  - Swift の wire model と TypeScript の型が乖離するリスクがある。
  - Xcode プロジェクト、署名、実機検証の運用が追加される。

### 選択肢 2: WebView または PWA のラッパー

- 概要: 現行 Web クライアントを WebView で包み、iOS アプリとして配布する。
- メリット:
  - 既存 UI を短期間で再利用できる。
  - データ契約を追加しなくてよい。
- デメリット:
  - iOS のネイティブなナビゲーション、アクセシビリティ、ライフサイクル制御が Web 側に依存する。
  - オフライン画像、キャッシュ、外部リンク、戻る操作の挙動が二重に複雑になる。
  - Preact 移行中の Web 実装の不安定さを iOS 配布物へ持ち込む。

### 選択肢 3: Flutter / React Native などのクロスプラットフォーム UI

- 概要: 将来の Android も想定し、別の UI ランタイムで iOS と Android を共有する。
- メリット:
  - UI コードを複数プラットフォームで共有できる可能性がある。
  - 将来の Android 展開時に同じ技術基盤を使える。
- デメリット:
  - 現時点では iOS のみが対象で、追加の SDK、ビルド、更新、習熟コストが先行する。
  - 既存 Web とのコード共有にもならず、データ以外の共有境界が曖昧になる。
  - 圏外表示とデータ契約の問題は解決するが、クロスプラットフォーム導入自体は解決しない。

## 決定事項

選択肢 1 を Proposed として採用する。

- SwiftUI の独立アプリとして `apps/ios/` に配置する。
- 画面は `AppModel`、データ取得は actor ベースの `ScheduleRepository`、通信は `URLSession`、キャッシュは `Application Support` の atomic JSON ファイルとする。
- `schedule.json`、JSON Schema、fixture、`schema_version`、鮮度の意味は Web と共有する。
- X の取得、OCR、名寄せ、チケット処理、ユーザー分析は iOS アプリへ移さない。
- SwiftData、SQLite、外部 Swift パッケージ、API サーバーは v1 の前提にしない。
- アプリ内の診断画面で、取得元、生成時刻、HTTP 結果、RTT、レスポンス情報、検証結果、raw JSON を確認できるようにする。情報は端末外へ送信しない。

## 影響とトレードオフ

### 正の影響

- Web の実装状況に依存せず、iOS のライフサイクルとアクセシビリティに合わせた画面を設計できる。
- 既存の静的配信を再利用するため、バックエンド、認証、データベースを増やさずに配布できる。
- seed とディスクキャッシュにより、通信失敗が初回表示の失敗に直結しない。
- 取得診断と raw JSON をユーザーが確認でき、モック表示と実データ表示を区別できる。

### 負の影響と緩和策

- Web と iOS に UI の重複が発生する。表示の正本を共有しようとせず、契約 fixture と受け入れ条件を共有する。
- Swift と TypeScript の型が乖離する。JSON Schema を正本とし、wire model の生成または CI drift check を導入する。
- 外部画像は初期版で完全オフラインにならない。画像失敗時のプレースホルダーと出典リンクを表示し、画像 disk cache は別の ADR で扱う。
- アプリの署名、最低対応 OS、feed URL は未確定である。データ層を URL 設定と fake source に分離し、これらの決定を待たずにテスト可能にする。

## 参照

- [iOS ネイティブクライアント設計案](../docs/ios-native-design.md)
- [ADR-0001: Web・ネイティブ共通の schedule.json データ契約](0001-schedule-json-data-contract.md)
- [ADR-0005: 静的 JSON 配信](0005-static-json-distribution.md)
- [ADR-0008: クライアント側のキャッシュとデータ鮮度](0008-client-offline-cache-and-freshness.md)
- [App | Apple Developer Documentation](https://developer.apple.com/documentation/swiftui/app)
- [ScenePhase | Apple Developer Documentation](https://developer.apple.com/documentation/swiftui/scenephase)
- [URLSession.data(for:) | Apple Developer Documentation](https://developer.apple.com/documentation/foundation/urlsession/data%28for%3A%29)
- [Using the file system effectively | Apple Developer Documentation](https://developer.apple.com/documentation/foundation/using-the-file-system-effectively)
